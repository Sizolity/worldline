package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/rule"
	"github.com/sizolity/worldline/internal/world/model"
)

func (tc *ToolContext) LookupRules(_ context.Context, params *LookupRulesParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	rpgRules := rule.FromWorldRules(tc.World.Rules)
	results := rule.Lookup(rpgRules, rule.LookupFilter{
		Category: params.Category,
		Tags:     params.Tags,
	})
	return rule.FormatRules(results), nil
}

func (tc *ToolContext) UpdateState(_ context.Context, params *UpdateStateParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	entityID := model.EntityID(params.EntityID)
	if _, ok := tc.World.Entities[entityID]; !ok {
		return "", fmt.Errorf("entity %q not found", params.EntityID)
	}
	value := model.Value{Kind: inferValueKind(params.Value), Raw: params.Value}
	tc.PendingEffects = append(tc.PendingEffects, model.Effect{
		Kind:     model.EffectUpdateEntityState,
		TargetID: params.EntityID,
		Payload:  map[string]model.Value{params.Key: value},
	})
	return fmt.Sprintf("OK: %s.%s = %v", params.EntityID, params.Key, params.Value), nil
}

func (tc *ToolContext) Roll(_ context.Context, params *RollParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	count := params.Count
	if count < 1 {
		count = 1
	}
	if params.Sides < 1 {
		return "", fmt.Errorf("sides must be >= 1")
	}
	rng := tc.rngLocked()
	rolls := make([]int, count)
	total := params.Modifier
	for i := range rolls {
		rolls[i] = rng.IntN(params.Sides) + 1
		total += rolls[i]
	}
	result, _ := json.Marshal(map[string]any{
		"rolls": rolls, "modifier": params.Modifier, "total": total,
	})
	return string(result), nil
}

// Random returns a JSON-encoded { "value": float64 } in [0, 1). No
// numeric output may surface to the player; the model receives the
// value, weaves an opaque qualitative description, and stops.
func (tc *ToolContext) Random(_ context.Context, _ *RandomParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	v := tc.rngLocked().Float64()
	data, _ := json.Marshal(map[string]any{"value": v})
	return string(data), nil
}

// Chance returns a JSON-encoded { "value": bool } with success
// probability params.Probability (clamped to [0,1]).
func (tc *ToolContext) Chance(_ context.Context, params *ChanceParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	p := params.Probability
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	v := tc.rngLocked().Float64() < p
	data, _ := json.Marshal(map[string]any{"value": v})
	return string(data), nil
}

// WeightedChoice picks one label from a weighted distribution. Negative
// weights are clamped to 0. If every weight is 0 (or the list is empty),
// returns an error so the model can degrade gracefully.
func (tc *ToolContext) WeightedChoice(_ context.Context, params *WeightedChoiceParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if len(params.Options) == 0 {
		return "", fmt.Errorf("weighted_choice: options is required and must be non-empty")
	}
	var total float64
	weights := make([]float64, len(params.Options))
	for i, opt := range params.Options {
		w := opt.Weight
		if w < 0 {
			w = 0
		}
		weights[i] = w
		total += w
	}
	if total == 0 {
		return "", fmt.Errorf("weighted_choice: all option weights are zero")
	}
	r := tc.rngLocked().Float64() * total
	choice := params.Options[len(params.Options)-1].Label
	cum := 0.0
	for i, w := range weights {
		cum += w
		if r < cum {
			choice = params.Options[i].Label
			break
		}
	}
	data, _ := json.Marshal(map[string]any{"value": choice})
	return string(data), nil
}

func (tc *ToolContext) GetEntityState(_ context.Context, params *GetEntityStateParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	entity, ok := tc.World.Entities[model.EntityID(params.EntityID)]
	if !ok {
		return "", fmt.Errorf("entity %q not found", params.EntityID)
	}
	out := map[string]any{
		"id": entity.ID, "name": entity.Name, "type": entity.Type,
		"tags": entity.Tags, "state": entity.State,
	}
	data, _ := json.Marshal(out)
	return string(data), nil
}

func (tc *ToolContext) ExploreKnowledge(_ context.Context, params *ExploreKnowledgeParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.Disclosure == nil {
		return "fog disabled", nil
	}

	level := fog.Explored
	if params.Level == "known" {
		level = fog.Known
	}

	action := fog.RevealAction{ToLevel: level, Piece: params.Piece}

	entityID := model.EntityID(params.TargetID)
	if _, ok := tc.World.Entities[entityID]; ok {
		action.EntityID = entityID
	} else {
		factID := model.FactID(params.TargetID)
		foundFact := false
		for _, f := range tc.World.Facts {
			if f.ID == factID {
				foundFact = true
				break
			}
		}
		if foundFact {
			action.FactID = factID
		} else {
			return "", fmt.Errorf("target %q not found as entity or fact", params.TargetID)
		}
	}

	fog.Reveal(tc.Disclosure, action)

	if params.Piece != "" {
		return fmt.Sprintf("unlocked piece %q for %s", params.Piece, params.TargetID), nil
	}
	return fmt.Sprintf("revealed %s (level: %s)", params.TargetID, level), nil
}
