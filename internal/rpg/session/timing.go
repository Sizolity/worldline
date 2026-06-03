package session

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// beatTimer records per-stage latencies for a single beat and, when the
// WORLDLINE_DEBUG_TIMING env var is set, emits a one-line breakdown to stderr
// so we can see exactly which stage dominates wall-clock time instead of
// guessing. It follows the same opt-in stderr-trace convention as the other
// WORLDLINE_DEBUG_* flags (LORE / INTENT / DICE).
//
// When the env var is unset every method is a couple of cheap time.Now()
// calls and nothing is printed, so it is safe to leave wired into the hot
// path permanently.
//
// Stages emitted by runBeatPipeline (in approximate sequential order):
//
//	lore_join      wait for the previous beat's BACKGROUND lorekeeper task
//	               to finish. The task is two-phase: Phase A (LLM extract)
//	               starts at narrative-end of the PRIOR beat, well before
//	               that beat's critical path completes; Phase B (merge +
//	               re-save) runs after the prior beat hands off its
//	               post-effects world. This wait is ≈0 when the player
//	               took normal think-time between beats (the common case);
//	               non-zero only when a beat is fired back-to-back with
//	               the prior one before its async enrichment finished.
//	               Shrinks by the prior beat's inline_choices + effects +
//	               save tail vs. the legacy single-phase design because
//	               Phase A overlaps that tail.
//	prep           load snapshot + fog + tools + views + system prompt
//	               (local, no LLM)
//	beat_ttfc      time-to-first-narrative-chunk — isolates the ReAct
//	               "reason + tool-call" round-trips (which produce no
//	               narrative) from the streamed narration. A large value
//	               means the tool-calling loop dominates, not the
//	               post-narrative work
//	beat           full beat-agent wall-time: reason/tool-loop + streamed
//	               narration + the trailing inline set_choices tool_call
//	               args. This is the SOLE LLM cost of a healthy beat —
//	               there is no separate post-beat LLM round-trip on the
//	               critical path
//	inline_choices time between the LAST narrative chunk and stream close
//	               — i.e. the wall-time spent streaming the set_choices
//	               tool_call args after prose ended. Always present (when
//	               at least one narrative chunk arrived); directly
//	               measures the "extra wait beyond seeing the narrative
//	               finish" experienced by the player. Healthy contract:
//	               ~200-500ms. If consistently >1s the inline schema is
//	               regressing toward the old separate-call cost
//	effects        apply event + clock advance + worldline tick
//	               (local, no LLM)
//	suggest        SuggestActions fallback wall-time — ONLY present when
//	               the inline set_choices call was missing or its args
//	               failed to parse. Absent on the healthy path; presence
//	               in a trace is the canonical signal of inline-contract
//	               degradation (model prompt drift, JSON corruption, etc.)
//	save           snapshot + disclosure persistence (local disk I/O)
//
// The previous-beat background lorekeeper extraction wall-time is
// reported separately by the CLI as "[timing] prev_lore=..." rather
// than appearing in this single-line trace because it is async — it
// already overlapped the player's read/think time during the previous
// beat and is not part of THIS beat's critical path.
type beatTimer struct {
	enabled bool
	start   time.Time
	last    time.Time
	order   []string
	dur     map[string]time.Duration
}

func newBeatTimer() *beatTimer {
	now := time.Now()
	return &beatTimer{
		enabled: os.Getenv("WORLDLINE_DEBUG_TIMING") != "",
		start:   now,
		last:    now,
		dur:     map[string]time.Duration{},
	}
}

// mark records the elapsed time since the previous checkpoint under name and
// advances the checkpoint to now. Use for sequential stages.
func (t *beatTimer) mark(name string) {
	if t == nil {
		return
	}
	now := time.Now()
	t.record(name, now.Sub(t.last))
	t.last = now
}

// set records an explicit duration for name without moving the sequential
// checkpoint. Use for a span that overlaps a mark window (e.g. time-to-first-
// chunk measured inside the larger beat-agent stage).
func (t *beatTimer) set(name string, d time.Duration) {
	if t == nil {
		return
	}
	t.record(name, d)
}

func (t *beatTimer) record(name string, d time.Duration) {
	if _, ok := t.dur[name]; !ok {
		t.order = append(t.order, name)
	}
	t.dur[name] = d
}

// format returns the accumulated one-line breakdown, or "" when timing is
// disabled. It is returned to the caller via BeatResult so the presentation
// layer controls exactly where the trace lands (after the beat's output),
// preventing it from interleaving with the streamed narrative. Safe to call
// via defer so partial timings still surface on early errors.
func (t *beatTimer) format(worldID string) string {
	if t == nil || !t.enabled {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[timing] world=%s total=%s", worldID, time.Since(t.start).Round(time.Millisecond))
	for _, name := range t.order {
		fmt.Fprintf(&b, " %s=%s", name, t.dur[name].Round(time.Millisecond))
	}
	return b.String()
}
