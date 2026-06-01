package status

import (
	"context"
	"path/filepath"
	"sort"

	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/store"
	"github.com/sizolity/worldline/internal/rpg/story"
)

// ThreadInfo holds display-ready data for one world thread.
type ThreadInfo struct {
	ID      worldmodel.ThreadID `json:"id"`
	Title   string              `json:"title"`
	Status  string              `json:"status"`
	Tension float64             `json:"tension"`
}

// WorldLineInfo holds display-ready data for one world line.
type WorldLineInfo struct {
	ID                string `json:"id"`
	ThreadID          string `json:"thread_id"`
	Visibility        string `json:"visibility"`
	MilestonesTotal   int    `json:"milestones_total"`
	MilestonesTrigged int    `json:"milestones_triggered"`
}

// EventInfo holds a condensed event log entry.
type EventInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// EntityCounts tallies entities by type.
type EntityCounts struct {
	Character int `json:"character"`
	Location  int `json:"location"`
	Item      int `json:"item"`
	Other     int `json:"other"`
	Total     int `json:"total"`
}

// MemoryCounts tallies memories by owner kind.
type MemoryCounts struct {
	World     int `json:"world"`
	Character int `json:"character"`
	Other     int `json:"other"`
	Total     int `json:"total"`
}

// NPCMemoryStat ranks a character entity by sedimented memory count.
type NPCMemoryStat struct {
	Name  string             `json:"name"`
	ID    worldmodel.EntityID `json:"id"`
	Count int                `json:"count"`
}

// Report is the structured result of Build, ready for JSON serialization
// by the server or text rendering by the CLI.
type Report struct {
	WorldID       string             `json:"world_id"`
	WorldName     string             `json:"world_name"`
	ClockSequence int64              `json:"clock_sequence"`
	ClockKind     worldmodel.WorldTimeKind `json:"clock_kind"`
	Threads       []ThreadInfo       `json:"threads"`
	WorldLines    []WorldLineInfo    `json:"world_lines,omitempty"`
	RecentEvents  []EventInfo        `json:"recent_events,omitempty"`
	EntityCounts  EntityCounts       `json:"entity_counts"`
	MemoryCounts  MemoryCounts       `json:"memory_counts"`
	NPCMemoryTop  []NPCMemoryStat    `json:"npc_memory_top,omitempty"`
}

// Build loads the world and assembles a status report. The tail parameter
// controls how many recent EventLog entries are included; use 0 to skip.
func Build(ctx context.Context, workspace, worldID string, tail int) (*Report, error) {
	w, err := store.NewFileStore(workspace).LoadSnapshot(ctx, worldID)
	if err != nil {
		return nil, err
	}

	r := &Report{
		WorldID:       string(w.ID),
		WorldName:     w.Name,
		ClockSequence: w.Clock.Sequence,
		ClockKind:     w.Clock.Current.Kind,
	}

	for _, t := range w.Threads {
		r.Threads = append(r.Threads, ThreadInfo{
			ID:      t.ID,
			Title:   t.Title,
			Status:  t.Status,
			Tension: t.Tension,
		})
	}

	lines, _ := story.NewStore(filepath.Join(workspace, "worlds")).Load(worldID)
	for _, l := range lines {
		triggered := 0
		for _, m := range l.Milestones {
			if m.Triggered {
				triggered++
			}
		}
		r.WorldLines = append(r.WorldLines, WorldLineInfo{
			ID:                l.ID,
			ThreadID:          string(l.ThreadID),
			Visibility:        string(l.Visibility),
			MilestonesTotal:   len(l.Milestones),
			MilestonesTrigged: triggered,
		})
	}

	if tail > 0 && len(w.EventLog) > 0 {
		start := len(w.EventLog) - tail
		if start < 0 {
			start = 0
		}
		for _, e := range w.EventLog[start:] {
			r.RecentEvents = append(r.RecentEvents, EventInfo{
				ID:          string(e.ID),
				Type:        e.Type,
				Source:      e.Source,
				Description: e.Description,
			})
		}
	}

	r.EntityCounts = buildEntityCounts(w)
	r.MemoryCounts = buildMemoryCounts(w)
	r.NPCMemoryTop = TopNPCsByMemoryCount(w, 5)

	return r, nil
}

func buildEntityCounts(w worldmodel.World) EntityCounts {
	ec := EntityCounts{Total: len(w.Entities)}
	for _, e := range w.Entities {
		switch e.Type {
		case "character":
			ec.Character++
		case "location":
			ec.Location++
		case "item":
			ec.Item++
		default:
			ec.Other++
		}
	}
	return ec
}

func buildMemoryCounts(w worldmodel.World) MemoryCounts {
	mc := MemoryCounts{Total: len(w.Memory)}
	for _, m := range w.Memory {
		switch m.Owner.Kind {
		case worldmodel.MemoryOwnerKindWorld:
			mc.World++
		case worldmodel.MemoryOwnerKindCharacter:
			mc.Character++
		default:
			mc.Other++
		}
	}
	return mc
}

// TopNPCsByMemoryCount returns the top-n character entities ranked by the
// number of memories owned by them. NPCs with zero memories are excluded.
// When n <= 0 the cap is disabled and all qualifying NPCs are returned.
func TopNPCsByMemoryCount(w worldmodel.World, n int) []NPCMemoryStat {
	counts := map[string]int{}
	for _, m := range w.Memory {
		if m.Owner.Kind != worldmodel.MemoryOwnerKindCharacter {
			continue
		}
		if m.Owner.ID == "" {
			continue
		}
		counts[m.Owner.ID]++
	}
	if len(counts) == 0 {
		return nil
	}
	stats := make([]NPCMemoryStat, 0, len(counts))
	for id, c := range counts {
		if c == 0 {
			continue
		}
		eid := worldmodel.EntityID(id)
		name := id
		if e, ok := w.Entities[eid]; ok && e.Name != "" {
			name = e.Name
		}
		stats = append(stats, NPCMemoryStat{Name: name, ID: eid, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].ID < stats[j].ID
	})
	if n > 0 && len(stats) > n {
		stats = stats[:n]
	}
	return stats
}
