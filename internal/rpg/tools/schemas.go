package tools

import "github.com/cloudwego/eino/schema"

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

// Schemas are expressed directly as eino [schema.ParamsOneOf] values so
// the model receives JSON-schema parameters via the native eino pipeline
// — there is no worldline-side schema-shape translation. ParameterInfo
// covers scalars, arrays of scalars, and nested objects via SubParams;
// switch to [schema.NewParamsOneOfByJSONSchema] only when you need
// features ParameterInfo cannot express (anyOf/oneOf/$ref, etc.).
var (
	lookupRulesSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"category": {Type: schema.String, Desc: "Rule category to look up", Required: true},
		"tags": {
			Type:     schema.Array,
			Desc:     "Optional tag filter",
			ElemInfo: &schema.ParameterInfo{Type: schema.String},
		},
	})

	updateStateSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"entity_id": {Type: schema.String, Desc: "Target entity ID", Required: true},
		"key":       {Type: schema.String, Desc: "State key to update", Required: true},
		"value":     {Type: schema.String, Desc: "New value (string, number, or boolean)", Required: true},
	})

	rollSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"sides":    {Type: schema.Integer, Desc: "Number of sides (e.g. 20 for d20)", Required: true},
		"count":    {Type: schema.Integer, Desc: "Number of dice (default 1)"},
		"modifier": {Type: schema.Integer, Desc: "Flat modifier added to total (default 0)"},
	})

	getEntityStateSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"entity_id": {Type: schema.String, Desc: "Entity to inspect", Required: true},
	})

	exploreKnowledgeSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"target_id": {Type: schema.String, Desc: "Entity or fact ID to reveal", Required: true},
		"level":     {Type: schema.String, Desc: "Target visibility: known or explored (default: explored)"},
		"piece":     {Type: schema.String, Desc: "Specific knowledge piece to unlock within an entity"},
	})

	// random takes no input parameters. Eino accepts an empty params map
	// (rendered as `{"type":"object","properties":{}}`) or a nil
	// ParamsOneOf altogether; an empty map matches the prior schema shape
	// most closely so the model continues to see the same envelope.
	randomSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})

	chanceSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"probability": {Type: schema.Number, Desc: "Success probability in [0,1]", Required: true},
	})

	weightedChoiceSchema = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"options": {
			Type:     schema.Array,
			Desc:     "Candidate options with non-negative weights",
			Required: true,
			ElemInfo: &schema.ParameterInfo{
				Type: schema.Object,
				SubParams: map[string]*schema.ParameterInfo{
					"label":  {Type: schema.String, Required: true},
					"weight": {Type: schema.Number, Required: true},
				},
			},
		},
	})
)
