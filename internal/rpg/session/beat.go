package session

import (
	"time"

	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

// BeatInput is the user-facing input for running a beat.
type BeatInput struct {
	WorldID      string
	Action       role.PlayerAction
	RecentEvents int

	// SuppressChoices, when true, tells the pipeline to skip ALL choice
	// resolution work for this beat — no inline set_choices extraction
	// from the streamed tool_calls, and no fallback to SuggestActions.
	// Result.Choices is set to a zero-value ActionChoices.
	//
	// This is the right setting for beats whose mod-side scene prompt
	// explicitly tells the narrator NOT to list options (e.g. recap /
	// prologue scenes that close on "a frozen moment for the player to
	// react to"). For such beats the narrator legitimately omits
	// set_choices; without this flag the pipeline would interpret that
	// omission as a degraded inline contract and fire a wasteful ~2s
	// SuggestActions LLM call — both unnecessary (the next normal beat
	// will produce choices) and contrary to the scene's design intent.
	//
	// Even when set, inline_choices timing is still recorded if the
	// narrator did stream any tool_call delta — the timing column just
	// reflects "model wrote some tool args we ignored", which is a
	// useful signal that the model is violating the no-options
	// instruction. The flag never adds latency: it removes a code path,
	// so leaving it false for normal action beats preserves existing
	// behavior bit-for-bit.
	SuppressChoices bool

	// MinimalTools, when true, runs the beat agent with an EMPTY toolset
	// (and MaxStep=1) instead of the GM's full game toolset. It is meant
	// for pure-narration beats — recap / prologue — whose entire context
	// already lives in the assembled system prompt and which therefore
	// need no get_entity_state / lookup_rules / explore_knowledge /
	// mutation round-trips before narrating.
	//
	// Why empty tools (not a lower MaxStep on the full toolset): with no
	// tools bound the model cannot emit a tool_call, so the ReAct loop
	// produces narrative in a SINGLE model round-trip (model→END) instead
	// of burning several sequential pre-narrative tool calls — the cause
	// of the ~17s beat_ttfc stall observed on resume. Crucially this
	// cannot trip eino's max-step-exceeded error the way merely lowering
	// MaxStep on a tool-bearing beat could (which would blank the recap).
	//
	// Orthogonal to SuppressChoices on purpose. At the CLI the two always
	// co-occur (recap / prologue both suppress choices AND need no tools),
	// but they answer different questions — "resolve next-step choices?"
	// vs. "give the agent tools?" — so they stay independent and each is
	// reasoned about / tested in isolation. Leaving it false preserves the
	// existing full-toolset behavior bit-for-bit for normal action beats.
	MinimalTools bool
}

// WrapPlayerAction frames a raw player input as an explicit "next-beat
// action" instruction for the Narrator. Without this prefix the LLM tends
// to treat short user messages as commentary on the previous narrative and
// re-tells the prior beat instead of executing the new action.
//
// The framing is deliberately strict: read prior events, advance from where
// the last beat ended, do not re-narrate established facts. This is beat-input
// orchestration shared by every entry point (CLI REPL today, HTTP /beat
// next), not presentation — so it lives here rather than in any single
// front-end to keep the two entry points behaviourally consistent.
//
// The trailing set_choices reminder is the per-beat counterweight to
// the model occasionally "forgetting" the compliance trailer's
// mandate after a long narrative — empirically a ~17% miss rate on
// normal action beats. Repeating the requirement right next to the
// action keeps the closing call salient at the moment the model is
// composing its reply. Recap / prologue beats route through different
// code paths and do NOT go through this wrapper, so this reminder
// only ever attaches to beats whose choices are NOT suppressed.
func WrapPlayerAction(content, playerName string) string {
	if playerName == "" {
		playerName = "玩家"
	}
	return "【" + playerName + "本回合行动】\n" + content + "\n\n" +
		"请阅读 ## 最近事件 中本回合之前的所有事件，明确从上一段叙事的结束状态接着推进；" +
		"必须执行上述行动并描述其直接后果与新出现的变化，不要重复上一段已经发生过的细节。\n\n" +
		"【收尾提醒】本回合无论叙事如何结束，都必须在回复末尾调用一次 `set_choices` 工具，给出 2-4 个下一步选项；不要把选项写进叙事正文。"
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

	// Lorekeeper extraction runs in a BACKGROUND task (see Session.RunBeat /
	// runLoreTask): it only enriches world memory for *future* beats, so the
	// player never waits for it. Consequently its outcome is not available
	// when this beat's result is delivered. Instead, the PREVIOUS beat's
	// lorekeeper outcome is surfaced here, joined at the start of this beat:
	//
	//   - PrevLoreErr aggregates the prior beat's extraction failure and/or
	//     the failure to persist its enriched snapshot. Graceful-degrade
	//     signal: the prior beat's world state (effects) was still saved
	//     synchronously; only the knowledge enrichment was lost.
	//   - PrevLoreReport summarizes what the prior beat's lorekeeper compiled
	//     (Inserted / Skipped / Filtered / Rejected counts, Notes, Aliases).
	//   - PrevLoreDur is the prior beat's background extraction wall-time,
	//     for the WORLDLINE_DEBUG_TIMING trace.
	//
	// All three are zero on the first beat (no prior task) and whenever no
	// Lorekeeper is configured. To observe the most recent beat's lore
	// outcome directly (e.g. tests, shutdown flush), call
	// Session.AwaitPendingLore.
	PrevLoreErr    error
	PrevLoreReport ingest.CompileReport
	PrevLoreDur    time.Duration

	// TimingTrace is a one-line per-stage latency breakdown for this beat,
	// populated only when WORLDLINE_DEBUG_TIMING is set (otherwise empty).
	// It is returned through the result — rather than printed from the beat
	// goroutine — so the presentation layer can emit it AFTER the beat's
	// output, where it cannot interleave with the streamed narrative.
	TimingTrace string
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
