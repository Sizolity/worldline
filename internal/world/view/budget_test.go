package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestEstimateBudgetEmptyWorld(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "w", Name: "W"}
	b := EstimateBudget(w)
	if b.TotalBytes == 0 {
		t.Error("empty world should still have non-zero bytes (clock/canon serialization)")
	}
	if b.TotalTokens == 0 {
		t.Error("expected non-zero total tokens")
	}
	if b.Entities.Items != 0 {
		t.Errorf("entities items = %d, want 0", b.Entities.Items)
	}
}

func TestEstimateBudgetCounts(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "location", Name: "Town"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "e1", Predicate: "age", Value: model.Value{Raw: 25}},
		},
		Memories: []model.MemoryRecord{
			{ID: "m1", Content: "saw fire"},
			{ID: "m2", Content: "heard scream"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: "quest", Title: "Find gem", Status: "active"},
		},
	}

	b := EstimateBudget(w)
	if b.Entities.Items != 2 {
		t.Errorf("entities items = %d, want 2", b.Entities.Items)
	}
	if b.Facts.Items != 1 {
		t.Errorf("facts items = %d, want 1", b.Facts.Items)
	}
	if b.Memories.Items != 2 {
		t.Errorf("memories items = %d, want 2", b.Memories.Items)
	}
	if b.Threads.Items != 1 {
		t.Errorf("threads items = %d, want 1", b.Threads.Items)
	}
	if b.TotalBytes <= 0 {
		t.Error("total bytes should be positive")
	}
	if b.TotalTokens <= 0 {
		t.Error("total tokens should be positive")
	}
}

func TestEstimateBudgetTokenRatio(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice", Description: "A brave warrior of the northern lands"},
		},
	}
	b := EstimateBudget(w)
	ratio := float64(b.TotalBytes) / float64(b.TotalTokens)
	if ratio < 3.0 || ratio > 5.0 {
		t.Errorf("byte/token ratio = %.1f, expected ~4.0", ratio)
	}
}

func TestEstimateBudgetSectionBytes(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "e1", Predicate: "age", Value: model.Value{Raw: 25}},
		},
	}
	b := EstimateBudget(w)
	if b.Entities.Bytes <= 0 {
		t.Error("entities bytes should be positive")
	}
	if b.Facts.Bytes <= 0 {
		t.Error("facts bytes should be positive")
	}
	sum := b.Entities.Bytes + b.Facts.Bytes + b.Relations.Bytes +
		b.Memories.Bytes + b.Threads.Bytes + b.EventLog.Bytes +
		b.EventQueue.Bytes + b.Rules.Bytes + b.Clock.Bytes + b.Canon.Bytes
	if sum != b.TotalBytes {
		t.Errorf("section sum %d != total %d", sum, b.TotalBytes)
	}
}

func TestFormatBudgetSmallWorld(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	out := FormatBudget(EstimateBudget(w))
	if !strings.Contains(out, "Context Budget Estimate") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "Entities") {
		t.Errorf("missing entities row:\n%s", out)
	}
	if !strings.Contains(out, "**Total**") {
		t.Errorf("missing total row:\n%s", out)
	}
	if !strings.Contains(out, "small") {
		t.Errorf("expected 'small' budget label:\n%s", out)
	}
}

func TestFormatBudgetOmitsEmptySections(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "w", Name: "W"}
	out := FormatBudget(EstimateBudget(w))
	if strings.Contains(out, "Entities") {
		t.Errorf("should omit empty entities section:\n%s", out)
	}
	if strings.Contains(out, "Threads") {
		t.Errorf("should omit empty threads section:\n%s", out)
	}
}
