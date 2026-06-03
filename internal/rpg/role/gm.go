package role

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

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
//
// SuggestActions is the FALLBACK path: a separate post-beat LLM
// round-trip that produces choices when the beat agent did not emit
// them inline (see InlineChoiceParser). It's kept on every GM because
// the engine relies on it as the safety net when inline extraction
// fails — but on healthy beats it is never called.
type Director interface {
	SuggestActions(ctx context.Context, w model.World, players []Player, narrative string) (ActionChoices, error)
}

// InlineChoiceParser extracts next-step action options the beat agent
// emitted INLINE alongside its narrative — the "inline schema" pattern
// where the model calls a structured tool (e.g. set_choices) as the
// final element of the same assistant message that carried the
// streamed prose. See internal/agent/react Result.ToolCalls for how
// these calls reach the session, and internal/rpg/narrator for the
// canonical implementation.
//
// Semantics of the return tuple:
//   - (choices, true,  nil): inline emission succeeded; session uses
//     these choices and skips the SuggestActions fallback entirely.
//   - (zero,    false, nil): the GM does not use the inline contract
//     (or the model did not emit the structured call); session falls
//     back to Director.SuggestActions.
//   - (zero,    true,  err): the inline call WAS present but its
//     arguments could not be parsed; session also falls back, and the
//     err is for diagnostics (telemetry, debug stderr).
//
// Splitting "found" from "err" lets the session distinguish a cold
// prompt drift ("model never called the tool") from a hot decoding
// issue ("model called but emitted bad JSON"); both route to fallback
// but warrant different alerting.
type InlineChoiceParser interface {
	ExtractInlineChoices(toolCalls []schema.ToolCall) (ActionChoices, bool, error)
}

// GM is the composite interface for a complete game master.
type GM interface {
	Persona
	Rulebook
	Director
	InlineChoiceParser
}
