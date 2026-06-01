package runtime

import (
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

type Rule interface {
	ID() model.RuleID
	Evaluate(RuleContext, model.WorldEvent) RuleDecision
}

type RuleContext struct {
	World model.World
}

type RuleDecision struct {
	Status string
	Reason string
}

const (
	RuleDecisionAllow  = "allow"
	RuleDecisionReject = "reject"
)

func (d RuleDecision) Validate() error {
	switch d.Status {
	case RuleDecisionAllow, RuleDecisionReject:
		return nil
	default:
		return fmt.Errorf("unsupported rule decision status %q", d.Status)
	}
}
