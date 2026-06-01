package tools

import (
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Registry returns the static [schema.ToolInfo] metadata for every RPG
// tool. Useful for documentation, debug commands, and unit tests that
// need to assert tool coverage — the *executable* tools must come from
// NewInvokableTools / NewDisclosedTools because they need a
// ToolContext binding.
//
// v1 set:
//
//	lookup_rules         — narrative rule prose lookup
//	update_state         — persist meaningful entity state changes
//	roll                 — internal randomness (dice)
//	get_entity_state     — read-only entity inspection
//	explore_knowledge    — fog disclosure (only useful when fog enabled)
//	random               — internal randomness (uniform float)
//	chance               — internal randomness (binary probability)
//	weighted_choice      — internal randomness (weighted pick)
//
// Per directive 2.3 the randomness tools (roll/random/chance/
// weighted_choice) are intentionally **invisible**: the model receives
// numeric results but must NOT echo them into the player-facing
// narrative.
func Registry() []*schema.ToolInfo {
	return []*schema.ToolInfo{
		{Name: "lookup_rules", Desc: descLookupRules, ParamsOneOf: lookupRulesSchema},
		{Name: "update_state", Desc: descUpdateState, ParamsOneOf: updateStateSchema},
		{Name: "roll", Desc: descRoll, ParamsOneOf: rollSchema},
		{Name: "get_entity_state", Desc: descGetEntityState, ParamsOneOf: getEntityStateSchema},
		{Name: "explore_knowledge", Desc: descExploreKnowledge, ParamsOneOf: exploreKnowledgeSchema},
		{Name: "random", Desc: descRandom, ParamsOneOf: randomSchema},
		{Name: "chance", Desc: descChance, ParamsOneOf: chanceSchema},
		{Name: "weighted_choice", Desc: descWeightedChoice, ParamsOneOf: weightedChoiceSchema},
	}
}

// NewInvokableTools creates tool instances bound to a ToolContext.
// Returns the full v1 tool set: lookup_rules, update_state, roll,
// get_entity_state, explore_knowledge, random, chance, weighted_choice.
//
// The slice element type is [einotool.BaseTool], not InvokableTool, so
// it can be passed directly to eino's [compose.ToolsNodeConfig]; eino
// type-asserts to InvokableTool / StreamableTool at call time.
func NewInvokableTools(tc *ToolContext) []einotool.BaseTool {
	return []einotool.BaseTool{
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
func NewDisclosedTools(tc *ToolContext) []einotool.BaseTool {
	out := make([]einotool.BaseTool, 0, 8)

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
