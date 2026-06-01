package session

import (
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

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
