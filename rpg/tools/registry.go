package tools

import agenttool "github.com/sizolity/worldline/agent/tool"

// Registry returns Info definitions for all RPG tools.
// Useful for prompt construction or documentation — the actual
// executable tools are created via NewInvokableTools.
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
func Registry() []agenttool.Info {
	return []agenttool.Info{
		{Name: "lookup_rules", Description: descLookupRules, Parameters: lookupRulesSchema},
		{Name: "update_state", Description: descUpdateState, Parameters: updateStateSchema},
		{Name: "roll", Description: descRoll, Parameters: rollSchema},
		{Name: "get_entity_state", Description: descGetEntityState, Parameters: getEntityStateSchema},
		{Name: "explore_knowledge", Description: descExploreKnowledge, Parameters: exploreKnowledgeSchema},
		{Name: "random", Description: descRandom, Parameters: randomSchema},
		{Name: "chance", Description: descChance, Parameters: chanceSchema},
		{Name: "weighted_choice", Description: descWeightedChoice, Parameters: weightedChoiceSchema},
	}
}
