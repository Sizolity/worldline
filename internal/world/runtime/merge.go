package runtime

import (
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

// MergeConflict describes a single conflict encountered during a 3-way merge.
type MergeConflict struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Desc string `json:"desc"`
}

// MergeReport summarizes the outcome of a 3-way merge.
type MergeReport struct {
	Conflicts       []MergeConflict `json:"conflicts"`
	EntitiesAdded   []string        `json:"entities_added"`
	EntitiesRemoved []string        `json:"entities_removed"`
	ThreadsAdded    []string        `json:"threads_added"`
	ThreadsRemoved  []string        `json:"threads_removed"`
	MemoriesAdded   []string        `json:"memories_added"`
	EventsAdded     []string        `json:"events_added"`
	FactsAdded      []string        `json:"facts_added"`
	RelationsAdded  []string        `json:"relations_added"`
	RulesAdded      []string        `json:"rules_added"`
}

// HasConflicts returns true if the merge encountered any conflicts.
func (r MergeReport) HasConflicts() bool {
	return len(r.Conflicts) > 0
}

// MergeWorlds performs a 3-way merge. It computes what changed between base
// and source, then applies non-conflicting changes to target.
//
// Conflict rules:
//   - Entity modified in both source and target (relative to base) → conflict, keep target version.
//   - Entity removed in source but modified in target → conflict, keep target version.
//   - Thread status changed in both branches → conflict, keep target version.
//   - Thread removed in source but status-changed in target → conflict, keep target version.
//
// Non-conflicting changes from source are applied:
//   - New entities, threads, facts, relations, memories, events, rules are added.
//   - Entities/threads removed in source (and untouched in target) are removed.
//   - Entity modifications in source (untouched in target) are applied.
//   - Thread status changes in source (untouched in target) are applied.
//   - Clock sequence is set to max(source, target).
//
// The returned world has the target's ID and metadata.
func MergeWorlds(base, source, target model.World) (model.World, MergeReport) {
	baseSrc := DiffWorlds(base, source)
	baseTgt := DiffWorlds(base, target)

	out := target.Clone()
	report := MergeReport{
		Conflicts:       []MergeConflict{},
		EntitiesAdded:   []string{},
		EntitiesRemoved: []string{},
		ThreadsAdded:    []string{},
		ThreadsRemoved:  []string{},
		MemoriesAdded:   []string{},
		EventsAdded:     []string{},
		FactsAdded:      []string{},
		RelationsAdded:  []string{},
		RulesAdded:      []string{},
	}

	mergeEntities(&out, base, source, target, baseSrc, baseTgt, &report)
	mergeThreads(&out, base, source, baseSrc, baseTgt, &report)
	mergeSlice(&out.Facts, source.Facts, baseSrc.Facts, &report.FactsAdded, "fact")
	mergeSlice(&out.Relations, source.Relations, baseSrc.Relations, &report.RelationsAdded, "relation")
	mergeSlice(&out.Memory, source.Memory, baseSrc.Memories, &report.MemoriesAdded, "memory")
	mergeSlice(&out.EventLog, source.EventLog, baseSrc.Events, &report.EventsAdded, "event")
	mergeSlice(&out.Rules, source.Rules, baseSrc.Rules, &report.RulesAdded, "rule")

	if source.Clock.Sequence > out.Clock.Sequence {
		out.Clock.Sequence = source.Clock.Sequence
	}

	return out, report
}

func mergeEntities(out *model.World, base, source, target model.World, baseSrc, baseTgt WorldDiff, report *MergeReport) {
	if out.Entities == nil {
		out.Entities = make(map[model.EntityID]model.Entity)
	}

	tgtChangedSet := toSet(ItemChangeIDs(baseTgt.Entities.Changed))
	tgtRemovedSet := toSet(baseTgt.Entities.Removed)

	for _, id := range baseSrc.Entities.Added {
		eid := model.EntityID(id)
		if _, exists := out.Entities[eid]; !exists {
			out.Entities[eid] = source.Entities[eid]
			report.EntitiesAdded = append(report.EntitiesAdded, id)
		}
	}

	for _, id := range ItemChangeIDs(baseSrc.Entities.Changed) {
		if tgtChangedSet[id] || tgtRemovedSet[id] {
			report.Conflicts = append(report.Conflicts, MergeConflict{
				Kind: "entity",
				ID:   id,
				Desc: fmt.Sprintf("entity %q modified in both branches", id),
			})
			continue
		}
		eid := model.EntityID(id)
		out.Entities[eid] = source.Entities[eid]
	}

	for _, id := range baseSrc.Entities.Removed {
		eid := model.EntityID(id)
		if tgtChangedSet[id] {
			report.Conflicts = append(report.Conflicts, MergeConflict{
				Kind: "entity",
				ID:   id,
				Desc: fmt.Sprintf("entity %q removed in source but modified in target", id),
			})
			continue
		}
		delete(out.Entities, eid)
		report.EntitiesRemoved = append(report.EntitiesRemoved, id)
	}
}

func mergeThreads(out *model.World, base, source model.World, baseSrc, baseTgt WorldDiff, report *MergeReport) {
	tgtStatusSet := make(map[string]bool)
	for _, sc := range baseTgt.Threads.StatusChanged {
		tgtStatusSet[sc.ID] = true
	}
	tgtRemovedSet := toSet(baseTgt.Threads.Removed)

	srcThreadMap := make(map[model.ThreadID]model.WorldThread, len(source.Threads))
	for _, t := range source.Threads {
		srcThreadMap[t.ID] = t
	}

	outThreadMap := make(map[model.ThreadID]int, len(out.Threads))
	for i, t := range out.Threads {
		outThreadMap[t.ID] = i
	}

	for _, id := range baseSrc.Threads.Added {
		tid := model.ThreadID(id)
		if _, exists := outThreadMap[tid]; !exists {
			if t, ok := srcThreadMap[tid]; ok {
				out.Threads = append(out.Threads, t)
				report.ThreadsAdded = append(report.ThreadsAdded, id)
			}
		}
	}

	for _, sc := range baseSrc.Threads.StatusChanged {
		if tgtStatusSet[sc.ID] || tgtRemovedSet[sc.ID] {
			report.Conflicts = append(report.Conflicts, MergeConflict{
				Kind: "thread",
				ID:   sc.ID,
				Desc: fmt.Sprintf("thread %q status changed in both branches", sc.ID),
			})
			continue
		}
		tid := model.ThreadID(sc.ID)
		if idx, ok := outThreadMap[tid]; ok {
			out.Threads[idx].Status = sc.StatusB
		}
	}

	for _, id := range baseSrc.Threads.Removed {
		tid := model.ThreadID(id)
		if tgtStatusSet[id] {
			report.Conflicts = append(report.Conflicts, MergeConflict{
				Kind: "thread",
				ID:   id,
				Desc: fmt.Sprintf("thread %q removed in source but status-changed in target", id),
			})
			continue
		}
		filtered := out.Threads[:0]
		for _, t := range out.Threads {
			if t.ID != tid {
				filtered = append(filtered, t)
			}
		}
		out.Threads = filtered
		report.ThreadsRemoved = append(report.ThreadsRemoved, id)
	}
}

func mergeSlice[T any](target *[]T, source []T, diff SliceDiff, added *[]string, _ string) {
	if len(diff.Added) == 0 {
		return
	}
	addedSet := toSet(diff.Added)
	existingIDs := sliceIDSet(*target)

	for _, item := range source {
		id := extractID(item)
		if addedSet[id] && !existingIDs[id] {
			*target = append(*target, item)
			*added = append(*added, id)
		}
	}
}

func extractID(v any) string {
	switch typed := v.(type) {
	case model.Fact:
		return string(typed.ID)
	case model.Relation:
		return string(typed.ID)
	case model.MemoryRecord:
		return string(typed.ID)
	case model.WorldEvent:
		return string(typed.ID)
	case model.Rule:
		return string(typed.ID)
	default:
		return ""
	}
}

func sliceIDSet[T any](s []T) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, item := range s {
		id := extractID(item)
		if id != "" {
			m[id] = true
		}
	}
	return m
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

