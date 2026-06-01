package narrator

import (
	"context"

	agenttool "github.com/sizolity/worldline/agent/tool"

	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/rpg/role"
	"github.com/sizolity/worldline/rpg/tools"
)

// Tools delegates to tools.NewDisclosedTools, which applies progressive
// disclosure (see rpg/tools/tools.go). Tools() is a pure function of
// (world, disclosure) — no LLM call.
func (n *Narrator) Tools(tc *tools.ToolContext) ([]agenttool.Tool, error) {
	return tools.NewDisclosedTools(tc), nil
}

// Judge is a no-op for the Narrator: there is no rule system to apply, so
// every action is treated as a successful narrative beat. The main ReAct loop
// produces the narrative description.
//
// Future GMs (DM/KP) implement Judge with deterministic rule + dice logic.
// Judge implementations MUST NOT call an LLM.
func (n *Narrator) Judge(_ context.Context, _ role.PlayerAction, _ model.World) (role.Judgment, error) {
	return role.Judgment{Outcome: "success"}, nil
}
