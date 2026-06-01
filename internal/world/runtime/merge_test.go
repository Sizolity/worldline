package runtime

import (
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func baseWorld() model.World {
	return model.World{
		ID: "base", Name: "Base",
		Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "character", Name: "Bob"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Mystery", Status: model.ThreadStatusOpen},
		},
		Facts:    []model.Fact{{ID: "f1"}},
		Memory:   []model.MemoryRecord{{ID: "m1"}},
		EventLog: []model.WorldEvent{{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}
}

func TestMergeWorldsNoConflict(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e3"] = model.Entity{ID: "e3", Type: "location", Name: "Market"}
	source.Memory = append(source.Memory, model.MemoryRecord{ID: "m2"})
	source.Clock.Sequence = 8

	target := baseWorld()
	target.ID = "target"
	target.Entities["e4"] = model.Entity{ID: "e4", Type: "character", Name: "Carol"}
	target.Clock.Sequence = 7

	merged, report := MergeWorlds(base, source, target)

	if report.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", report.Conflicts)
	}
	if merged.ID != "target" {
		t.Errorf("merged ID = %q, want target", merged.ID)
	}
	if _, ok := merged.Entities["e3"]; !ok {
		t.Error("missing entity e3 from source")
	}
	if _, ok := merged.Entities["e4"]; !ok {
		t.Error("missing entity e4 from target")
	}
	if len(report.EntitiesAdded) != 1 || report.EntitiesAdded[0] != "e3" {
		t.Errorf("entities_added = %v, want [e3]", report.EntitiesAdded)
	}
	if len(report.MemoriesAdded) != 1 || report.MemoriesAdded[0] != "m2" {
		t.Errorf("memories_added = %v, want [m2]", report.MemoriesAdded)
	}
	if merged.Clock.Sequence != 8 {
		t.Errorf("clock = %d, want 8 (max of source 8, target 7)", merged.Clock.Sequence)
	}
}

func TestMergeWorldsEntityConflict(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Brave"}

	target := baseWorld()
	target.ID = "target"
	target.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Wise"}

	merged, report := MergeWorlds(base, source, target)

	if !report.HasConflicts() {
		t.Fatal("expected conflict for e1")
	}
	if len(report.Conflicts) != 1 || report.Conflicts[0].ID != "e1" {
		t.Errorf("conflicts = %+v", report.Conflicts)
	}
	if merged.Entities["e1"].Name != "Alice the Wise" {
		t.Errorf("conflict should keep target version, got %q", merged.Entities["e1"].Name)
	}
}

func TestMergeWorldsEntityRemovedInSourceModifiedInTarget(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	delete(source.Entities, "e2")

	target := baseWorld()
	target.ID = "target"
	target.Entities["e2"] = model.Entity{ID: "e2", Type: "character", Name: "Bob the Bold"}

	merged, report := MergeWorlds(base, source, target)

	if !report.HasConflicts() {
		t.Fatal("expected conflict for e2")
	}
	if merged.Entities["e2"].Name != "Bob the Bold" {
		t.Error("should keep target version when remove conflicts with modify")
	}
}

func TestMergeWorldsEntityRemovedCleanly(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	delete(source.Entities, "e2")

	target := baseWorld()
	target.ID = "target"

	merged, report := MergeWorlds(base, source, target)

	if report.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", report.Conflicts)
	}
	if _, ok := merged.Entities["e2"]; ok {
		t.Error("e2 should have been removed")
	}
	if len(report.EntitiesRemoved) != 1 || report.EntitiesRemoved[0] != "e2" {
		t.Errorf("entities_removed = %v, want [e2]", report.EntitiesRemoved)
	}
}

func TestMergeWorldsThreadStatusConflict(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Threads[0].Status = model.ThreadStatusResolved

	target := baseWorld()
	target.ID = "target"
	target.Threads[0].Status = model.ThreadStatusDormant

	merged, report := MergeWorlds(base, source, target)

	if !report.HasConflicts() {
		t.Fatal("expected conflict for t1 status")
	}
	if merged.Threads[0].Status != model.ThreadStatusDormant {
		t.Errorf("should keep target status, got %q", merged.Threads[0].Status)
	}
}

func TestMergeWorldsThreadStatusApplied(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Threads[0].Status = model.ThreadStatusResolved

	target := baseWorld()
	target.ID = "target"

	merged, report := MergeWorlds(base, source, target)

	if report.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", report.Conflicts)
	}
	if merged.Threads[0].Status != model.ThreadStatusResolved {
		t.Errorf("thread status = %q, want resolved", merged.Threads[0].Status)
	}
}

func TestMergeWorldsNewThreadFromSource(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Threads = append(source.Threads, model.WorldThread{
		ID: "t3", Kind: model.ThreadKindConflict, Title: "War", Status: model.ThreadStatusActive,
	})

	target := baseWorld()
	target.ID = "target"

	merged, report := MergeWorlds(base, source, target)

	if report.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", report.Conflicts)
	}
	found := false
	for _, th := range merged.Threads {
		if th.ID == "t3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("thread t3 from source should have been merged")
	}
	if len(report.ThreadsAdded) != 1 || report.ThreadsAdded[0] != "t3" {
		t.Errorf("threads_added = %v, want [t3]", report.ThreadsAdded)
	}
}

func TestMergeWorldsPreservesTargetIdentity(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"

	target := baseWorld()
	target.ID = "target"
	target.Name = "Target Branch"
	target.Metadata = model.WorldMetadata{Tags: []string{"branch"}}

	merged, _ := MergeWorlds(base, source, target)

	if merged.ID != "target" {
		t.Errorf("merged ID = %q", merged.ID)
	}
	if merged.Name != "Target Branch" {
		t.Errorf("merged Name = %q", merged.Name)
	}
}

func TestMergeWorldsDoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e3"] = model.Entity{ID: "e3", Type: "location", Name: "Market"}

	target := baseWorld()
	target.ID = "target"

	originalTargetLen := len(target.Entities)
	MergeWorlds(base, source, target)

	if len(target.Entities) != originalTargetLen {
		t.Error("target entities were mutated")
	}
}

func TestMergeWorldsFactsAndEventsAdded(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Facts = append(source.Facts, model.Fact{ID: "f2"})
	source.EventLog = append(source.EventLog, model.WorldEvent{ID: "ev2", Type: model.EventTypeNote, Source: model.EventSourceDirector})

	target := baseWorld()
	target.ID = "target"

	merged, report := MergeWorlds(base, source, target)

	if len(report.FactsAdded) != 1 || report.FactsAdded[0] != "f2" {
		t.Errorf("facts_added = %v, want [f2]", report.FactsAdded)
	}
	if len(report.EventsAdded) != 1 || report.EventsAdded[0] != "ev2" {
		t.Errorf("events_added = %v, want [ev2]", report.EventsAdded)
	}

	factFound := false
	for _, f := range merged.Facts {
		if string(f.ID) == "f2" {
			factFound = true
		}
	}
	if !factFound {
		t.Error("fact f2 not in merged world")
	}
}
