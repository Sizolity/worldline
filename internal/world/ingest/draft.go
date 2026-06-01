package ingest

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

// Parser extracts structured world data from a complete source document.
// Implementations may use LLM, rule-based extraction, or any other method.
// The ingest package never provides a concrete implementation.
type Parser interface {
	Parse(ctx context.Context, doc SourceDocument) (Draft, error)
}

// ChunkParser extracts structured world data from a single source chunk.
// Use with ParseChunks to process large documents chunk-by-chunk, then
// automatically merge partial drafts with deduplication.
type ChunkParser interface {
	ParseChunk(ctx context.Context, chunk SourceChunk) (Draft, error)
}

// ParseChunks runs a ChunkParser on each chunk sequentially, then merges
// all partial drafts into one deduplicated Draft.
func ParseChunks(ctx context.Context, cp ChunkParser, chunks []SourceChunk) (Draft, error) {
	var merged Draft
	for _, chunk := range chunks {
		partial, err := cp.ParseChunk(ctx, chunk)
		if err != nil {
			return Draft{}, fmt.Errorf("chunk %q: %w", chunk.ID, err)
		}
		merged = MergeDrafts(merged, partial)
	}
	return merged, nil
}

// MergeDrafts combines two drafts, deduplicating by ID. When the same ID
// appears in both drafts, the entry with higher Confidence wins. SourceRefs
// are always merged (union).
func MergeDrafts(a, b Draft) Draft {
	var out Draft

	out.Canon = mergeCanonDrafts(a.Canon, b.Canon)
	out.Entities = mergeDraftSlice(a.Entities, b.Entities, draftEntityID, draftEntityConfidence, mergeDraftEntity)
	out.Relations = mergeDraftSlice(a.Relations, b.Relations, draftRelationID, draftRelationConfidence, mergeDraftRelation)
	out.Facts = mergeDraftSlice(a.Facts, b.Facts, draftFactID, draftFactConfidence, mergeDraftFact)
	out.Threads = mergeDraftSlice(a.Threads, b.Threads, draftThreadID, draftThreadConfidence, mergeDraftThread)
	out.Memories = mergeDraftSlice(a.Memories, b.Memories, draftMemoryID, draftMemoryConfidence, mergeDraftMemory)

	return out
}

func mergeCanonDrafts(a, b *DraftCanon) *DraftCanon {
	if a == nil && b == nil {
		return nil
	}
	out := &DraftCanon{}
	if a != nil {
		*out = *a
	}
	if b != nil {
		if len(b.Genre) > 0 {
			out.Genre = appendUnique(out.Genre, b.Genre...)
		}
		if len(b.Tone) > 0 {
			out.Tone = appendUnique(out.Tone, b.Tone...)
		}
		if b.Premise != "" {
			out.Premise = b.Premise
		}
		if len(b.Laws) > 0 {
			out.Laws = appendUnique(out.Laws, b.Laws...)
		}
		if len(b.Boundaries) > 0 {
			out.Boundaries = appendUnique(out.Boundaries, b.Boundaries...)
		}
	}
	return out
}

func appendUnique(base []string, items ...string) []string {
	seen := map[string]bool{}
	for _, s := range base {
		seen[s] = true
	}
	out := append([]string(nil), base...)
	for _, s := range items {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

type draftItem interface {
	DraftEntity | DraftRelation | DraftFact | DraftThread | DraftMemory
}

func mergeDraftSlice[T draftItem](
	a, b []T,
	idFn func(T) string,
	confFn func(T) float64,
	mergeFn func(existing, incoming T) T,
) []T {
	index := map[string]int{}
	var out []T
	for _, item := range a {
		id := idFn(item)
		index[id] = len(out)
		out = append(out, item)
	}
	for _, item := range b {
		id := idFn(item)
		if idx, exists := index[id]; exists {
			if confFn(item) >= confFn(out[idx]) {
				out[idx] = mergeFn(out[idx], item)
			} else {
				out[idx] = mergeFn(item, out[idx])
			}
		} else {
			index[id] = len(out)
			out = append(out, item)
		}
	}
	return out
}

func mergeSourceRefs(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range a {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	for _, s := range b {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

// preferString returns high if non-empty, else low. Used in MergeDrafts so
// that a higher-confidence draft with an empty scalar does not silently
// wipe out a lower-confidence draft's non-empty value.
func preferString(high, low string) string {
	if high != "" {
		return high
	}
	return low
}

// preferFloat returns high if non-zero, else low. Same rationale as
// preferString — a high-conf draft that omitted a numeric field (so it
// defaulted to 0) should not overwrite a low-conf draft's explicit value.
// Note: this means "0" cannot survive a merge if the other side has a
// non-zero value. Callers needing to distinguish "set to 0" from "unset"
// should use a pointer-backed schema (intentionally not adopted here).
func preferFloat(high, low float64) float64 {
	if high != 0 {
		return high
	}
	return low
}

func draftEntityID(e DraftEntity) string          { return e.ID }
func draftEntityConfidence(e DraftEntity) float64 { return e.Confidence }
func mergeDraftEntity(low, high DraftEntity) DraftEntity {
	out := high
	out.Type = preferString(high.Type, low.Type)
	out.Name = preferString(high.Name, low.Name)
	out.Description = preferString(high.Description, low.Description)
	out.SourceRefs = mergeSourceRefs(low.SourceRefs, high.SourceRefs)
	out.Aliases = appendUnique(high.Aliases, low.Aliases...)
	out.Tags = appendUnique(high.Tags, low.Tags...)
	return out
}

func draftRelationID(r DraftRelation) string          { return r.ID }
func draftRelationConfidence(r DraftRelation) float64 { return r.Confidence }
func mergeDraftRelation(low, high DraftRelation) DraftRelation {
	out := high
	out.Type = preferString(high.Type, low.Type)
	out.SourceID = preferString(high.SourceID, low.SourceID)
	out.TargetID = preferString(high.TargetID, low.TargetID)
	out.SourceRefs = mergeSourceRefs(low.SourceRefs, high.SourceRefs)
	return out
}

func draftFactID(f DraftFact) string          { return f.ID }
func draftFactConfidence(f DraftFact) float64 { return f.Confidence }
func mergeDraftFact(low, high DraftFact) DraftFact {
	out := high
	out.SubjectID = preferString(high.SubjectID, low.SubjectID)
	out.Predicate = preferString(high.Predicate, low.Predicate)
	out.Value = preferString(high.Value, low.Value)
	out.SourceRefs = mergeSourceRefs(low.SourceRefs, high.SourceRefs)
	return out
}

func draftThreadID(t DraftThread) string          { return t.ID }
func draftThreadConfidence(t DraftThread) float64 { return t.Confidence }
func mergeDraftThread(low, high DraftThread) DraftThread {
	out := high
	out.Kind = preferString(high.Kind, low.Kind)
	out.Title = preferString(high.Title, low.Title)
	out.Summary = preferString(high.Summary, low.Summary)
	out.Status = preferString(high.Status, low.Status)
	out.LocationID = preferString(high.LocationID, low.LocationID)
	out.Priority = preferFloat(high.Priority, low.Priority)
	out.Tension = preferFloat(high.Tension, low.Tension)
	out.ParticipantIDs = appendUnique(high.ParticipantIDs, low.ParticipantIDs...)
	out.SourceRefs = mergeSourceRefs(low.SourceRefs, high.SourceRefs)
	return out
}

func draftMemoryID(m DraftMemory) string          { return m.ID }
func draftMemoryConfidence(m DraftMemory) float64 { return m.Confidence }
func mergeDraftMemory(low, high DraftMemory) DraftMemory {
	out := high
	out.OwnerKind = preferString(high.OwnerKind, low.OwnerKind)
	out.OwnerID = preferString(high.OwnerID, low.OwnerID)
	out.Content = preferString(high.Content, low.Content)
	out.Summary = preferString(high.Summary, low.Summary)
	out.Scope = preferString(high.Scope, low.Scope)
	out.Kind = preferString(high.Kind, low.Kind)
	out.TruthStatus = preferString(high.TruthStatus, low.TruthStatus)
	out.Importance = preferFloat(high.Importance, low.Importance)
	out.SubjectIDs = appendUnique(high.SubjectIDs, low.SubjectIDs...)
	out.EventIDs = appendUnique(high.EventIDs, low.EventIDs...)
	out.SourceRefs = mergeSourceRefs(low.SourceRefs, high.SourceRefs)
	return out
}

// Draft holds the raw extraction output from a Parser before validation
// and compilation into world model types.
type Draft struct {
	Canon     *DraftCanon    `json:"canon,omitempty"`
	Entities  []DraftEntity  `json:"entities,omitempty"`
	Relations []DraftRelation `json:"relations,omitempty"`
	Facts     []DraftFact    `json:"facts,omitempty"`
	Threads   []DraftThread  `json:"threads,omitempty"`
	Memories  []DraftMemory  `json:"memories,omitempty"`
}

type DraftCanon struct {
	Genre      []string `json:"genre,omitempty"`
	Tone       []string `json:"tone,omitempty"`
	Premise    string   `json:"premise,omitempty"`
	Laws       []string `json:"laws,omitempty"`
	Boundaries []string `json:"boundaries,omitempty"`
}

type DraftEntity struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	// Aliases is the set of additional human-readable names for this entity
	// (nicknames, epithets, translations). The compiler dedups and unions
	// these with any existing entity Aliases.
	Aliases    []string `json:"aliases,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftRelation struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	SourceID   string   `json:"source_id"`
	TargetID   string   `json:"target_id"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftFact struct {
	ID         string   `json:"id"`
	SubjectID  string   `json:"subject_id"`
	Predicate  string   `json:"predicate"`
	Value      string   `json:"value"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftThread struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Status  string `json:"status,omitempty"`
	// ParticipantIDs is the set of entity IDs taking part in this thread.
	// Resolved through AliasResolver alongside other entity references.
	ParticipantIDs []string `json:"participant_ids,omitempty"`
	// LocationID is the optional entity ID where this thread takes place.
	LocationID string   `json:"location_id,omitempty"`
	Priority   float64  `json:"priority,omitempty"`
	Tension    float64  `json:"tension,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftMemory struct {
	ID        string `json:"id"`
	OwnerKind string `json:"owner_kind"`
	OwnerID   string `json:"owner_id,omitempty"`
	// SubjectIDs is the set of entity IDs this memory is about. Resolved
	// through AliasResolver alongside other entity references.
	SubjectIDs []string `json:"subject_ids,omitempty"`
	// EventIDs is the set of event IDs this memory references. Not subject
	// to alias resolution (events are not entities).
	EventIDs    []string `json:"event_ids,omitempty"`
	Content     string   `json:"content,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	TruthStatus string   `json:"truth_status,omitempty"`
	Importance  float64  `json:"importance,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	SourceRefs  []string `json:"source_refs,omitempty"`
}

// ValidationReport holds errors and warnings from draft validation.
type ValidationReport struct {
	Errors   []string
	Warnings []string
}

// ValidateDraft checks a draft for structural issues without compiling it.
//
// Errors (compile blockers):
//   - empty/duplicate/unsafe IDs
//   - missing required fields (entity name/type, relation type, memory content)
//   - thread Priority/Tension out of [0, 1] (model invariant)
//   - memory Owner.ID required when Owner.Kind != "world"
//   - any empty / malformed ID in thread.participant_ids, thread.location_id,
//     memory.subject_ids, memory.event_ids
//
// Warnings (advisory, do not block compile):
//   - Confidence out of [0, 1] on any draft item
//   - dangling refs within draft: relation endpoints, fact subjects,
//     thread participants / location, memory subjects
func ValidateDraft(draft Draft) ValidationReport {
	var report ValidationReport
	entityIDs := map[string]bool{}

	for i, e := range draft.Entities {
		if e.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(e.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d]: %v", i, err))
		}
		if e.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: name is required", i, e.ID))
		}
		if e.Type == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: type is required", i, e.ID))
		}
		if entityIDs[e.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: duplicate id", i, e.ID))
		}
		entityIDs[e.ID] = true
		for j, alias := range e.Aliases {
			if alias == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: aliases[%d] must not be empty", i, e.ID, j))
			}
		}
		warnConfidence(&report, fmt.Sprintf("entities[%d] %q", i, e.ID), e.Confidence)
	}

	relationIDs := map[string]bool{}
	for i, r := range draft.Relations {
		if r.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(r.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d]: %v", i, err))
		}
		if relationIDs[r.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d] %q: duplicate id", i, r.ID))
		}
		relationIDs[r.ID] = true
		if r.Type == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d] %q: type is required", i, r.ID))
		}
		if r.SourceID != "" && !entityIDs[r.SourceID] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("relations[%d] %q: source_id %q not in draft entities", i, r.ID, r.SourceID))
		}
		if r.TargetID != "" && !entityIDs[r.TargetID] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("relations[%d] %q: target_id %q not in draft entities", i, r.ID, r.TargetID))
		}
		warnConfidence(&report, fmt.Sprintf("relations[%d] %q", i, r.ID), r.Confidence)
	}

	factIDs := map[string]bool{}
	for i, f := range draft.Facts {
		if f.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(f.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d]: %v", i, err))
		}
		if factIDs[f.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d] %q: duplicate id", i, f.ID))
		}
		factIDs[f.ID] = true
		if f.SubjectID != "" && !entityIDs[f.SubjectID] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("facts[%d] %q: subject_id %q not in draft entities", i, f.ID, f.SubjectID))
		}
		warnConfidence(&report, fmt.Sprintf("facts[%d] %q", i, f.ID), f.Confidence)
	}

	threadIDs := map[string]bool{}
	for i, th := range draft.Threads {
		if th.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(th.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d]: %v", i, err))
		}
		if threadIDs[th.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: duplicate id", i, th.ID))
		}
		threadIDs[th.ID] = true
		if th.Priority < 0 || th.Priority > 1 {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: priority must be in [0,1]", i, th.ID))
		}
		if th.Tension < 0 || th.Tension > 1 {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: tension must be in [0,1]", i, th.ID))
		}
		for j, pid := range th.ParticipantIDs {
			if pid == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: participant_ids[%d] must not be empty", i, th.ID, j))
				continue
			}
			if err := model.ValidateID(pid); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: participant_ids[%d]: %v", i, th.ID, j, err))
			}
			if !entityIDs[pid] {
				report.Warnings = append(report.Warnings, fmt.Sprintf("threads[%d] %q: participant_id %q not in draft entities", i, th.ID, pid))
			}
		}
		if th.LocationID != "" {
			if err := model.ValidateID(th.LocationID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: location_id: %v", i, th.ID, err))
			}
			if !entityIDs[th.LocationID] {
				report.Warnings = append(report.Warnings, fmt.Sprintf("threads[%d] %q: location_id %q not in draft entities", i, th.ID, th.LocationID))
			}
		}
		warnConfidence(&report, fmt.Sprintf("threads[%d] %q", i, th.ID), th.Confidence)
	}

	memoryIDs := map[string]bool{}
	for i, m := range draft.Memories {
		if m.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(m.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d]: %v", i, err))
		}
		if memoryIDs[m.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: duplicate id", i, m.ID))
		}
		memoryIDs[m.ID] = true
		if m.Content == "" && m.Summary == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: content or summary is required", i, m.ID))
		}
		if m.OwnerKind != "" && m.OwnerKind != model.MemoryOwnerKindWorld && m.OwnerID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: owner_id is required when owner_kind is %q", i, m.ID, m.OwnerKind))
		}
		if m.Importance < 0 || m.Importance > 1 {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: importance must be in [0,1]", i, m.ID))
		}
		for j, sid := range m.SubjectIDs {
			if sid == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: subject_ids[%d] must not be empty", i, m.ID, j))
				continue
			}
			if err := model.ValidateID(sid); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: subject_ids[%d]: %v", i, m.ID, j, err))
			}
			if !entityIDs[sid] {
				report.Warnings = append(report.Warnings, fmt.Sprintf("memories[%d] %q: subject_id %q not in draft entities", i, m.ID, sid))
			}
		}
		for j, eid := range m.EventIDs {
			if eid == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: event_ids[%d] must not be empty", i, m.ID, j))
				continue
			}
			if err := model.ValidateID(eid); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: event_ids[%d]: %v", i, m.ID, j, err))
			}
		}
		warnConfidence(&report, fmt.Sprintf("memories[%d] %q", i, m.ID), m.Confidence)
	}

	return report
}

func warnConfidence(report *ValidationReport, label string, c float64) {
	if c < 0 || c > 1 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s: confidence %g outside [0,1]", label, c))
	}
}
