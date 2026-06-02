package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// FormatEntity produces a human-readable detail view of a single entity,
// including its components, state, tags, and cross-references from the world.
func FormatEntity(e model.Entity, w model.World) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", e.Name)
	fmt.Fprintf(&b, "ID: %s\nType: %s\n", e.ID, e.Type)
	if e.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", e.Description)
	}
	if len(e.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(e.Tags, ", "))
	}

	formatEntityComponents(&b, e)
	formatEntityState(&b, e.State)
	formatEntityRelations(&b, e.ID, w)
	formatEntityFacts(&b, e.ID, w)
	formatEntityMemories(&b, e.ID, w)

	return b.String()
}

// FormatEntityList produces a compact listing of all entities in a world.
func FormatEntityList(w model.World) string {
	if len(w.Entities) == 0 {
		return "no entities\n"
	}
	var b strings.Builder
	byType := map[string][]model.Entity{}
	for _, e := range w.Entities {
		byType[e.Type] = append(byType[e.Type], e)
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		ents := byType[t]
		sort.Slice(ents, func(i, j int) bool { return ents[i].ID < ents[j].ID })
		fmt.Fprintf(&b, "%s (%d):\n", t, len(ents))
		for _, e := range ents {
			fmt.Fprintf(&b, "  %s  %s", e.ID, e.Name)
			if e.Description != "" {
				fmt.Fprintf(&b, " — %s", TruncateRunes(e.Description, 60))
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatEntityComponents(b *strings.Builder, e model.Entity) {
	if len(e.Components) == 0 {
		return
	}
	b.WriteString("\n## Components\n")

	if p, ok := e.ProfileComponent(); ok {
		b.WriteString("\n### Profile\n")
		if p.Name != "" {
			fmt.Fprintf(b, "Name: %s\n", p.Name)
		}
		if p.Description != "" {
			fmt.Fprintf(b, "Description: %s\n", p.Description)
		}
	}
	if a, ok := e.ActorComponent(); ok {
		b.WriteString("\n### Actor\n")
		fmt.Fprintf(b, "Can act: %v\n", a.CanAct)
		if len(a.Goals) > 0 {
			b.WriteString("Goals:\n")
			for _, g := range a.Goals {
				fmt.Fprintf(b, "- %s\n", g)
			}
		}
	}
	if s, ok := e.SpatialComponent(); ok {
		b.WriteString("\n### Spatial\n")
		fmt.Fprintf(b, "Location: %s\n", s.LocationID)
	}
	if inv, ok := e.InventoryComponent(); ok {
		fmt.Fprintf(b, "\n### Inventory (%d)\n", len(inv.ItemIDs))
		for _, id := range inv.ItemIDs {
			fmt.Fprintf(b, "- %s\n", id)
		}
	}
	if st, ok := e.StatsComponent(); ok {
		fmt.Fprintf(b, "\n### Stats (%d)\n", len(st.Values))
		keys := make([]string, 0, len(st.Values))
		for k := range st.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(b, "- %s = %v\n", k, st.Values[k].Raw)
		}
	}
}

func formatEntityState(b *strings.Builder, state map[string]model.Value) {
	if len(state) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## State (%d)\n\n", len(state))
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "- %s = %v\n", k, state[k].Raw)
	}
}

func formatEntityRelations(b *strings.Builder, id model.EntityID, w model.World) {
	var outgoing, incoming []model.Relation
	for _, r := range w.Relations {
		if r.SourceID == id {
			outgoing = append(outgoing, r)
		} else if r.TargetID == id {
			incoming = append(incoming, r)
		}
	}
	if len(outgoing)+len(incoming) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Relations (%d)\n\n", len(outgoing)+len(incoming))
	for _, r := range outgoing {
		name := entityName(r.TargetID, w.Entities)
		fmt.Fprintf(b, "→ %s [%s]\n", name, r.Type)
	}
	for _, r := range incoming {
		name := entityName(r.SourceID, w.Entities)
		fmt.Fprintf(b, "← %s [%s]\n", name, r.Type)
	}
}

func formatEntityFacts(b *strings.Builder, id model.EntityID, w model.World) {
	var facts []model.Fact
	for _, f := range w.Facts {
		if f.SubjectID == id {
			facts = append(facts, f)
		}
	}
	if len(facts) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Facts (%d)\n\n", len(facts))
	for _, f := range facts {
		fmt.Fprintf(b, "- %s = %v\n", f.Predicate, f.Value.Raw)
	}
}

func formatEntityMemories(b *strings.Builder, id model.EntityID, w model.World) {
	var memories []model.MemoryRecord
	for _, m := range w.Memories {
		if m.Owner.ID == string(id) || containsEntity(m.SubjectIDs, id) {
			memories = append(memories, m)
		}
	}
	if len(memories) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Memories (%d)\n\n", len(memories))
	for _, m := range memories {
		text := m.Content
		if text == "" {
			text = m.Summary
		}
		fmt.Fprintf(b, "- [%s] %s\n", m.Kind, TruncateRunes(text, 80))
	}
}

func containsEntity(ids []model.EntityID, target model.EntityID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
