package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func richWorld() model.World {
	return model.World{
		ID:          "w1",
		Name:        "The Fractured Realm",
		Description: "A kingdom torn by betrayal.",
		Canon: model.Canon{
			Genre:      []string{"fantasy", "mystery"},
			Tone:       []string{"dark"},
			Premise:    "A conspiracy threatens the throne.",
			Laws:       []string{"Magic requires sacrifice"},
			Boundaries: []string{"No firearms"},
		},
		Clock: model.WorldClock{Sequence: 42},
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice", Description: "A brave knight."},
			"bob":   {ID: "bob", Type: "character", Name: "Bob"},
			"tower": {ID: "tower", Type: "location", Name: "Dark Tower", Description: "An ominous spire."},
		},
		Relations: []model.Relation{
			{ID: "r1", Type: "ally", SourceID: "alice", TargetID: "bob"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "alice", Predicate: "status", Value: model.Value{Kind: "string", Raw: "alive"}},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindMystery, Title: "Find the traitor", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindQuest, Title: "Old quest", Status: model.ThreadStatusResolved},
		},
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Peace was declared.", TruthStatus: model.TruthStatusTrue},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "alice"}, Content: "Alice saw the shadow.", TruthStatus: model.TruthStatusUnknown},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "The sun set."},
			{ID: "ev2", Type: model.EventTypeMove, Source: model.EventSourceRuntime, Description: "Alice moved."},
			{ID: "ev3", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "A bell tolled."},
		},
	}
}

func TestFormatWorldSummaryHeader(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.HasPrefix(out, "# The Fractured Realm\n") {
		t.Errorf("missing title header:\n%s", out[:80])
	}
	if !strings.Contains(out, "A kingdom torn by betrayal.") {
		t.Errorf("missing description")
	}
	if !strings.Contains(out, "Sequence: 42") {
		t.Errorf("missing sequence")
	}
}

func TestFormatWorldSummaryCanon(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Canon") {
		t.Errorf("missing canon section")
	}
	if !strings.Contains(out, "A conspiracy threatens the throne.") {
		t.Errorf("missing premise")
	}
	if !strings.Contains(out, "Genre: fantasy, mystery") {
		t.Errorf("missing genre")
	}
	if !strings.Contains(out, "Magic requires sacrifice") {
		t.Errorf("missing law")
	}
}

func TestFormatWorldSummaryEntities(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Entities (3)") {
		t.Errorf("missing entities header")
	}
	if !strings.Contains(out, "**Alice**") {
		t.Errorf("missing Alice")
	}
	if !strings.Contains(out, "A brave knight.") {
		t.Errorf("missing Alice description")
	}
	if !strings.Contains(out, "**Dark Tower**") {
		t.Errorf("missing Dark Tower")
	}
}

func TestFormatWorldSummaryRelations(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Relations (1)") {
		t.Errorf("missing relations header")
	}
	if !strings.Contains(out, "Alice → Bob [ally]") {
		t.Errorf("missing relation with resolved names")
	}
}

func TestFormatWorldSummaryFacts(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Facts (1)") {
		t.Errorf("missing facts header")
	}
	if !strings.Contains(out, "Alice.status = alive") {
		t.Errorf("missing fact with resolved name")
	}
}

func TestFormatWorldSummaryThreads(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Threads (2, 1 active)") {
		t.Errorf("missing threads header with active count")
	}
	if !strings.Contains(out, "[active] **Find the traitor**") {
		t.Errorf("missing active thread")
	}
}

func TestFormatWorldSummaryMemories(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Memories (2)") {
		t.Errorf("missing memories header")
	}
	if !strings.Contains(out, "world: 1") {
		t.Errorf("missing world memory count")
	}
	if !strings.Contains(out, "character: 1") {
		t.Errorf("missing character memory count")
	}
}

func TestFormatWorldSummaryEvents(t *testing.T) {
	t.Parallel()
	out := FormatWorldSummary(richWorld())
	if !strings.Contains(out, "## Event Log (3)") {
		t.Errorf("missing event log header")
	}
	if !strings.Contains(out, "note: 2") {
		t.Errorf("missing note count")
	}
	if !strings.Contains(out, "Last event: A bell tolled.") {
		t.Errorf("missing last event")
	}
}

func TestFormatWorldSummaryEmptyWorld(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "empty", Name: "Empty World"}
	out := FormatWorldSummary(w)
	if !strings.Contains(out, "# Empty World") {
		t.Errorf("missing title")
	}
	if strings.Contains(out, "## Canon") {
		t.Errorf("empty canon should be omitted")
	}
	if strings.Contains(out, "## Entities") {
		t.Errorf("empty entities should be omitted")
	}
}

func TestFormatWorldSummaryEventQueue(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "q", Name: "Q",
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}
	out := FormatWorldSummary(w)
	if !strings.Contains(out, "1 pending event(s)") {
		t.Errorf("missing event queue:\n%s", out)
	}
}
