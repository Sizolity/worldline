// Package prompt provides a flattened, LLM-friendly projection of world state.
// It is a leaf package depending only on model, so both director and view can
// import it without circular dependencies.
package prompt

import (
	"sort"

	"github.com/sizolity/worldline/internal/world/model"
)

// Context is a flattened, LLM-friendly projection of world state.
// It contains exactly the fields needed by director prompts without
// exposing internal model complexity (aliases, components, event log, etc.).
type Context struct {
	WorldID     string            `json:"world_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Clock       int64             `json:"clock"`
	Entities    []EntitySummary   `json:"entities,omitempty"`
	Facts       []FactSummary     `json:"facts,omitempty"`
	Relations   []RelationSummary `json:"relations,omitempty"`
	Memories    []MemorySummary   `json:"memories,omitempty"`
	Threads     []ThreadSummary   `json:"threads,omitempty"`
}

// EntitySummary is a compact entity representation for LLM prompts.
type EntitySummary struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	State       map[string]any `json:"state,omitempty"`
}

// FactSummary is a compact fact representation for LLM prompts.
type FactSummary struct {
	ID        string `json:"id"`
	SubjectID string `json:"subject_id"`
	Predicate string `json:"predicate"`
	Value     any    `json:"value"`
}

// RelationSummary is a compact relation representation for LLM prompts.
type RelationSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}

// MemorySummary is a compact memory representation for LLM prompts.
type MemorySummary struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id,omitempty"`
	Content     string `json:"content"`
	TruthStatus string `json:"truth_status"`
}

// ThreadSummary is a compact thread representation for LLM prompts.
type ThreadSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// Render projects a model.World into a flat, JSON-serialisable Context.
// Entities are sorted by ID for deterministic output.
func Render(w model.World) Context {
	return Context{
		WorldID:     string(w.ID),
		Name:        w.Name,
		Description: w.Description,
		Clock:       w.Clock.Sequence,
		Entities:    renderEntities(w.Entities),
		Facts:       renderFacts(w.Facts),
		Relations:   renderRelations(w.Relations),
		Memories:    renderMemories(w.Memories),
		Threads:     renderThreads(w.Threads),
	}
}

func renderEntities(entities map[model.EntityID]model.Entity) []EntitySummary {
	if len(entities) == 0 {
		return nil
	}
	ids := make([]model.EntityID, 0, len(entities))
	for id := range entities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := make([]EntitySummary, 0, len(ids))
	for _, id := range ids {
		e := entities[id]
		var state map[string]any
		if len(e.State) > 0 {
			state = make(map[string]any, len(e.State))
			for k, v := range e.State {
				state[k] = v.Raw
			}
		}
		out = append(out, EntitySummary{
			ID:          string(e.ID),
			Type:        e.Type,
			Name:        e.Name,
			Description: e.Description,
			State:       state,
		})
	}
	return out
}

func renderFacts(facts []model.Fact) []FactSummary {
	if len(facts) == 0 {
		return nil
	}
	out := make([]FactSummary, 0, len(facts))
	for _, f := range facts {
		out = append(out, FactSummary{
			ID:        string(f.ID),
			SubjectID: string(f.SubjectID),
			Predicate: f.Predicate,
			Value:     f.Value.Raw,
		})
	}
	return out
}

func renderRelations(relations []model.Relation) []RelationSummary {
	if len(relations) == 0 {
		return nil
	}
	out := make([]RelationSummary, 0, len(relations))
	for _, r := range relations {
		out = append(out, RelationSummary{
			ID:       string(r.ID),
			Type:     r.Type,
			SourceID: string(r.SourceID),
			TargetID: string(r.TargetID),
		})
	}
	return out
}

func renderMemories(memories []model.MemoryRecord) []MemorySummary {
	if len(memories) == 0 {
		return nil
	}
	out := make([]MemorySummary, 0, len(memories))
	for _, m := range memories {
		out = append(out, MemorySummary{
			ID:          string(m.ID),
			OwnerID:     m.Owner.ID,
			Content:     m.Content,
			TruthStatus: m.TruthStatus,
		})
	}
	return out
}

func renderThreads(threads []model.WorldThread) []ThreadSummary {
	if len(threads) == 0 {
		return nil
	}
	out := make([]ThreadSummary, 0, len(threads))
	for _, th := range threads {
		out = append(out, ThreadSummary{
			ID:     string(th.ID),
			Title:  th.Title,
			Status: th.Status,
		})
	}
	return out
}
