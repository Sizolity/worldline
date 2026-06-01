// Package story defines RPG-layer dynamic world lines: long-running narrative
// tracks that evolve over time independently of player action.
//
// A WorldLine is product-layer metadata layered on top of model.WorldThread.
// The thread is the runtime fact source (tension, status, participants); the
// WorldLine carries RPG-specific knobs the world framework intentionally does
// not encode: drift per time scale, milestone triggers, visibility to the PL.
//
// This package is purely deterministic. Per spec §2.4 LLM Boundary it MUST NOT
// call an LLM. The scheduler reads world state, advances tension by drift,
// evaluates milestone conditions in Go, and emits model.WorldEvent values that
// the session applies through worldruntime.ApplyEvent.
package story

import (
	"fmt"

	"github.com/sizolity/worldline/world/model"
)

// Visibility marks how aware the player is of a WorldLine.
// Distinct from rpg/fog visibility (which is entity-level): a hidden line can
// still leak signals (rumors, environmental anomalies) without exposing the
// underlying entities.
type Visibility string

const (
	VisibilityHidden   Visibility = "hidden"
	VisibilityHinted   Visibility = "hinted"
	VisibilityKnown    Visibility = "known"
	VisibilityActive   Visibility = "active"
	VisibilityResolved Visibility = "resolved"
)

// IsValid reports whether v is a known visibility constant.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityHidden, VisibilityHinted, VisibilityKnown, VisibilityActive, VisibilityResolved:
		return true
	}
	return false
}

// Drift defines how much tension the line accrues per time-scale tick, when no
// player action explicitly advances it. Values are tension deltas, applied as
// `thread.Tension = clamp(thread.Tension + drift, 0, 1)` by the scheduler.
//
// Use small positive values for slow-burn lines (e.g. Scene: 0.02) and larger
// values for urgent ones (e.g. Chapter: 0.35).
type Drift struct {
	Scene   float64 `json:"scene,omitempty"`
	Day     float64 `json:"day,omitempty"`
	Chapter float64 `json:"chapter,omitempty"`
}

// MilestoneCondition is a deterministic predicate evaluated by the scheduler.
//
// Supported kinds (MVP):
//   - "thread_tension_gte": Args{"thread_id": string, "threshold": float64}
//   - "entity_state_eq":    Args{"entity_id": string, "key": string, "value": any}
//   - "fact_exists":        Args{"subject_id": string, "predicate": string}
//
// Future kinds may be added; unknown kinds cause the scheduler to surface an
// error rather than silently skip, so config errors are loud.
type MilestoneCondition struct {
	Kind string         `json:"kind"`
	Args map[string]any `json:"args,omitempty"`
}

// Milestone is a one-shot effect bundle gated on a condition. When the
// condition first becomes true, the scheduler emits a WorldEvent carrying
// Effects, then sets Triggered=true so the milestone never fires again.
type Milestone struct {
	ID        string             `json:"id"`
	Condition MilestoneCondition `json:"condition"`
	Effects   []model.Effect     `json:"effects,omitempty"`
	Triggered bool               `json:"triggered,omitempty"`
}

// WorldLine is one product-layer evolution track. The runtime tension and
// status live on the linked model.WorldThread; this struct carries only the
// RPG-specific orchestration metadata.
type WorldLine struct {
	ID           string         `json:"id"`
	ThreadID     model.ThreadID `json:"thread_id"`
	Visibility   Visibility     `json:"visibility"`
	CurrentStage string         `json:"current_stage,omitempty"`
	Drift        Drift          `json:"drift,omitempty"`
	Milestones   []Milestone    `json:"milestones,omitempty"`
}

// Validate checks that the line satisfies static invariants.
func (l WorldLine) Validate() error {
	if l.ID == "" {
		return fmt.Errorf("worldline.id is required")
	}
	if l.ThreadID == "" {
		return fmt.Errorf("worldline.thread_id is required")
	}
	if l.Visibility != "" && !l.Visibility.IsValid() {
		return fmt.Errorf("worldline.visibility %q is invalid", l.Visibility)
	}
	for i, m := range l.Milestones {
		if m.ID == "" {
			return fmt.Errorf("worldline.milestones[%d].id is required", i)
		}
		if m.Condition.Kind == "" {
			return fmt.Errorf("worldline.milestones[%d].condition.kind is required", i)
		}
	}
	return nil
}
