package template

import "github.com/sizolity/worldline/world/model"

func fantasyTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "fantasy",
		Description: "A medieval fantasy world of kingdoms, magic, and ancient mysteries.",
		Canon: model.Canon{
			Genre:      []string{"fantasy", "adventure"},
			Tone:       []string{"epic", "mysterious"},
			Premise:    "An ancient power stirs beneath the mountains, and the balance of the realm hangs by a thread.",
			Laws:       []string{"Magic requires a cost — power always demands sacrifice.", "The dead do not return unchanged."},
			Boundaries: []string{"No modern technology.", "No breaking the established magic system without consequence."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "Kael", Description: "A wandering swordsman with a mysterious past.",
				Tags: []string{"brave", "secretive", "skilled"}},
			"char_sage": {ID: "char_sage", Type: "character", Name: "Mirael", Description: "An aging scholar who guards forgotten lore.",
				Tags: []string{"wise", "cautious", "knowledgeable"}},
			"loc_village": {ID: "loc_village", Type: "location", Name: "Thornhaven", Description: "A quiet village at the edge of the Darkwood."},
			"loc_tower":   {ID: "loc_tower", Type: "location", Name: "The Shattered Tower", Description: "Ruins of an ancient mage's tower, still humming with residual magic."},
		},
		Relations: []model.Relation{
			{ID: "rel_mentor", Type: "mentor", SourceID: "char_sage", TargetID: "char_hero"},
		},
		Facts: []model.Fact{
			{ID: "fact_seal", SubjectID: "loc_tower", Predicate: "contains", Value: model.Value{Kind: model.ValueKindString, Raw: "a broken seal over a deep vault"}},
			{ID: "fact_hero_origin", SubjectID: "char_hero", Predicate: "origin", Value: model.Value{Kind: model.ValueKindString, Raw: "unknown — arrived in Thornhaven five years ago"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_seal", Kind: model.ThreadKindMystery, Title: "The Broken Seal", Status: model.ThreadStatusOpen, Priority: 0.8, Tension: 0.4},
		},
		Rules: []model.Rule{
			templateRule("rule_fantasy_core_death", "core", 0, "When an entity's health reaches 0, it enters the 'dead' state and can no longer act."),
			templateRule("rule_fantasy_core_thread", "core", 0, "Each beat may advance at most one narrative thread by one stage."),
			templateRule("rule_fantasy_core_magic", "core", 0, "Magic requires a cost — power always demands sacrifice."),
			templateRule("rule_fantasy_combat_attack", "combat", 2, "Attack resolution: roll d20 + attacker's attack_mod vs target's defense. Hit if roll >= defense.", "melee", "ranged"),
			templateRule("rule_fantasy_social_affection", "social", 2, "NPC affection changes by at most ±2 per interaction."),
		},
	}
}
