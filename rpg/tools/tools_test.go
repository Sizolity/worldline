package tools

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/rpg/rule"
)

func testWorld() model.World {
	combatRule := rule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 1,
		Content: "Attack rolls use d20 + modifier", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}
	socialRule := rule.Rule{
		ID: "rule-social-01", Category: "social", Level: 1,
		Content: "Persuasion checks use d20 + charisma", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"dialogue"},
	}
	return model.World{
		ID:   "world-test",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"hero-01": {
				ID: "hero-01", Type: "character", Name: "Arin",
				Tags: []string{"player"},
				State: map[string]model.Value{
					"hp": {Kind: model.ValueKindNumber, Raw: float64(20)},
				},
			},
		},
		Rules: []model.Rule{
			rule.ToModelRule(combatRule),
			rule.ToModelRule(socialRule),
		},
	}
}

func TestLookupRules_FiltersByCategory(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	result, err := tc.LookupRules(ctx, &LookupRulesParams{Category: "combat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result for combat rules")
	}
	if !contains(result, "Attack rolls") {
		t.Errorf("expected combat rule content, got: %s", result)
	}
	if contains(result, "Persuasion") {
		t.Errorf("should not contain social rule, got: %s", result)
	}
}

func TestUpdateState_ValidEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	result, err := tc.UpdateState(ctx, &UpdateStateParams{
		EntityID: "hero-01", Key: "hp", Value: float64(15),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	effects := tc.GetPendingEffects()
	if len(effects) != 1 {
		t.Fatalf("expected 1 pending effect, got %d", len(effects))
	}
	effect := effects[0]
	if effect.Kind != model.EffectUpdateEntityState {
		t.Errorf("expected kind %q, got %q", model.EffectUpdateEntityState, effect.Kind)
	}
	if effect.TargetID != "hero-01" {
		t.Errorf("expected target_id %q, got %q", "hero-01", effect.TargetID)
	}
	val, ok := effect.Payload["hp"]
	if !ok {
		t.Fatal("expected payload to contain 'hp' key")
	}
	if val.Kind != model.ValueKindNumber {
		t.Errorf("expected value kind %q, got %q", model.ValueKindNumber, val.Kind)
	}
}

func TestUpdateState_UnknownEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	_, err := tc.UpdateState(ctx, &UpdateStateParams{
		EntityID: "nonexistent", Key: "hp", Value: float64(10),
	})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRoll_ValidRoll(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 99))
	tc := &ToolContext{World: testWorld(), Rng: rng}
	ctx := context.Background()

	result, err := tc.Roll(ctx, &RollParams{Sides: 20, Count: 1, Modifier: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := parsed["total"]; !ok {
		t.Error("expected 'total' in result")
	}
	if _, ok := parsed["rolls"]; !ok {
		t.Error("expected 'rolls' in result")
	}
	total := parsed["total"].(float64)
	if total < 4 || total > 23 {
		t.Errorf("total %v out of range [4,23] for 1d20+3", total)
	}
}

func TestRoll_InvalidSides(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	_, err := tc.Roll(ctx, &RollParams{Sides: 0, Count: 1})
	if err == nil {
		t.Fatal("expected error for sides < 1")
	}
	if !contains(err.Error(), "sides must be >= 1") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetEntityState_ValidEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	result, err := tc.GetEntityState(ctx, &GetEntityStateParams{EntityID: "hero-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["id"] != "hero-01" {
		t.Errorf("expected id 'hero-01', got %v", parsed["id"])
	}
	if parsed["name"] != "Arin" {
		t.Errorf("expected name 'Arin', got %v", parsed["name"])
	}
}

func TestGetEntityState_UnknownEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	ctx := context.Background()

	_, err := tc.GetEntityState(ctx, &GetEntityStateParams{EntityID: "ghost-99"})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestNewInvokableTools(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	invokableTools := NewInvokableTools(tc)
	const wantCount = 8
	if len(invokableTools) != wantCount {
		t.Fatalf("expected %d tools, got %d", wantCount, len(invokableTools))
	}
	for _, tl := range invokableTools {
		info := tl.Info()
		if info.Name == "" {
			t.Error("tool name should not be empty")
		}
	}
}

func TestRandom_ReturnsFloatInRange(t *testing.T) {
	tc := &ToolContext{World: testWorld(), Rng: rand.New(rand.NewPCG(7, 11))}
	out, err := tc.Random(context.Background(), &RandomParams{})
	if err != nil {
		t.Fatalf("Random: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("non-json output: %v", err)
	}
	v, ok := parsed["value"].(float64)
	if !ok {
		t.Fatalf("expected float64 value, got %T", parsed["value"])
	}
	if v < 0 || v >= 1 {
		t.Errorf("value %v out of [0,1)", v)
	}
}

func TestChance_RespectsBoundaries(t *testing.T) {
	tc := &ToolContext{World: testWorld(), Rng: rand.New(rand.NewPCG(3, 5))}

	out, err := tc.Chance(context.Background(), &ChanceParams{Probability: 1.0})
	if err != nil {
		t.Fatalf("Chance: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("non-json output: %v", err)
	}
	if v, _ := parsed["value"].(bool); !v {
		t.Errorf("p=1.0 must always succeed, got %v", parsed["value"])
	}

	out, _ = tc.Chance(context.Background(), &ChanceParams{Probability: 0.0})
	_ = json.Unmarshal([]byte(out), &parsed)
	if v, _ := parsed["value"].(bool); v {
		t.Errorf("p=0.0 must always fail, got %v", parsed["value"])
	}
}

func TestWeightedChoice_PicksFromOptions(t *testing.T) {
	tc := &ToolContext{World: testWorld(), Rng: rand.New(rand.NewPCG(9, 13))}

	out, err := tc.WeightedChoice(context.Background(), &WeightedChoiceParams{Options: []WeightedOption{
		{Label: "alpha", Weight: 1},
		{Label: "beta", Weight: 1},
		{Label: "gamma", Weight: 1},
	}})
	if err != nil {
		t.Fatalf("WeightedChoice: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("non-json output: %v", err)
	}
	label, _ := parsed["value"].(string)
	if label != "alpha" && label != "beta" && label != "gamma" {
		t.Errorf("unexpected label %q", label)
	}
}

func TestWeightedChoice_EmptyOptions(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	if _, err := tc.WeightedChoice(context.Background(), &WeightedChoiceParams{Options: nil}); err == nil {
		t.Fatal("expected error on empty options")
	}
}

func TestWeightedChoice_AllZeroWeights(t *testing.T) {
	tc := &ToolContext{World: testWorld()}
	_, err := tc.WeightedChoice(context.Background(), &WeightedChoiceParams{Options: []WeightedOption{
		{Label: "a", Weight: 0},
		{Label: "b", Weight: 0},
	}})
	if err == nil {
		t.Fatal("expected error when all weights are zero")
	}
}

func TestNewDisclosedTools_NoStateNoFog(t *testing.T) {
	// A world with no entity state (the v1 mod default) and no fog must
	// drop update_state and explore_knowledge from the disclosed tool set
	// while keeping the always-available randomness tools.
	w := testWorld()
	for id, e := range w.Entities {
		e.State = nil
		w.Entities[id] = e
	}
	tc := &ToolContext{World: w}
	disclosed := NewDisclosedTools(tc)
	names := map[string]bool{}
	for _, tl := range disclosed {
		names[tl.Info().Name] = true
	}
	for _, want := range []string{"get_entity_state", "roll", "random", "chance", "weighted_choice", "lookup_rules"} {
		if !names[want] {
			t.Errorf("expected %q in disclosed set, got %v", want, names)
		}
	}
	for _, forbidden := range []string{"update_state", "explore_knowledge"} {
		if names[forbidden] {
			t.Errorf("%q must NOT be disclosed when stateless+nofog, got %v", forbidden, names)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
