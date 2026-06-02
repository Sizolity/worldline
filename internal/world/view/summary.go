package view

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/sizolity/worldline/internal/world/model"
)

// FormatWorldSummary produces a human-readable prose summary of a world snapshot.
func FormatWorldSummary(w model.World) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", w.Name)
	if w.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", w.Description)
	}
	fmt.Fprintf(&b, "\nID: %s | Sequence: %d\n", w.ID, w.Clock.Sequence)

	formatCanon(&b, w.Canon)
	formatEntities(&b, w.Entities)
	formatRelations(&b, w.Relations, w.Entities)
	formatFacts(&b, w.Facts, w.Entities)
	formatThreads(&b, w.Threads)
	formatMemorySummary(&b, w.Memories)
	formatEventSummary(&b, w.EventLog)

	if len(w.EventQueue) > 0 {
		fmt.Fprintf(&b, "\n## Event Queue\n\n%d pending event(s)\n", len(w.EventQueue))
	}

	return b.String()
}

func formatCanon(b *strings.Builder, c model.Canon) {
	if c.Premise == "" && len(c.Genre) == 0 && len(c.Tone) == 0 && len(c.Laws) == 0 {
		return
	}
	b.WriteString("\n## Canon\n")
	if c.Premise != "" {
		fmt.Fprintf(b, "\n%s\n", c.Premise)
	}
	if len(c.Genre) > 0 {
		fmt.Fprintf(b, "\nGenre: %s\n", strings.Join(c.Genre, ", "))
	}
	if len(c.Tone) > 0 {
		fmt.Fprintf(b, "Tone: %s\n", strings.Join(c.Tone, ", "))
	}
	if len(c.Laws) > 0 {
		b.WriteString("\nLaws:\n")
		for _, law := range c.Laws {
			fmt.Fprintf(b, "- %s\n", law)
		}
	}
	if len(c.Boundaries) > 0 {
		b.WriteString("\nBoundaries:\n")
		for _, bd := range c.Boundaries {
			fmt.Fprintf(b, "- %s\n", bd)
		}
	}
}

func formatEntities(b *strings.Builder, entities map[model.EntityID]model.Entity) {
	if len(entities) == 0 {
		return
	}
	byType := map[string][]model.Entity{}
	for _, e := range entities {
		byType[e.Type] = append(byType[e.Type], e)
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	fmt.Fprintf(b, "\n## Entities (%d)\n", len(entities))
	for _, t := range types {
		ents := byType[t]
		sort.Slice(ents, func(i, j int) bool { return ents[i].ID < ents[j].ID })
		fmt.Fprintf(b, "\n### %s (%d)\n\n", cases.Title(language.Und).String(t), len(ents))
		for _, e := range ents {
			fmt.Fprintf(b, "- **%s** (%s)", e.Name, e.ID)
			if e.Description != "" {
				fmt.Fprintf(b, " — %s", e.Description)
			}
			b.WriteByte('\n')
		}
	}
}

func formatRelations(b *strings.Builder, relations []model.Relation, entities map[model.EntityID]model.Entity) {
	if len(relations) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Relations (%d)\n\n", len(relations))
	for _, r := range relations {
		src := entityName(r.SourceID, entities)
		tgt := entityName(r.TargetID, entities)
		fmt.Fprintf(b, "- %s → %s [%s]\n", src, tgt, r.Type)
	}
}

func formatFacts(b *strings.Builder, facts []model.Fact, entities map[model.EntityID]model.Entity) {
	if len(facts) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Facts (%d)\n\n", len(facts))
	for _, f := range facts {
		subj := entityName(f.SubjectID, entities)
		fmt.Fprintf(b, "- %s.%s = %v\n", subj, f.Predicate, f.Value.Raw)
	}
}

func formatThreads(b *strings.Builder, threads []model.WorldThread) {
	if len(threads) == 0 {
		return
	}
	active := 0
	for _, th := range threads {
		if th.Status == model.ThreadStatusActive || th.Status == model.ThreadStatusOpen {
			active++
		}
	}
	fmt.Fprintf(b, "\n## Threads (%d, %d active)\n\n", len(threads), active)
	for _, th := range threads {
		fmt.Fprintf(b, "- [%s] **%s** (%s)", th.Status, th.Title, th.Kind)
		if th.Summary != "" {
			fmt.Fprintf(b, " — %s", th.Summary)
		}
		b.WriteByte('\n')
	}
}

func formatMemorySummary(b *strings.Builder, memories []model.MemoryRecord) {
	if len(memories) == 0 {
		return
	}
	byOwner := map[string]int{}
	for _, m := range memories {
		byOwner[m.Owner.Kind]++
	}
	fmt.Fprintf(b, "\n## Memories (%d)\n\n", len(memories))
	kinds := make([]string, 0, len(byOwner))
	for k := range byOwner {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Fprintf(b, "- %s: %d\n", k, byOwner[k])
	}
}

func formatEventSummary(b *strings.Builder, events []model.WorldEvent) {
	if len(events) == 0 {
		return
	}
	byType := map[string]int{}
	for _, e := range events {
		byType[e.Type]++
	}
	fmt.Fprintf(b, "\n## Event Log (%d)\n\n", len(events))
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Fprintf(b, "- %s: %d\n", t, byType[t])
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		desc := last.Description
		if desc == "" {
			desc = last.Intent
		}
		if desc != "" {
			fmt.Fprintf(b, "\nLast event: %s\n", desc)
		}
	}
}

func entityName(id model.EntityID, entities map[model.EntityID]model.Entity) string {
	if e, ok := entities[id]; ok {
		return e.Name
	}
	return string(id)
}
