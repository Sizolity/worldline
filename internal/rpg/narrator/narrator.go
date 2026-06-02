package narrator

import (
	"fmt"

	"github.com/sizolity/worldline/internal/agent/typed"
	"github.com/sizolity/worldline/internal/rpg/mod"
)

// Narrator is a generic LLM-driven GM with no rule system. It provides
// narrative generation, progressive tool disclosure, and action suggestion.
//
// Per v1 mod migration: the narrator carries the three persona docs
// (narrator / lorekeeper / suggester) sourced from mod/styles/<id>/. The
// engine ships zero hardcoded prompt copies — callers MUST supply a
// mod.Style via WithStyle, and that style MUST contain all three
// persona docs. New returns an error otherwise.
type Narrator struct {
	suggestAgent      typed.Agent[SuggestParams]
	narratorPersona   *mod.Document
	lorekeeperPersona *mod.Document
	suggesterPersona  *mod.Document
}

// Option customizes the Narrator at construction time.
type Option func(*Narrator)

// WithStyle wires a loaded mod.Style into the narrator. The style is
// the sole source of persona docs — the engine ships no fallback copy.
// Pass a style that has all three persona docs (narrator, lorekeeper,
// action_suggester) populated, or New will reject construction.
func WithStyle(style *mod.Style) Option {
	return func(n *Narrator) {
		if style == nil {
			return
		}
		if style.NarratorPersona != nil {
			n.narratorPersona = style.NarratorPersona
		}
		if style.LorekeeperPersona != nil {
			n.lorekeeperPersona = style.LorekeeperPersona
		}
		if style.SuggesterPersona != nil {
			n.suggesterPersona = style.SuggesterPersona
		}
	}
}

// New constructs a Narrator. suggestAgent is the structured agent used for
// SuggestActions (forced tool-call mode). A style with all three persona
// docs (narrator, lorekeeper, action_suggester) MUST be supplied via
// WithStyle — the engine carries no embedded fallback. Missing persona
// is an error.
func New(suggestAgent typed.Agent[SuggestParams], opts ...Option) (*Narrator, error) {
	if suggestAgent == nil {
		return nil, fmt.Errorf("suggestAgent is required: GM is LLM-driven")
	}
	n := &Narrator{suggestAgent: suggestAgent}
	for _, opt := range opts {
		opt(n)
	}
	if n.narratorPersona == nil || n.lorekeeperPersona == nil || n.suggesterPersona == nil {
		return nil, fmt.Errorf("narrator: style required; use WithStyle(mod.LoadStyle(...)) and ensure persona/{narrator,lorekeeper,action_suggester}.md exist")
	}
	return n, nil
}

// Role returns the stable, human-readable label "Narrator". Used in prompts
// and logs; not a machine identifier.
func (n *Narrator) Role() string {
	return "Narrator"
}
