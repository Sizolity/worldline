package rule

import (
	"encoding/json"
	"fmt"

	"github.com/sizolity/worldline/world/model"
)

const (
	SourceSystem = "system"
	SourceUser   = "user"
	Kind         = "rpg_narrative"
)

type Rule struct {
	ID          model.RuleID `json:"id"`
	Category    string       `json:"category"`
	Level       int          `json:"level"`
	Content     string       `json:"content"`
	Source      string       `json:"source"`
	Enabled     bool         `json:"enabled"`
	Tags        []string     `json:"tags,omitempty"`
	SceneFilter string       `json:"scene_filter,omitempty"`
}

func (r Rule) Validate() error {
	if err := model.ValidateID(string(r.ID)); err != nil {
		return fmt.Errorf("rule.id: %w", err)
	}
	if r.Category == "" {
		return fmt.Errorf("rule.category is required")
	}
	if r.Content == "" {
		return fmt.Errorf("rule.content is required")
	}
	if r.Level < 0 || r.Level > 2 {
		return fmt.Errorf("rule.level must be 0, 1, or 2")
	}
	if r.Source != SourceSystem && r.Source != SourceUser {
		return fmt.Errorf("rule.source must be %q or %q", SourceSystem, SourceUser)
	}
	return nil
}

func ToModelRule(r Rule) model.Rule {
	return model.Rule{ID: r.ID, Kind: Kind, Enabled: r.Enabled, Data: r}
}

func FromModelRule(mr model.Rule) (Rule, bool) {
	if mr.Kind != Kind {
		return Rule{}, false
	}
	switch data := mr.Data.(type) {
	case Rule:
		return data, true
	case map[string]any:
		b, _ := json.Marshal(data)
		var r Rule
		if json.Unmarshal(b, &r) == nil {
			return r, true
		}
	}
	return Rule{}, false
}

func FromWorldRules(rules []model.Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, mr := range rules {
		if r, ok := FromModelRule(mr); ok {
			out = append(out, r)
		}
	}
	return out
}
