// Package session orchestrates the RPG beat pipeline using Eino's ReAct agent.
// The GM role (injected via role.GM) controls prompt generation, tool selection,
// and action suggestion. See rpg/gm/ for concrete GM implementations.
//
// Session itself is pure orchestration — no LLM logic, no prompt construction,
// no effect application. Each beat:
//
//  1. Loads the world (+ disclosure if fog enabled)
//  2. Asks the GM for the disclosed toolset and the system prompt
//  3. Runs the Eino ReAct agent with the resulting tools + prompt
//  4. Applies any pending effects via world/runtime.ApplyEvent
//  5. Runs the Lorekeeper (if configured) to extract structured world
//     knowledge from the narrative and compile it into the snapshot.
//     Failure is non-fatal; surfaces via BeatResult.LoreErr.
//  6. Persists the updated world + disclosure
//  7. Asks the GM to suggest next-step ActionChoices for the PL
package session

import (
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"

	"github.com/sizolity/worldline/agent/beat"

	"github.com/sizolity/worldline/world/ingest"
	worldmodel "github.com/sizolity/worldline/world/model"
	worldruntime "github.com/sizolity/worldline/world/runtime"
	"github.com/sizolity/worldline/world/store"
	"github.com/sizolity/worldline/world/view"
	"github.com/sizolity/worldline/rpg/fog"
	"github.com/sizolity/worldline/rpg/role"
	"github.com/sizolity/worldline/rpg/story"
	"github.com/sizolity/worldline/rpg/tools"
)

// Session manages a single RPG game session tied to one world.
type Session struct {
	gm         role.GM
	players    []role.Player
	store      *store.FileStore
	fogStore   *fog.Store
	storyStore *story.Store
	lorekeeper role.Lorekeeper
	runtime    worldruntime.Runtime
	beatAgent  beat.Agent
	rng        *rand.Rand
	maxStep    int
	fogEnabled bool
}

// Config holds parameters for creating a new Session.
type Config struct {
	GM            role.GM
	Players       []role.Player
	WorkspacePath string // root for all data; worlds, fog, and worldlines colocated under {WorkspacePath}/worlds/{worldID}/
	BeatAgent     beat.Agent
	Rng           *rand.Rand
	MaxStep       int  // max tool-calling iterations per beat (default 10)
	FogEnabled    bool // enable progressive world disclosure (fog of war)
	// StoryEnabled toggles the WorldLine scheduler. When true, the session
	// loads worldlines.json at beat start, ticks them after player effects
	// apply, applies emitted events, and persists updated lines. Default off
	// keeps existing sessions unchanged.
	StoryEnabled bool
	// Lorekeeper is optional. When set, after each beat the lorekeeper
	// extracts an ingest.Draft from the narrative and compiles it into
	// the world snapshot. When nil, the lore pipeline is skipped entirely.
	// Lorekeeper failure never aborts a beat — errors surface via
	// BeatResult.LoreErr.
	Lorekeeper role.Lorekeeper
}

func New(cfg Config) (*Session, error) {
	if cfg.GM == nil {
		return nil, fmt.Errorf("GM is required")
	}
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	if cfg.BeatAgent == nil {
		return nil, fmt.Errorf("beat agent is required")
	}
	rng := cfg.Rng
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	maxStep := cfg.MaxStep
	if maxStep <= 0 {
		maxStep = 10
	}
	worldsDir := filepath.Join(cfg.WorkspacePath, "worlds")
	sess := &Session{
		gm:         cfg.GM,
		players:    cfg.Players,
		store:      store.NewFileStore(cfg.WorkspacePath),
		fogStore:   fog.NewStore(worldsDir),
		lorekeeper: cfg.Lorekeeper,
		runtime:    worldruntime.NewRuntime(),
		beatAgent:  cfg.BeatAgent,
		rng:        rng,
		maxStep:    maxStep,
		fogEnabled: cfg.FogEnabled,
	}
	if cfg.StoryEnabled {
		sess.storyStore = story.NewStore(worldsDir)
	}
	return sess, nil
}

// BeatInput is the user-facing input for running a beat.
type BeatInput struct {
	WorldID      string
	Action       role.PlayerAction
	RecentEvents int
}

// BeatResult is the final outcome of a beat after the narrative stream has
// fully drained. Callers receive it on BeatOutput.Done after iterating
// BeatOutput.NarrativeStream to completion.
type BeatResult struct {
	// Err is set when the beat could not complete (load world, build tools,
	// agent stream, persistence). The narrative stream may have partial
	// content prior to the failure. Distinct from SuggestErr which is a
	// graceful-degrade signal.
	Err error

	// World is the post-beat world snapshot. Zero-value when Err is set.
	World worldmodel.World
	// Narrative is the full concatenation of every chunk that was emitted
	// to BeatOutput.NarrativeStream, in order. Convenient for callers that
	// just want the final text (e.g. tests, Lorekeeper ingestion).
	Narrative   string
	ToolEffects []worldmodel.Effect
	Choices     role.ActionChoices
	// SuggestErr is non-nil when SuggestActions failed for this beat (e.g.
	// transient LLM JSON error). The narrative and world updates have
	// still been applied; Choices falls back to a single custom slot so
	// the player can continue. Callers may surface this for visibility.
	SuggestErr error
	// LoreErr is non-nil when the Lorekeeper pipeline (Parse → Validate →
	// Compile) failed for this beat. This is graceful-degrade signal,
	// same status as SuggestErr: the narrative and tool-effects have
	// still been applied; the world simply has no new ingested knowledge
	// from this beat. Callers may surface this for logging.
	LoreErr error
	// LoreReport summarizes what the Lorekeeper extracted and compiled
	// (Inserted / Skipped / Filtered / Rejected counts, Notes, Aliases).
	// Zero-value when no Lorekeeper is configured or LoreErr is set.
	LoreReport ingest.CompileReport
}

// BeatOutput exposes a beat's streaming narrative and its eventual final
// result through two channels:
//
//   - NarrativeStream carries the narrative as a series of delta chunks
//     (strings). It closes once the LLM finishes producing the narrative.
//     Callers MUST drain it (e.g. `for chunk := range out.NarrativeStream`)
//     before reading Done, otherwise the producing goroutine will block.
//   - Done delivers exactly one BeatResult after the post-narrative
//     pipeline (apply effects, worldline tick, save, suggest actions) has
//     completed. The Done channel is buffered (size 1) so the producer
//     never blocks on it.
//
// Typical usage:
//
//	out := sess.RunBeat(ctx, in)
//	for chunk := range out.NarrativeStream {
//	    fmt.Print(chunk)
//	}
//	result := <-out.Done
//	if result.Err != nil { ... }
//
// Tests or other non-streaming callers can use BeatOutput.Wait() for a
// one-shot synchronous result.
type BeatOutput struct {
	NarrativeStream <-chan string
	Done            <-chan BeatResult
}

// Wait synchronously drains the narrative stream and returns the final
// BeatResult. Use this in tests or scripts where streaming UX is not
// required. The full narrative is still available via result.Narrative.
//
// Wait panics if called more than once on the same BeatOutput because
// both channels are single-use.
func (b BeatOutput) Wait() BeatResult {
	for range b.NarrativeStream {
		// discard chunks; result.Narrative carries the full text
	}
	return <-b.Done
}

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

	stream := s.beatAgent.Run(ctx, beat.Request{
		SystemPrompt: systemPrompt,
		UserMessage:  input.Action.Content,
		Tools:        gmTools,
		MaxStep:      s.maxStep,
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

// runLorekeeper drives the Parse → Validate → Compile pipeline for a
// single beat. Returns the updated world on success and the input world
// unchanged on error, alongside a CompileReport (non-zero on success,
// zero on error) and an error value the caller treats as a soft-fail
// signal.
//
// Return contract: on error, the input world is returned unchanged and
// the CompileReport is zero-valued. This means a caller that ignores
// the error and unconditionally assigns the returned world still gets a
// valid, non-half-mutated world — the function is safe to misuse. The
// current call site nonetheless gates assignment on the error so the
// post-effects world flows through to SaveSnapshot only via the success
// branch, which keeps intent explicit.
//
// sourceID is the SourceDocument.ID that the Lorekeeper will see; the
// caller passes the beat event ID so compiled lore is traceable back to
// the EventLog entry that produced it.
//
// ValidateDraft warnings are emitted via Notes in the returned
// CompileReport so callers (CLI, tests) can surface them without
// bringing their own validator. ValidateDraft errors are NOT a hard
// abort — CompileDraft does per-item rejection so a malformed item
// does not poison the whole draft. We still log Validate errors into
// CompileReport.Notes so they show up alongside Compile-time rejects.
func (s *Session) runLorekeeper(
	ctx context.Context,
	world worldmodel.World,
	sourceID string,
	narrative string,
) (worldmodel.World, ingest.CompileReport, error) {
	doc := ingest.SourceDocument{
		ID:   sourceID,
		Kind: "rpg_beat",
		Text: narrative,
	}
	draft, err := s.lorekeeper.Parse(ctx, doc)
	if err != nil {
		return world, ingest.CompileReport{}, fmt.Errorf("lorekeeper extract: %w", err)
	}
	validation := ingest.ValidateDraft(draft)
	newWorld, report, err := ingest.CompileDraft(world, draft, ingest.CompileOptions{
		ConflictPolicy: ingest.ConflictPolicySkip,
		// Resolver left nil: CompileDraft falls back to NoopAliasResolver.
		// A narrator-driven AliasResolver for NPC name dedup is a future sub.
	})
	if err != nil {
		return world, ingest.CompileReport{}, fmt.Errorf("lorekeeper compile: %w", err)
	}
	for _, e := range validation.Errors {
		report.Notes = append(report.Notes, "validate-error: "+e)
	}
	for _, w := range validation.Warnings {
		report.Notes = append(report.Notes, "validate-warn: "+w)
	}
	return newWorld, report, nil
}

// LoadWorld loads a world snapshot by ID.
func (s *Session) LoadWorld(ctx context.Context, worldID string) (worldmodel.World, error) {
	return s.store.LoadSnapshot(ctx, worldID)
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
	summary := truncateRunes(strings.TrimSpace(narrative), beatNarrativeBudget)
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

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
