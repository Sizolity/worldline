package narrator

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/tools"
	"github.com/sizolity/worldline/internal/world/model"
)

// Tools delegates to tools.NewDisclosedTools, which applies progressive
// disclosure (see rpg/tools/tools.go). Tools() is a pure function of
// (world, disclosure) — no LLM call.
func (n *Narrator) Tools(tc any) ([]einotool.BaseTool, error) {
	return tools.NewDisclosedTools(tc.(*tools.ToolContext)), nil
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
