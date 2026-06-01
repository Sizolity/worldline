package role

import (
	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/world/view"
)

// Player represents a player bound to a character entity.
type Player struct {
	ID          string
	CharacterID model.EntityID
	Name        string
}

// PlayerAction is what the PL submits each beat.
type PlayerAction struct {
	PlayerID string
	Content  string
}

// ActionType is a constrained enum for categorizing player action options.
// LLM-generated SuggestActions must pick from this set (enforced via JSON schema).
type ActionType string

const (
	ActionTypeExplore     ActionType = "explore"
	ActionTypeSocial      ActionType = "social"
	ActionTypeCombat      ActionType = "combat"
	ActionTypeInvestigate ActionType = "investigate"
	ActionTypeUseItem     ActionType = "use_item"
	ActionTypeRest        ActionType = "rest"
	ActionTypeCustom      ActionType = "custom"
)

// ActionOption is one choice presented to the player after a beat.
// Label is LLM-generated natural language; Type is a constrained enum.
type ActionOption struct {
	Label string     `json:"label" jsonschema:"description=Short natural-language description of the action. Empty when type is custom."`
	Type  ActionType `json:"type" jsonschema:"required,enum=explore,enum=social,enum=combat,enum=investigate,enum=use_item,enum=rest,enum=custom,description=Categorical kind. Use custom (with empty label) as a trailing open slot unless the scene is a critical plot lock-in."`
}

// ActionChoices is the set of options after a beat.
type ActionChoices struct {
	Options []ActionOption `json:"options"`
}

// WithCustomSlot returns a copy with a trailing blank custom slot.
// The slot has an empty Label; if the player selects it without providing
// input, the consumer should fall back to Options[0].
func (c ActionChoices) WithCustomSlot() ActionChoices {
	out := make([]ActionOption, len(c.Options), len(c.Options)+1)
	copy(out, c.Options)
	out = append(out, ActionOption{Type: ActionTypeCustom})
	return ActionChoices{Options: out}
}

// HasCustomSlot reports whether the last option is a custom slot.
func (c ActionChoices) HasCustomSlot() bool {
	if len(c.Options) == 0 {
		return false
	}
	return c.Options[len(c.Options)-1].Type == ActionTypeCustom
}

// PromptOptions carries pre-rendered world projections into SystemPrompt.
// The Session renders all three views (NarrativeView, WorldDebugView,
// CharacterContextView) over the *visible* (post-fog) world before calling
// the GM, so the GM never iterates raw model.World fields directly.
type PromptOptions struct {
	// WorldCtx is the GM-facing full projection — entities, rules, relations
	// — over the visible world. Used to assemble Characters / Locations /
	// Rules sections in the prompt.
	WorldCtx view.WorldDebugContext

	// NarrativeCtx is the narrative-filtered slice — recent events (truncated),
	// active threads only, public memories only. Used for narrative-state
	// sections in the prompt.
	NarrativeCtx view.NarrativeContext

	// CharacterCtx is one entry per player carrying the perspective entity
	// plus that player's visible memories.
	CharacterCtx []view.CharacterContext

	// FogEnabled toggles the Discovery Protocol section in the prompt.
	FogEnabled bool
}
