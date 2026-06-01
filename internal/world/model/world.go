package model

import (
	"encoding/json"
	"fmt"
)

type WorldID string
type EntityID string
type EventID string
type MemoryID string
type RuleID string
type ThreadID string
type RelationID string
type FactID string

type World struct {
	ID          WorldID             `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Canon       Canon               `json:"canon,omitempty"`
	Clock       WorldClock          `json:"clock,omitempty"`
	Entities    map[EntityID]Entity `json:"entities,omitempty"`
	Relations   []Relation          `json:"relations,omitempty"`
	Facts       []Fact              `json:"facts,omitempty"`
	Rules       []Rule              `json:"rules,omitempty"`
	Threads     []WorldThread       `json:"threads,omitempty"`
	EventLog    []WorldEvent        `json:"event_log,omitempty"`
	EventQueue  []EventQueueItem    `json:"event_queue,omitempty"`
	Memory      []MemoryRecord      `json:"memory,omitempty"`
	Metadata    WorldMetadata       `json:"metadata,omitempty"`
}

func (w World) Validate() error {
	if err := ValidateID(string(w.ID)); err != nil {
		return fmt.Errorf("world.id: %w", err)
	}
	if w.Name == "" {
		return fmt.Errorf("world.name is required")
	}
	return nil
}

type Canon struct {
	Genre      []string   `json:"genre,omitempty"`
	Tone       []string   `json:"tone,omitempty"`
	StyleGuide []string   `json:"style_guide,omitempty"`
	Premise    string     `json:"premise,omitempty"`
	Laws       []string   `json:"laws,omitempty"`
	Boundaries []string   `json:"boundaries,omitempty"`
	Secrets    []EntityID `json:"secrets,omitempty"`
}

type WorldClock struct {
	Current   WorldTime `json:"current,omitempty"`
	Calendar  string    `json:"calendar,omitempty"`
	TimeScale string    `json:"time_scale,omitempty"`
	Sequence  int64     `json:"sequence,omitempty"`
}

type WorldTimeKind string

const (
	WorldTimeTick     WorldTimeKind = "tick"
	WorldTimeTurn     WorldTimeKind = "turn"
	WorldTimeScene    WorldTimeKind = "scene"
	WorldTimeChapter  WorldTimeKind = "chapter"
	WorldTimeDay      WorldTimeKind = "day"
	WorldTimeCalendar WorldTimeKind = "calendar_time"
)

type WorldTime struct {
	Kind     WorldTimeKind  `json:"kind,omitempty"`
	Tick     int64          `json:"tick,omitempty"`
	Label    string         `json:"label,omitempty"`
	Calendar map[string]int `json:"calendar,omitempty"`
}

type WorldMetadata struct {
	SchemaVersion string    `json:"schema_version,omitempty"`
	Source        string    `json:"source,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	Fork          *ForkInfo `json:"fork,omitempty"`
}

// ForkInfo records the lineage of a forked world.
type ForkInfo struct {
	ParentWorldID WorldID `json:"parent_world_id"`
	ForkSequence  int64   `json:"fork_sequence"`
}

type EventQueueItem struct {
	Event       WorldEvent `json:"event"`
	Priority    int        `json:"priority,omitempty"`
	NotBefore   WorldTime  `json:"not_before,omitempty"`
	CreatedBy   string     `json:"created_by,omitempty"`
	ErrorPolicy string     `json:"error_policy,omitempty"`
	Attempts    int        `json:"attempts,omitempty"`
	MaxAttempts int        `json:"max_attempts,omitempty"`
}

const (
	QueueErrorPolicyFail  = "fail"
	QueueErrorPolicySkip  = "skip"
	QueueErrorPolicyRetry = "retry"
)

func (i EventQueueItem) Validate() error {
	if err := i.Event.Validate(); err != nil {
		return fmt.Errorf("event: %w", err)
	}
	if i.CreatedBy != "" {
		if err := ValidateID(i.CreatedBy); err != nil {
			return fmt.Errorf("created_by: %w", err)
		}
	}
	if !isSupportedQueueErrorPolicy(i.ErrorPolicy) {
		return fmt.Errorf("unsupported error_policy %q", i.ErrorPolicy)
	}
	if i.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts must be >= 0")
	}
	return nil
}

func isSupportedQueueErrorPolicy(policy string) bool {
	switch policy {
	case "", QueueErrorPolicyFail, QueueErrorPolicySkip, QueueErrorPolicyRetry:
		return true
	default:
		return false
	}
}

func (i *EventQueueItem) UnmarshalJSON(data []byte) error {
	type eventQueueItem EventQueueItem
	var item eventQueueItem
	if err := json.Unmarshal(data, &item); err == nil && item.Event.ID != "" {
		*i = EventQueueItem(item)
		return nil
	}

	var event WorldEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	*i = EventQueueItem{Event: event}
	return nil
}
