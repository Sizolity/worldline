package session

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

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
