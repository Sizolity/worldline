package view

import (
	"slices"
	"sort"

	"github.com/sizolity/worldline/internal/world/model"
)

// WorldDebugContext is a read-only GM/debug projection that exposes the
// complete world truth with no ownership or visibility filtering.
type WorldDebugContext struct {
	World      WorldSummary           `json:"world"`
	Entities   []model.Entity         `json:"entities"`
	Facts      []model.Fact           `json:"facts"`
	Relations  []model.Relation       `json:"relations"`
	Memories   []model.MemoryRecord   `json:"memories"`
	Threads    []model.WorldThread    `json:"threads"`
	Rules      []model.Rule           `json:"rules"`
	EventLog   []model.WorldEvent     `json:"event_log"`
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
			Canon: model.Canon{
				Genre:      nonNilClone(w.Canon.Genre),
				Tone:       nonNilClone(w.Canon.Tone),
				StyleGuide: nonNilClone(w.Canon.StyleGuide),
				Premise:    w.Canon.Premise,
				Laws:       nonNilClone(w.Canon.Laws),
				Boundaries: nonNilClone(w.Canon.Boundaries),
				Secrets:    nonNilClone(w.Canon.Secrets),
			},
			Clock: w.Clock.Clone(),
			Metadata: model.WorldMetadata{
				SchemaVersion: w.Metadata.SchemaVersion,
				Source:        w.Metadata.Source,
				Tags:          nonNilClone(w.Metadata.Tags),
				Fork:          w.Metadata.Fork,
			},
		},
		Entities:   entities,
		Facts:      factsNonNil(w.Facts),
		Relations:  nonNilClone(w.Relations),
		Memories:   memoriesNonNil(w.Memories),
		Threads:    nonNilClone(w.Threads),
		Rules:      nonNilClone(w.Rules),
		EventLog:   eventsNonNil(w.EventLog),
		EventQueue: eventQueueNonNil(w.EventQueue),
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
		e := m[id].Clone()
		if e.Tags == nil {
			e.Tags = []string{}
		}
		out[i] = e
	}
	return out
}

func factsNonNil(facts []model.Fact) []model.Fact {
	out := model.CloneFacts(facts)
	if out == nil {
		return []model.Fact{}
	}
	return out
}

func memoriesNonNil(memories []model.MemoryRecord) []model.MemoryRecord {
	out := model.CloneMemories(memories)
	if out == nil {
		return []model.MemoryRecord{}
	}
	for i := range out {
		if out[i].SubjectIDs == nil {
			out[i].SubjectIDs = []model.EntityID{}
		}
		if out[i].EventIDs == nil {
			out[i].EventIDs = []model.EventID{}
		}
	}
	return out
}

func eventsNonNil(events []model.WorldEvent) []model.WorldEvent {
	out := model.CloneEvents(events)
	if out == nil {
		return []model.WorldEvent{}
	}
	for i := range out {
		if out[i].ActorIDs == nil {
			out[i].ActorIDs = []model.EntityID{}
		}
		if out[i].TargetIDs == nil {
			out[i].TargetIDs = []model.EntityID{}
		}
		if out[i].Effects == nil {
			out[i].Effects = []model.Effect{}
		}
	}
	return out
}

func eventQueueNonNil(items []model.EventQueueItem) []model.EventQueueItem {
	out := model.CloneEventQueue(items)
	if out == nil {
		return []model.EventQueueItem{}
	}
	for i := range out {
		if out[i].Event.ActorIDs == nil {
			out[i].Event.ActorIDs = []model.EntityID{}
		}
		if out[i].Event.TargetIDs == nil {
			out[i].Event.TargetIDs = []model.EntityID{}
		}
		if out[i].Event.Effects == nil {
			out[i].Event.Effects = []model.Effect{}
		}
	}
	return out
}

func nonNilClone[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return slices.Clone(s)
}
