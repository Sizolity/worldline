package view

import "github.com/sizolity/worldline/internal/world/model"

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
		RecentEvents:   recentEvents(w.EventLog, req.RecentEventLimit),
		ActiveThreads:  ActiveThreads(w.Threads),
		Facts:          factsNonNil(w.Facts),
		PublicMemories: publicWorldMemories(w.Memory),
	}
}

func recentEvents(events []model.WorldEvent, limit int) []model.WorldEvent {
	if limit <= 0 || limit >= len(events) {
		return eventsNonNil(events)
	}
	return eventsNonNil(events[len(events)-limit:])
}

// ActiveThreads returns threads with status Open or Active.
func ActiveThreads(threads []model.WorldThread) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, thread := range threads {
		switch thread.Status {
		case model.ThreadStatusOpen, model.ThreadStatusActive:
			out = append(out, thread)
		}
	}
	return nonNilClone(out)
}

// TruncateRunes clips s to at most n runes, appending "…" when truncated.
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "…"
		}
		count++
	}
	return s
}

func publicWorldMemories(memories []model.MemoryRecord) []model.MemoryRecord {
	out := make([]model.MemoryRecord, 0, len(memories))
	for _, memory := range memories {
		if memory.Owner.Kind == model.MemoryOwnerKindWorld && isPublicWorldTruthStatus(memory.TruthStatus) {
			out = append(out, memory)
		}
	}
	return memoriesNonNil(out)
}
