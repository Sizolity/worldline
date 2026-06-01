package template

import "github.com/sizolity/worldline/world/model"

func mysteryTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "mystery",
		Description: "A classic whodunit set in an isolated manor during a storm.",
		Canon: model.Canon{
			Genre:      []string{"mystery", "suspense"},
			Tone:       []string{"atmospheric", "claustrophobic"},
			Premise:    "Eight guests are trapped in Blackmoor Manor when a blizzard cuts all roads. Then the host is found dead.",
			Laws:       []string{"The murderer is among the guests.", "All clues are fair — no information is hidden from the reader that characters could know."},
			Boundaries: []string{"No supernatural explanations.", "The solution must be logically deducible from presented clues."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_detective": {ID: "char_detective", Type: "character", Name: "Inspector Harlow", Description: "A retired inspector who happens to be among the guests.",
				Tags: []string{"observant", "methodical", "dry-witted"}},
			"char_host": {ID: "char_host", Type: "character", Name: "Lord Blackmoor", Description: "The wealthy and controversial host, now deceased.",
				Tags: []string{"enigmatic", "wealthy", "deceased"}},
			"loc_manor": {ID: "loc_manor", Type: "location", Name: "Blackmoor Manor", Description: "A grand but aging estate, now snowbound."},
			"loc_study": {ID: "loc_study", Type: "location", Name: "The Study", Description: "Where the body was found, locked from the inside."},
		},
		Relations: []model.Relation{
			{ID: "rel_guest", Type: "guest_of", SourceID: "char_detective", TargetID: "char_host"},
		},
		Facts: []model.Fact{
			{ID: "fact_locked", SubjectID: "loc_study", Predicate: "state", Value: model.Value{Kind: model.ValueKindString, Raw: "locked from the inside when the body was discovered"}},
			{ID: "fact_death", SubjectID: "char_host", Predicate: "cause_of_death", Value: model.Value{Kind: model.ValueKindString, Raw: "apparent poisoning"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_murder", Kind: model.ThreadKindMystery, Title: "Who Killed Lord Blackmoor?", Status: model.ThreadStatusActive, Priority: 1.0, Tension: 0.8},
		},
		Rules: []model.Rule{
			templateRule("rule_mystery_core_thread", "core", 0, "Each beat may advance at most one narrative thread by one stage."),
			templateRule("rule_mystery_core_murderer", "core", 0, "The murderer is among the guests."),
			templateRule("rule_mystery_core_fairclue", "core", 0, "All clues are fair — no information hidden from the reader that characters could know."),
			templateRule("rule_mystery_narrative_deducible", "narrative", 2, "The solution must be logically deducible from presented clues."),
		},
	}
}
