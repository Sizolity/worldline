package ingest

import (
	"context"

	"github.com/sizolity/worldline/internal/world/model"
)

// AliasResolver maps a draft entity to a canonical existing world entity ID.
//
// Returning a non-empty canonicalID means "treat this draft entity as a new
// nickname / alias for that existing entity". The compiler will:
//
//   - drop the draft entity ID and merge field updates onto the canonical entity
//     (subject to ConflictPolicy);
//   - rewrite every reference (relation endpoints, fact subjects) from the
//     draft ID to the canonical ID;
//   - record the mapping in CompileReport.Aliases for audit.
//
// Returning "" means "no alias — keep this as a new entity".
//
// The ingest framework provides only the interface and a NoopAliasResolver.
// Concrete implementations (LLM-based name matching, embedding similarity,
// rule-based fuzzy match) belong in the product layer.
type AliasResolver interface {
	Resolve(ctx context.Context, e DraftEntity, world model.World) (canonicalID string, err error)
}

// NoopAliasResolver never merges aliases — every draft entity keeps its ID.
// Used as the default when CompileOptions.Resolver is nil.
type NoopAliasResolver struct{}

func (NoopAliasResolver) Resolve(_ context.Context, _ DraftEntity, _ model.World) (string, error) {
	return "", nil
}

// BatchAliasResolver is an optional extension of AliasResolver for resolvers
// whose Resolve is expensive (LLM round-trip, embedding lookup, network call).
// When CompileDraft sees a Resolver that also implements BatchAliasResolver,
// it calls ResolveBatch once instead of looping over single-entity Resolve.
//
// The returned map is keyed by DraftEntity.ID. Missing keys mean "no alias —
// keep this ID". An empty-string value also means "no alias". Implementations
// MAY skip including non-aliased IDs in the result.
//
// Implementations should still implement Resolve for callers that only invoke
// the single-entity path. CompileDraft prefers ResolveBatch when available.
type BatchAliasResolver interface {
	AliasResolver
	ResolveBatch(ctx context.Context, entities []DraftEntity, world model.World) (map[string]string, error)
}
