package rule

import (
	"strings"
	"testing"
)

func TestAssemblePromptSection_Empty(t *testing.T) {
	got := AssemblePromptSection(nil)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestAssemblePromptSection_AllDisabled(t *testing.T) {
	rules := []NarrativeRule{
		{ID: "r1", Category: "combat", Level: 0, Content: "x", Source: SourceSystem, Enabled: false},
	}
	got := AssemblePromptSection(rules)
	if got != "" {
		t.Fatalf("expected empty for all disabled, got %q", got)
	}
}

func TestAssemblePromptSection_L0InFullText(t *testing.T) {
	rules := []NarrativeRule{
		{ID: "r1", Category: "combat", Level: 0, Content: "Always roll dice", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "magic", Level: 0, Content: "Magic costs mana", Source: SourceSystem, Enabled: true},
	}
	got := AssemblePromptSection(rules)
	if !strings.Contains(got, "Always roll dice") {
		t.Fatal("L0 content missing")
	}
	if !strings.Contains(got, "Magic costs mana") {
		t.Fatal("L0 content missing")
	}
	if !strings.Contains(got, "### Core Rules (always active)") {
		t.Fatal("core rules header missing")
	}
}

func TestAssemblePromptSection_CategoryIndex(t *testing.T) {
	rules := []NarrativeRule{
		{ID: "r1", Category: "combat", Level: 1, Content: "detail A", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "combat", Level: 1, Content: "detail B", Source: SourceSystem, Enabled: true},
		{ID: "r3", Category: "magic", Level: 2, Content: "secret spell", Source: SourceSystem, Enabled: true},
	}
	got := AssemblePromptSection(rules)
	if !strings.Contains(got, "### Available Rule Categories") {
		t.Fatal("category index header missing")
	}
	if !strings.Contains(got, "combat") {
		t.Fatal("combat category missing from index")
	}
	if !strings.Contains(got, "magic") {
		t.Fatal("magic category missing from index")
	}
	if !strings.Contains(got, "2 rules") {
		t.Fatal("combat count wrong")
	}
}

func TestAssemblePromptSection_L2ContentNotInPrompt(t *testing.T) {
	rules := []NarrativeRule{
		{ID: "r1", Category: "combat", Level: 0, Content: "Core rule", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "secrets", Level: 2, Content: "HIDDEN_SECRET_CONTENT", Source: SourceSystem, Enabled: true},
	}
	got := AssemblePromptSection(rules)
	if strings.Contains(got, "HIDDEN_SECRET_CONTENT") {
		t.Fatal("L2 content should NOT appear in prompt")
	}
}

func TestAssemblePromptSection_DisabledSkipped(t *testing.T) {
	rules := []NarrativeRule{
		{ID: "r1", Category: "combat", Level: 0, Content: "Active rule", Source: SourceSystem, Enabled: true},
		{ID: "r2", Category: "combat", Level: 0, Content: "Disabled rule", Source: SourceSystem, Enabled: false},
	}
	got := AssemblePromptSection(rules)
	if !strings.Contains(got, "Active rule") {
		t.Fatal("active rule missing")
	}
	if strings.Contains(got, "Disabled rule") {
		t.Fatal("disabled rule should not appear")
	}
}
