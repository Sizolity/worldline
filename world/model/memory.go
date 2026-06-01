package model

import "fmt"

type MemoryRecord struct {
	ID          MemoryID    `json:"id"`
	Owner       MemoryOwner `json:"owner"`
	Scope       string      `json:"scope,omitempty"`
	Kind        string      `json:"kind,omitempty"`
	SubjectIDs  []EntityID  `json:"subject_ids,omitempty"`
	EventIDs    []EventID   `json:"event_ids,omitempty"`
	Content     string      `json:"content,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	TruthStatus string      `json:"truth_status,omitempty"`
	Confidence  float64     `json:"confidence,omitempty"`
	Importance  float64     `json:"importance,omitempty"`
}

type MemoryOwner struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

const (
	MemoryOwnerKindWorld     = "world"
	MemoryOwnerKindCharacter = "character"
	MemoryOwnerKindFaction   = "faction"
	MemoryOwnerKindNarrator  = "narrator"
)

const (
	MemoryScopeCanonical  = "canonical"
	MemoryScopeFactual    = "factual"
	MemoryScopeSubjective = "subjective"
	MemoryScopeRumor      = "rumor"
	MemoryScopeEmotional  = "emotional"
	MemoryScopeProcedural = "procedural"
)

const (
	MemoryKindObservation = "observation"
	MemoryKindBelief      = "belief"
	MemoryKindRumor       = "rumor"
	MemoryKindSummary     = "summary"
)

const (
	TruthStatusTrue     = "true"
	TruthStatusFalse    = "false"
	TruthStatusUnknown  = "unknown"
	TruthStatusDisputed = "disputed"
	TruthStatusOutdated = "outdated"
	TruthStatusSecret   = "secret"
)

func (m MemoryRecord) Validate() error {
	if err := ValidateID(string(m.ID)); err != nil {
		return fmt.Errorf("memory.id: %w", err)
	}
	if m.Owner.Kind == "" {
		return fmt.Errorf("memory.owner.kind is required")
	}
	switch m.Owner.Kind {
	case MemoryOwnerKindWorld, MemoryOwnerKindCharacter, MemoryOwnerKindFaction, MemoryOwnerKindNarrator:
	default:
		return fmt.Errorf("unsupported memory owner kind %q", m.Owner.Kind)
	}
	if m.Owner.Kind != MemoryOwnerKindWorld && m.Owner.ID == "" {
		return fmt.Errorf("memory.owner.id is required")
	}
	if m.Owner.ID != "" {
		if err := ValidateID(m.Owner.ID); err != nil {
			return fmt.Errorf("memory.owner.id: %w", err)
		}
	}
	if !isSupportedMemoryScope(m.Scope) {
		return fmt.Errorf("unsupported memory scope %q", m.Scope)
	}
	if !isSupportedMemoryKind(m.Kind) {
		return fmt.Errorf("unsupported memory kind %q", m.Kind)
	}
	if !isSupportedTruthStatus(m.TruthStatus) {
		return fmt.Errorf("unsupported memory truth status %q", m.TruthStatus)
	}
	if m.Content == "" && m.Summary == "" {
		return fmt.Errorf("memory content or summary is required")
	}
	if m.Confidence < 0 || m.Confidence > 1 {
		return fmt.Errorf("memory.confidence must be between 0 and 1")
	}
	if m.Importance < 0 || m.Importance > 1 {
		return fmt.Errorf("memory.importance must be between 0 and 1")
	}
	return nil
}

func isSupportedMemoryScope(scope string) bool {
	switch scope {
	case "", MemoryScopeCanonical, MemoryScopeFactual, MemoryScopeSubjective, MemoryScopeRumor, MemoryScopeEmotional, MemoryScopeProcedural:
		return true
	default:
		return false
	}
}

func isSupportedMemoryKind(kind string) bool {
	switch kind {
	case "", MemoryKindObservation, MemoryKindBelief, MemoryKindRumor, MemoryKindSummary:
		return true
	default:
		return false
	}
}

func isSupportedTruthStatus(status string) bool {
	switch status {
	case "", TruthStatusTrue, TruthStatusFalse, TruthStatusUnknown, TruthStatusDisputed, TruthStatusOutdated, TruthStatusSecret:
		return true
	default:
		return false
	}
}
