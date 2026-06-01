package model

import "fmt"

type WorldThread struct {
	ID             ThreadID   `json:"id"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary,omitempty"`
	Status         string     `json:"status"`
	Priority       float64    `json:"priority,omitempty"`
	Tension        float64    `json:"tension,omitempty"`
	ParticipantIDs []EntityID `json:"participant_ids,omitempty"`
	LocationID     EntityID   `json:"location_id,omitempty"`
}

const (
	ThreadKindQuest        = "quest"
	ThreadKindConflict     = "conflict"
	ThreadKindMystery      = "mystery"
	ThreadKindRelationship = "relationship"
	ThreadKindPersonal     = "personal"
	ThreadKindWorldEvent   = "world_event"
)

const (
	ThreadStatusOpen      = "open"
	ThreadStatusActive    = "active"
	ThreadStatusDormant   = "dormant"
	ThreadStatusResolved  = "resolved"
	ThreadStatusFailed    = "failed"
	ThreadStatusAbandoned = "abandoned"
)

func (t WorldThread) Validate() error {
	if err := ValidateID(string(t.ID)); err != nil {
		return fmt.Errorf("thread.id: %w", err)
	}
	if t.Title == "" {
		return fmt.Errorf("thread.title is required")
	}
	if t.Kind == "" {
		return fmt.Errorf("thread.kind is required")
	}
	if !isSupportedThreadKind(t.Kind) {
		return fmt.Errorf("unsupported thread kind %q", t.Kind)
	}
	if t.Status == "" {
		return fmt.Errorf("thread.status is required")
	}
	if !isSupportedThreadStatus(t.Status) {
		return fmt.Errorf("unsupported thread status %q", t.Status)
	}
	if t.Priority < 0 || t.Priority > 1 {
		return fmt.Errorf("thread.priority must be between 0 and 1")
	}
	if t.Tension < 0 || t.Tension > 1 {
		return fmt.Errorf("thread.tension must be between 0 and 1")
	}
	return nil
}

func isSupportedThreadKind(kind string) bool {
	switch kind {
	case ThreadKindQuest, ThreadKindConflict, ThreadKindMystery, ThreadKindRelationship, ThreadKindPersonal, ThreadKindWorldEvent:
		return true
	default:
		return false
	}
}

func isSupportedThreadStatus(status string) bool {
	switch status {
	case ThreadStatusOpen, ThreadStatusActive, ThreadStatusDormant, ThreadStatusResolved, ThreadStatusFailed, ThreadStatusAbandoned:
		return true
	default:
		return false
	}
}
