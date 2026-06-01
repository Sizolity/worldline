package ingest

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

const (
	ConflictPolicySkip    = ""
	ConflictPolicyReplace = "replace"
)

// CompileOptions controls how a draft is merged into an existing world.
type CompileOptions struct {
	ConflictPolicy    string
	AllowDanglingRefs bool
	// MinConfidence filters out draft items whose Confidence is below this
	// threshold. Zero means no filtering (all items are accepted).
	MinConfidence float64
	// Resolver optionally normalizes draft entity IDs to canonical world IDs
	// to handle aliases (e.g. "char_kael" and "char_kael_brave" same person).
	// When nil, NoopAliasResolver is used (no merging).
	// Must remain product-neutral: the framework never ships a concrete LLM-
	// or embedding-backed implementation.
	Resolver AliasResolver
	// Ctx is the context propagated to the Resolver. When nil, context.Background()
	// is used. Callers that want cancellation should set this.
	Ctx context.Context
}

// CompileReport summarizes the result of compiling a draft into a world.
//
//   - Inserted: items successfully written into the world
//   - Skipped:  items skipped because their ID already exists and ConflictPolicy is "skip"
//   - Filtered: items dropped by MinConfidence before any compile attempt
//   - Rejected: items dropped because the synthesized model object failed Validate()
//     (e.g. unsupported thread kind, illegal memory owner). Recorded in Notes.
//   - Aliases:  draft entity IDs that were rewritten to canonical world IDs
//     by the AliasResolver (audit trail).
type CompileReport struct {
	Inserted   int
	Skipped    int
	Filtered   int
	Rejected   int
	Notes      []string
	Aliases    map[string]string
	Provenance []ProvenanceEntry
}

// ProvenanceEntry records which source chunks produced a given world ID.
type ProvenanceEntry struct {
	WorldID    string   `json:"world_id"`
	Kind       string   `json:"kind"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

// CompileDraft merges a validated draft into an existing world, returning
// the updated world and a report of what was inserted/skipped.
//
// CompileDraft is non-destructive on the caller's world value: the input
// world's Entities map and Relations/Facts/Threads/Memory slices are detached
// (shallow-copied) before any mutation, so a returned error never leaves the
// caller's world in a half-mutated state. Callers may safely discard the
// returned World on error.
func CompileDraft(world model.World, draft Draft, opts CompileOptions) (model.World, CompileReport, error) {
	var report CompileReport

	world = detachWorld(world)

	if opts.MinConfidence > 0 {
		draft, report.Filtered = filterDraftByConfidence(draft, opts.MinConfidence)
	}

	resolver := opts.Resolver
	if resolver == nil {
		resolver = NoopAliasResolver{}
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	resolved, aliases, err := resolveAliases(ctx, resolver, draft, world)
	if err != nil {
		return model.World{}, report, fmt.Errorf("alias resolver: %w", err)
	}
	draft = resolved
	report.Aliases = aliases

	if draft.Canon != nil {
		world.Canon = mergeCanon(world.Canon, *draft.Canon)
	}

	for _, de := range draft.Entities {
		eid := model.EntityID(de.ID)
		existing, hasExisting := world.Entities[eid]
		if hasExisting && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		entity := model.Entity{
			ID:          eid,
			Type:        de.Type,
			Name:        de.Name,
			Description: de.Description,
			Aliases:     append([]string(nil), de.Aliases...),
			Tags:        de.Tags,
		}
		if hasExisting && opts.ConflictPolicy == ConflictPolicyReplace {
			// Ingest is a knowledge-layer update, not a runtime-state reset.
			// Preserve runtime fields (Components, State) from the existing
			// entity; the draft only authoritatively replaces descriptive
			// fields (Type/Name/Description/Tags). Aliases are accumulative
			// (existing ∪ draft) — never lose known names.
			entity.Components = existing.Components
			entity.State = existing.State
			entity.Aliases = appendUnique(existing.Aliases, de.Aliases...)
		}
		if err := entity.Validate(); err != nil {
			report.Rejected++
			report.Notes = append(report.Notes, fmt.Sprintf("entity %q rejected: %v", de.ID, err))
			continue
		}
		world.Entities[eid] = entity
		report.Inserted++
		if len(de.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    de.ID,
				Kind:       "entity",
				SourceRefs: de.SourceRefs,
			})
		}
	}

	allEntityIDs := collectAllEntityIDs(world, draft)

	for _, dr := range draft.Relations {
		if !opts.AllowDanglingRefs {
			if !allEntityIDs[dr.SourceID] {
				return model.World{}, report, fmt.Errorf("relation %q: dangling source_id %q", dr.ID, dr.SourceID)
			}
			if !allEntityIDs[dr.TargetID] {
				return model.World{}, report, fmt.Errorf("relation %q: dangling target_id %q", dr.ID, dr.TargetID)
			}
		}
		rid := model.RelationID(dr.ID)
		if relationExists(world.Relations, rid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if relationExists(world.Relations, rid) {
			world.Relations = removeRelation(world.Relations, rid)
		}
		world.Relations = append(world.Relations, model.Relation{
			ID:       rid,
			Type:     dr.Type,
			SourceID: model.EntityID(dr.SourceID),
			TargetID: model.EntityID(dr.TargetID),
		})
		report.Inserted++
		if len(dr.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dr.ID,
				Kind:       "relation",
				SourceRefs: dr.SourceRefs,
			})
		}
	}

	for _, df := range draft.Facts {
		fid := model.FactID(df.ID)
		if factExists(world.Facts, fid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if factExists(world.Facts, fid) {
			world.Facts = removeFact(world.Facts, fid)
		}
		world.Facts = append(world.Facts, model.Fact{
			ID:        fid,
			SubjectID: model.EntityID(df.SubjectID),
			Predicate: df.Predicate,
			Value:     model.Value{Kind: model.ValueKindString, Raw: df.Value},
		})
		report.Inserted++
		if len(df.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    df.ID,
				Kind:       "fact",
				SourceRefs: df.SourceRefs,
			})
		}
	}

	for _, dt := range draft.Threads {
		tid := model.ThreadID(dt.ID)
		existing, hasExisting := findThread(world.Threads, tid)
		if hasExisting && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		status := dt.Status
		if status == "" {
			status = model.ThreadStatusOpen
		}
		thread := model.WorldThread{
			ID:             tid,
			Kind:           dt.Kind,
			Title:          dt.Title,
			Summary:        dt.Summary,
			Status:         status,
			Priority:       dt.Priority,
			Tension:        dt.Tension,
			ParticipantIDs: toEntityIDs(dt.ParticipantIDs),
			LocationID:     model.EntityID(dt.LocationID),
		}
		if hasExisting && opts.ConflictPolicy == ConflictPolicyReplace {
			// Knowledge update: union participants (never lose a known member);
			// only override LocationID when the draft supplies one (treat empty
			// LocationID as "no opinion").
			thread.ParticipantIDs = mergeEntityIDs(existing.ParticipantIDs, toEntityIDs(dt.ParticipantIDs))
			if dt.LocationID == "" {
				thread.LocationID = existing.LocationID
			}
		}
		if err := thread.Validate(); err != nil {
			report.Rejected++
			report.Notes = append(report.Notes, fmt.Sprintf("thread %q rejected: %v", dt.ID, err))
			continue
		}
		if hasExisting {
			world.Threads = removeThread(world.Threads, tid)
		}
		world.Threads = append(world.Threads, thread)
		report.Inserted++
		if len(dt.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dt.ID,
				Kind:       "thread",
				SourceRefs: dt.SourceRefs,
			})
		}
	}

	for _, dm := range draft.Memories {
		mid := model.MemoryID(dm.ID)
		existing, hasExisting := findMemory(world.Memory, mid)
		if hasExisting && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		ownerKind := dm.OwnerKind
		if ownerKind == "" {
			ownerKind = model.MemoryOwnerKindWorld
		}
		memory := model.MemoryRecord{
			ID:          mid,
			Owner:       model.MemoryOwner{Kind: ownerKind, ID: dm.OwnerID},
			Content:     dm.Content,
			Summary:     dm.Summary,
			Scope:       dm.Scope,
			Kind:        dm.Kind,
			TruthStatus: dm.TruthStatus,
			Confidence:  dm.Confidence,
			Importance:  dm.Importance,
			SubjectIDs:  toEntityIDs(dm.SubjectIDs),
			EventIDs:    toEventIDs(dm.EventIDs),
		}
		if hasExisting && opts.ConflictPolicy == ConflictPolicyReplace {
			// Knowledge update: union subjects/events. Never silently drop
			// previously recorded references.
			memory.SubjectIDs = mergeEntityIDs(existing.SubjectIDs, toEntityIDs(dm.SubjectIDs))
			memory.EventIDs = mergeEventIDs(existing.EventIDs, toEventIDs(dm.EventIDs))
		}
		if err := memory.Validate(); err != nil {
			report.Rejected++
			report.Notes = append(report.Notes, fmt.Sprintf("memory %q rejected: %v", dm.ID, err))
			continue
		}
		if hasExisting {
			world.Memory = removeMemory(world.Memory, mid)
		}
		world.Memory = append(world.Memory, memory)
		report.Inserted++
		if len(dm.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dm.ID,
				Kind:       "memory",
				SourceRefs: dm.SourceRefs,
			})
		}
	}

	return world, report, nil
}

// detachWorld returns a copy of world whose mutable reference fields
// (Entities map and Relations/Facts/Threads/Memory slices) no longer alias
// the caller's backing storage. Entity, Relation, Fact, WorldThread and
// MemoryRecord values are themselves not deep-copied — CompileDraft never
// mutates fields on existing values, only replaces whole records.
func detachWorld(world model.World) model.World {
	entities := make(map[model.EntityID]model.Entity, len(world.Entities))
	for id, e := range world.Entities {
		entities[id] = e
	}
	world.Entities = entities
	if world.Relations != nil {
		world.Relations = append([]model.Relation(nil), world.Relations...)
	}
	if world.Facts != nil {
		world.Facts = append([]model.Fact(nil), world.Facts...)
	}
	if world.Threads != nil {
		world.Threads = append([]model.WorldThread(nil), world.Threads...)
	}
	if world.Memory != nil {
		world.Memory = append([]model.MemoryRecord(nil), world.Memory...)
	}
	return world
}

func mergeCanon(existing model.Canon, draft DraftCanon) model.Canon {
	if len(draft.Genre) > 0 {
		existing.Genre = appendUnique(existing.Genre, draft.Genre...)
	}
	if len(draft.Tone) > 0 {
		existing.Tone = appendUnique(existing.Tone, draft.Tone...)
	}
	if draft.Premise != "" {
		existing.Premise = draft.Premise
	}
	if len(draft.Laws) > 0 {
		existing.Laws = appendUnique(existing.Laws, draft.Laws...)
	}
	if len(draft.Boundaries) > 0 {
		existing.Boundaries = appendUnique(existing.Boundaries, draft.Boundaries...)
	}
	return existing
}

func collectAllEntityIDs(world model.World, draft Draft) map[string]bool {
	ids := map[string]bool{}
	for eid := range world.Entities {
		ids[string(eid)] = true
	}
	for _, de := range draft.Entities {
		ids[de.ID] = true
	}
	return ids
}

func relationExists(relations []model.Relation, id model.RelationID) bool {
	for _, r := range relations {
		if r.ID == id {
			return true
		}
	}
	return false
}

func removeRelation(relations []model.Relation, id model.RelationID) []model.Relation {
	out := make([]model.Relation, 0, len(relations))
	for _, r := range relations {
		if r.ID != id {
			out = append(out, r)
		}
	}
	return out
}

func factExists(facts []model.Fact, id model.FactID) bool {
	for _, f := range facts {
		if f.ID == id {
			return true
		}
	}
	return false
}

func removeFact(facts []model.Fact, id model.FactID) []model.Fact {
	out := make([]model.Fact, 0, len(facts))
	for _, f := range facts {
		if f.ID != id {
			out = append(out, f)
		}
	}
	return out
}

func findThread(threads []model.WorldThread, id model.ThreadID) (model.WorldThread, bool) {
	for _, t := range threads {
		if t.ID == id {
			return t, true
		}
	}
	return model.WorldThread{}, false
}

func removeThread(threads []model.WorldThread, id model.ThreadID) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, t := range threads {
		if t.ID != id {
			out = append(out, t)
		}
	}
	return out
}

func findMemory(memories []model.MemoryRecord, id model.MemoryID) (model.MemoryRecord, bool) {
	for _, m := range memories {
		if m.ID == id {
			return m, true
		}
	}
	return model.MemoryRecord{}, false
}

func removeMemory(memories []model.MemoryRecord, id model.MemoryID) []model.MemoryRecord {
	out := make([]model.MemoryRecord, 0, len(memories))
	for _, m := range memories {
		if m.ID != id {
			out = append(out, m)
		}
	}
	return out
}

// resolveAliases consults the AliasResolver for every entity ID referenced in
// the draft (entities list AND endpoints of relations/facts/memories) and
// rewrites cross-references from each draft ID to the canonical world ID.
//
// Entities that resolve to a canonical ID are dropped from the draft entity
// list — their field updates are not auto-merged onto the canonical entity
// to keep this layer simple. Products that want field-level merge should
// pre-merge before calling CompileDraft.
//
// For IDs that appear only as references (e.g. a relation endpoint with no
// matching draft entity), the resolver receives a stub DraftEntity carrying
// only the ID. Resolvers should be defensive about missing fields.
func resolveAliases(ctx context.Context, resolver AliasResolver, draft Draft, world model.World) (Draft, map[string]string, error) {
	if _, isNoop := resolver.(NoopAliasResolver); isNoop {
		return draft, nil, nil
	}

	known := map[string]DraftEntity{}
	for _, e := range draft.Entities {
		known[e.ID] = e
	}
	addRef := func(id string) {
		if id == "" {
			return
		}
		if _, ok := known[id]; !ok {
			known[id] = DraftEntity{ID: id}
		}
	}
	for _, r := range draft.Relations {
		addRef(r.SourceID)
		addRef(r.TargetID)
	}
	for _, f := range draft.Facts {
		addRef(f.SubjectID)
	}
	for _, th := range draft.Threads {
		for _, pid := range th.ParticipantIDs {
			addRef(pid)
		}
		addRef(th.LocationID)
	}
	for _, m := range draft.Memories {
		addRef(m.OwnerID)
		for _, sid := range m.SubjectIDs {
			addRef(sid)
		}
	}

	aliases := map[string]string{}
	if batch, ok := resolver.(BatchAliasResolver); ok {
		// Fast path: one round-trip for all referenced IDs.
		entities := make([]DraftEntity, 0, len(known))
		for _, stub := range known {
			entities = append(entities, stub)
		}
		mapping, err := batch.ResolveBatch(ctx, entities, world)
		if err != nil {
			return Draft{}, nil, fmt.Errorf("batch resolver: %w", err)
		}
		for id, canonical := range mapping {
			if canonical == "" || canonical == id {
				continue
			}
			if _, isKnown := known[id]; !isKnown {
				// Resolver returned a mapping for an ID we did not ask about;
				// ignore it to keep the contract local.
				continue
			}
			aliases[id] = canonical
		}
	} else {
		for id, stub := range known {
			canonical, err := resolver.Resolve(ctx, stub, world)
			if err != nil {
				return Draft{}, nil, fmt.Errorf("entity %q: %w", id, err)
			}
			if canonical != "" && canonical != id {
				aliases[id] = canonical
			}
		}
	}

	if len(aliases) == 0 {
		return draft, nil, nil
	}

	// Promote draft Name + Aliases into the canonical entity's Aliases so
	// the information the resolver "consumed" is not lost. Two ways the
	// promotion lands:
	//   - canonical already in world.Entities → mutate the world copy in place
	//     (mutates the caller-owned world; acceptable because CompileDraft
	//     already mutates world.Entities for new inserts)
	//   - canonical only appears as a draft entity → append to that draft
	//     entity's Aliases so the subsequent insert/replace carries it
	draftEntityIndex := map[string]int{}
	for i, e := range draft.Entities {
		draftEntityIndex[e.ID] = i
	}
	for draftID, canonical := range aliases {
		stub := known[draftID]
		var newNames []string
		if stub.Name != "" {
			newNames = append(newNames, stub.Name)
		}
		newNames = append(newNames, stub.Aliases...)
		if len(newNames) == 0 {
			continue
		}
		if idx, inDraft := draftEntityIndex[canonical]; inDraft {
			draft.Entities[idx].Aliases = appendUnique(draft.Entities[idx].Aliases, newNames...)
			continue
		}
		if existing, inWorld := world.Entities[model.EntityID(canonical)]; inWorld {
			existing.Aliases = appendUnique(existing.Aliases, newNames...)
			world.Entities[model.EntityID(canonical)] = existing
		}
	}

	var keptEntities []DraftEntity
	for _, e := range draft.Entities {
		if _, mapped := aliases[e.ID]; mapped {
			continue
		}
		keptEntities = append(keptEntities, e)
	}
	draft.Entities = keptEntities

	for i, r := range draft.Relations {
		if c, ok := aliases[r.SourceID]; ok {
			draft.Relations[i].SourceID = c
		}
		if c, ok := aliases[r.TargetID]; ok {
			draft.Relations[i].TargetID = c
		}
	}
	for i, f := range draft.Facts {
		if c, ok := aliases[f.SubjectID]; ok {
			draft.Facts[i].SubjectID = c
		}
	}
	for i, th := range draft.Threads {
		if c, ok := aliases[th.LocationID]; ok {
			draft.Threads[i].LocationID = c
		}
		if len(th.ParticipantIDs) > 0 {
			rewritten := make([]string, 0, len(th.ParticipantIDs))
			seen := map[string]bool{}
			for _, pid := range th.ParticipantIDs {
				if c, ok := aliases[pid]; ok {
					pid = c
				}
				if pid == "" || seen[pid] {
					continue
				}
				rewritten = append(rewritten, pid)
				seen[pid] = true
			}
			draft.Threads[i].ParticipantIDs = rewritten
		}
	}
	for i, m := range draft.Memories {
		if c, ok := aliases[m.OwnerID]; ok {
			draft.Memories[i].OwnerID = c
		}
		if len(m.SubjectIDs) > 0 {
			rewritten := make([]string, 0, len(m.SubjectIDs))
			seen := map[string]bool{}
			for _, sid := range m.SubjectIDs {
				if c, ok := aliases[sid]; ok {
					sid = c
				}
				if sid == "" || seen[sid] {
					continue
				}
				rewritten = append(rewritten, sid)
				seen[sid] = true
			}
			draft.Memories[i].SubjectIDs = rewritten
		}
	}
	return draft, aliases, nil
}

func toEntityIDs(ids []string) []model.EntityID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]model.EntityID, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		out = append(out, model.EntityID(id))
	}
	return out
}

func toEventIDs(ids []string) []model.EventID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]model.EventID, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		out = append(out, model.EventID(id))
	}
	return out
}

func mergeEntityIDs(a, b []model.EntityID) []model.EntityID {
	seen := map[model.EntityID]bool{}
	var out []model.EntityID
	for _, id := range a {
		if !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	for _, id := range b {
		if !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	return out
}

func mergeEventIDs(a, b []model.EventID) []model.EventID {
	seen := map[model.EventID]bool{}
	var out []model.EventID
	for _, id := range a {
		if !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	for _, id := range b {
		if !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	return out
}

func filterDraftByConfidence(d Draft, minConf float64) (Draft, int) {
	filtered := 0
	var out Draft
	out.Canon = d.Canon

	for _, e := range d.Entities {
		if e.Confidence >= minConf {
			out.Entities = append(out.Entities, e)
		} else {
			filtered++
		}
	}
	for _, r := range d.Relations {
		if r.Confidence >= minConf {
			out.Relations = append(out.Relations, r)
		} else {
			filtered++
		}
	}
	for _, f := range d.Facts {
		if f.Confidence >= minConf {
			out.Facts = append(out.Facts, f)
		} else {
			filtered++
		}
	}
	for _, t := range d.Threads {
		if t.Confidence >= minConf {
			out.Threads = append(out.Threads, t)
		} else {
			filtered++
		}
	}
	for _, m := range d.Memories {
		if m.Confidence >= minConf {
			out.Memories = append(out.Memories, m)
		} else {
			filtered++
		}
	}
	return out, filtered
}
