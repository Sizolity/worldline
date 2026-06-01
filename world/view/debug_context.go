package view

import (
	"slices"
	"sort"

	"github.com/sizolity/worldline/world/model"
)

// WorldDebugContext is a read-only GM/debug projection that exposes the
// complete world truth with no ownership or visibility filtering.
type WorldDebugContext struct {
	World      WorldSummary         `json:"world"`
	Entities   []model.Entity       `json:"entities"`
	Facts      []model.Fact         `json:"facts"`
	Relations  []model.Relation     `json:"relations"`
	Memories   []model.MemoryRecord `json:"memories"`
	Threads    []model.WorldThread  `json:"threads"`
	Rules      []model.Rule         `json:"rules"`
	EventLog   []model.WorldEvent   `json:"event_log"`
	EventQueue []model.EventQueueItem `json:"event_queue"`
}

// WorldSummary captures top-level world metadata without the collection fields.
type WorldSummary struct {
	ID          model.WorldID       `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Canon       model.Canon         `json:"canon,omitempty"`
	Clock       model.WorldClock    `json:"clock,omitempty"`
	Metadata    model.WorldMetadata `json:"metadata,omitempty"`
}

// WorldDebugView renders the full world state for GM / debug inspection.
// Unlike CharacterContextView it applies no ownership or truth-status
// filtering — secrets, private memories, and hidden narrator knowledge
// are all included.
type WorldDebugView struct{}

// Render projects the world into a WorldDebugContext.  Entities are sorted
// by ID for deterministic output.  All slice fields are guaranteed non-nil.
func (v WorldDebugView) Render(w model.World) WorldDebugContext {
	entities := entitiesSorted(w.Entities)

	return WorldDebugContext{
		World: WorldSummary{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Canon:       cloneCanon(w.Canon),
			Clock:       cloneWorldClock(w.Clock),
			Metadata:    cloneWorldMetadata(w.Metadata),
		},
		Entities:   entities,
		Facts:      cloneFacts(w.Facts),
		Relations:  nonNilClone(w.Relations),
		Memories:   cloneMemories(w.Memory),
		Threads:    nonNilClone(w.Threads),
		Rules:      nonNilClone(w.Rules),
		EventLog:   cloneEvents(w.EventLog),
		EventQueue: cloneEventQueueItems(w.EventQueue),
	}
}

func entitiesSorted(m map[model.EntityID]model.Entity) []model.Entity {
	if len(m) == 0 {
		return []model.Entity{}
	}
	ids := make([]model.EntityID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]model.Entity, len(ids))
	for i, id := range ids {
		out[i] = cloneEntity(m[id])
	}
	return out
}

func cloneCanon(c model.Canon) model.Canon {
	c.Genre = nonNilClone(c.Genre)
	c.Tone = nonNilClone(c.Tone)
	c.StyleGuide = nonNilClone(c.StyleGuide)
	c.Laws = nonNilClone(c.Laws)
	c.Boundaries = nonNilClone(c.Boundaries)
	c.Secrets = nonNilClone(c.Secrets)
	return c
}

func cloneWorldClock(c model.WorldClock) model.WorldClock {
	c.Current.Calendar = cloneIntMap(c.Current.Calendar)
	return c
}

func cloneWorldMetadata(m model.WorldMetadata) model.WorldMetadata {
	m.Tags = nonNilClone(m.Tags)
	return m
}

func cloneEntity(e model.Entity) model.Entity {
	e.Components = cloneAnyMap(e.Components)
	e.State = cloneValueMap(e.State)
	e.Tags = nonNilClone(e.Tags)
	return e
}

func cloneFacts(facts []model.Fact) []model.Fact {
	out := nonNilClone(facts)
	for i := range out {
		out[i].Value = cloneValue(out[i].Value)
	}
	return out
}

func cloneMemories(memories []model.MemoryRecord) []model.MemoryRecord {
	out := nonNilClone(memories)
	for i := range out {
		out[i].SubjectIDs = nonNilClone(out[i].SubjectIDs)
		out[i].EventIDs = nonNilClone(out[i].EventIDs)
	}
	return out
}

func cloneEvents(events []model.WorldEvent) []model.WorldEvent {
	out := nonNilClone(events)
	for i := range out {
		out[i].ActorIDs = nonNilClone(out[i].ActorIDs)
		out[i].TargetIDs = nonNilClone(out[i].TargetIDs)
		out[i].Effects = cloneEffects(out[i].Effects)
	}
	return out
}

func cloneEventQueueItems(items []model.EventQueueItem) []model.EventQueueItem {
	out := nonNilClone(items)
	for i := range out {
		out[i].Event.ActorIDs = nonNilClone(out[i].Event.ActorIDs)
		out[i].Event.TargetIDs = nonNilClone(out[i].Event.TargetIDs)
		out[i].Event.Effects = cloneEffects(out[i].Event.Effects)
	}
	return out
}

func cloneEffects(effects []model.Effect) []model.Effect {
	out := nonNilClone(effects)
	for i := range out {
		out[i].Payload = cloneValueMap(out[i].Payload)
	}
	return out
}

func cloneValueMap(in map[string]model.Value) map[string]model.Value {
	if in == nil {
		return nil
	}
	out := make(map[string]model.Value, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(v model.Value) model.Value {
	v.Raw = cloneAny(v.Raw)
	return v
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneAny(value)
	}
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAny(item)
		}
		return out
	case []string:
		return nonNilClone(typed)
	case []int:
		return nonNilClone(typed)
	case []float64:
		return nonNilClone(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case map[string]int:
		return cloneIntMap(typed)
	default:
		return v
	}
}

func nonNilClone[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return slices.Clone(s)
}
