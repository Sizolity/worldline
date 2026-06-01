package template

import "github.com/sizolity/worldline/world/model"

func scifiTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "scifi",
		Description: "A far-future setting aboard a generation ship drifting between stars.",
		Canon: model.Canon{
			Genre:      []string{"science fiction", "thriller"},
			Tone:       []string{"tense", "cerebral"},
			Premise:    "The colony ship Meridian has been in transit for 300 years. The AI overseer has gone silent.",
			Laws:       []string{"Faster-than-light travel is impossible.", "AI sentience is legally recognized but socially contested."},
			Boundaries: []string{"No magic or supernatural elements.", "Technology must be plausible within hard-SF constraints."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_captain": {ID: "char_captain", Type: "character", Name: "Yara Osei", Description: "Acting captain of the Meridian, thrust into command after the AI shutdown.",
				Tags: []string{"pragmatic", "decisive", "burdened"}},
			"char_engineer": {ID: "char_engineer", Type: "character", Name: "Dex Varro", Description: "Chief systems engineer who suspects sabotage.",
				Tags: []string{"analytical", "paranoid", "loyal"}},
			"loc_bridge": {ID: "loc_bridge", Type: "location", Name: "Command Bridge", Description: "The nerve center of the Meridian."},
			"loc_core":   {ID: "loc_core", Type: "location", Name: "AI Core Chamber", Description: "A sealed chamber housing the ship's central intelligence."},
		},
		Relations: []model.Relation{
			{ID: "rel_crew", Type: "colleague", SourceID: "char_captain", TargetID: "char_engineer"},
		},
		Facts: []model.Fact{
			{ID: "fact_ai_silent", SubjectID: "loc_core", Predicate: "status", Value: model.Value{Kind: model.ValueKindString, Raw: "AI core unresponsive for 72 hours"}},
			{ID: "fact_population", SubjectID: "loc_bridge", Predicate: "ship_population", Value: model.Value{Kind: model.ValueKindNumber, Raw: float64(12400)}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_ai", Kind: model.ThreadKindMystery, Title: "The Silent Overseer", Status: model.ThreadStatusActive, Priority: 0.9, Tension: 0.7},
		},
		Rules: []model.Rule{
			templateRule("rule_scifi_core_death", "core", 0, "When an entity's health reaches 0, it enters the 'dead' state and can no longer act."),
			templateRule("rule_scifi_core_thread", "core", 0, "Each beat may advance at most one narrative thread by one stage."),
			templateRule("rule_scifi_core_ai", "core", 0, "AI sentience is legally recognized but socially contested."),
			templateRule("rule_scifi_env_ftl", "environment", 2, "FTL travel is impossible. All interstellar journeys take years."),
		},
	}
}
