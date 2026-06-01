package view

import (
	"sort"

	"github.com/sizolity/worldline/world/model"
)

// MemoryFilter selects a subset of memories by structural attributes.
// All criteria are optional; unset fields are not applied. Criteria
// are ANDed: a memory must pass every set filter to be included.
type MemoryFilter struct {
	// OwnerKinds limits to memories whose owner kind is in the set.
	// Empty means all owner kinds are accepted.
	OwnerKinds []string

	// SubjectIDs keeps memories that mention at least one of these
	// entity IDs in their SubjectIDs field. Empty means no subject
	// filtering. Memories with empty SubjectIDs always pass.
	SubjectIDs []model.EntityID

	// MinImportance drops memories below this threshold (0-1).
	// Zero means no threshold.
	MinImportance float64

	// ExcludeTruthStatus drops memories with any of these truth statuses.
	ExcludeTruthStatus []string

	// MaxCount caps the result after sorting by importance descending.
	// Zero means no cap.
	MaxCount int
}

// Filter applies all criteria and returns the matching memories.
// The returned slice is a new allocation; the input is not modified.
func (f MemoryFilter) Filter(memories []model.MemoryRecord) []model.MemoryRecord {
	out := make([]model.MemoryRecord, 0, len(memories))
	ownerSet := toStringSet(f.OwnerKinds)
	excludeSet := toStringSet(f.ExcludeTruthStatus)
	subjectSet := toEntityIDSet(f.SubjectIDs)

	for _, m := range memories {
		if len(ownerSet) > 0 && !ownerSet[m.Owner.Kind] {
			continue
		}
		if len(excludeSet) > 0 && excludeSet[m.TruthStatus] {
			continue
		}
		if f.MinImportance > 0 && m.Importance < f.MinImportance {
			continue
		}
		if len(subjectSet) > 0 && len(m.SubjectIDs) > 0 && !hasOverlap(m.SubjectIDs, subjectSet) {
			continue
		}
		out = append(out, m)
	}

	if f.MaxCount > 0 && len(out) > f.MaxCount {
		sort.Slice(out, func(i, j int) bool {
			return out[i].Importance > out[j].Importance
		})
		out = out[:f.MaxCount]
	}

	return out
}

func toStringSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func toEntityIDSet(ids []model.EntityID) map[model.EntityID]bool {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[model.EntityID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func hasOverlap(ids []model.EntityID, set map[model.EntityID]bool) bool {
	for _, id := range ids {
		if set[id] {
			return true
		}
	}
	return false
}
