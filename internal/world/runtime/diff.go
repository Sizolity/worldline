package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// WorldDiff describes the structural differences between two worlds.
type WorldDiff struct {
	WorldA string `json:"world_a"`
	WorldB string `json:"world_b"`

	ClockA int64 `json:"clock_a"`
	ClockB int64 `json:"clock_b"`

	Entities  EntityDiff `json:"entities"`
	Facts     SliceDiff  `json:"facts"`
	Relations SliceDiff  `json:"relations"`
	Memories  SliceDiff  `json:"memories"`
	Threads   ThreadDiff `json:"threads"`
	Events    SliceDiff  `json:"events"`
	Rules     SliceDiff  `json:"rules"`
}

// EntityDiff summarizes entity-level changes.
type EntityDiff struct {
	Added   []string     `json:"added"`
	Removed []string     `json:"removed"`
	Changed []ItemChange `json:"changed"`
}

// SliceDiff summarizes additions/removals/changes for ID-bearing collections.
type SliceDiff struct {
	Added   []string     `json:"added"`
	Removed []string     `json:"removed"`
	Changed []ItemChange `json:"changed,omitempty"`
}

// ThreadDiff includes status changes on top of add/remove.
type ThreadDiff struct {
	Added         []string       `json:"added"`
	Removed       []string       `json:"removed"`
	StatusChanged []ThreadChange `json:"status_changed"`
	Changed       []ItemChange   `json:"changed,omitempty"`
}

// ThreadChange records a thread whose status differs between worlds.
type ThreadChange struct {
	ID      string `json:"id"`
	StatusA string `json:"status_a"`
	StatusB string `json:"status_b"`
}

// ItemChange records field-level changes on an item that exists in both worlds.
type ItemChange struct {
	ID     string       `json:"id"`
	Fields []FieldDelta `json:"fields"`
}

// FieldDelta records a single field difference.
type FieldDelta struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// ItemChangeIDs extracts just the IDs from a slice of ItemChange.
func ItemChangeIDs(changes []ItemChange) []string {
	ids := make([]string, len(changes))
	for i, c := range changes {
		ids[i] = c.ID
	}
	return ids
}

// DiffWorlds computes the structural delta between two world snapshots.
func DiffWorlds(a, b model.World) WorldDiff {
	d := WorldDiff{
		WorldA: string(a.ID),
		WorldB: string(b.ID),
		ClockA: a.Clock.Sequence,
		ClockB: b.Clock.Sequence,
	}

	d.Entities = diffEntities(a.Entities, b.Entities)
	d.Facts = diffFacts(a.Facts, b.Facts)
	d.Relations = diffRelations(a.Relations, b.Relations)
	d.Memories = diffMemories(a.Memory, b.Memory)
	d.Threads = diffThreads(a.Threads, b.Threads)
	d.Events = diffByID(eventIDs(a.EventLog), eventIDs(b.EventLog))
	d.Rules = diffByID(ruleIDs(a.Rules), ruleIDs(b.Rules))

	return d
}

func diffEntities(a, b map[model.EntityID]model.Entity) EntityDiff {
	d := EntityDiff{
		Added:   []string{},
		Removed: []string{},
		Changed: []ItemChange{},
	}

	for id, entA := range a {
		entB, ok := b[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if fields := diffEntityFields(entA, entB); len(fields) > 0 {
			d.Changed = append(d.Changed, ItemChange{ID: string(id), Fields: fields})
		}
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func diffEntityFields(a, b model.Entity) []FieldDelta {
	var deltas []FieldDelta
	if a.Name != b.Name {
		deltas = append(deltas, FieldDelta{"name", a.Name, b.Name})
	}
	if a.Type != b.Type {
		deltas = append(deltas, FieldDelta{"type", a.Type, b.Type})
	}
	if a.Description != b.Description {
		deltas = append(deltas, FieldDelta{"description", truncateDiff(a.Description, 60), truncateDiff(b.Description, 60)})
	}
	if !stringSliceEqual(a.Tags, b.Tags) {
		deltas = append(deltas, FieldDelta{"tags", strings.Join(a.Tags, ", "), strings.Join(b.Tags, ", ")})
	}
	return deltas
}

func diffFacts(a, b []model.Fact) SliceDiff {
	aMap := make(map[model.FactID]model.Fact, len(a))
	for _, f := range a {
		aMap[f.ID] = f
	}
	bMap := make(map[model.FactID]model.Fact, len(b))
	for _, f := range b {
		bMap[f.ID] = f
	}

	d := SliceDiff{Added: []string{}, Removed: []string{}, Changed: []ItemChange{}}
	for id, fa := range aMap {
		fb, ok := bMap[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if fields := diffFactFields(fa, fb); len(fields) > 0 {
			d.Changed = append(d.Changed, ItemChange{ID: string(id), Fields: fields})
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func diffFactFields(a, b model.Fact) []FieldDelta {
	var deltas []FieldDelta
	if a.SubjectID != b.SubjectID {
		deltas = append(deltas, FieldDelta{"subject_id", string(a.SubjectID), string(b.SubjectID)})
	}
	if a.Predicate != b.Predicate {
		deltas = append(deltas, FieldDelta{"predicate", a.Predicate, b.Predicate})
	}
	aVal := formatValueCompact(a.Value)
	bVal := formatValueCompact(b.Value)
	if aVal != bVal {
		deltas = append(deltas, FieldDelta{"value", aVal, bVal})
	}
	return deltas
}

func diffRelations(a, b []model.Relation) SliceDiff {
	aMap := make(map[model.RelationID]model.Relation, len(a))
	for _, r := range a {
		aMap[r.ID] = r
	}
	bMap := make(map[model.RelationID]model.Relation, len(b))
	for _, r := range b {
		bMap[r.ID] = r
	}

	d := SliceDiff{Added: []string{}, Removed: []string{}, Changed: []ItemChange{}}
	for id, ra := range aMap {
		rb, ok := bMap[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if fields := diffRelationFields(ra, rb); len(fields) > 0 {
			d.Changed = append(d.Changed, ItemChange{ID: string(id), Fields: fields})
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func diffRelationFields(a, b model.Relation) []FieldDelta {
	var deltas []FieldDelta
	if a.Type != b.Type {
		deltas = append(deltas, FieldDelta{"type", a.Type, b.Type})
	}
	if a.SourceID != b.SourceID {
		deltas = append(deltas, FieldDelta{"source_id", string(a.SourceID), string(b.SourceID)})
	}
	if a.TargetID != b.TargetID {
		deltas = append(deltas, FieldDelta{"target_id", string(a.TargetID), string(b.TargetID)})
	}
	return deltas
}

func diffMemories(a, b []model.MemoryRecord) SliceDiff {
	aMap := make(map[model.MemoryID]model.MemoryRecord, len(a))
	for _, m := range a {
		aMap[m.ID] = m
	}
	bMap := make(map[model.MemoryID]model.MemoryRecord, len(b))
	for _, m := range b {
		bMap[m.ID] = m
	}

	d := SliceDiff{Added: []string{}, Removed: []string{}, Changed: []ItemChange{}}
	for id, ma := range aMap {
		mb, ok := bMap[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if fields := diffMemoryFields(ma, mb); len(fields) > 0 {
			d.Changed = append(d.Changed, ItemChange{ID: string(id), Fields: fields})
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func diffMemoryFields(a, b model.MemoryRecord) []FieldDelta {
	var deltas []FieldDelta
	if a.Kind != b.Kind {
		deltas = append(deltas, FieldDelta{"kind", a.Kind, b.Kind})
	}
	if a.Content != b.Content {
		deltas = append(deltas, FieldDelta{"content", truncateDiff(a.Content, 60), truncateDiff(b.Content, 60)})
	}
	if a.Summary != b.Summary {
		deltas = append(deltas, FieldDelta{"summary", truncateDiff(a.Summary, 60), truncateDiff(b.Summary, 60)})
	}
	if a.TruthStatus != b.TruthStatus {
		deltas = append(deltas, FieldDelta{"truth_status", a.TruthStatus, b.TruthStatus})
	}
	if a.Importance != b.Importance {
		deltas = append(deltas, FieldDelta{"importance", fmt.Sprintf("%.2f", a.Importance), fmt.Sprintf("%.2f", b.Importance)})
	}
	if a.Confidence != b.Confidence {
		deltas = append(deltas, FieldDelta{"confidence", fmt.Sprintf("%.2f", a.Confidence), fmt.Sprintf("%.2f", b.Confidence)})
	}
	return deltas
}

func diffByID(a, b map[string]bool) SliceDiff {
	d := SliceDiff{
		Added:   []string{},
		Removed: []string{},
	}
	for id := range a {
		if !b[id] {
			d.Removed = append(d.Removed, id)
		}
	}
	for id := range b {
		if !a[id] {
			d.Added = append(d.Added, id)
		}
	}
	return d
}

func diffThreads(a, b []model.WorldThread) ThreadDiff {
	d := ThreadDiff{
		Added:         []string{},
		Removed:       []string{},
		StatusChanged: []ThreadChange{},
		Changed:       []ItemChange{},
	}

	aMap := make(map[model.ThreadID]model.WorldThread, len(a))
	for _, t := range a {
		aMap[t.ID] = t
	}
	bMap := make(map[model.ThreadID]model.WorldThread, len(b))
	for _, t := range b {
		bMap[t.ID] = t
	}

	for id, ta := range aMap {
		tb, ok := bMap[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if ta.Status != tb.Status {
			d.StatusChanged = append(d.StatusChanged, ThreadChange{
				ID:      string(id),
				StatusA: ta.Status,
				StatusB: tb.Status,
			})
		}
		if fields := diffThreadFields(ta, tb); len(fields) > 0 {
			d.Changed = append(d.Changed, ItemChange{ID: string(id), Fields: fields})
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func diffThreadFields(a, b model.WorldThread) []FieldDelta {
	var deltas []FieldDelta
	if a.Title != b.Title {
		deltas = append(deltas, FieldDelta{"title", a.Title, b.Title})
	}
	if a.Kind != b.Kind {
		deltas = append(deltas, FieldDelta{"kind", a.Kind, b.Kind})
	}
	return deltas
}


func eventIDs(events []model.WorldEvent) map[string]bool {
	m := make(map[string]bool, len(events))
	for _, e := range events {
		m[string(e.ID)] = true
	}
	return m
}

func ruleIDs(rules []model.Rule) map[string]bool {
	m := make(map[string]bool, len(rules))
	for _, r := range rules {
		m[string(r.ID)] = true
	}
	return m
}

// --- helpers ---

func truncateDiff(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aSorted := append([]string(nil), a...)
	bSorted := append([]string(nil), b...)
	sort.Strings(aSorted)
	sort.Strings(bSorted)
	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}
	return true
}

func formatValueCompact(v model.Value) string {
	if v.Raw == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v.Raw)
	if v.Unit != "" {
		s += " " + v.Unit
	}
	return s
}
