package narrator

import (
	"context"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/tools"
	"github.com/sizolity/worldline/internal/world/model"
)

// Tools returns the full beat-agent tool surface for one beat: the
// progressively-disclosed world-mutation tools (rpg/tools.NewDisclosedTools)
// plus the inline set_choices output-schema tool the model is required
// to call at the end of its narrative.
//
// set_choices is appended LAST so the model's tool index for the existing
// game tools does not shift across the registration change — useful for
// per-tool log lines / metrics that key on order. It does not participate
// in progressive disclosure: every beat needs it for the narrative→choices
// inline contract, regardless of fog state.
//
// Tools() is a pure function of (world, disclosure) — no LLM call. The
// set_choices BaseTool itself is also pure: see NewSetChoicesTool for
// why its handler is a no-op (the pipeline extracts choices from the
// streamed assistant message, not from a tool invocation).
func (n *Narrator) Tools(tc any) ([]einotool.BaseTool, error) {
	game := tools.NewDisclosedTools(tc.(*tools.ToolContext))
	choicesTool, err := NewSetChoicesTool()
	if err != nil {
		return nil, fmt.Errorf("build set_choices tool: %w", err)
	}
	return append(game, choicesTool), nil
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
