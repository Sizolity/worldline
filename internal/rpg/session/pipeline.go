package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	tmr := newBeatTimer()
	// Capture the trace into the result on every return path (including
	// early errors) so the CLI can print it AFTER the beat output instead
	// of the beat goroutine writing it mid-stream.
	defer func() { result.TimingTrace = tmr.format(input.WorldID) }()

	// Join the previous beat's background lorekeeper task before we touch the
	// world on disk. It may still be merging + re-saving the snapshot, and we
	// must (a) avoid racing its writer and (b) LoadSnapshot the lore-enriched
	// world it produced. Because it completed after the prior beat had already
	// returned, its outcome surfaces on THIS beat's result as PrevLore*.
	if prev := s.joinPendingLore(); prev != nil {
		result.PrevLoreErr = prev.loreErr
		result.PrevLoreReport = prev.report
		result.PrevLoreDur = prev.dur
		if prev.saveErr != nil {
			result.PrevLoreErr = errors.Join(result.PrevLoreErr, prev.saveErr)
		}
	}
	tmr.mark("lore_join") // wait for prior beat's background lore (≈0 if it finished during think time)

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

	tmr.mark("prep") // load + fog + tools + views + system prompt

	// Recap / prologue beats (input.MinimalTools) are pure narration of
	// context that is ALREADY in the system prompt, so they need no game
	// tools. Handing the beat agent an EMPTY toolset makes the model
	// physically unable to emit tool_calls, so the ReAct branch routes
	// model→END after a SINGLE round-trip — eliminating the several
	// sequential pre-narrative tool calls that ballooned resume's
	// beat_ttfc to ~17s. Unlike merely lowering MaxStep on the full
	// toolset, an empty toolset cannot trip eino's ErrExceedMaxSteps (the
	// model never asks to loop). MaxStep=1 is a safe belt-and-suspenders:
	// eino checks `step >= maxSteps` at the top of each super-step and the
	// model→END path returns at step 0 (isEnd) before the counter reaches
	// 1, so it never errors here — whereas with tools bound the same cap
	// WOULD error on the first tool call, which is exactly why the two go
	// together. The normal (full-toolset) path is left byte-for-byte
	// unchanged.
	beatTools := gmTools
	maxStep := s.maxStep
	if input.MinimalTools {
		beatTools = nil
		maxStep = 1
	}

	beatStart := time.Now()
	stream := s.beatAgent.Run(ctx, react.Request{
		Messages: []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(input.Action.Content),
		},
		Tools:   beatTools,
		MaxStep: maxStep,
	})

	var narrativeBuf strings.Builder
	gotFirstChunk := false
	var lastChunkAt time.Time
	for chunk := range stream.Narrative {
		if !gotFirstChunk {
			// Time-to-first-chunk isolates the ReAct "reason + tool-call"
			// round-trips (which produce no narrative) from the final
			// streamed narration. A large gap here means the bottleneck is
			// the tool-calling loop, not the post-narrative pipeline.
			tmr.set("beat_ttfc", time.Since(beatStart))
			gotFirstChunk = true
		}
		narrativeBuf.WriteString(chunk)
		lastChunkAt = time.Now()
		select {
		case <-ctx.Done():
			result.Err = ctx.Err()
			return
		case narrativeCh <- chunk:
		}
	}

	// Narrative content phase ended (react.go closes stream.Narrative on
	// the first tool_call chunk, not on full stream end). The model is
	// now streaming set_choices tool_call args, which we still need to
	// await on <-stream.Done; but the narrative TEXT is complete, so
	// we can kick off lorekeeper Phase A (LLM extract from narrative)
	// IMMEDIATELY and let it overlap with the tool_call tail + apply
	// effects + synchronous save. The next beat's lore_join joins the
	// task at this earlier start time, so any wall-time wait shrinks
	// by the inline_choices + effects + save tail (~1s in practice).
	//
	// beatEvent.ID is computed from world.Clock.Sequence BEFORE the
	// per-beat increment downstream (line where world.Clock.Sequence++
	// runs), so the ID we pass here matches what beatEvent.ID becomes
	// later — provenance traceability is preserved.
	narrative := narrativeBuf.String()
	beatEventID := fmt.Sprintf("beat_%s_%d", input.WorldID, world.Clock.Sequence)
	var loreTask *pendingLore
	if s.lorekeeper != nil {
		loreTask = s.startLoreExtraction(ctx, input.WorldID, beatEventID, narrative)
		// Settle the lore task exactly once on every return path: the
		// success path below attaches the post-effects world (which
		// flips task.attached so this deferred abort becomes a no-op),
		// every early-return error path falls through to this defer
		// and aborts the task so its goroutine doesn't leak waiting
		// on a world that will never arrive.
		defer s.abortLoreTask(loreTask)
	}

	beatResult := <-stream.Done
	tmr.mark("beat") // total beat-agent time (reason/tool-loop + streamed narration + inline tool_call tail)
	// inline_choices = the time between the LAST narrative chunk and the
	// stream closing. With the inline-choices contract, this tail is
	// where the model is streaming the set_choices tool_call args after
	// the prose has ended; it directly measures "extra wall-time the
	// player waits beyond seeing the narrative finish before the agent
	// hands over". The old separate SuggestActions LLM call was ~2s;
	// this should be a few hundred ms when the contract is healthy.
	// Only meaningful when at least one narrative chunk arrived.
	if !lastChunkAt.IsZero() {
		tmr.set("inline_choices", time.Since(lastChunkAt))
	}
	if beatResult.Err != nil {
		result.Err = fmt.Errorf("beat agent: %w", beatResult.Err)
		return
	}

	effects := tc.GetPendingEffects()

	// In-fiction time advance declared by the narrator via advance_time.
	// The per-beat baseline is one scene; days/chapters come from explicit
	// narrative skips. This delta both drives worldline drift below and is
	// honestly reflected in the clock so projected world state reads right.
	scenes, days, chapters := tc.GetPendingTimeAdvance()
	delta := story.TimeDelta{Scenes: 1 + scenes, Days: days, Chapters: chapters}

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
		// ID must equal the beatEventID we already passed to the
		// lorekeeper task above — provenance traceability requires the
		// same ID land in the event log and the source-doc the
		// extracted lore points back to.
		ID:          worldmodel.EventID(beatEventID),
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

	// Diegetic-time advance: move the clock forward by the per-beat delta so
	// the worldline scheduler (and any future calendar-aware system) sees the
	// in-fiction time that actually elapsed. Scenes always advance the tick
	// counter; declared day/chapter skips accumulate into the calendar map.
	// This runs regardless of whether story is enabled — the clock should
	// advance every beat; only the story.Tick below is gated on storyStore.
	world.Clock.Current.Tick += int64(delta.Scenes)
	if delta.Days > 0 || delta.Chapters > 0 {
		if world.Clock.Current.Calendar == nil {
			world.Clock.Current.Calendar = make(map[string]int)
		}
		if delta.Days > 0 {
			world.Clock.Current.Calendar["day"] += delta.Days
		}
		if delta.Chapters > 0 {
			world.Clock.Current.Calendar["chapter"] += delta.Chapters
		}
	}

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
				World: world,
				Lines: lines,
				Delta: delta,
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

	tmr.mark("effects") // apply event + clock advance + worldline tick (local, no LLM)

	// === Post-narrative work ===
	// Iterative measurement collapsed the post-narrative stall in two
	// stages:
	//  (1) Lorekeeper extraction (~5-6.5s) — moved OFF the critical path
	//      into a background task joined by the next beat (see
	//      startLoreTask / joinPendingLore). Overlaps player read/think
	//      time so lore_join is reliably ~0 on subsequent beats.
	//  (2) SuggestActions (~1.9s) — collapsed INTO the beat agent via
	//      the set_choices inline-schema tool. The beat agent is now
	//      required to call set_choices as the last element of its
	//      streamed assistant message; we extract the choices from the
	//      message's tool_calls (Result.ToolCalls). No separate LLM
	//      round-trip on the happy path. SuggestActions remains as the
	//      graceful-degrade fallback for when the model omits the tool
	//      call or emits unparseable args.
	//
	// Ordering & safety:
	//  - Choice resolution (inline vs fallback) happens BEFORE the
	//    synchronous save so the result we hand to the caller is
	//    complete; the disk write is then the last critical-path step.
	//  - Background lorekeeper task is kicked off AFTER the synchronous
	//    save so its enriched re-save never races a concurrent writer
	//    (the next beat joins it before LoadSnapshot, see joinPendingLore).
	// Scenes that opt out of choice generation (recap / prologue mod
	// styles whose prompt explicitly says "末尾不列选项") set
	// input.SuppressChoices=true. Skipping the entire resolve path
	// avoids the ~2s SuggestActions fallback that would otherwise
	// fire on those beats — see BeatInput.SuppressChoices docs.
	var (
		choices    role.ActionChoices
		suggestErr error
	)
	if input.SuppressChoices {
		if os.Getenv("WORLDLINE_DEBUG_TIMING") != "" {
			fmt.Fprintln(os.Stderr, "[choices] suppressed (BeatInput.SuppressChoices) — recap/prologue scene")
		}
	} else {
		choices, suggestErr = s.resolveBeatChoices(ctx, beatResult.ToolCalls, world, disclosure, narrative, tmr)
	}

	// Persist the post-effects world synchronously (durable + immediate
	// error feedback). The background lorekeeper will re-save an
	// enriched copy strictly AFTER this write because the next beat
	// joinPendingLore before its own LoadSnapshot/save.
	saveStart := time.Now()
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
	tmr.set("save", time.Since(saveStart))

	// === Background lorekeeper extraction (Phase B handoff) ===
	// Phase A (LLM extract from narrative) has been running in the
	// background since narrative content streaming ended; see the
	// startLoreExtraction call earlier in this function. Now that we
	// have the post-effects world saved synchronously, hand it to the
	// task so Phase B (Compile + re-save enriched snapshot) can
	// proceed. If Phase A finished during the stream tail + effects +
	// save window, Phase B starts immediately on this attach call.
	// `world` is handed by value; CompileDraft detaches before mutating
	// so the result.World we expose below is never mutated by the task.
	//
	// Source-doc ID was passed to startLoreExtraction up top using the
	// same beatEventID literal we used to construct beatEvent.ID here,
	// preserving the EventLog↔lore provenance invariant.
	//
	// attachLoreWorld flips loreTask.attached=true so the deferred
	// abortLoreTask becomes a no-op — exactly-once settlement.
	if loreTask != nil {
		s.attachLoreWorld(loreTask, world)
	}

	result.World = world
	result.Narrative = narrative
	result.ToolEffects = effects
	result.Choices = choices
	result.SuggestErr = suggestErr
}

// resolveBeatChoices implements the inline-first / fallback-to-suggest
// choice resolution policy. The happy path is pure CPU: parse the beat
// agent's set_choices tool_call from the streamed assistant message
// (zero extra LLM calls). The fallback path is the legacy
// SuggestActions LLM round-trip (~2s); it runs only when the model
// omitted the inline call or its args could not be parsed.
//
// On total failure (both paths exhausted) the beat does NOT abort —
// choices degrade to a single custom slot so the player keeps agency
// via free-form input. suggestErr surfaces the diagnostic for the CLI
// to display; the world state and narrative remain durable regardless.
//
// Timing labels written:
//   - inline_choices: already set in the caller above (the time from
//     last narrative chunk to stream close). Present always.
//   - suggest: SuggestActions wall-time. ONLY set when the fallback
//     fires — its absence in a normal trace is a positive signal that
//     the inline contract is healthy.
func (s *Session) resolveBeatChoices(
	ctx context.Context,
	toolCalls []schema.ToolCall,
	world worldmodel.World,
	disclosure fog.DisclosureState,
	narrative string,
	tmr *beatTimer,
) (role.ActionChoices, error) {
	inlineChoices, found, parseErr := s.gm.ExtractInlineChoices(toolCalls)
	if found && parseErr == nil {
		return inlineChoices, nil
	}
	// Fallback diagnostic: gate behind WORLDLINE_DEBUG_TIMING so it
	// only surfaces when the operator is already inspecting timings;
	// no per-beat noise during normal play. The seen tool_call names
	// list is the most useful signal in practice — it distinguishes
	// "model emitted no tool_calls at all" (discipline gap, prompt
	// engineering needed) from "model emitted SOME tool_calls but
	// none was set_choices" (name mismatch, schema drift, or a stray
	// world-mutating tool call after the narrative).
	if os.Getenv("WORLDLINE_DEBUG_TIMING") != "" {
		seenNames := make([]string, 0, len(toolCalls))
		for _, tc := range toolCalls {
			seenNames = append(seenNames, tc.Function.Name)
		}
		narrativeRunes := len([]rune(narrative))
		switch {
		case parseErr != nil:
			fmt.Fprintf(os.Stderr,
				"[choices] inline parse failed (%v); narrative_runes=%d tool_calls=%d seen=%v; falling back to SuggestActions\n",
				parseErr, narrativeRunes, len(toolCalls), seenNames,
			)
		case !found:
			fmt.Fprintf(os.Stderr,
				"[choices] inline set_choices missing; narrative_runes=%d tool_calls=%d seen=%v; falling back to SuggestActions\n",
				narrativeRunes, len(toolCalls), seenNames,
			)
		}
	}

	visibleForSuggest := world
	if s.fogEnabled {
		visibleForSuggest = fog.FilterWorld(world, disclosure)
	}
	suggestStart := time.Now()
	choices, err := s.gm.SuggestActions(ctx, visibleForSuggest, s.players, narrative)
	tmr.set("suggest", time.Since(suggestStart))
	if err != nil {
		return role.ActionChoices{
			Options: []role.ActionOption{{Type: role.ActionTypeCustom}},
		}, err
	}
	return choices, nil
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
