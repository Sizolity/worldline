package tools

import (
	"context"
	"encoding/json"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Each tool below implements eino's [einotool.InvokableTool] directly:
//
//   Info(ctx)            advertises name / description / JSON-schema
//   InvokableRun(ctx,…)  parses the model's JSON arguments and dispatches
//                        to the ToolContext receiver in tools.go.
//
// The opts variadic on InvokableRun is part of the eino interface but
// unused — worldline tools have no per-call options.

type lookupRulesTool struct{ tc *ToolContext }

func (t *lookupRulesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "lookup_rules", Desc: descLookupRules, ParamsOneOf: lookupRulesSchema}, nil
}

func (t *lookupRulesTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params LookupRulesParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.LookupRules(ctx, &params)
}

type updateStateTool struct{ tc *ToolContext }

func (t *updateStateTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "update_state", Desc: descUpdateState, ParamsOneOf: updateStateSchema}, nil
}

func (t *updateStateTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params UpdateStateParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.UpdateState(ctx, &params)
}

// rollTool is intentionally SHELVED in v1: it is kept (along with Roll /
// RollParams / rollSchema / descRoll) for a future dice/rule resolution
// system, but it is NOT registered in Registry() nor disclosed by
// NewInvokableTools / NewDisclosedTools because v1 has no rule system that
// would consume a roll. To revive it, re-add it to those three sites.
type rollTool struct{ tc *ToolContext }

func (t *rollTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "roll", Desc: descRoll, ParamsOneOf: rollSchema}, nil
}

func (t *rollTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params RollParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.Roll(ctx, &params)
}

type advanceTimeTool struct{ tc *ToolContext }

func (t *advanceTimeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "advance_time", Desc: descAdvanceTime, ParamsOneOf: advanceTimeSchema}, nil
}

func (t *advanceTimeTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params AdvanceTimeParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.AdvanceTime(ctx, &params)
}

type getEntityStateTool struct{ tc *ToolContext }

func (t *getEntityStateTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "get_entity_state", Desc: descGetEntityState, ParamsOneOf: getEntityStateSchema}, nil
}

func (t *getEntityStateTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params GetEntityStateParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.GetEntityState(ctx, &params)
}

type exploreKnowledgeTool struct{ tc *ToolContext }

func (t *exploreKnowledgeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "explore_knowledge", Desc: descExploreKnowledge, ParamsOneOf: exploreKnowledgeSchema}, nil
}

func (t *exploreKnowledgeTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params ExploreKnowledgeParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.ExploreKnowledge(ctx, &params)
}

type randomTool struct{ tc *ToolContext }

func (t *randomTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "random", Desc: descRandom, ParamsOneOf: randomSchema}, nil
}

func (t *randomTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params RandomParams
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &params); err != nil {
			return "", fmt.Errorf("parse arguments: %w", err)
		}
	}
	return t.tc.Random(ctx, &params)
}

type chanceTool struct{ tc *ToolContext }

func (t *chanceTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "chance", Desc: descChance, ParamsOneOf: chanceSchema}, nil
}

func (t *chanceTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params ChanceParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.Chance(ctx, &params)
}

type weightedChoiceTool struct{ tc *ToolContext }

func (t *weightedChoiceTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "weighted_choice", Desc: descWeightedChoice, ParamsOneOf: weightedChoiceSchema}, nil
}

func (t *weightedChoiceTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	var params WeightedChoiceParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	return t.tc.WeightedChoice(ctx, &params)
}

// Compile-time guarantee that every adapter satisfies eino's invokable
// tool contract; if any method drifts from the interface the build fails
// here rather than at the call site in [react.Run].
var (
	_ einotool.InvokableTool = (*lookupRulesTool)(nil)
	_ einotool.InvokableTool = (*updateStateTool)(nil)
	_ einotool.InvokableTool = (*rollTool)(nil)
	_ einotool.InvokableTool = (*advanceTimeTool)(nil)
	_ einotool.InvokableTool = (*getEntityStateTool)(nil)
	_ einotool.InvokableTool = (*exploreKnowledgeTool)(nil)
	_ einotool.InvokableTool = (*randomTool)(nil)
	_ einotool.InvokableTool = (*chanceTool)(nil)
	_ einotool.InvokableTool = (*weightedChoiceTool)(nil)
)
