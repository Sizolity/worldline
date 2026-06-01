package template

import "github.com/sizolity/worldline/world/model"

func modernTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "modern",
		Description: "A contemporary urban setting where ordinary lives intersect with hidden forces.",
		Canon: model.Canon{
			Genre:      []string{"contemporary fiction", "drama"},
			Tone:       []string{"grounded", "suspenseful"},
			Premise:    "In a seemingly ordinary city, a network of secrets connects strangers who have never met.",
			Laws:       []string{"No supernatural elements — all events have rational explanations.", "Consequences are proportional and realistic."},
			Boundaries: []string{"Keep the setting recognizably modern-day.", "No gratuitous violence."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_detective": {ID: "char_detective", Type: "character", Name: "Sam Reyes", Description: "A city detective investigating a cold case that just heated up.",
				Tags: []string{"tenacious", "empathetic", "overworked"}},
			"char_journalist": {ID: "char_journalist", Type: "character", Name: "Lia Chen", Description: "An investigative journalist who stumbled onto the same trail.",
				Tags: []string{"curious", "resourceful", "idealistic"}},
			"loc_precinct": {ID: "loc_precinct", Type: "location", Name: "12th Precinct", Description: "A busy police station in the downtown core."},
			"loc_cafe":     {ID: "loc_cafe", Type: "location", Name: "Blue Door Cafe", Description: "A quiet corner cafe where sources prefer to meet."},
		},
		Relations: []model.Relation{
			{ID: "rel_contact", Type: "informant", SourceID: "char_journalist", TargetID: "char_detective"},
		},
		Facts: []model.Fact{
			{ID: "fact_case", SubjectID: "char_detective", Predicate: "investigating", Value: model.Value{Kind: model.ValueKindString, Raw: "the disappearance of city councilor Ward"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_case", Kind: model.ThreadKindMystery, Title: "The Ward Disappearance", Status: model.ThreadStatusActive, Priority: 0.8, Tension: 0.5},
		},
		Rules: []model.Rule{
			templateRule("rule_modern_core_death", "core", 0, "When an entity's health reaches 0, it enters the 'dead' state and can no longer act."),
			templateRule("rule_modern_core_thread", "core", 0, "Each beat may advance at most one narrative thread by one stage."),
			templateRule("rule_modern_core_rational", "core", 0, "No supernatural elements — all events have rational explanations."),
			templateRule("rule_modern_social_consequences", "social", 2, "Consequences are proportional and realistic."),
		},
	}
}
