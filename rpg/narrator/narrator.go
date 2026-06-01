package narrator

import (
	"fmt"

	"github.com/sizolity/worldline/agent/structured"
	"github.com/sizolity/worldline/internal/app/mod"
)

// Narrator is a generic LLM-driven GM with no rule system. It provides
// narrative generation, progressive tool disclosure, and action suggestion.
//
// Per v1 mod migration: the narrator carries optional style persona docs
// (narrator / lorekeeper / suggester) sourced from mod/styles/<id>/. When
// they are nil — e.g. legacy tests — the narrator falls back to embedded
// default personas defined in default_persona.go.
type Narrator struct {
	suggestAgent     structured.Agent[SuggestParams]
	narratorPersona  *mod.Document
	lorekeeperPersona *mod.Document
	suggesterPersona *mod.Document
}

// Option customizes the Narrator at construction time.
type Option func(*Narrator)

// WithStyle wires a loaded mod.Style into the narrator. Any nil persona
// inside the style falls back to the embedded default for that role.
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
// SuggestActions (forced tool-call mode). When no Option configures a
// style persona the narrator uses embedded defaults so existing callers
// keep working unchanged.
func New(suggestAgent structured.Agent[SuggestParams], opts ...Option) (*Narrator, error) {
	if suggestAgent == nil {
		return nil, fmt.Errorf("suggestAgent is required: GM is LLM-driven")
	}
	n := &Narrator{
		suggestAgent:      suggestAgent,
		narratorPersona:   defaultNarratorPersona(),
		lorekeeperPersona: defaultLorekeeperPersona(),
		suggesterPersona:  defaultSuggesterPersona(),
	}
	for _, opt := range opts {
		opt(n)
	}
	return n, nil
}

// Role returns the stable, human-readable label "Narrator". Used in prompts
// and logs; not a machine identifier.
func (n *Narrator) Role() string {
	return "Narrator"
}
