package runtime

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestEntityExistsRuleRejectsMissingActor(t *testing.T) {
	rt := Runtime{Rules: []Rule{EntityExistsRule{}}}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil for missing actor")
	}
}

func TestEntityExistsRuleAllowsKnownParticipants(t *testing.T) {
	rt := Runtime{Rules: []Rule{EntityExistsRule{}}}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1":    {ID: "actor_1", Type: "character", Name: "Actor"},
			"target_1":   {ID: "target_1", Type: "item", Name: "Target"},
			"location_1": {ID: "location_1", Type: "location", Name: "Location"},
		},
	}
	event := model.WorldEvent{
		ID:         "event_1",
		Type:       model.EventTypeNote,
		Source:     model.EventSourceTest,
		ActorIDs:   []model.EntityID{"actor_1"},
		TargetIDs:  []model.EntityID{"target_1"},
		LocationID: "location_1",
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}

func TestActorAliveRuleRejectsDeadActorBeforeEffects(t *testing.T) {
	rt := Runtime{Rules: []Rule{ActorAliveRule{}}}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {
				ID:   "actor_1",
				Type: "character",
				Name: "Actor",
				State: map[string]model.Value{
					"alive": {Kind: model.ValueKindBoolean, Raw: false},
				},
			},
			"door_1": {ID: "door_1", Type: "door", Name: "Door"},
		},
	}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"actor_1"},
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "door_1",
			Payload: map[string]model.Value{
				"locked": {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for dead actor")
	}
	if got.Entities["door_1"].State != nil {
		t.Fatalf("effect was applied after dead actor rejection: %#v", got.Entities["door_1"].State)
	}
}

func TestActorAliveRuleAllowsActorWithoutAliveState(t *testing.T) {
	rt := Runtime{Rules: []Rule{ActorAliveRule{}}}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {ID: "actor_1", Type: "character", Name: "Actor"},
		},
	}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"actor_1"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}
