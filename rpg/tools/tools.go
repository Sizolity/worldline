package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"

	agenttool "github.com/sizolity/worldline/agent/tool"
	"github.com/sizolity/worldline/rpg/fog"
	"github.com/sizolity/worldline/rpg/rule"
	"github.com/sizolity/worldline/world/model"
)

// Per v1 mod migration the randomness tools are **internal**: the model
// can call them to drive uncertainty, but result formatting and any
// downstream narration must keep dice/probability/numbers OUT of the
// player-facing text. The descriptions below are deliberately written
// to discourage exposing mechanics in narration.
const (
	descLookupRules      = "Look up active world-rule prose by category. Use to recall narrative constraints (causality, taboos, faction codes) before deciding what happens next. Not a mechanic / formula lookup."
	descUpdateState      = "Persist a meaningful state transition on an entity (mood shifted, relationship changed, item acquired). Only exposed when entities already carry mutable state."
	descRoll             = "Internal randomness: roll N M-sided dice for uncertain outcomes. The numeric result MUST NOT appear in the player-facing narration — translate it into qualitative description (\"barely\", \"narrowly\", \"with ease\") and stop there."
	descGetEntityState   = "Read-only inspection of an entity's current state."
	descExploreKnowledge = "Reveal a hidden entity or fact to make it available in future beats. Call when the player discovers something new through exploration, interaction, or study."
	descRandom           = "Internal randomness: returns a float in [0,1). Use to drive subtle uncertainty (a tone, a rumor's accuracy). Result MUST NOT surface as a number in narration."
	descChance           = "Internal randomness: given probability p in [0,1], return true with probability p. Use for yes/no branches that should not feel deterministic. Result MUST NOT surface as a probability in narration."
	descWeightedChoice   = "Internal randomness: choose one label from a weighted option list. Use to pick a flavor among many (which NPC mutters first, which rumor surfaces). Result MUST NOT surface as a weight or odds in narration."
)

var (
	lookupRulesSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{"type": "string", "description": "Rule category to look up"},
			"tags":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional tag filter"},
		},
		"required": []any{"category"},
	}
	updateStateSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entity_id": map[string]any{"type": "string", "description": "Target entity ID"},
			"key":       map[string]any{"type": "string", "description": "State key to update"},
			"value":     map[string]any{"type": "string", "description": "New value (string, number, or boolean)"},
		},
		"required": []any{"entity_id", "key", "value"},
	}
	rollSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sides":    map[string]any{"type": "integer", "description": "Number of sides (e.g. 20 for d20)"},
			"count":    map[string]any{"type": "integer", "description": "Number of dice (default 1)"},
			"modifier": map[string]any{"type": "integer", "description": "Flat modifier added to total (default 0)"},
		},
		"required": []any{"sides"},
	}
	getEntityStateSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entity_id": map[string]any{"type": "string", "description": "Entity to inspect"},
		},
		"required": []any{"entity_id"},
	}
	exploreKnowledgeSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target_id": map[string]any{"type": "string", "description": "Entity or fact ID to reveal"},
			"level":     map[string]any{"type": "string", "description": "Target visibility: known or explored (default: explored)"},
			"piece":     map[string]any{"type": "string", "description": "Specific knowledge piece to unlock within an entity"},
		},
		"required": []any{"target_id"},
	}
	randomSchema = map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
	chanceSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"probability": map[string]any{
				"type":        "number",
				"description": "Success probability in [0,1]",
			},
		},
		"required": []any{"probability"},
	}
	weightedChoiceSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"label":  map[string]any{"type": "string"},
						"weight": map[string]any{"type": "number"},
					},
					"required": []any{"label", "weight"},
				},
				"description": "Candidate options with non-negative weights",
			},
		},
		"required": []any{"options"},
	}
)

type LookupRulesParams struct {
	Category string   `json:"category"`
	Tags     []string `json:"tags,omitempty"`
}

type UpdateStateParams struct {
	EntityID string `json:"entity_id"`
	Key      string `json:"key"`
	Value    any    `json:"value"`
}

type RollParams struct {
	Sides    int `json:"sides"`
	Count    int `json:"count,omitempty"`
	Modifier int `json:"modifier,omitempty"`
}

type GetEntityStateParams struct {
	EntityID string `json:"entity_id"`
}

type ExploreKnowledgeParams struct {
	TargetID string `json:"target_id"`
	Level    string `json:"level,omitempty"`
	Piece    string `json:"piece,omitempty"`
}

// RandomParams is intentionally empty: random() takes no inputs.
type RandomParams struct{}

// ChanceParams carries the probability for a chance() call.
type ChanceParams struct {
	Probability float64 `json:"probability"`
}

// WeightedChoiceParams is the input for weighted_choice().
type WeightedChoiceParams struct {
	Options []WeightedOption `json:"options"`
}

// WeightedOption is one entry in a weighted_choice option list.
type WeightedOption struct {
	Label  string  `json:"label"`
	Weight float64 `json:"weight"`
}

// ToolContext holds mutable state for tool execution within a single beat.
// It is goroutine-safe for use within Eino's tool node.
type ToolContext struct {
	mu             sync.Mutex
	World          model.World
	PendingEffects []model.Effect
	Rng            *rand.Rand
	Disclosure     *fog.DisclosureState // nil = fog disabled
	// BeatID, when set, is folded into debug-log lines emitted by the
	// internal randomness tools. Optional; tools work without it.
	BeatID string
}

// GetPendingEffects returns a copy of accumulated effects (goroutine-safe).
func (tc *ToolContext) GetPendingEffects() []model.Effect {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	out := make([]model.Effect, len(tc.PendingEffects))
	copy(out, tc.PendingEffects)
	return out
}

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
	logDice(tc.BeatID, "roll", fmt.Sprintf("sides=%d count=%d modifier=%d -> total=%d", params.Sides, count, params.Modifier, total))
	return string(result), nil
}

// Random returns a JSON-encoded { "value": float64 } in [0, 1). No
// numeric output may surface to the player; the model receives the
// value, weaves an opaque qualitative description, and stops.
func (tc *ToolContext) Random(_ context.Context, _ *RandomParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	v := tc.rngLocked().Float64()
	logDice(tc.BeatID, "random", fmt.Sprintf("-> %.6f", v))
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
	logDice(tc.BeatID, "chance", fmt.Sprintf("p=%.3f -> %v", p, v))
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
	logDice(tc.BeatID, "weighted_choice", fmt.Sprintf("options=%d total=%.3f -> %q", len(params.Options), total, choice))
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

// NewInvokableTools creates tool instances bound to a ToolContext.
// Returns the full v1 tool set: lookup_rules, update_state, roll,
// get_entity_state, explore_knowledge, random, chance, weighted_choice.
func NewInvokableTools(tc *ToolContext) []agenttool.Tool {
	return []agenttool.Tool{
		&lookupRulesTool{tc: tc},
		&updateStateTool{tc: tc},
		&rollTool{tc: tc},
		&getEntityStateTool{tc: tc},
		&exploreKnowledgeTool{tc: tc},
		&randomTool{tc: tc},
		&chanceTool{tc: tc},
		&weightedChoiceTool{tc: tc},
	}
}

// NewDisclosedTools returns the subset of tools appropriate for the current
// beat, given the world state and disclosure context. Progressive disclosure:
// the LLM never sees tools that have no useful effect right now.
//
// Always disclosed:
//   - get_entity_state, roll, random, chance, weighted_choice
//
// Conditionally disclosed:
//   - lookup_rules        when the world has any active rules
//   - update_state        when at least one entity carries mutable state
//   - explore_knowledge   when fog is enabled
func NewDisclosedTools(tc *ToolContext) []agenttool.Tool {
	out := make([]agenttool.Tool, 0, 8)

	out = append(out, &getEntityStateTool{tc: tc})
	out = append(out, &rollTool{tc: tc})
	out = append(out, &randomTool{tc: tc})
	out = append(out, &chanceTool{tc: tc})
	out = append(out, &weightedChoiceTool{tc: tc})

	if len(tc.World.Rules) > 0 {
		out = append(out, &lookupRulesTool{tc: tc})
	}

	if hasMutableEntities(tc.World) {
		out = append(out, &updateStateTool{tc: tc})
	}

	if tc.Disclosure != nil {
		out = append(out, &exploreKnowledgeTool{tc: tc})
	}

	return out
}

// --- Tool implementations ---

type lookupRulesTool struct{ tc *ToolContext }

func (t *lookupRulesTool) Info() agenttool.Info {
	return agenttool.Info{Name: "lookup_rules", Description: descLookupRules, Parameters: lookupRulesSchema}
}

func (t *lookupRulesTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params LookupRulesParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.LookupRules(ctx, &params)
}

type updateStateTool struct{ tc *ToolContext }

func (t *updateStateTool) Info() agenttool.Info {
	return agenttool.Info{Name: "update_state", Description: descUpdateState, Parameters: updateStateSchema}
}

func (t *updateStateTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params UpdateStateParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.UpdateState(ctx, &params)
}

type rollTool struct{ tc *ToolContext }

func (t *rollTool) Info() agenttool.Info {
	return agenttool.Info{Name: "roll", Description: descRoll, Parameters: rollSchema}
}

func (t *rollTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params RollParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.Roll(ctx, &params)
}

type getEntityStateTool struct{ tc *ToolContext }

func (t *getEntityStateTool) Info() agenttool.Info {
	return agenttool.Info{Name: "get_entity_state", Description: descGetEntityState, Parameters: getEntityStateSchema}
}

func (t *getEntityStateTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params GetEntityStateParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.GetEntityState(ctx, &params)
}

type exploreKnowledgeTool struct{ tc *ToolContext }

func (t *exploreKnowledgeTool) Info() agenttool.Info {
	return agenttool.Info{Name: "explore_knowledge", Description: descExploreKnowledge, Parameters: exploreKnowledgeSchema}
}

func (t *exploreKnowledgeTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params ExploreKnowledgeParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.ExploreKnowledge(ctx, &params)
}

type randomTool struct{ tc *ToolContext }

func (t *randomTool) Info() agenttool.Info {
	return agenttool.Info{Name: "random", Description: descRandom, Parameters: randomSchema}
}

func (t *randomTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params RandomParams
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &params); err != nil {
			return "", fmt.Errorf("parse arguments: %w", err)
		}
	}
	return t.tc.Random(ctx, &params)
}

type chanceTool struct{ tc *ToolContext }

func (t *chanceTool) Info() agenttool.Info {
	return agenttool.Info{Name: "chance", Description: descChance, Parameters: chanceSchema}
}

func (t *chanceTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params ChanceParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.Chance(ctx, &params)
}

type weightedChoiceTool struct{ tc *ToolContext }

func (t *weightedChoiceTool) Info() agenttool.Info {
	return agenttool.Info{Name: "weighted_choice", Description: descWeightedChoice, Parameters: weightedChoiceSchema}
}

func (t *weightedChoiceTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params WeightedChoiceParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.WeightedChoice(ctx, &params)
}

func inferValueKind(v any) string {
	switch v.(type) {
	case float64, float32, int:
		return model.ValueKindNumber
	case bool:
		return model.ValueKindBoolean
	default:
		return model.ValueKindString
	}
}

func hasMutableEntities(w model.World) bool {
	for _, e := range w.Entities {
		if len(e.State) > 0 {
			return true
		}
		if _, ok := e.StatsComponent(); ok {
			return true
		}
	}
	return false
}

// rngLocked returns the bound RNG, lazily allocating a deterministic
// PCG seed when the context did not get one. Must be called with mu
// held.
func (tc *ToolContext) rngLocked() *rand.Rand {
	if tc.Rng == nil {
		tc.Rng = rand.New(rand.NewPCG(0, 0))
	}
	return tc.Rng
}

// logDice emits a single-line stderr trace for each internal-randomness
// call, gated by the WORLDLINE_DEBUG_DICE environment variable per
// directive 2.3. Output is **stderr only** — the player-facing stdout
// stream never sees these so the "zero visible judgment" red line
// stays intact.
func logDice(beatID, tool, detail string) {
	if os.Getenv("WORLDLINE_DEBUG_DICE") == "" {
		return
	}
	if beatID != "" {
		fmt.Fprintf(os.Stderr, "dice[%s]: %s(%s)\n", beatID, tool, detail)
		return
	}
	fmt.Fprintf(os.Stderr, "dice: %s(%s)\n", tool, detail)
}
