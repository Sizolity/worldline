package runtime

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestDiffWorldsIdentical(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
		Facts:    []model.Fact{{ID: "f1"}},
		EventLog: []model.WorldEvent{{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}

	d := DiffWorlds(w, w)
	if len(d.Entities.Added) != 0 || len(d.Entities.Removed) != 0 || len(d.Entities.Changed) != 0 {
		t.Fatalf("identical worlds should have no entity diff: %+v", d.Entities)
	}
	if len(d.Facts.Added) != 0 || len(d.Facts.Removed) != 0 {
		t.Fatalf("identical worlds should have no fact diff: %+v", d.Facts)
	}
	if len(d.Events.Added) != 0 || len(d.Events.Removed) != 0 {
		t.Fatalf("identical worlds should have no event diff: %+v", d.Events)
	}
}

func TestDiffWorldsEntityChanges(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "character", Name: "Bob"},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice the Brave"},
			"e3": {ID: "e3", Type: "location", Name: "Market"},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Entities.Added) != 1 || d.Entities.Added[0] != "e3" {
		t.Errorf("added = %v, want [e3]", d.Entities.Added)
	}
	if len(d.Entities.Removed) != 1 || d.Entities.Removed[0] != "e2" {
		t.Errorf("removed = %v, want [e2]", d.Entities.Removed)
	}
	if len(d.Entities.Changed) != 1 || d.Entities.Changed[0].ID != "e1" {
		t.Errorf("changed = %v, want [e1]", d.Entities.Changed)
	}
	if len(d.Entities.Changed[0].Fields) == 0 {
		t.Error("expected field-level deltas for changed entity")
	}
}

func TestDiffWorldsThreadStatusChange(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Mystery", Status: model.ThreadStatusOpen},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusResolved},
			{ID: "t3", Kind: model.ThreadKindConflict, Title: "War", Status: model.ThreadStatusActive},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Threads.Added) != 1 || d.Threads.Added[0] != "t3" {
		t.Errorf("threads added = %v, want [t3]", d.Threads.Added)
	}
	if len(d.Threads.Removed) != 1 || d.Threads.Removed[0] != "t2" {
		t.Errorf("threads removed = %v, want [t2]", d.Threads.Removed)
	}
	if len(d.Threads.StatusChanged) != 1 {
		t.Fatalf("status_changed = %d, want 1", len(d.Threads.StatusChanged))
	}
	sc := d.Threads.StatusChanged[0]
	if sc.ID != "t1" || sc.StatusA != model.ThreadStatusActive || sc.StatusB != model.ThreadStatusResolved {
		t.Errorf("status change = %+v", sc)
	}
}

func TestDiffWorldsMemoriesAndEvents(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Memory: []model.MemoryRecord{
			{ID: "m1"}, {ID: "m2"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Memory: []model.MemoryRecord{
			{ID: "m2"}, {ID: "m3"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
			{ID: "ev2", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Memories.Added) != 1 || d.Memories.Added[0] != "m3" {
		t.Errorf("memories added = %v, want [m3]", d.Memories.Added)
	}
	if len(d.Memories.Removed) != 1 || d.Memories.Removed[0] != "m1" {
		t.Errorf("memories removed = %v, want [m1]", d.Memories.Removed)
	}
	if len(d.Events.Added) != 1 || d.Events.Added[0] != "ev2" {
		t.Errorf("events added = %v, want [ev2]", d.Events.Added)
	}
	if len(d.Events.Removed) != 0 {
		t.Errorf("events removed = %v, want []", d.Events.Removed)
	}
}

func TestDiffWorldsClockSequence(t *testing.T) {
	t.Parallel()

	a := model.World{ID: "a", Name: "A", Clock: model.WorldClock{Sequence: 5}}
	b := model.World{ID: "b", Name: "B", Clock: model.WorldClock{Sequence: 12}}

	d := DiffWorlds(a, b)
	if d.ClockA != 5 {
		t.Errorf("clock_a = %d, want 5", d.ClockA)
	}
	if d.ClockB != 12 {
		t.Errorf("clock_b = %d, want 12", d.ClockB)
	}
}

func TestDiffWorldsEmptyCollections(t *testing.T) {
	t.Parallel()

	a := model.World{ID: "a", Name: "A"}
	b := model.World{ID: "b", Name: "B"}

	d := DiffWorlds(a, b)
	if d.Entities.Added == nil || d.Entities.Removed == nil || d.Entities.Changed == nil {
		t.Fatal("entity diff slices should be non-nil")
	}
	if d.Facts.Added == nil || d.Facts.Removed == nil {
		t.Fatal("facts diff slices should be non-nil")
	}
	if d.Threads.Added == nil || d.Threads.Removed == nil || d.Threads.StatusChanged == nil {
		t.Fatal("threads diff slices should be non-nil")
	}
}

func TestDiffWorldsEntityFieldDeltas(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice", Description: "A girl", Tags: []string{"brave"}},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice", Description: "A warrior", Tags: []string{"brave", "strong"}},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Entities.Changed) != 1 {
		t.Fatalf("changed = %d, want 1", len(d.Entities.Changed))
	}
	ic := d.Entities.Changed[0]
	if ic.ID != "e1" {
		t.Errorf("id = %q, want e1", ic.ID)
	}
	fieldNames := make(map[string]bool)
	for _, fd := range ic.Fields {
		fieldNames[fd.Field] = true
	}
	if !fieldNames["description"] {
		t.Error("expected description field delta")
	}
	if !fieldNames["tags"] {
		t.Error("expected tags field delta")
	}
	if fieldNames["name"] || fieldNames["type"] {
		t.Error("name/type did not change, should not appear")
	}
}

func TestDiffWorldsFactChanges(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "e1", Predicate: "age", Value: model.Value{Kind: "number", Raw: 25}},
			{ID: "f2", SubjectID: "e1", Predicate: "title"},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "e1", Predicate: "age", Value: model.Value{Kind: "number", Raw: 30}},
			{ID: "f3", SubjectID: "e2", Predicate: "rank"},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Facts.Added) != 1 || d.Facts.Added[0] != "f3" {
		t.Errorf("facts added = %v, want [f3]", d.Facts.Added)
	}
	if len(d.Facts.Removed) != 1 || d.Facts.Removed[0] != "f2" {
		t.Errorf("facts removed = %v, want [f2]", d.Facts.Removed)
	}
	if len(d.Facts.Changed) != 1 || d.Facts.Changed[0].ID != "f1" {
		t.Errorf("facts changed = %v, want [f1]", d.Facts.Changed)
	}
	found := false
	for _, fd := range d.Facts.Changed[0].Fields {
		if fd.Field == "value" {
			found = true
		}
	}
	if !found {
		t.Error("expected value field delta for f1")
	}
}

func TestDiffWorldsRelationChanges(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Relations: []model.Relation{
			{ID: "r1", Type: "friend", SourceID: "e1", TargetID: "e2"},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Relations: []model.Relation{
			{ID: "r1", Type: "enemy", SourceID: "e1", TargetID: "e2"},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Relations.Changed) != 1 || d.Relations.Changed[0].ID != "r1" {
		t.Fatalf("relations changed = %v, want [r1]", d.Relations.Changed)
	}
	if d.Relations.Changed[0].Fields[0].Old != "friend" || d.Relations.Changed[0].Fields[0].New != "enemy" {
		t.Errorf("field delta = %+v", d.Relations.Changed[0].Fields[0])
	}
}

func TestDiffWorldsMemoryChanges(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Memory: []model.MemoryRecord{
			{ID: "m1", Kind: "observation", Content: "saw fire", TruthStatus: "believed", Importance: 0.5},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Memory: []model.MemoryRecord{
			{ID: "m1", Kind: "observation", Content: "saw dragon fire", TruthStatus: "confirmed", Importance: 0.9},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Memories.Changed) != 1 || d.Memories.Changed[0].ID != "m1" {
		t.Fatalf("memories changed = %v, want [m1]", d.Memories.Changed)
	}
	fieldNames := make(map[string]bool)
	for _, fd := range d.Memories.Changed[0].Fields {
		fieldNames[fd.Field] = true
	}
	if !fieldNames["content"] || !fieldNames["truth_status"] || !fieldNames["importance"] {
		t.Errorf("expected content, truth_status, importance deltas; got fields = %v", d.Memories.Changed[0].Fields)
	}
}

func TestDiffWorldsThreadTitleChange(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: "quest", Title: "Find the gem", Status: "active"},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: "quest", Title: "Retrieve the lost gem", Status: "active"},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Threads.StatusChanged) != 0 {
		t.Errorf("no status change expected, got %v", d.Threads.StatusChanged)
	}
	if len(d.Threads.Changed) != 1 || d.Threads.Changed[0].ID != "t1" {
		t.Fatalf("threads changed = %v, want [t1]", d.Threads.Changed)
	}
	if d.Threads.Changed[0].Fields[0].Field != "title" {
		t.Errorf("field = %q, want title", d.Threads.Changed[0].Fields[0].Field)
	}
}

func TestDiffWorldsRulesAndRelations(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Rules:     []model.Rule{{ID: "r1"}, {ID: "r2"}},
		Relations: []model.Relation{{ID: "rel1"}},
	}
	b := model.World{
		ID: "b", Name: "B",
		Rules:     []model.Rule{{ID: "r2"}, {ID: "r3"}},
		Relations: []model.Relation{{ID: "rel1"}, {ID: "rel2"}},
	}

	d := DiffWorlds(a, b)
	if len(d.Rules.Added) != 1 || d.Rules.Added[0] != "r3" {
		t.Errorf("rules added = %v, want [r3]", d.Rules.Added)
	}
	if len(d.Rules.Removed) != 1 || d.Rules.Removed[0] != "r1" {
		t.Errorf("rules removed = %v, want [r1]", d.Rules.Removed)
	}
	if len(d.Relations.Added) != 1 || d.Relations.Added[0] != "rel2" {
		t.Errorf("relations added = %v, want [rel2]", d.Relations.Added)
	}
}
