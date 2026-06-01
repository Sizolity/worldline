package template

import "github.com/sizolity/worldline/world/model"

func templateRule(id model.RuleID, category string, level int, content string, tags ...string) model.Rule {
	data := map[string]any{
		"id":       id,
		"category": category,
		"level":    level,
		"content":  content,
		"source":   "system",
		"enabled":  true,
	}
	if len(tags) > 0 {
		data["tags"] = tags
	}
	return model.Rule{
		ID:      id,
		Kind:    "rpg_narrative",
		Enabled: true,
		Data:    data,
	}
}
