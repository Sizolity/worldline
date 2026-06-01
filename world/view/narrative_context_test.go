package view

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestNarrativeViewEmptyWorldReturnsNonNilSlices(t *testing.T) {
	t.Parallel()

	got := NarrativeView{}.Render(model.World{ID: "test_world", Name: "Empty"}, NarrativeContextRequest{})

	if got.World.ID != "test_world" {
		t.Fatalf("world id = %q, want test_world", got.World.ID)
	}
	if got.RecentEvents == nil {
		t.Fatal("RecentEvents is nil, want non-nil empty slice")
	}
	if got.ActiveThreads == nil {
		t.Fatal("ActiveThreads is nil, want non-nil empty slice")
	}
	if got.Facts == nil {
		t.Fatal("Facts is nil, want non-nil empty slice")
	}
	if got.PublicMemories == nil {
		t.Fatal("PublicMemories is nil, want non-nil empty slice")
	}
}

func TestNarrativeViewLimitsRecentEventsToNewestEvents(t *testing.T) {
	t.Parallel()

	world := narrativeWorld()
	got := NarrativeView{}.Render(world, NarrativeContextRequest{RecentEventLimit: 2})

	if len(got.RecentEvents) != 2 {
		t.Fatalf("RecentEvents count = %d, want 2: %#v", len(got.RecentEvents), got.RecentEvents)
	}
	if got.RecentEvents[0].ID != "event_2" || got.RecentEvents[1].ID != "event_3" {
		t.Fatalf("RecentEvents = %#v, want event_2 then event_3", got.RecentEvents)
	}
}

func TestNarrativeViewIncludesOnlyOpenAndActiveThreads(t *testing.T) {
	t.Parallel()

	got := NarrativeView{}.Render(narrativeWorld(), NarrativeContextRequest{})

	if len(got.ActiveThreads) != 2 {
		t.Fatalf("ActiveThreads count = %d, want 2: %#v", len(got.ActiveThreads), got.ActiveThreads)
	}
	if got.ActiveThreads[0].ID != "thread_open" || got.ActiveThreads[1].ID != "thread_active" {
		t.Fatalf("ActiveThreads = %#v, want open then active", got.ActiveThreads)
	}
}

func TestNarrativeViewIncludesOnlyPublicWorldMemories(t *testing.T) {
	t.Parallel()

	got := NarrativeView{}.Render(narrativeWorld(), NarrativeContextRequest{})

	if len(got.PublicMemories) != 1 {
		t.Fatalf("PublicMemories count = %d, want 1: %#v", len(got.PublicMemories), got.PublicMemories)
	}
	if got.PublicMemories[0].ID != "memory_world_public" {
		t.Fatalf("PublicMemories = %#v, want memory_world_public", got.PublicMemories)
	}
}

func TestNarrativeViewDoesNotAliasMutableWorldState(t *testing.T) {
	t.Parallel()

	world := narrativeWorld()
	world.Canon.Genre = []string{"mystery"}
	world.Metadata.Tags = []string{"narrative"}
	world.EventLog[0].ActorIDs = []model.EntityID{"char_a"}
	world.EventLog[0].Effects = []model.Effect{{
		Kind:     model.EffectUpdateEntityState,
		TargetID: "char_a",
		Payload: map[string]model.Value{
			"mood": {Kind: model.ValueKindString, Raw: "calm"},
		},
	}}
	world.Memory[0].SubjectIDs = []model.EntityID{"char_a"}

	got := NarrativeView{}.Render(world, NarrativeContextRequest{})
	got.World.Canon.Genre[0] = "changed"
	got.World.Metadata.Tags[0] = "changed"
	got.RecentEvents[0].ActorIDs[0] = "char_b"
	got.RecentEvents[0].Effects[0].Payload["mood"] = model.Value{Kind: model.ValueKindString, Raw: "angry"}
	got.PublicMemories[0].SubjectIDs[0] = "char_b"

	if world.Canon.Genre[0] != "mystery" {
		t.Fatalf("world canon was mutated: %#v", world.Canon.Genre)
	}
	if world.Metadata.Tags[0] != "narrative" {
		t.Fatalf("world metadata was mutated: %#v", world.Metadata.Tags)
	}
	if world.EventLog[0].ActorIDs[0] != "char_a" {
		t.Fatalf("event actors were mutated: %#v", world.EventLog[0].ActorIDs)
	}
	if world.EventLog[0].Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("event payload was mutated: %#v", world.EventLog[0].Effects[0].Payload)
	}
	if world.Memory[0].SubjectIDs[0] != "char_a" {
		t.Fatalf("memory subjects were mutated: %#v", world.Memory[0].SubjectIDs)
	}
}

func narrativeWorld() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Narrative World",
		Facts: []model.Fact{
			{ID: "fact_1", SubjectID: "char_a", Predicate: "location", Value: model.Value{Kind: model.ValueKindString, Raw: "tower"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_open", Kind: model.ThreadKindMystery, Title: "Open thread", Status: model.ThreadStatusOpen},
			{ID: "thread_active", Kind: model.ThreadKindConflict, Title: "Active thread", Status: model.ThreadStatusActive},
			{ID: "thread_resolved", Kind: model.ThreadKindQuest, Title: "Resolved thread", Status: model.ThreadStatusResolved},
		},
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "event_3", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_public",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The tower bell rang.",
				TruthStatus: model.TruthStatusTrue,
			},
			{
				ID:          "memory_world_secret",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The assassin escaped.",
				TruthStatus: model.TruthStatusSecret,
			},
			{
				ID:          "memory_character_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "I am being followed.",
				TruthStatus: model.TruthStatusUnknown,
			},
		},
	}
}
