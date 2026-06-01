package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func inspectTestWorld() model.World {
	return model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {
				ID: "alice", Type: "character", Name: "Alice",
				Description: "A brave adventurer.",
				Tags:        []string{"brave", "curious"},
				Components: map[string]any{
					model.ComponentActor: model.NewActorComponent(true, []string{"find the truth", "survive"}),
				},
				State: map[string]model.Value{
					"health": {Kind: "number", Raw: 100},
					"mood":   {Kind: "string", Raw: "determined"},
				},
			},
			"bob": {
				ID: "bob", Type: "character", Name: "Bob",
				Components: map[string]any{
					model.ComponentSpatial: model.NewSpatialComponent("tavern"),
				},
			},
			"tavern": {
				ID: "tavern", Type: "location", Name: "The Tavern",
				Description: "A cozy place.",
			},
			"sword": {
				ID: "sword", Type: "item", Name: "Magic Sword",
			},
		},
		Relations: []model.Relation{
			{ID: "r1", Type: "ally", SourceID: "alice", TargetID: "bob"},
			{ID: "r2", Type: "enemy", SourceID: "bob", TargetID: "alice"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "alice", Predicate: "status", Value: model.Value{Kind: "string", Raw: "alive"}},
		},
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "alice"}, Kind: model.MemoryKindObservation, Content: "Saw the shadow move.", TruthStatus: model.TruthStatusUnknown},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Kind: model.MemoryKindObservation, Content: "World memory.", SubjectIDs: []model.EntityID{"alice"}, TruthStatus: model.TruthStatusTrue},
		},
	}
}

func TestFormatEntityHeader(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.HasPrefix(out, "# Alice\n") {
		t.Errorf("missing title:\n%s", out[:60])
	}
	if !strings.Contains(out, "Type: character") {
		t.Errorf("missing type")
	}
	if !strings.Contains(out, "A brave adventurer.") {
		t.Errorf("missing description")
	}
	if !strings.Contains(out, "Tags: brave, curious") {
		t.Errorf("missing tags")
	}
}

func TestFormatEntityActorComponent(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.Contains(out, "### Actor") {
		t.Errorf("missing actor section")
	}
	if !strings.Contains(out, "Can act: true") {
		t.Errorf("missing can_act")
	}
	if !strings.Contains(out, "- find the truth") {
		t.Errorf("missing goal")
	}
}

func TestFormatEntitySpatialComponent(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["bob"], w)
	if !strings.Contains(out, "### Spatial") {
		t.Errorf("missing spatial section")
	}
	if !strings.Contains(out, "Location: tavern") {
		t.Errorf("missing location")
	}
}

func TestFormatEntityState(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.Contains(out, "## State (2)") {
		t.Errorf("missing state section")
	}
	if !strings.Contains(out, "health = 100") {
		t.Errorf("missing health")
	}
	if !strings.Contains(out, "mood = determined") {
		t.Errorf("missing mood")
	}
}

func TestFormatEntityRelations(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.Contains(out, "## Relations (2)") {
		t.Errorf("missing relations section")
	}
	if !strings.Contains(out, "→ Bob [ally]") {
		t.Errorf("missing outgoing relation")
	}
	if !strings.Contains(out, "← Bob [enemy]") {
		t.Errorf("missing incoming relation")
	}
}

func TestFormatEntityFacts(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.Contains(out, "## Facts (1)") {
		t.Errorf("missing facts section")
	}
	if !strings.Contains(out, "status = alive") {
		t.Errorf("missing fact")
	}
}

func TestFormatEntityMemories(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["alice"], w)
	if !strings.Contains(out, "## Memories (2)") {
		t.Errorf("missing memories section:\n%s", out)
	}
	if !strings.Contains(out, "Saw the shadow move.") {
		t.Errorf("missing owned memory")
	}
	if !strings.Contains(out, "World memory.") {
		t.Errorf("missing subject-referenced memory")
	}
}

func TestFormatEntityNoComponents(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntity(w.Entities["tavern"], w)
	if strings.Contains(out, "## Components") {
		t.Errorf("entity without components should omit section")
	}
}

func TestFormatEntityListGroupsByType(t *testing.T) {
	t.Parallel()
	w := inspectTestWorld()
	out := FormatEntityList(w)
	if !strings.Contains(out, "character (2):") {
		t.Errorf("missing character group:\n%s", out)
	}
	if !strings.Contains(out, "location (1):") {
		t.Errorf("missing location group:\n%s", out)
	}
	if !strings.Contains(out, "item (1):") {
		t.Errorf("missing item group:\n%s", out)
	}
}

func TestFormatEntityListEmpty(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "e", Name: "E", Entities: map[model.EntityID]model.Entity{}}
	out := FormatEntityList(w)
	if out != "no entities\n" {
		t.Errorf("got %q", out)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	if got := truncate("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := truncate("this is a very long string", 10); got != "this is..." {
		t.Errorf("got %q", got)
	}
}
