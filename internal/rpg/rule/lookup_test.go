package rule

import (
	"strings"
	"testing"
)

func TestLookup_FilterByCategory(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Category: "combat", Level: 1, Content: "A", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "magic", Level: 1, Content: "B", Source: SourceSystem, Enabled: true},
		{ID: "r3", Category: "combat", Level: 2, Content: "C", Source: SourceSystem, Enabled: true},
	}
	got := Lookup(rules, LookupFilter{Category: "combat"})
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	for _, r := range got {
		if r.Category != "combat" {
			t.Fatalf("unexpected category %q", r.Category)
		}
	}
}

func TestLookup_FilterByTags(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Category: "combat", Level: 1, Content: "A", Source: SourceSystem, Enabled: true, Tags: []string{"melee", "sword"}},
		{ID: "r2", Category: "combat", Level: 1, Content: "B", Source: SourceSystem, Enabled: true, Tags: []string{"ranged"}},
		{ID: "r3", Category: "magic", Level: 1, Content: "C", Source: SourceSystem, Enabled: true, Tags: []string{"fire"}},
	}
	got := Lookup(rules, LookupFilter{Tags: []string{"sword", "fire"}})
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestLookup_SkipDisabled(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Category: "combat", Level: 1, Content: "A", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "combat", Level: 1, Content: "B", Source: SourceSystem, Enabled: false},
	}
	got := Lookup(rules, LookupFilter{Category: "combat"})
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
}

func TestLookup_CategoryAndTags(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Category: "combat", Level: 1, Content: "A", Source: SourceSystem, Enabled: true, Tags: []string{"melee"}},
		{ID: "r2", Category: "combat", Level: 1, Content: "B", Source: SourceSystem, Enabled: true, Tags: []string{"ranged"}},
		{ID: "r3", Category: "magic", Level: 1, Content: "C", Source: SourceSystem, Enabled: true, Tags: []string{"melee"}},
	}
	got := Lookup(rules, LookupFilter{Category: "combat", Tags: []string{"melee"}})
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].ID != "r1" {
		t.Fatalf("expected r1, got %s", got[0].ID)
	}
}

func TestFormatRules_NonEmpty(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Category: "combat", Level: 1, Content: "Strike first", Source: SourceSystem, Enabled: true, Tags: []string{"melee"}},
		{ID: "r2", Category: "combat", Level: 1, Content: "Parry second", Source: SourceSystem, Enabled: true},
	}
	got := FormatRules(rules)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "1. Strike first [melee]") {
		t.Fatalf("expected formatted rule with tags, got %q", got)
	}
	if !strings.Contains(got, "2. Parry second") {
		t.Fatalf("expected second rule, got %q", got)
	}
}

func TestFormatRules_Empty(t *testing.T) {
	got := FormatRules(nil)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
