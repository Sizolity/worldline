package store

import (
	"bytes"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func archiveTestWorld() model.World {
	return model.World{
		ID: "export_test", Name: "Export World",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
			"loc_1":  {ID: "loc_1", Type: "location", Name: "Market"},
		},
		Relations: []model.Relation{
			{ID: "rel_1", Type: "ally", SourceID: "char_a", TargetID: "loc_1"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "char_a", Predicate: "alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}},
		},
		Memories: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "hello"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
		},
		Clock: model.WorldClock{Sequence: 5},
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	w := archiveTestWorld()

	var buf bytes.Buffer
	if err := ExportWorld(w, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	got, err := ImportWorld(&buf, "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if string(got.ID) != "export_test" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Name != "Export World" {
		t.Errorf("Name = %q", got.Name)
	}
	if len(got.Entities) != 2 {
		t.Errorf("entities = %d, want 2", len(got.Entities))
	}
	if len(got.Relations) != 1 {
		t.Errorf("relations = %d", len(got.Relations))
	}
	if len(got.Facts) != 1 {
		t.Errorf("facts = %d", len(got.Facts))
	}
	if len(got.EventLog) != 1 {
		t.Errorf("events = %d", len(got.EventLog))
	}
	if len(got.Memories) != 1 {
		t.Errorf("memories = %d", len(got.Memories))
	}
	if len(got.Threads) != 1 {
		t.Errorf("threads = %d", len(got.Threads))
	}
	if got.Clock.Sequence != 5 {
		t.Errorf("clock = %d", got.Clock.Sequence)
	}
}

func TestImportWithNewID(t *testing.T) {
	t.Parallel()
	w := archiveTestWorld()

	var buf bytes.Buffer
	if err := ExportWorld(w, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	got, err := ImportWorld(&buf, "renamed_world")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if string(got.ID) != "renamed_world" {
		t.Errorf("ID = %q, want renamed_world", got.ID)
	}
}

func TestExportPreservesEntityData(t *testing.T) {
	t.Parallel()
	w := archiveTestWorld()

	var buf bytes.Buffer
	if err := ExportWorld(w, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	got, err := ImportWorld(&buf, "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	alice, ok := got.Entities["char_a"]
	if !ok {
		t.Fatal("missing char_a")
	}
	if alice.Name != "Alice" || alice.Type != "character" {
		t.Errorf("alice = %+v", alice)
	}
}

func TestExportEmptyCollections(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "empty_world", Name: "Empty"}
	w.Entities = map[model.EntityID]model.Entity{}

	var buf bytes.Buffer
	if err := ExportWorld(w, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	got, err := ImportWorld(&buf, "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(got.Relations) != 0 {
		t.Errorf("relations = %d", len(got.Relations))
	}
	if len(got.Facts) != 0 {
		t.Errorf("facts = %d", len(got.Facts))
	}
}

func TestImportInvalidArchive(t *testing.T) {
	t.Parallel()
	_, err := ImportWorld(bytes.NewReader([]byte("not a gzip")), "")
	if err == nil {
		t.Fatal("expected error for invalid archive")
	}
}

func TestExportImportViaFileStore(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := t.Context()
	fs := NewFileStore(workspace)

	w := archiveTestWorld()
	if err := fs.SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := ExportToFileStore(ctx, fs, "export_test", &buf); err != nil {
		t.Fatalf("ExportToFileStore: %v", err)
	}

	imported, err := ImportToFileStore(ctx, fs, &buf, "imported_copy")
	if err != nil {
		t.Fatalf("ImportToFileStore: %v", err)
	}
	if string(imported.ID) != "imported_copy" {
		t.Errorf("ID = %q", imported.ID)
	}

	loaded, err := fs.LoadSnapshot(ctx, "imported_copy")
	if err != nil {
		t.Fatalf("load imported: %v", err)
	}
	if len(loaded.Entities) != 2 {
		t.Errorf("loaded entities = %d", len(loaded.Entities))
	}
}
