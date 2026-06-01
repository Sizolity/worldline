package view

import (
	"fmt"
	"strings"

	"github.com/sizolity/worldline/internal/world/runtime"
)

// FormatDiff produces a human-readable text summary of a WorldDiff.
// Returns "no changes" if the worlds are identical.
func FormatDiff(d runtime.WorldDiff) string {
	var b strings.Builder

	fmt.Fprintf(&b, "diff %s → %s\n", d.WorldA, d.WorldB)

	if d.ClockA != d.ClockB {
		fmt.Fprintf(&b, "  clock: %d → %d\n", d.ClockA, d.ClockB)
	}

	changes := 0
	changes += formatEntityDiff(&b, d.Entities)
	changes += formatSliceDiff(&b, "facts", d.Facts)
	changes += formatSliceDiff(&b, "relations", d.Relations)
	changes += formatSliceDiff(&b, "memories", d.Memories)
	changes += formatThreadDiff(&b, d.Threads)
	changes += formatSliceDiff(&b, "events", d.Events)
	changes += formatSliceDiff(&b, "rules", d.Rules)

	if changes == 0 && d.ClockA == d.ClockB {
		fmt.Fprintf(&b, "  no changes\n")
	}

	return b.String()
}

func formatEntityDiff(b *strings.Builder, d runtime.EntityDiff) int {
	n := 0
	for _, id := range d.Added {
		fmt.Fprintf(b, "  + entity %s\n", id)
		n++
	}
	for _, id := range d.Removed {
		fmt.Fprintf(b, "  - entity %s\n", id)
		n++
	}
	for _, ic := range d.Changed {
		fmt.Fprintf(b, "  ~ entity %s\n", ic.ID)
		formatFieldDeltas(b, ic.Fields)
		n++
	}
	return n
}

func formatSliceDiff(b *strings.Builder, collection string, d runtime.SliceDiff) int {
	n := 0
	for _, id := range d.Added {
		fmt.Fprintf(b, "  + %s %s\n", collection, id)
		n++
	}
	for _, id := range d.Removed {
		fmt.Fprintf(b, "  - %s %s\n", collection, id)
		n++
	}
	for _, ic := range d.Changed {
		fmt.Fprintf(b, "  ~ %s %s\n", collection, ic.ID)
		formatFieldDeltas(b, ic.Fields)
		n++
	}
	return n
}

func formatThreadDiff(b *strings.Builder, d runtime.ThreadDiff) int {
	n := 0
	for _, id := range d.Added {
		fmt.Fprintf(b, "  + thread %s\n", id)
		n++
	}
	for _, id := range d.Removed {
		fmt.Fprintf(b, "  - thread %s\n", id)
		n++
	}
	for _, tc := range d.StatusChanged {
		fmt.Fprintf(b, "  ~ thread %s: %s → %s\n", tc.ID, tc.StatusA, tc.StatusB)
		n++
	}
	for _, ic := range d.Changed {
		fmt.Fprintf(b, "  ~ thread %s\n", ic.ID)
		formatFieldDeltas(b, ic.Fields)
		n++
	}
	return n
}

func formatFieldDeltas(b *strings.Builder, fields []runtime.FieldDelta) {
	for _, fd := range fields {
		old := fd.Old
		if old == "" {
			old = "(empty)"
		}
		nw := fd.New
		if nw == "" {
			nw = "(empty)"
		}
		fmt.Fprintf(b, "      %s: %s → %s\n", fd.Field, old, nw)
	}
}
