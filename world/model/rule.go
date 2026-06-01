package model

import "fmt"

type Rule struct {
	ID      RuleID `json:"id"`
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
	Data    any    `json:"data,omitempty"`
}

func (r Rule) Validate() error {
	if err := ValidateID(string(r.ID)); err != nil {
		return fmt.Errorf("rule.id: %w", err)
	}
	if r.Kind == "" {
		return fmt.Errorf("rule.kind is required")
	}
	return nil
}
