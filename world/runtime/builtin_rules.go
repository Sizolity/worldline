package runtime

import (
	"fmt"

	"github.com/sizolity/worldline/world/model"
)

type EntityExistsRule struct{}

func (r EntityExistsRule) ID() model.RuleID {
	return "entity_exists"
}

func (r EntityExistsRule) Evaluate(ctx RuleContext, event model.WorldEvent) RuleDecision {
	for _, id := range event.ActorIDs {
		if _, ok := ctx.World.Entities[id]; !ok {
			return RuleDecision{Status: RuleDecisionReject, Reason: fmt.Sprintf("actor %q does not exist", id)}
		}
	}
	for _, id := range event.TargetIDs {
		if _, ok := ctx.World.Entities[id]; !ok {
			return RuleDecision{Status: RuleDecisionReject, Reason: fmt.Sprintf("target %q does not exist", id)}
		}
	}
	if event.LocationID != "" {
		if _, ok := ctx.World.Entities[event.LocationID]; !ok {
			return RuleDecision{Status: RuleDecisionReject, Reason: fmt.Sprintf("location %q does not exist", event.LocationID)}
		}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}

type ActorAliveRule struct{}

func (r ActorAliveRule) ID() model.RuleID {
	return "actor_alive"
}

func (r ActorAliveRule) Evaluate(ctx RuleContext, event model.WorldEvent) RuleDecision {
	for _, id := range event.ActorIDs {
		entity, ok := ctx.World.Entities[id]
		if !ok {
			continue
		}
		alive, ok := entity.State["alive"]
		if !ok {
			continue
		}
		if raw, ok := alive.Raw.(bool); ok && !raw {
			return RuleDecision{Status: RuleDecisionReject, Reason: fmt.Sprintf("actor %q is not alive", id)}
		}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}
