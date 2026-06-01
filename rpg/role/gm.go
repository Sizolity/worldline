package role

import (
	"context"

	agenttool "github.com/sizolity/worldline/agent/tool"

	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/world/store"
	"github.com/sizolity/worldline/rpg/tools"
)

type WorldTemplate = store.WorldTemplate

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
// Judge is deterministic: it must NOT call an LLM. It returns a numeric /
// categorical Judgment that the ReAct loop uses to drive narrative prose.
type Rulebook interface {
	Tools(tc *tools.ToolContext) ([]agenttool.Tool, error)
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
