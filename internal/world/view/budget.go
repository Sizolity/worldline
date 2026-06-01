package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// BudgetEstimate breaks down the estimated token cost of a world snapshot.
type BudgetEstimate struct {
	Entities   SectionBudget `json:"entities"`
	Facts      SectionBudget `json:"facts"`
	Relations  SectionBudget `json:"relations"`
	Memories   SectionBudget `json:"memories"`
	Threads    SectionBudget `json:"threads"`
	EventLog   SectionBudget `json:"event_log"`
	EventQueue SectionBudget `json:"event_queue"`
	Rules      SectionBudget `json:"rules"`
	Clock      SectionBudget `json:"clock"`
	Canon      SectionBudget `json:"canon"`

	TotalBytes  int `json:"total_bytes"`
	TotalTokens int `json:"total_tokens"`
}

// SectionBudget holds the byte count, estimated token count, and item count for one section.
type SectionBudget struct {
	Items  int `json:"items"`
	Bytes  int `json:"bytes"`
	Tokens int `json:"tokens"`
}

// EstimateBudget computes a token-budget breakdown for a world snapshot.
// Token estimation uses ~4 bytes per token which is a reasonable average for
// JSON-serialized English text sent to LLMs.
func EstimateBudget(w model.World) BudgetEstimate {
	var b BudgetEstimate

	b.Entities = sectionBudget(len(w.Entities), w.Entities)
	b.Facts = sectionBudget(len(w.Facts), w.Facts)
	b.Relations = sectionBudget(len(w.Relations), w.Relations)
	b.Memories = sectionBudget(len(w.Memory), w.Memory)
	b.Threads = sectionBudget(len(w.Threads), w.Threads)
	b.EventLog = sectionBudget(len(w.EventLog), w.EventLog)
	b.EventQueue = sectionBudget(len(w.EventQueue), w.EventQueue)
	b.Rules = sectionBudget(len(w.Rules), w.Rules)
	b.Clock = sectionBudget(1, w.Clock)
	b.Canon = sectionBudget(1, w.Canon)

	b.TotalBytes = b.Entities.Bytes + b.Facts.Bytes + b.Relations.Bytes +
		b.Memories.Bytes + b.Threads.Bytes + b.EventLog.Bytes +
		b.EventQueue.Bytes + b.Rules.Bytes + b.Clock.Bytes + b.Canon.Bytes
	b.TotalTokens = bytesToTokens(b.TotalBytes)

	return b
}

func sectionBudget(items int, data any) SectionBudget {
	raw, _ := json.Marshal(data)
	n := len(raw)
	return SectionBudget{
		Items:  items,
		Bytes:  n,
		Tokens: bytesToTokens(n),
	}
}

func bytesToTokens(n int) int {
	return (n + 3) / 4
}

// FormatBudget renders a human-readable budget table.
func FormatBudget(b BudgetEstimate) string {
	var sb strings.Builder

	sb.WriteString("## Context Budget Estimate\n\n")
	sb.WriteString(fmt.Sprintf("| %-14s | %6s | %8s | %8s |\n", "Section", "Items", "Bytes", "~Tokens"))
	sb.WriteString(fmt.Sprintf("| %-14s | %6s | %8s | %8s |\n", "--------------", "------", "--------", "--------"))

	rows := []struct {
		name string
		s    SectionBudget
	}{
		{"Entities", b.Entities},
		{"Facts", b.Facts},
		{"Relations", b.Relations},
		{"Memories", b.Memories},
		{"Threads", b.Threads},
		{"Event Log", b.EventLog},
		{"Event Queue", b.EventQueue},
		{"Rules", b.Rules},
		{"Clock", b.Clock},
		{"Canon", b.Canon},
	}

	for _, r := range rows {
		if r.s.Bytes == 0 || (r.s.Items == 0 && r.s.Bytes <= 10) {
			continue
		}
		sb.WriteString(fmt.Sprintf("| %-14s | %6d | %8d | %8d |\n", r.name, r.s.Items, r.s.Bytes, r.s.Tokens))
	}

	sb.WriteString(fmt.Sprintf("| %-14s | %6s | %8d | %8d |\n", "**Total**", "", b.TotalBytes, b.TotalTokens))

	sb.WriteString("\n")
	if b.TotalTokens < 2000 {
		sb.WriteString("Budget: **small** — fits comfortably in most context windows.\n")
	} else if b.TotalTokens < 8000 {
		sb.WriteString("Budget: **medium** — leaves room for system prompts and responses.\n")
	} else if b.TotalTokens < 32000 {
		sb.WriteString("Budget: **large** — may need trimming for smaller context windows.\n")
	} else {
		sb.WriteString("Budget: **very large** — consider reducing world size or using summarization.\n")
	}

	return sb.String()
}
