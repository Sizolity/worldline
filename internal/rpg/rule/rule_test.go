package rule

import (
	"encoding/json"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func validRule() NarrativeRule {
	return NarrativeRule{
		ID:       "combat-01",
		Category: "combat",
		Level:    0,
		Content:  "All attacks require a dice roll.",
		Source:   SourceSystem,
		Enabled:  true,
		Tags:     []string{"combat", "dice"},
	}
}

func TestValidate_Valid(t *testing.T) {
	r := validRule()
	if err := r.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_UserSource(t *testing.T) {
	r := validRule()
	r.Source = SourceUser
	if err := r.Validate(); err != nil {
		t.Fatalf("expected valid with user source, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	r := validRule()
	r.ID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestValidate_EmptyCategory(t *testing.T) {
	r := validRule()
	r.Category = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty category")
	}
}

func TestValidate_EmptyContent(t *testing.T) {
	r := validRule()
	r.Content = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestValidate_InvalidLevel(t *testing.T) {
	r := validRule()
	r.Level = 3
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for level 3")
	}
	r.Level = -1
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for level -1")
	}
}

func TestValidate_InvalidSource(t *testing.T) {
	r := validRule()
	r.Source = "npc"
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestRoundTrip_Direct(t *testing.T) {
	r := validRule()
	mr := ToModelRule(r)
	if mr.Kind != Kind {
		t.Fatalf("expected kind %q, got %q", Kind, mr.Kind)
	}
	if mr.ID != r.ID {
		t.Fatalf("expected ID %q, got %q", r.ID, mr.ID)
	}
	got, ok := FromModelRule(mr)
	if !ok {
		t.Fatal("FromModelRule returned false")
	}
	if got.Content != r.Content {
		t.Fatalf("content mismatch: %q vs %q", got.Content, r.Content)
	}
}

func TestRoundTrip_MapPath(t *testing.T) {
	r := validRule()
	mr := ToModelRule(r)

	b, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var mr2 model.Rule
	if err := json.Unmarshal(b, &mr2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got, ok := FromModelRule(mr2)
	if !ok {
		t.Fatal("FromModelRule returned false for map path")
	}
	if got.ID != r.ID {
		t.Fatalf("ID mismatch: %q vs %q", got.ID, r.ID)
	}
	if got.Category != r.Category {
		t.Fatalf("category mismatch: %q vs %q", got.Category, r.Category)
	}
	if got.Content != r.Content {
		t.Fatalf("content mismatch: %q vs %q", got.Content, r.Content)
	}
}

func TestFromModelRule_WrongKind(t *testing.T) {
	mr := model.Rule{ID: "x", Kind: "other", Enabled: true, Data: NarrativeRule{}}
	_, ok := FromModelRule(mr)
	if ok {
		t.Fatal("expected false for wrong kind")
	}
}

func TestFromWorldRules(t *testing.T) {
	r1 := validRule()
	r2 := validRule()
	r2.ID = "combat-02"
	other := model.Rule{ID: "unrelated", Kind: "other_kind", Enabled: true}

	rules := []model.Rule{ToModelRule(r1), other, ToModelRule(r2)}
	got := FromWorldRules(rules)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
}
