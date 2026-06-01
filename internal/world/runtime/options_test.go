package runtime

import (
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestNewRuntimeUsesDefaultRules(t *testing.T) {
	rt := NewRuntime()
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil with missing actor under default rules")
	}
}

func TestNewRuntimeWithoutRulesDisablesDefaultRules(t *testing.T) {
	rt := NewRuntime(WithoutRules())
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}

func TestNewRuntimeWithRulesReplacesDefaultRules(t *testing.T) {
	rt := NewRuntime(WithRules(testRule{
		id:       "allow_only",
		decision: RuleDecision{Status: RuleDecisionAllow},
	}))
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}
