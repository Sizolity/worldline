package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// WorldStats holds aggregate counts and metrics for a world snapshot.
type WorldStats struct {
	WorldID  string `json:"world_id"`
	Name     string `json:"name"`
	Sequence int64  `json:"sequence"`
	Tick     int64  `json:"tick"`

	EntityCount     int            `json:"entity_count"`
	EntitiesByType  map[string]int `json:"entities_by_type"`
	RelationCount   int            `json:"relation_count"`
	FactCount       int            `json:"fact_count"`
	MemoryCount     int            `json:"memory_count"`
	MemoriesByOwner map[string]int `json:"memories_by_owner"`
	ThreadCount     int            `json:"thread_count"`
	ThreadsByStatus map[string]int `json:"threads_by_status"`
	EventLogCount   int            `json:"event_log_count"`
	QueueDepth      int            `json:"queue_depth"`
	RuleCount       int            `json:"rule_count"`
}

// ComputeStats gathers aggregate metrics from a world snapshot.
func ComputeStats(w model.World) WorldStats {
	s := WorldStats{
		WorldID:         string(w.ID),
		Name:            w.Name,
		Sequence:        w.Clock.Sequence,
		Tick:            w.Clock.Current.Tick,
		EntityCount:     len(w.Entities),
		EntitiesByType:  countByField(w.Entities),
		RelationCount:   len(w.Relations),
		FactCount:       len(w.Facts),
		MemoryCount:     len(w.Memories),
		MemoriesByOwner: countMemoriesByOwner(w.Memories),
		ThreadCount:     len(w.Threads),
		ThreadsByStatus: countThreadsByStatus(w.Threads),
		EventLogCount:   len(w.EventLog),
		QueueDepth:      len(w.EventQueue),
		RuleCount:       len(w.Rules),
	}
	return s
}

func countByField(entities map[model.EntityID]model.Entity) map[string]int {
	m := make(map[string]int)
	for _, e := range entities {
		t := e.Type
		if t == "" {
			t = "(untyped)"
		}
		m[t]++
	}
	return m
}

func countMemoriesByOwner(memories []model.MemoryRecord) map[string]int {
	m := make(map[string]int)
	for _, mem := range memories {
		key := mem.Owner.Kind
		if mem.Owner.ID != "" {
			key += ":" + mem.Owner.ID
		}
		m[key]++
	}
	return m
}

func countThreadsByStatus(threads []model.WorldThread) map[string]int {
	m := make(map[string]int)
	for _, th := range threads {
		m[th.Status]++
	}
	return m
}

// FormatStats returns a human-readable text summary of world statistics.
func FormatStats(s WorldStats) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Stats — %s (%s)\n\n", s.Name, s.WorldID)
	fmt.Fprintf(&b, "Clock: seq=%d tick=%d\n\n", s.Sequence, s.Tick)

	fmt.Fprintf(&b, "| Collection   | Count |\n")
	fmt.Fprintf(&b, "|--------------|-------|\n")
	fmt.Fprintf(&b, "| Entities     | %5d |\n", s.EntityCount)
	fmt.Fprintf(&b, "| Relations    | %5d |\n", s.RelationCount)
	fmt.Fprintf(&b, "| Facts        | %5d |\n", s.FactCount)
	fmt.Fprintf(&b, "| Memories     | %5d |\n", s.MemoryCount)
	fmt.Fprintf(&b, "| Threads      | %5d |\n", s.ThreadCount)
	fmt.Fprintf(&b, "| Event Log    | %5d |\n", s.EventLogCount)
	fmt.Fprintf(&b, "| Event Queue  | %5d |\n", s.QueueDepth)
	fmt.Fprintf(&b, "| Rules        | %5d |\n", s.RuleCount)

	if len(s.EntitiesByType) > 0 {
		fmt.Fprintf(&b, "\nEntities by type: ")
		fmt.Fprintln(&b, formatCountMap(s.EntitiesByType))
	}
	if len(s.ThreadsByStatus) > 0 {
		fmt.Fprintf(&b, "Threads by status: ")
		fmt.Fprintln(&b, formatCountMap(s.ThreadsByStatus))
	}
	if len(s.MemoriesByOwner) > 0 {
		fmt.Fprintf(&b, "Memories by owner: ")
		fmt.Fprintln(&b, formatCountMap(s.MemoriesByOwner))
	}

	return b.String()
}

func formatCountMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}
