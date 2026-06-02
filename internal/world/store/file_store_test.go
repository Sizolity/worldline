package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestFileStoreSaveLoadWorld(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 1},
		},
	}

	if err := st.SaveWorld(ctx, world); err != nil {
		t.Fatalf("SaveWorld returned error: %v", err)
	}
	got, err := st.LoadWorld(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadWorld returned error: %v", err)
	}
	if got.ID != world.ID || got.Name != world.Name {
		t.Fatalf("loaded world mismatch: got %#v want %#v", got, world)
	}
}

func TestFileStorePersistsWorldArtifacts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	worldID := "test_world"

	entity := model.Entity{ID: "hero", Type: "character", Name: "Hero"}
	relation := model.Relation{ID: "relation_1", Type: "owns", SourceID: "hero", TargetID: "sword"}
	fact := model.Fact{ID: "fact_1", SubjectID: "door", Predicate: "locked", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	if err := st.SaveEntity(ctx, worldID, entity); err != nil {
		t.Fatalf("SaveEntity returned error: %v", err)
	}
	if err := st.SaveRelations(ctx, worldID, []model.Relation{relation}); err != nil {
		t.Fatalf("SaveRelations returned error: %v", err)
	}
	if err := st.SaveFacts(ctx, worldID, []model.Fact{fact}); err != nil {
		t.Fatalf("SaveFacts returned error: %v", err)
	}
	if err := st.AppendEvent(ctx, worldID, event); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	gotEntity, err := st.LoadEntity(ctx, worldID, "hero")
	if err != nil {
		t.Fatalf("LoadEntity returned error: %v", err)
	}
	if gotEntity.Name != entity.Name {
		t.Fatalf("entity mismatch: %#v", gotEntity)
	}

	gotRelations, err := st.LoadRelations(ctx, worldID)
	if err != nil {
		t.Fatalf("LoadRelations returned error: %v", err)
	}
	if len(gotRelations) != 1 || gotRelations[0].ID != relation.ID {
		t.Fatalf("relations mismatch: %#v", gotRelations)
	}

	gotFacts, err := st.LoadFacts(ctx, worldID)
	if err != nil {
		t.Fatalf("LoadFacts returned error: %v", err)
	}
	if len(gotFacts) != 1 || gotFacts[0].ID != fact.ID {
		t.Fatalf("facts mismatch: %#v", gotFacts)
	}

	gotEvents, err := st.ListEvents(ctx, worldID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(gotEvents) != 1 || gotEvents[0].ID != event.ID {
		t.Fatalf("events mismatch: %#v", gotEvents)
	}
}

func TestFileStoreRejectsInvalidEntityComponents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	entity := model.Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			model.ComponentSpatial: map[string]any{"location_id": "../bad"},
		},
	}

	if err := st.SaveEntity(ctx, "test_world", entity); err == nil {
		t.Fatal("SaveEntity returned nil for invalid entity component")
	}
}

func TestFileStorePersistsMemories(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	worldID := "test_world"
	memories := []model.MemoryRecord{{
		ID:          "memory_1",
		Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
		Scope:       model.MemoryScopeSubjective,
		Kind:        model.MemoryKindBelief,
		Content:     "A killed the king.",
		TruthStatus: model.TruthStatusUnknown,
		Confidence:  0.8,
		Importance:  0.7,
	}}

	if err := st.SaveMemories(ctx, worldID, memories); err != nil {
		t.Fatalf("SaveMemories returned error: %v", err)
	}
	got, err := st.LoadMemories(ctx, worldID)
	if err != nil {
		t.Fatalf("LoadMemories returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != memories[0].ID {
		t.Fatalf("memories mismatch: %#v", got)
	}
	if got[0].Owner.Kind != model.MemoryOwnerKindCharacter || got[0].Owner.ID != "char_b" {
		t.Fatalf("memory owner mismatch: %#v", got[0])
	}
}

func TestFileStorePersistsThreads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	worldID := "test_world"
	threads := []model.WorldThread{{
		ID:       "thread_1",
		Kind:     model.ThreadKindMystery,
		Title:    "Find the killer",
		Status:   model.ThreadStatusOpen,
		Priority: 0.8,
		Tension:  0.6,
	}}

	if err := st.SaveThreads(ctx, worldID, threads); err != nil {
		t.Fatalf("SaveThreads returned error: %v", err)
	}
	got, err := st.LoadThreads(ctx, worldID)
	if err != nil {
		t.Fatalf("LoadThreads returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != threads[0].ID {
		t.Fatalf("threads mismatch: %#v", got)
	}
	if got[0].Status != model.ThreadStatusOpen || got[0].Tension != 0.6 {
		t.Fatalf("thread fields mismatch: %#v", got[0])
	}
}

func TestFileStoreSaveLoadSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
			"door": {ID: "door", Type: "door", Name: "Door"},
		},
		Relations: []model.Relation{{
			ID:       "relation_1",
			Type:     "owns",
			SourceID: "hero",
			TargetID: "door",
		}},
		Facts: []model.Fact{{
			ID:        "fact_1",
			SubjectID: "door",
			Predicate: "locked",
			Value:     model.Value{Kind: model.ValueKindBoolean, Raw: true},
		}},
		EventLog: []model.WorldEvent{{
			ID:     "event_1",
			Type:   model.EventTypeNote,
			Source: model.EventSourceTest,
		}},
		EventQueue: []model.EventQueueItem{{
			Event: model.WorldEvent{
				ID:     "event_queued",
				Type:   model.EventTypeNote,
				Source: model.EventSourceRuntime,
			},
			Priority: 5,
		}},
		Memories: []model.MemoryRecord{{
			ID:         "memory_1",
			Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "hero"},
			Content:    "The door is locked.",
			Confidence: 0.8,
			Importance: 0.5,
		}},
		Threads: []model.WorldThread{{
			ID:       "thread_1",
			Kind:     model.ThreadKindMystery,
			Title:    "Open the door",
			Status:   model.ThreadStatusOpen,
			Priority: 0.4,
			Tension:  0.2,
		}},
	}

	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	got, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if got.ID != world.ID || got.Name != world.Name {
		t.Fatalf("world mismatch: %#v", got)
	}
	if len(got.Entities) != 2 || got.Entities["hero"].Name != "Hero" {
		t.Fatalf("entities mismatch: %#v", got.Entities)
	}
	if len(got.Relations) != 1 || got.Relations[0].ID != "relation_1" {
		t.Fatalf("relations mismatch: %#v", got.Relations)
	}
	if len(got.Facts) != 1 || got.Facts[0].ID != "fact_1" {
		t.Fatalf("facts mismatch: %#v", got.Facts)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != "event_1" {
		t.Fatalf("event log mismatch: %#v", got.EventLog)
	}
	if len(got.EventQueue) != 1 || got.EventQueue[0].Event.ID != "event_queued" || got.EventQueue[0].Priority != 5 {
		t.Fatalf("event queue mismatch: %#v", got.EventQueue)
	}
	if len(got.Memories) != 1 || got.Memories[0].ID != "memory_1" {
		t.Fatalf("memory mismatch: %#v", got.Memories)
	}
	if len(got.Threads) != 1 || got.Threads[0].ID != "thread_1" {
		t.Fatalf("threads mismatch: %#v", got.Threads)
	}
}

func TestFileStoreLoadSnapshotAcceptsLegacyBareEventQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspace := t.TempDir()
	st := NewFileStore(workspace)
	worldDir := filepath.Join(workspace, "worlds", "test_world")
	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "world.json"), []byte(`{
		"id": "test_world",
		"name": "Test World",
		"event_queue": [
			{"id": "event_queued", "type": "note", "source": "runtime"}
		]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(got.EventQueue) != 1 || got.EventQueue[0].Event.ID != "event_queued" {
		t.Fatalf("legacy queue mismatch: %#v", got.EventQueue)
	}
}

func TestFileStoreSaveSnapshotRewritesEventLog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		EventLog: []model.WorldEvent{{
			ID:     "event_1",
			Type:   model.EventTypeNote,
			Source: model.EventSourceTest,
		}},
	}

	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("first SaveSnapshot returned error: %v", err)
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("second SaveSnapshot returned error: %v", err)
	}
	got, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event log duplicated: %#v", got.EventLog)
	}
}

func TestFileStoreSaveSnapshotDoesNotReplaceExistingSnapshotWhenValidationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("initial SaveSnapshot returned error: %v", err)
	}

	invalid := model.World{
		ID:   "test_world",
		Name: "Invalid World",
		Entities: map[model.EntityID]model.Entity{
			"villain": {ID: "villain", Type: "character", Name: "Villain"},
		},
		Memories: []model.MemoryRecord{{
			ID:         "memory_1",
			Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Content:    "Invalid memory.",
			Confidence: 1.1,
		}},
	}
	if err := st.SaveSnapshot(ctx, invalid); err == nil {
		t.Fatal("SaveSnapshot returned nil for invalid snapshot")
	}

	got, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if got.Name != "Test World" {
		t.Fatalf("existing world metadata was replaced: %#v", got)
	}
	if _, ok := got.Entities["hero"]; !ok {
		t.Fatalf("existing entity was removed: %#v", got.Entities)
	}
	if _, ok := got.Entities["villain"]; ok {
		t.Fatalf("invalid snapshot entity was persisted: %#v", got.Entities)
	}
}

func TestFileStoreSaveSnapshotRejectsInvalidEntityComponents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentInventory: map[string]any{"item_ids": []any{"key_1", "../bad"}},
				},
			},
		},
	}

	if err := st.SaveSnapshot(ctx, world); err == nil {
		t.Fatal("SaveSnapshot returned nil for invalid entity component")
	}
}

func TestFileStoreListEventsRejectsInvalidEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspace := t.TempDir()
	st := NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	path := filepath.Join(workspace, "worlds", "test_world", "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"event_1","type":"note"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := st.ListEvents(ctx, "test_world"); err == nil {
		t.Fatal("ListEvents returned nil for invalid event")
	}
}

func TestFileStoreSaveSnapshotRejectsInvalidQueuedEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		EventQueue: []model.EventQueueItem{{
			Event: model.WorldEvent{
				ID:   "event_queued",
				Type: model.EventTypeNote,
			},
		}},
	}

	if err := st.SaveSnapshot(ctx, world); err == nil {
		t.Fatal("SaveSnapshot returned nil for invalid queued event")
	}
}

func TestFileStoreSaveAndLoadCheckpoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:    "test_world",
		Name:  "Test World",
		Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
		Facts: []model.Fact{{
			ID:        "fact_1",
			SubjectID: "hero",
			Predicate: "alive",
			Value:     model.Value{Kind: model.ValueKindBoolean, Raw: true},
		}},
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	seq, err := st.SaveCheckpoint(ctx, "test_world")
	if err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}
	if seq != 5 {
		t.Fatalf("SaveCheckpoint seq = %d, want 5", seq)
	}

	got, err := st.LoadCheckpoint(ctx, "test_world", 5)
	if err != nil {
		t.Fatalf("LoadCheckpoint returned error: %v", err)
	}
	if got.Clock.Sequence != 5 {
		t.Fatalf("checkpoint sequence = %d, want 5", got.Clock.Sequence)
	}
	if len(got.Entities) != 1 || got.Entities["hero"].Name != "Hero" {
		t.Fatalf("checkpoint entities mismatch: %#v", got.Entities)
	}
	if len(got.Facts) != 1 || got.Facts[0].ID != "fact_1" {
		t.Fatalf("checkpoint facts mismatch: %#v", got.Facts)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != "event_1" {
		t.Fatalf("checkpoint event log mismatch: %#v", got.EventLog)
	}
}

func TestFileStoreListCheckpoints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{ID: "test_world", Name: "Test World"}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	seqs, err := st.ListCheckpoints(ctx, "test_world")
	if err != nil {
		t.Fatalf("ListCheckpoints returned error: %v", err)
	}
	if len(seqs) != 0 {
		t.Fatalf("expected no checkpoints, got %v", seqs)
	}

	world.Clock.Sequence = 2
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "test_world"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	world.Clock.Sequence = 7
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "test_world"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	seqs, err = st.ListCheckpoints(ctx, "test_world")
	if err != nil {
		t.Fatalf("ListCheckpoints returned error: %v", err)
	}
	if len(seqs) != 2 {
		t.Fatalf("expected 2 checkpoints, got %v", seqs)
	}
}

func TestFileStoreCheckpointIsIndependentOfCurrentSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:    "test_world",
		Name:  "Version A",
		Clock: model.WorldClock{Sequence: 3},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "test_world"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	world.Name = "Version B"
	world.Clock.Sequence = 10
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	cp, err := st.LoadCheckpoint(ctx, "test_world", 3)
	if err != nil {
		t.Fatalf("LoadCheckpoint returned error: %v", err)
	}
	if cp.Name != "Version A" {
		t.Fatalf("checkpoint name = %q, want Version A", cp.Name)
	}

	current, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if current.Name != "Version B" {
		t.Fatalf("current name = %q, want Version B", current.Name)
	}
}

func TestFileStoreForkWorldFromCurrentState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:    "source_world",
		Name:  "Original",
		Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	forked, err := st.ForkWorld(ctx, "source_world", "branch_a", 0)
	if err != nil {
		t.Fatalf("ForkWorld returned error: %v", err)
	}
	if string(forked.ID) != "branch_a" {
		t.Fatalf("forked ID = %q, want branch_a", forked.ID)
	}
	if forked.Name != "Original" {
		t.Fatalf("forked name = %q, want Original", forked.Name)
	}
	if forked.Metadata.Fork == nil {
		t.Fatal("forked world missing ForkInfo")
	}
	if string(forked.Metadata.Fork.ParentWorldID) != "source_world" {
		t.Fatalf("parent = %q", forked.Metadata.Fork.ParentWorldID)
	}
	if forked.Metadata.Fork.ForkSequence != 5 {
		t.Fatalf("fork sequence = %d, want 5", forked.Metadata.Fork.ForkSequence)
	}
	if len(forked.Entities) != 1 || forked.Entities["hero"].Name != "Hero" {
		t.Fatalf("forked entities mismatch: %#v", forked.Entities)
	}
	if len(forked.EventLog) != 1 {
		t.Fatalf("forked event log = %d events, want 1", len(forked.EventLog))
	}

	loaded, err := st.LoadSnapshot(ctx, "branch_a")
	if err != nil {
		t.Fatalf("LoadSnapshot branch_a returned error: %v", err)
	}
	if loaded.Metadata.Fork == nil || string(loaded.Metadata.Fork.ParentWorldID) != "source_world" {
		t.Fatal("persisted fork info lost")
	}
}

func TestFileStoreForkWorldFromCheckpoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:    "source_world",
		Name:  "At Checkpoint",
		Clock: model.WorldClock{Sequence: 3},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "source_world"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	world.Name = "Advanced"
	world.Clock.Sequence = 10
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	forked, err := st.ForkWorld(ctx, "source_world", "branch_b", 3)
	if err != nil {
		t.Fatalf("ForkWorld returned error: %v", err)
	}
	if forked.Name != "At Checkpoint" {
		t.Fatalf("forked name = %q, want 'At Checkpoint'", forked.Name)
	}
	if forked.Clock.Sequence != 3 {
		t.Fatalf("forked clock = %d, want 3", forked.Clock.Sequence)
	}
	if forked.Metadata.Fork.ForkSequence != 3 {
		t.Fatalf("fork sequence = %d, want 3", forked.Metadata.Fork.ForkSequence)
	}

	source, err := st.LoadSnapshot(ctx, "source_world")
	if err != nil {
		t.Fatalf("LoadSnapshot source returned error: %v", err)
	}
	if source.Name != "Advanced" {
		t.Fatal("source world was modified by fork")
	}
}

func TestFileStoreForkWorldRejectsSameID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	if err := st.SaveSnapshot(ctx, model.World{ID: "w", Name: "W"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.ForkWorld(ctx, "w", "w", 0); err == nil {
		t.Fatal("expected error when forking to same ID")
	}
}

func TestFileStoreForkWorldClearsEventQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "source_world",
		Name: "With Queue",
		EventQueue: []model.EventQueueItem{{
			Event: model.WorldEvent{ID: "queued_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime},
		}},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	forked, err := st.ForkWorld(ctx, "source_world", "branch_c", 0)
	if err != nil {
		t.Fatalf("ForkWorld returned error: %v", err)
	}
	if len(forked.EventQueue) != 0 {
		t.Fatalf("forked world should have empty event queue, got %d", len(forked.EventQueue))
	}
}

func TestFileStoreLoadSnapshotRejectsInvalidMemoriesAndThreads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspace := t.TempDir()
	st := NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	worldDir := filepath.Join(workspace, "worlds", "test_world")
	if err := os.WriteFile(filepath.Join(worldDir, "memories.json"), []byte(`[{"id":"memory_1","owner":{"kind":"world"},"content":"bad","truth_status":"hidden"}]`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := st.LoadSnapshot(ctx, "test_world"); err == nil {
		t.Fatal("LoadSnapshot returned nil for invalid memory")
	}

	if err := os.WriteFile(filepath.Join(worldDir, "memories.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "threads.json"), []byte(`[{"id":"thread_1","kind":"mystery","title":"Bad","status":"stuck"}]`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := st.LoadSnapshot(ctx, "test_world"); err == nil {
		t.Fatal("LoadSnapshot returned nil for invalid thread")
	}
}
