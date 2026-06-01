package role

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/sizolity/worldline/internal/world/model"
)

// Persona defines the GM's narrative identity and voice.
type Persona interface {
	// Role returns a stable, human-readable label for this GM (e.g. "Narrator",
	// "Dungeon Master", "Keeper"). Used in prompts and logs; not a machine ID.
	Role() string

	// SystemPrompt builds the LLM system prompt from the pre-rendered views in
	// opts. The GM owns prompt structure; callers do not template strings here.
	SystemPrompt(players []Player, opts PromptOptions) string
}

// Rulebook governs game mechanics.
//
// Tools uses progressive disclosure — available tools depend on world state,
// fog settings, and narrative context. Not all tools are exposed every beat.
// Tools() is a pure function of (world, disclosure) — no LLM, no I/O.
//
// The tc parameter is a *tools.ToolContext; the untyped signature avoids an
// import cycle between the interface layer (role) and the implementation
// package (rpg/tools).
//
// Judge is deterministic: it must NOT call an LLM. It returns a numeric /
// categorical Judgment that the ReAct loop uses to drive narrative prose.
type Rulebook interface {
	Tools(tc any) ([]einotool.BaseTool, error)
	Judge(ctx context.Context, action PlayerAction, w model.World) (Judgment, error)
}

// Director handles pacing and player guidance.
type Director interface {
	SuggestActions(ctx context.Context, w model.World, players []Player, narrative string) (ActionChoices, error)
}

// GM is the composite interface for a complete game master.
type GM interface {
	Persona
	Rulebook
	Director
}
