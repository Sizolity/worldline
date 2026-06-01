package model

import "testing"

func TestEntityValidateRequiresCoreFields(t *testing.T) {
	t.Parallel()

	entity := Entity{Type: "character", Name: "Alice"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	entity = Entity{ID: "char_alice", Name: "Alice"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without Type")
	}

	entity = Entity{ID: "char_alice", Type: "character"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without Name")
	}
}

func TestEntityValidateAcceptsAliases(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:      "char_kael",
		Type:    "character",
		Name:    "Kael",
		Aliases: []string{"Kael the Brave", "凯尔", "K"},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate rejected valid aliases: %v", err)
	}
}

func TestEntityValidateRejectsEmptyAlias(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:      "char_kael",
		Type:    "character",
		Name:    "Kael",
		Aliases: []string{"Kael the Brave", ""},
	}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate accepted empty alias")
	}
}

func TestEntityValidateAcceptsKnownComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile: map[string]any{
				"name":        "Alice",
				"description": "A careful investigator.",
			},
			ComponentActor: map[string]any{
				"can_act": true,
				"goals":   []any{"find the truth"},
			},
			ComponentSpatial: map[string]any{
				"location_id": "tower",
			},
			ComponentInventory: map[string]any{
				"item_ids": []any{"key_1", "map_1"},
			},
			ComponentStats: map[string]any{
				"values": map[string]any{
					"strength": float64(3),
				},
			},
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityValidateRejectsUnsupportedComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			"unknown": map[string]any{},
		},
	}

	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil for unsupported component")
	}
}

func TestEntityValidateRejectsInvalidComponentFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "profile.name must be string",
			components: map[string]any{
				ComponentProfile: map[string]any{"name": 42},
			},
		},
		{
			name: "actor.can_act must be bool",
			components: map[string]any{
				ComponentActor: map[string]any{"can_act": "yes"},
			},
		},
		{
			name: "actor.goals must be string list",
			components: map[string]any{
				ComponentActor: map[string]any{"goals": []any{"find", 1}},
			},
		},
		{
			name: "spatial.location_id must be valid id",
			components: map[string]any{
				ComponentSpatial: map[string]any{"location_id": "../bad"},
			},
		},
		{
			name: "inventory.item_ids must be id list",
			components: map[string]any{
				ComponentInventory: map[string]any{"item_ids": []any{"key_1", "../bad"}},
			},
		},
		{
			name: "stats.values must be object",
			components: map[string]any{
				ComponentStats: map[string]any{"values": "strong"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{
				ID:         "char_alice",
				Type:       "character",
				Name:       "Alice",
				Components: tc.components,
			}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid component")
			}
		})
	}
}

func TestEntityComponentBuildersProduceValidComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile:   NewProfileComponent("Alice", "A careful investigator."),
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentSpatial:   NewSpatialComponent("tower"),
			ComponentInventory: NewInventoryComponent("key_1", "map_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityComponentBuildersCopyInputs(t *testing.T) {
	t.Parallel()

	goals := []string{"find the truth"}
	itemIDs := []EntityID{"key_1"}
	stats := map[string]Value{
		"strength": {Kind: ValueKindNumber, Raw: float64(3)},
	}

	actor := NewActorComponent(true, goals)
	inventory := NewInventoryComponent(itemIDs...)
	statsComponent := NewStatsComponent(stats)

	goals[0] = "changed"
	itemIDs[0] = "changed"
	stats["strength"] = Value{Kind: ValueKindNumber, Raw: float64(99)}

	if actor["goals"].([]string)[0] != "find the truth" {
		t.Fatalf("actor goals aliased input slice: %#v", actor)
	}
	if inventory["item_ids"].([]string)[0] != "key_1" {
		t.Fatalf("inventory item_ids aliased input slice: %#v", inventory)
	}
	if statsComponent["values"].(map[string]Value)["strength"].Raw != float64(3) {
		t.Fatalf("stats values aliased input map: %#v", statsComponent)
	}
}

func TestEntityValidateAcceptsStatsValueMap(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentStats: map[string]any{
				"values": map[string]Value{
					"strength": {Kind: ValueKindNumber, Raw: float64(3)},
				},
			},
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityTypedComponentAccessors(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile:   NewProfileComponent("Alice", "A careful investigator."),
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentSpatial:   NewSpatialComponent("tower"),
			ComponentInventory: NewInventoryComponent("key_1", "map_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	profile, ok := entity.ProfileComponent()
	if !ok {
		t.Fatal("ProfileComponent ok = false, want true")
	}
	if profile.Name != "Alice" || profile.Description != "A careful investigator." {
		t.Fatalf("profile mismatch: %#v", profile)
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	if !actor.CanAct || len(actor.Goals) != 1 || actor.Goals[0] != "find the truth" {
		t.Fatalf("actor mismatch: %#v", actor)
	}

	spatial, ok := entity.SpatialComponent()
	if !ok {
		t.Fatal("SpatialComponent ok = false, want true")
	}
	if spatial.LocationID != "tower" {
		t.Fatalf("spatial mismatch: %#v", spatial)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	if len(inventory.ItemIDs) != 2 || inventory.ItemIDs[0] != "key_1" || inventory.ItemIDs[1] != "map_1" {
		t.Fatalf("inventory mismatch: %#v", inventory)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	if stats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats mismatch: %#v", stats)
	}
}

func TestEntityTypedComponentAccessorsReturnFalseForMissingComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.ProfileComponent(); ok {
		t.Fatal("ProfileComponent ok = true, want false")
	}
	if _, ok := entity.ActorComponent(); ok {
		t.Fatal("ActorComponent ok = true, want false")
	}
	if _, ok := entity.SpatialComponent(); ok {
		t.Fatal("SpatialComponent ok = true, want false")
	}
	if _, ok := entity.InventoryComponent(); ok {
		t.Fatal("InventoryComponent ok = true, want false")
	}
	if _, ok := entity.StatsComponent(); ok {
		t.Fatal("StatsComponent ok = true, want false")
	}
}

func TestEntityTypedComponentAccessorsCopyReturnedData(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentInventory: NewInventoryComponent("key_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	actor.Goals[0] = "changed"
	againActor, _ := entity.ActorComponent()
	if againActor.Goals[0] != "find the truth" {
		t.Fatalf("actor accessor returned aliased goals: %#v", againActor.Goals)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	inventory.ItemIDs[0] = "changed"
	againInventory, _ := entity.InventoryComponent()
	if againInventory.ItemIDs[0] != "key_1" {
		t.Fatalf("inventory accessor returned aliased item ids: %#v", againInventory.ItemIDs)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	stats.Values["strength"] = Value{Kind: ValueKindNumber, Raw: float64(99)}
	againStats, _ := entity.StatsComponent()
	if againStats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats accessor returned aliased values: %#v", againStats.Values)
	}
}

func TestEntityTypedComponentAccessorsReadJSONShapedComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor: map[string]any{
				"can_act": true,
				"goals":   []any{"find the truth"},
			},
			ComponentInventory: map[string]any{
				"item_ids": []any{"key_1"},
			},
			ComponentStats: map[string]any{
				"values": map[string]any{
					"strength": map[string]any{
						"kind": "number",
						"raw":  float64(3),
					},
				},
			},
		},
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	if len(actor.Goals) != 1 || actor.Goals[0] != "find the truth" {
		t.Fatalf("actor goals mismatch: %#v", actor.Goals)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	if len(inventory.ItemIDs) != 1 || inventory.ItemIDs[0] != "key_1" {
		t.Fatalf("inventory ids mismatch: %#v", inventory.ItemIDs)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	if stats.Values["strength"].Kind != ValueKindNumber || stats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats values mismatch: %#v", stats.Values)
	}
}

func TestEntityTypedComponentAccessorsRejectInvalidExistingComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor:     map[string]any{"goals": []any{"find", 1}},
			ComponentInventory: map[string]any{"item_ids": []any{"key_1", 2}},
			ComponentStats:     map[string]any{"values": "strong"},
		},
	}

	if _, ok := entity.ActorComponent(); ok {
		t.Fatal("ActorComponent ok = true for invalid component")
	}
	if _, ok := entity.InventoryComponent(); ok {
		t.Fatal("InventoryComponent ok = true for invalid component")
	}
	if _, ok := entity.StatsComponent(); ok {
		t.Fatal("StatsComponent ok = true for invalid component")
	}
}
