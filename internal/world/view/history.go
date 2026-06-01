package view

import (
	"fmt"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// HistoryEntry is one line in a formatted history.
type HistoryEntry struct {
	Index       int    `json:"index"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Summary     string `json:"summary"`
	EffectCount int    `json:"effect_count"`
}

// BuildHistory converts an event log into a structured history.
func BuildHistory(events []model.WorldEvent) []HistoryEntry {
	entries := make([]HistoryEntry, len(events))
	for i, ev := range events {
		entries[i] = HistoryEntry{
			Index:       i + 1,
			ID:          string(ev.ID),
			Type:        ev.Type,
			Source:      ev.Source,
			Summary:     eventSummary(ev),
			EffectCount: len(ev.Effects),
		}
	}
	return entries
}

// FormatHistory produces a human-readable timeline from an event log.
func FormatHistory(events []model.WorldEvent, entityNames map[model.EntityID]string) string {
	if len(events) == 0 {
		return "no events\n"
	}

	var b strings.Builder
	for i, ev := range events {
		fmt.Fprintf(&b, "%3d. [%s] %s", i+1, ev.Type, ev.Source)

		actors := resolveNames(ev.ActorIDs, entityNames)
		if actors != "" {
			fmt.Fprintf(&b, " by %s", actors)
		}
		targets := resolveNames(ev.TargetIDs, entityNames)
		if targets != "" {
			fmt.Fprintf(&b, " → %s", targets)
		}
		if ev.LocationID != "" {
			loc := resolveName(ev.LocationID, entityNames)
			fmt.Fprintf(&b, " @ %s", loc)
		}
		b.WriteByte('\n')

		desc := eventSummary(ev)
		if desc != "" {
			fmt.Fprintf(&b, "     %s\n", desc)
		}
		if len(ev.Effects) > 0 {
			fmt.Fprintf(&b, "     %d effect(s):", len(ev.Effects))
			for _, eff := range ev.Effects {
				fmt.Fprintf(&b, " %s", eff.Kind)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func eventSummary(ev model.WorldEvent) string {
	if ev.Description != "" {
		return ev.Description
	}
	return ev.Intent
}

func resolveNames(ids []model.EntityID, names map[model.EntityID]string) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = resolveName(id, names)
	}
	return strings.Join(parts, ", ")
}

func resolveName(id model.EntityID, names map[model.EntityID]string) string {
	if n, ok := names[id]; ok {
		return n
	}
	return string(id)
}
