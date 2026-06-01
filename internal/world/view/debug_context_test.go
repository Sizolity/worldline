package view

import (
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestWorldDebugViewEmptyWorldReturnsNonNilSlices(t *testing.T) {
	w := model.World{ID: "test_world", Name: "Empty"}
	got := WorldDebugView{}.Render(w)

	if got.World.ID != "test_world" {
		t.Fatalf("world id = %q, want test_world", got.World.ID)
	}
	if got.Entities == nil {
		t.Fatal("Entities is nil, want non-nil empty slice")
	}
	if got.Facts == nil {
		t.Fatal("Facts is nil, want non-nil empty slice")
	}
	if got.Relations == nil {
		t.Fatal("Relations is nil, want non-nil empty slice")
	}
	if got.Memories == nil {
		t.Fatal("Memories is nil, want non-nil empty slice")
	}
	if got.Threads == nil {
		t.Fatal("Threads is nil, want non-nil empty slice")
	}
	if got.EventLog == nil {
		t.Fatal("EventLog is nil, want non-nil empty slice")
	}
	if got.Rules == nil {
		t.Fatal("Rules is nil, want non-nil empty slice")
	}
	if got.EventQueue == nil {
		t.Fatal("EventQueue is nil, want non-nil empty slice")
	}
}

func TestWorldDebugViewIncludesAllCollections(t *testing.T) {
	w := populatedWorld()
	got := WorldDebugView{}.Render(w)

	if len(got.Entities) != 2 {
		t.Fatalf("Entities count = %d, want 2", len(got.Entities))
	}
	if len(got.Facts) != 1 {
		t.Fatalf("Facts count = %d, want 1", len(got.Facts))
	}
	if len(got.Relations) != 1 {
		t.Fatalf("Relations count = %d, want 1", len(got.Relations))
	}
	if len(got.Memories) != 3 {
		t.Fatalf("Memories count = %d, want 3", len(got.Memories))
	}
	if len(got.Threads) != 1 {
		t.Fatalf("Threads count = %d, want 1", len(got.Threads))
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("EventLog count = %d, want 1", len(got.EventLog))
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules count = %d, want 1", len(got.Rules))
	}
	if len(got.EventQueue) != 1 {
		t.Fatalf("EventQueue count = %d, want 1", len(got.EventQueue))
	}
}

func TestWorldDebugViewEntitiesSortedByID(t *testing.T) {
	w := populatedWorld()
	got := WorldDebugView{}.Render(w)

	if got.Entities[0].ID != "char_alice" {
		t.Fatalf("first entity = %q, want char_alice", got.Entities[0].ID)
	}
	if got.Entities[1].ID != "char_bob" {
		t.Fatalf("second entity = %q, want char_bob", got.Entities[1].ID)
	}
}

func TestWorldDebugViewIncludesSecretAndPrivateMemories(t *testing.T) {
	w := populatedWorld()
	got := WorldDebugView{}.Render(w)

	hasSecret := false
	hasPrivate := false
	for _, m := range got.Memories {
		if m.TruthStatus == model.TruthStatusSecret {
			hasSecret = true
		}
		if m.Owner.Kind == model.MemoryOwnerKindCharacter {
			hasPrivate = true
		}
	}
	if !hasSecret {
		t.Fatal("debug view should include secret memories")
	}
	if !hasPrivate {
		t.Fatal("debug view should include private character memories")
	}
}

func TestWorldDebugViewCapturesWorldSummary(t *testing.T) {
	w := populatedWorld()
	got := WorldDebugView{}.Render(w)

	if got.World.Name != "Testland" {
		t.Fatalf("summary name = %q, want Testland", got.World.Name)
	}
	if got.World.Description != "A test world" {
		t.Fatalf("summary description = %q", got.World.Description)
	}
	if got.World.Clock.Sequence != 42 {
		t.Fatalf("summary clock sequence = %d, want 42", got.World.Clock.Sequence)
	}
}

func TestWorldDebugViewDoesNotAliasMutableWorldState(t *testing.T) {
	w := populatedWorld()
	w.Canon.Genre = []string{"mystery"}
	w.Clock.Current.Calendar = map[string]int{"year": 1}
	w.Metadata.Tags = []string{"debug"}
	w.Entities["char_alice"] = model.Entity{
		ID: "char_alice", Type: "character", Name: "Alice",
		Components: map[string]any{
			"profile": map[string]any{"title": "knight"},
		},
		State: map[string]model.Value{
			"alive": {Kind: model.ValueKindBoolean, Raw: true},
			"nested": {Kind: model.ValueKindObject, Raw: map[string]any{
				"score": float64(1),
			}},
		},
		Tags: []string{"hero"},
	}
	w.Memory[0].SubjectIDs = []model.EntityID{"char_alice"}
	w.Memory[0].EventIDs = []model.EventID{"evt_1"}
	w.EventLog[0].ActorIDs = []model.EntityID{"char_alice"}
	w.EventLog[0].TargetIDs = []model.EntityID{"char_bob"}
	w.EventLog[0].Effects = []model.Effect{{
		Kind:     model.EffectUpdateEntityState,
		TargetID: "char_alice",
		Payload: map[string]model.Value{
			"mood": {Kind: model.ValueKindString, Raw: "calm"},
		},
	}}

	got := WorldDebugView{}.Render(w)
	got.World.Canon.Genre[0] = "changed"
	got.World.Clock.Current.Calendar["year"] = 999
	got.World.Metadata.Tags[0] = "changed"
	got.Entities[0].Tags[0] = "changed"
	got.Entities[0].State["alive"] = model.Value{Kind: model.ValueKindBoolean, Raw: false}
	got.Entities[0].State["nested"].Raw.(map[string]any)["score"] = float64(99)
	got.Entities[0].Components["profile"].(map[string]any)["title"] = "changed"
	got.Memories[0].SubjectIDs[0] = "char_bob"
	got.Memories[0].EventIDs[0] = "evt_2"
	got.EventLog[0].ActorIDs[0] = "char_bob"
	got.EventLog[0].TargetIDs[0] = "char_alice"
	got.EventLog[0].Effects[0].Payload["mood"] = model.Value{Kind: model.ValueKindString, Raw: "angry"}

	if w.Canon.Genre[0] != "mystery" {
		t.Fatalf("world canon was mutated: %#v", w.Canon.Genre)
	}
	if w.Clock.Current.Calendar["year"] != 1 {
		t.Fatalf("world clock calendar was mutated: %#v", w.Clock.Current.Calendar)
	}
	if w.Metadata.Tags[0] != "debug" {
		t.Fatalf("world metadata tags were mutated: %#v", w.Metadata.Tags)
	}
	if w.Entities["char_alice"].Tags[0] != "hero" {
		t.Fatalf("entity tags were mutated: %#v", w.Entities["char_alice"].Tags)
	}
	if w.Entities["char_alice"].State["alive"].Raw != true {
		t.Fatalf("entity state was mutated: %#v", w.Entities["char_alice"].State)
	}
	if w.Entities["char_alice"].State["nested"].Raw.(map[string]any)["score"] != float64(1) {
		t.Fatalf("entity nested state was mutated: %#v", w.Entities["char_alice"].State["nested"])
	}
	if w.Entities["char_alice"].Components["profile"].(map[string]any)["title"] != "knight" {
		t.Fatalf("entity components were mutated: %#v", w.Entities["char_alice"].Components)
	}
	if w.Memory[0].SubjectIDs[0] != "char_alice" || w.Memory[0].EventIDs[0] != "evt_1" {
		t.Fatalf("memory references were mutated: %#v", w.Memory[0])
	}
	if w.EventLog[0].ActorIDs[0] != "char_alice" || w.EventLog[0].TargetIDs[0] != "char_bob" {
		t.Fatalf("event references were mutated: %#v", w.EventLog[0])
	}
	if w.EventLog[0].Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("event payload was mutated: %#v", w.EventLog[0].Effects[0].Payload)
	}
}

func populatedWorld() model.World {
	return model.World{
		ID:          "test_world",
		Name:        "Testland",
		Description: "A test world",
		Clock:       model.WorldClock{Sequence: 42},
		Entities: map[model.EntityID]model.Entity{
			"char_bob":   {ID: "char_bob", Type: "character", Name: "Bob"},
			"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
		},
		Facts: []model.Fact{
			{ID: "fact_1", SubjectID: "char_alice", Predicate: "is_alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}},
		},
		Relations: []model.Relation{
			{ID: "rel_1", Type: "ally", SourceID: "char_alice", TargetID: "char_bob"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "mem_world_secret",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The dragon sleeps under the castle.",
				TruthStatus: model.TruthStatusSecret,
				Confidence:  1.0,
				Importance:  1.0,
			},
			{
				ID:          "mem_alice_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "Bob is hiding something.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.6,
				Importance:  0.5,
			},
			{
				ID:          "mem_world_public",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The king declared war.",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.9,
			},
		},
		Threads: []model.WorldThread{
			{ID: "thread_1", Kind: model.ThreadKindMystery, Title: "Who poisoned the well?", Status: model.ThreadStatusOpen},
		},
		Rules: []model.Rule{
			{ID: "rule_1", Kind: "system", Enabled: true},
		},
		EventLog: []model.WorldEvent{
			{ID: "evt_1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Session start."},
		},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "evt_queued", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Queued."}},
		},
	}
}
