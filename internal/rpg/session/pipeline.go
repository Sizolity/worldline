package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/react"
	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/story"
	"github.com/sizolity/worldline/internal/rpg/tools"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/view"
)

// RunBeat starts a streaming beat. It returns immediately with a
// BeatOutput whose channels expose the narrative chunks and the eventual
// BeatResult; the caller MUST consume NarrativeStream to completion
// before reading Done (see BeatOutput).
//
// All I/O and orchestration errors flow through BeatResult.Err on Done
// rather than a separate error return value, so callers have a single
// place to check for failure.
func (s *Session) RunBeat(ctx context.Context, input BeatInput) BeatOutput {
	narrativeCh := make(chan string, 32)
	doneCh := make(chan BeatResult, 1)
	go s.runBeatStream(ctx, input, narrativeCh, doneCh)
	return BeatOutput{NarrativeStream: narrativeCh, Done: doneCh}
}

// runBeatStream is the goroutine body that executes a single beat. It
// guarantees:
//   - narrativeCh is always closed (even on early errors) so range loops
//     in callers terminate.
//   - doneCh receives exactly one BeatResult.
//   - narrativeCh is closed BEFORE doneCh receives so the typical caller
//     pattern (range NarrativeStream, then <-Done) does not deadlock.
func (s *Session) runBeatStream(ctx context.Context, input BeatInput, narrativeCh chan<- string, doneCh chan<- BeatResult) {
	var result BeatResult
	defer func() {
		close(narrativeCh)
		doneCh <- result
	}()
	s.runBeatPipeline(ctx, input, narrativeCh, &result)
}

// runBeatPipeline contains the actual orchestration logic. Errors are
// written into *result.Err and signal early termination; the deferred
// cleanup in runBeatStream still closes channels and delivers the result.
func (s *Session) runBeatPipeline(ctx context.Context, input BeatInput, narrativeCh chan<- string, result *BeatResult) {
	world, err := s.store.LoadSnapshot(ctx, input.WorldID)
	if err != nil {
		result.Err = fmt.Errorf("load world: %w", err)
		return
	}

	var disclosure fog.DisclosureState
	if s.fogEnabled {
		disclosure, err = s.fogStore.Load(input.WorldID)
		if err != nil {
			result.Err = fmt.Errorf("load disclosure: %w", err)
			return
		}
	}

	tc := &tools.ToolContext{World: world, Rng: s.rng}
	if s.fogEnabled {
		tc.Disclosure = &disclosure
	}
	gmTools, err := s.gm.Tools(tc)
	if err != nil {
		result.Err = fmt.Errorf("create tools: %w", err)
		return
	}

	// Fog filter is applied to the PL-facing world view; the GM still sees the
	// full world internally for narration consistency post-disclosure.
	visibleWorld := world
	if s.fogEnabled {
		visibleWorld = fog.FilterWorld(world, disclosure)
	}

	// Pre-render the three projections the GM consumes — keeps the GM free of
	// raw model.World iteration.
	worldCtx := view.WorldDebugView{}.Render(visibleWorld)
	narrativeCtx := view.NarrativeView{}.Render(visibleWorld, view.NarrativeContextRequest{
		RecentEventLimit: input.RecentEvents,
	})
	var charCtxs []view.CharacterContext
	for _, p := range s.players {
		if p.CharacterID == "" {
			continue
		}
		if cc, err := (view.CharacterContextView{}).Render(visibleWorld, view.CharacterContextRequest{
			PerspectiveID: p.CharacterID,
		}); err == nil {
			charCtxs = append(charCtxs, cc)
		}
	}

	systemPrompt := s.gm.SystemPrompt(s.players, role.PromptOptions{
		WorldCtx:     worldCtx,
		NarrativeCtx: narrativeCtx,
		CharacterCtx: charCtxs,
		FogEnabled:   s.fogEnabled,
	})

	stream := s.beatAgent.Run(ctx, react.Request{
		Messages: []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(input.Action.Content),
		},
		Tools:   gmTools,
		MaxStep: s.maxStep,
	})

	var narrativeBuf strings.Builder
	for chunk := range stream.Narrative {
		narrativeBuf.WriteString(chunk)
		select {
		case <-ctx.Done():
			result.Err = ctx.Err()
			return
		case narrativeCh <- chunk:
		}
	}

	beatResult := <-stream.Done
	if beatResult.Err != nil {
		result.Err = fmt.Errorf("beat agent: %w", beatResult.Err)
		return
	}

	narrative := narrativeBuf.String()
	effects := tc.GetPendingEffects()

	// Synthesize a single per-beat event that captures BOTH the player's
	// action and a summary of the narrator's response, plus any pending
	// tool effects. This event is written unconditionally so that downstream
	// beats can see what happened in prior turns via NarrativeView's
	// RecentEvents — without it, beats where the LLM did not call tools
	// would leave no trace in EventLog and the next prompt would have
	// amnesia about the player's recent choices and the GM's narration.
	//
	// The narrative is truncated to keep the events section of the prompt
	// bounded; the full narrative is still returned in BeatOutput.Narrative.
	beatEvent := worldmodel.WorldEvent{
		ID:          worldmodel.EventID(fmt.Sprintf("beat_%s_%d", input.WorldID, world.Clock.Sequence)),
		Type:        worldmodel.EventTypeNote,
		Source:      worldmodel.EventSourceUser,
		Description: buildBeatEventDescription(input.Action.Content, narrative),
		Effects:     effects,
	}
	world, err = s.runtime.ApplyEvent(world, beatEvent)
	if err != nil {
		result.Err = fmt.Errorf("apply beat event: %w", err)
		return
	}

	// Sequence increments once per beat regardless of effects, mirroring the
	// pre-refactor behavior (one tick == one PL action).
	world.Clock.Sequence++

	// === WorldLine scheduler ===
	// Runs after player tool-effects are applied and clock advances, so it
	// sees the post-action world. Emitted events flow through the same
	// runtime as player effects and persist with the same SaveSnapshot below.
	if s.storyStore != nil {
		lines, err := s.storyStore.Load(input.WorldID)
		if err != nil {
			result.Err = fmt.Errorf("load worldlines: %w", err)
			return
		}
		if len(lines) > 0 {
			tickOut, err := story.Tick(story.TickInput{
				World:     world,
				Lines:     lines,
				TimeScale: world.Clock.Current.Kind,
			}, s.rng)
			if err != nil {
				result.Err = fmt.Errorf("worldline tick: %w", err)
				return
			}
			for _, ev := range tickOut.Events {
				world, err = s.runtime.ApplyEvent(world, ev)
				if err != nil {
					result.Err = fmt.Errorf("apply worldline event: %w", err)
					return
				}
			}
			if err := s.storyStore.Save(input.WorldID, tickOut.UpdatedLines); err != nil {
				result.Err = fmt.Errorf("save worldlines: %w", err)
				return
			}
		}
	}

	// === Lorekeeper extraction ===
	// Translate the just-streamed narrative into structured world
	// knowledge (entities/relations/facts/threads/memories) and merge
	// it into the world snapshot. Runs after player effects, clock
	// advance, and WorldLine tick so the draft can reference any new
	// events those steps produced. Failure here is non-fatal: the world
	// remains in its post-effects state and the beat completes with
	// LoreErr surfaced.
	//
	// Source-doc ID is beatEvent.ID by design: every lore item compiled
	// from this beat is traceable back to the single EventLog entry that
	// produced it (via CompileReport.Provenance.SourceRefs and any
	// downstream caller that records the source doc). Do not change this
	// without updating callers that rely on the invariant.
	if s.lorekeeper != nil {
		newWorld, report, loreErr := s.runLorekeeper(ctx, world, string(beatEvent.ID), narrative)
		if loreErr != nil {
			result.LoreErr = loreErr
		} else {
			world = newWorld
			result.LoreReport = report
		}
	}

	if err := s.store.SaveSnapshot(ctx, world); err != nil {
		result.Err = fmt.Errorf("save world: %w", err)
		return
	}

	if s.fogEnabled {
		if err := s.fogStore.Save(input.WorldID, disclosure); err != nil {
			result.Err = fmt.Errorf("save disclosure: %w", err)
			return
		}
	}

	// Re-filter post-effect world so SuggestActions only sees what the PL can.
	visibleAfter := world
	if s.fogEnabled {
		visibleAfter = fog.FilterWorld(world, disclosure)
	}
	// SuggestActions is a "nice to have" — it shapes UX (suggested next
	// options) but is not load-bearing for the world simulation. Errors
	// here (transient LLM JSON failures, network blips during the second
	// LLM call) must NOT discard the narrative and applied effects we
	// already committed above. We surface the error via SuggestErr so the
	// caller can log/display, and return an empty Choices set with just
	// the custom slot so the player can still continue with free-form
	// input.
	choices, suggestErr := s.gm.SuggestActions(ctx, visibleAfter, s.players, narrative)
	if suggestErr != nil {
		choices = role.ActionChoices{
			Options: []role.ActionOption{{Type: role.ActionTypeCustom}},
		}
	}

	result.World = world
	result.Narrative = narrative
	result.ToolEffects = effects
	result.Choices = choices
	result.SuggestErr = suggestErr
}

// beatNarrativeBudget caps the per-beat narrative summary placed into the
// EventLog. We size it large enough (in runes) that the *consequence* of a
// player action — not just the opening flourish — survives into the next
// beat's RecentEvents window. Empirically ~800 runes (~400 CJK characters)
// keeps the rendered events section bounded while preserving the key
// developments that the LLM needs to maintain narrative continuity.
const beatNarrativeBudget = 800

// buildBeatEventDescription assembles a compact "what happened in this beat"
// summary for the EventLog. The downstream NarrativeView renders these as
// "## Recent Events" entries on the next prompt, giving the GM continuity
// across beats. We keep the player line verbatim (it's typically short) and
// truncate the LLM narrative to a bounded prefix counted in runes (not
// bytes) so multi-byte scripts are not chopped mid-character.
func buildBeatEventDescription(playerAction, narrative string) string {
	playerAction = strings.TrimSpace(playerAction)
	summary := view.TruncateRunes(strings.TrimSpace(narrative), beatNarrativeBudget)
	switch {
	case playerAction == "" && summary == "":
		return "RPG beat (no input, no narrative)"
	case playerAction == "":
		return "Narrative: " + summary
	case summary == "":
		return "Player: " + playerAction
	default:
		return "Player: " + playerAction + "\nNarrative: " + summary
	}
}

