package rule

import (
	"encoding/json"
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

const (
	SourceSystem = "system"
	SourceUser   = "user"
	Kind         = "rpg_narrative"
)

type NarrativeRule struct {
	ID          model.RuleID `json:"id"`
	Category    string       `json:"category"`
	Level       int          `json:"level"`
	Content     string       `json:"content"`
	Source      string       `json:"source"`
	Enabled     bool         `json:"enabled"`
	Tags        []string     `json:"tags,omitempty"`
	SceneFilter string       `json:"scene_filter,omitempty"`
}

func (r NarrativeRule) Validate() error {
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

func ToModelRule(r NarrativeRule) model.Rule {
	return model.Rule{ID: r.ID, Kind: Kind, Enabled: r.Enabled, Data: r}
}

func FromModelRule(mr model.Rule) (NarrativeRule, bool) {
	if mr.Kind != Kind {
		return NarrativeRule{}, false
	}
	switch data := mr.Data.(type) {
	case NarrativeRule:
		return data, true
	case map[string]any:
		b, _ := json.Marshal(data)
		var r NarrativeRule
		if json.Unmarshal(b, &r) == nil {
			return r, true
		}
	}
	return NarrativeRule{}, false
}

func FromWorldRules(rules []model.Rule) []NarrativeRule {
	out := make([]NarrativeRule, 0, len(rules))
	for _, mr := range rules {
		if r, ok := FromModelRule(mr); ok {
			out = append(out, r)
		}
	}
	return out
}
