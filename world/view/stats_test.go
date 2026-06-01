package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func statsTestWorld() model.World {
	return model.World{
		ID: "w1", Name: "TestWorld",
		Clock: model.WorldClock{Sequence: 10, Current: model.WorldTime{Tick: 42}},
		Entities: map[model.EntityID]model.Entity{
			"c1": {ID: "c1", Type: "character", Name: "Alice"},
			"c2": {ID: "c2", Type: "character", Name: "Bob"},
			"l1": {ID: "l1", Type: "location", Name: "Town"},
		},
		Relations: []model.Relation{
			{ID: "r1", Type: "ally", SourceID: "c1", TargetID: "c2"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "c1", Predicate: "age"},
			{ID: "f2", SubjectID: "c2", Predicate: "alive"},
		},
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "x"},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "c1"}, Content: "y"},
			{ID: "m3", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "c1"}, Content: "z"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Q1", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Q2", Status: model.ThreadStatusOpen},
			{ID: "t3", Kind: model.ThreadKindQuest, Title: "Q3", Status: model.ThreadStatusActive},
		},
		EventLog: []model.WorldEvent{
			{ID: "e1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "q2", Type: model.EventTypeMove, Source: model.EventSourceUser}},
		},
	}
}

func TestComputeStatsCounts(t *testing.T) {
	t.Parallel()
	s := ComputeStats(statsTestWorld())

	if s.WorldID != "w1" || s.Name != "TestWorld" {
		t.Errorf("identity: %s / %s", s.WorldID, s.Name)
	}
	if s.Sequence != 10 || s.Tick != 42 {
		t.Errorf("clock: seq=%d tick=%d", s.Sequence, s.Tick)
	}
	if s.EntityCount != 3 {
		t.Errorf("entities = %d", s.EntityCount)
	}
	if s.RelationCount != 1 {
		t.Errorf("relations = %d", s.RelationCount)
	}
	if s.FactCount != 2 {
		t.Errorf("facts = %d", s.FactCount)
	}
	if s.MemoryCount != 3 {
		t.Errorf("memories = %d", s.MemoryCount)
	}
	if s.ThreadCount != 3 {
		t.Errorf("threads = %d", s.ThreadCount)
	}
	if s.EventLogCount != 1 {
		t.Errorf("event_log = %d", s.EventLogCount)
	}
	if s.QueueDepth != 2 {
		t.Errorf("queue = %d", s.QueueDepth)
	}
}

func TestComputeStatsEntitiesByType(t *testing.T) {
	t.Parallel()
	s := ComputeStats(statsTestWorld())

	if s.EntitiesByType["character"] != 2 {
		t.Errorf("characters = %d", s.EntitiesByType["character"])
	}
	if s.EntitiesByType["location"] != 1 {
		t.Errorf("locations = %d", s.EntitiesByType["location"])
	}
}

func TestComputeStatsThreadsByStatus(t *testing.T) {
	t.Parallel()
	s := ComputeStats(statsTestWorld())

	if s.ThreadsByStatus["active"] != 2 {
		t.Errorf("active = %d", s.ThreadsByStatus["active"])
	}
	if s.ThreadsByStatus["open"] != 1 {
		t.Errorf("open = %d", s.ThreadsByStatus["open"])
	}
}

func TestComputeStatsMemoriesByOwner(t *testing.T) {
	t.Parallel()
	s := ComputeStats(statsTestWorld())

	if s.MemoriesByOwner["world"] != 1 {
		t.Errorf("world = %d", s.MemoriesByOwner["world"])
	}
	if s.MemoriesByOwner["character:c1"] != 2 {
		t.Errorf("character:c1 = %d", s.MemoriesByOwner["character:c1"])
	}
}

func TestComputeStatsEmptyWorld(t *testing.T) {
	t.Parallel()
	s := ComputeStats(model.World{ID: "empty", Name: "E", Entities: map[model.EntityID]model.Entity{}})

	if s.EntityCount != 0 || s.MemoryCount != 0 || s.ThreadCount != 0 {
		t.Errorf("expected all zeros: entities=%d mem=%d threads=%d", s.EntityCount, s.MemoryCount, s.ThreadCount)
	}
}

func TestFormatStatsContainsTable(t *testing.T) {
	t.Parallel()
	s := ComputeStats(statsTestWorld())
	text := FormatStats(s)

	if !strings.Contains(text, "# Stats — TestWorld") {
		t.Errorf("missing header:\n%s", text)
	}
	if !strings.Contains(text, "seq=10") || !strings.Contains(text, "tick=42") {
		t.Errorf("missing clock:\n%s", text)
	}
	if !strings.Contains(text, "Entities") || !strings.Contains(text, "3") {
		t.Errorf("missing entity count:\n%s", text)
	}
	if !strings.Contains(text, "character=2") {
		t.Errorf("missing entities by type:\n%s", text)
	}
	if !strings.Contains(text, "active=2") {
		t.Errorf("missing threads by status:\n%s", text)
	}
}
