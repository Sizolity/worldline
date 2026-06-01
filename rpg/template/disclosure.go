package template

import (
	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/rpg/fog"
)

// InitialDisclosure returns the starting disclosure state for a given template.
// Player characters and their immediate surroundings start revealed;
// distant locations, secrets, and deeper lore start hidden.
var InitialDisclosure = map[string]fog.DisclosureState{
	"fantasy": fantasyDisclosure(),
	"scifi":   scifiDisclosure(),
	"modern":  modernDisclosure(),
	"mystery": mysteryDisclosure(),
}

func fantasyDisclosure() fog.DisclosureState {
	return fog.DisclosureState{
		Entities: map[model.EntityID]fog.EntityDisclosure{
			"char_hero":   {Level: fog.Explored},
			"char_sage":   {Level: fog.Known},
			"loc_village": {Level: fog.Explored},
			"loc_tower":   {Level: fog.Known}, // player knows it exists but hasn't been there
		},
		Facts: map[model.FactID]bool{
			"fact_hero_origin": true,  // player knows their own backstory
			"fact_seal":        false, // tower's secret not yet discovered
		},
		Relations: map[model.RelationID]bool{
			"rel_mentor": true, // player knows sage is their mentor
		},
	}
}

func scifiDisclosure() fog.DisclosureState {
	return fog.DisclosureState{
		Entities: map[model.EntityID]fog.EntityDisclosure{
			"char_engineer":   {Level: fog.Explored},
			"char_ai":        {Level: fog.Known},
			"loc_bridge":     {Level: fog.Explored},
			"loc_cryo_bay":   {Level: fog.Known},
			"loc_lower_deck": {Level: fog.Hidden},
		},
		Facts: map[model.FactID]bool{
			"fact_destination_unknown": true,
		},
		Relations: map[model.RelationID]bool{
			"rel_crew": true,
		},
	}
}

func modernDisclosure() fog.DisclosureState {
	return fog.DisclosureState{
		Entities: map[model.EntityID]fog.EntityDisclosure{
			"char_detective": {Level: fog.Explored},
			"char_contact":  {Level: fog.Known},
			"loc_office":    {Level: fog.Explored},
			"loc_warehouse": {Level: fog.Known},
		},
		Facts: map[model.FactID]bool{
			"fact_case_open": true,
		},
		Relations: map[model.RelationID]bool{
			"rel_informant": true,
		},
	}
}

func mysteryDisclosure() fog.DisclosureState {
	return fog.DisclosureState{
		Entities: map[model.EntityID]fog.EntityDisclosure{
			"char_investigator": {Level: fog.Explored},
			"char_butler":      {Level: fog.Known},
			"loc_manor_foyer":  {Level: fog.Explored},
			"loc_manor_study":  {Level: fog.Known},
			"loc_manor_cellar": {Level: fog.Hidden},
		},
		Facts: map[model.FactID]bool{
			"fact_missing_heir": true,
		},
	}
}
