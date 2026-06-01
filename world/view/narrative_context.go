package view

import "github.com/sizolity/worldline/world/model"

// NarrativeContextRequest controls how much world history a narrative
// projection should expose.
type NarrativeContextRequest struct {
	RecentEventLimit int
}

// NarrativeContext is a prose-ready, structured projection of world state.
// It is intentionally not prose itself and does not call an LLM.
type NarrativeContext struct {
	World          WorldSummary         `json:"world"`
	RecentEvents   []model.WorldEvent   `json:"recent_events"`
	ActiveThreads  []model.WorldThread  `json:"active_threads"`
	Facts          []model.Fact         `json:"facts"`
	PublicMemories []model.MemoryRecord `json:"public_memories"`
}

// NarrativeView renders the bounded context needed by narrative output layers.
type NarrativeView struct{}

func (v NarrativeView) Render(w model.World, req NarrativeContextRequest) NarrativeContext {
	return NarrativeContext{
		World: WorldSummary{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Canon:       cloneCanon(w.Canon),
			Clock:       cloneWorldClock(w.Clock),
			Metadata:    cloneWorldMetadata(w.Metadata),
		},
		RecentEvents:   recentEvents(w.EventLog, req.RecentEventLimit),
		ActiveThreads:  activeThreads(w.Threads),
		Facts:          cloneFacts(w.Facts),
		PublicMemories: publicWorldMemories(w.Memory),
	}
}

func recentEvents(events []model.WorldEvent, limit int) []model.WorldEvent {
	if limit <= 0 || limit >= len(events) {
		return cloneEvents(events)
	}
	return cloneEvents(events[len(events)-limit:])
}

func activeThreads(threads []model.WorldThread) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, thread := range threads {
		switch thread.Status {
		case model.ThreadStatusOpen, model.ThreadStatusActive:
			out = append(out, thread)
		}
	}
	return nonNilClone(out)
}

func publicWorldMemories(memories []model.MemoryRecord) []model.MemoryRecord {
	out := make([]model.MemoryRecord, 0, len(memories))
	for _, memory := range memories {
		if memory.Owner.Kind == model.MemoryOwnerKindWorld && isPublicWorldTruthStatus(memory.TruthStatus) {
			out = append(out, memory)
		}
	}
	return cloneMemories(out)
}
