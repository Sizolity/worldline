package fog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func testWorld() model.World {
	return model.World{
		ID:   "world-fog-test",
		Name: "Fog Test World",
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Kael",
				Description: "A wandering swordsman.", Tags: []string{"player"},
				State: map[string]model.Value{"hp": {Kind: model.ValueKindNumber, Raw: float64(25)}},
				Components: map[string]any{
					"actor":   map[string]any{"can_act": true, "goals": []any{"survive"}},
					"secrets": map[string]any{"hidden_power": "fire magic"},
				},
			},
			"sage": {ID: "sage", Type: "character", Name: "Mirael",
				Description: "An aging scholar.", Tags: []string{"npc", "wise"}},
			"tower": {ID: "tower", Type: "location", Name: "Shattered Tower",
				Description: "Ruins of an ancient mage's tower."},
			"village": {ID: "village", Type: "location", Name: "Thornhaven",
				Description: "A quiet village at the forest edge."},
		},
		Facts: []model.Fact{
			{ID: "fact_seal", SubjectID: "tower", Predicate: "contains", Value: model.Value{Kind: model.ValueKindString, Raw: "broken seal"}},
			{ID: "fact_origin", SubjectID: "hero", Predicate: "origin", Value: model.Value{Kind: model.ValueKindString, Raw: "unknown"}},
		},
		Relations: []model.Relation{
			{ID: "rel_mentor", Type: "mentor", SourceID: "sage", TargetID: "hero"},
			{ID: "rel_located", Type: "located_in", SourceID: "sage", TargetID: "village"},
		},
		Rules: []model.Rule{
			{ID: "rule-01", Kind: "rpg_narrative", Enabled: true},
		},
		EventLog: []model.WorldEvent{
			{ID: "evt-1", Type: "note", Description: "Session started"},
		},
	}
}

func TestFilterWorld_FullFog(t *testing.T) {
	w := testWorld()
	state := DisclosureState{} // empty = everything hidden

	filtered := FilterWorld(w, state)

	if len(filtered.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(filtered.Entities))
	}
	if len(filtered.Facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(filtered.Facts))
	}
	if len(filtered.Relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(filtered.Relations))
	}
	// Rules bypass fog
	if len(filtered.Rules) != 1 {
		t.Errorf("expected rules to bypass fog, got %d", len(filtered.Rules))
	}
	// EventLog bypasses fog
	if len(filtered.EventLog) != 1 {
		t.Errorf("expected event log to bypass fog, got %d", len(filtered.EventLog))
	}
}

func TestFilterWorld_MixedVisibility(t *testing.T) {
	w := testWorld()
	state := DisclosureState{
		Entities: map[model.EntityID]EntityDisclosure{
			"hero":    {Level: Explored},
			"sage":    {Level: Known},
			"village": {Level: Explored},
			// "tower" not in map → hidden
		},
		Facts: map[model.FactID]bool{
			"fact_origin": true,
			// "fact_seal" not visible (tower is hidden anyway)
		},
		Relations: map[model.RelationID]bool{
			"rel_mentor":  true,
			"rel_located": true,
		},
	}

	filtered := FilterWorld(w, state)

	// hero = explored → full content
	hero, ok := filtered.Entities["hero"]
	if !ok {
		t.Fatal("hero should be visible")
	}
	if hero.Description == "" {
		t.Error("explored entity should have description")
	}
	if hero.State == nil {
		t.Error("explored entity should have state")
	}

	// sage = known → name/type/tags only
	sage, ok := filtered.Entities["sage"]
	if !ok {
		t.Fatal("sage should be visible (known)")
	}
	if sage.Name != "Mirael" {
		t.Errorf("known entity name: got %q", sage.Name)
	}
	if sage.Description != "" {
		t.Error("known entity should NOT have description")
	}

	// tower = hidden → absent
	if _, ok := filtered.Entities["tower"]; ok {
		t.Error("tower should be hidden")
	}

	// facts
	if len(filtered.Facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(filtered.Facts))
	}

	// relations: rel_mentor OK (sage=known, hero=explored), rel_located OK (sage=known, village=explored)
	if len(filtered.Relations) != 2 {
		t.Errorf("expected 2 relations, got %d", len(filtered.Relations))
	}
}

func TestFilterWorld_PieceLocking(t *testing.T) {
	w := testWorld()
	state := DisclosureState{
		Entities: map[model.EntityID]EntityDisclosure{
			"hero": {
				Level:  Explored,
				Pieces: map[string]bool{"actor": true, "secrets": false}, // secrets locked
			},
		},
	}

	filtered := FilterWorld(w, state)
	hero := filtered.Entities["hero"]

	if _, ok := hero.Components["actor"]; !ok {
		t.Error("actor component should be visible")
	}
	if _, ok := hero.Components["secrets"]; ok {
		t.Error("secrets component should be locked/hidden")
	}
}

func TestReveal_EntityLevelProgression(t *testing.T) {
	state := DisclosureState{}

	Reveal(&state, RevealAction{EntityID: "npc-1", ToLevel: Known})
	if state.GetEntityLevel("npc-1") != Known {
		t.Errorf("expected Known, got %s", state.GetEntityLevel("npc-1"))
	}

	Reveal(&state, RevealAction{EntityID: "npc-1", ToLevel: Explored})
	if state.GetEntityLevel("npc-1") != Explored {
		t.Errorf("expected Explored, got %s", state.GetEntityLevel("npc-1"))
	}

	// Cannot downgrade
	Reveal(&state, RevealAction{EntityID: "npc-1", ToLevel: Known})
	if state.GetEntityLevel("npc-1") != Explored {
		t.Error("visibility should not decrease")
	}
}

func TestReveal_Piece(t *testing.T) {
	state := DisclosureState{
		Entities: map[model.EntityID]EntityDisclosure{
			"npc-1": {Level: Explored, Pieces: map[string]bool{"secrets": false}},
		},
	}

	if state.IsPieceVisible("npc-1", "secrets") {
		t.Error("secrets should be locked initially")
	}

	Reveal(&state, RevealAction{EntityID: "npc-1", Piece: "secrets"})
	if !state.IsPieceVisible("npc-1", "secrets") {
		t.Error("secrets should be unlocked after reveal")
	}
}

func TestReveal_FactAndRelation(t *testing.T) {
	state := DisclosureState{}

	Reveal(&state, RevealAction{FactID: "fact-1"})
	if !state.IsFactVisible("fact-1") {
		t.Error("fact should be visible after reveal")
	}

	Reveal(&state, RevealAction{RelationID: "rel-1"})
	if !state.IsRelationVisible("rel-1") {
		t.Error("relation should be visible after reveal")
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	state := DisclosureState{
		Entities: map[model.EntityID]EntityDisclosure{
			"hero": {Level: Explored},
			"npc":  {Level: Known, Pieces: map[string]bool{"secrets": false}},
		},
		Facts: map[model.FactID]bool{"fact-1": true},
	}

	if err := store.Save("world-01", state); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file exists
	p := filepath.Join(dir, "world-01", "disclosure.json")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	loaded, err := store.Load("world-01")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.GetEntityLevel("hero") != Explored {
		t.Error("hero should be explored")
	}
	if loaded.GetEntityLevel("npc") != Known {
		t.Error("npc should be known")
	}
	if loaded.IsPieceVisible("npc", "secrets") {
		t.Error("npc secrets should be locked")
	}
	if !loaded.IsFactVisible("fact-1") {
		t.Error("fact-1 should be visible")
	}
}

func TestStore_LoadMissing(t *testing.T) {
	store := NewStore(t.TempDir())

	state, err := store.Load("nonexistent")
	if err != nil {
		t.Fatalf("load missing should not error: %v", err)
	}
	if len(state.Entities) != 0 {
		t.Error("empty state expected for missing file")
	}
}

func TestStore_Exists(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if store.Exists("world-01") {
		t.Error("should not exist yet")
	}

	_ = store.Save("world-01", DisclosureState{})
	if !store.Exists("world-01") {
		t.Error("should exist after save")
	}
}

func TestFilterWorld_RelationHiddenEndpoint(t *testing.T) {
	w := testWorld()
	state := DisclosureState{
		Entities: map[model.EntityID]EntityDisclosure{
			"sage":    {Level: Known},
			"village": {Level: Explored},
			// hero not visible → rel_mentor should be filtered out
		},
		Relations: map[model.RelationID]bool{
			"rel_mentor":  true,
			"rel_located": true,
		},
	}

	filtered := FilterWorld(w, state)

	// rel_mentor: sage=known but hero=hidden → should be excluded
	for _, r := range filtered.Relations {
		if r.ID == "rel_mentor" {
			t.Error("rel_mentor should be filtered (hero is hidden)")
		}
	}
	// rel_located: sage=known, village=explored → should be included
	found := false
	for _, r := range filtered.Relations {
		if r.ID == "rel_located" {
			found = true
		}
	}
	if !found {
		t.Error("rel_located should be visible")
	}
}
