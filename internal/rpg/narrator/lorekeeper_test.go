package narrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/world/ingest"
	"github.com/sizolity/worldline/internal/rpg/role"
)

// mockLoreAgent is a configurable typed.Agent[LoreDraft] for LoreParser tests.
type mockLoreAgent struct {
	err        error
	payload    LoreDraft
	rawContent string
	called     bool
}

func (m *mockLoreAgent) Call(_ context.Context, _ []*schema.Message) (LoreDraft, error) {
	m.called = true
	if m.err != nil {
		return LoreDraft{}, m.err
	}
	if m.rawContent != "" {
		var ld LoreDraft
		if err := json.Unmarshal([]byte(m.rawContent), &ld); err != nil {
			return LoreDraft{}, err
		}
		return ld, nil
	}
	return m.payload, nil
}

func TestLoreParser_ParseSuccess(t *testing.T) {
	payload := LoreDraft{
		Entities: []ingest.DraftEntity{{
			ID:         "ent_sun_wukong",
			Type:       "character",
			Name:       "孙悟空",
			Aliases:    []string{"美猴王"},
			Confidence: 0.9,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
		Relations: []ingest.DraftRelation{{
			ID:         "rel_wukong_subudhi",
			Type:       "disciple_of",
			SourceID:   "ent_sun_wukong",
			TargetID:   "ent_subudhi",
			Confidence: 0.8,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
		Memories: []ingest.DraftMemory{{
			ID:         "mem_first_meeting",
			OwnerKind:  "world",
			Content:    "悟空初见菩提祖师，被收为弟子。",
			Scope:      "canonical",
			Kind:       "observation",
			Importance: 0.7,
			Confidence: 0.85,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
	}
	n, err := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	parser := NewLoreParser(&mockLoreAgent{payload: payload}, n)
	doc := ingest.SourceDocument{ID: "doc-xiyou-01", Text: "悟空拜师菩提祖师..."}

	got, err := parser.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.Canon != nil {
		t.Errorf("Canon should be nil (world-level metadata, not per-beat), got %+v", got.Canon)
	}

	if len(got.Entities) != 1 {
		t.Fatalf("entities: got %d, want 1", len(got.Entities))
	}
	if got.Entities[0].ID != "ent_sun_wukong" || got.Entities[0].Name != "孙悟空" {
		t.Errorf("entity: got %+v", got.Entities[0])
	}
	if got.Entities[0].Confidence != 0.9 {
		t.Errorf("entity confidence: got %v, want 0.9", got.Entities[0].Confidence)
	}

	if len(got.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(got.Relations))
	}
	if got.Relations[0].ID != "rel_wukong_subudhi" {
		t.Errorf("relation: got %+v", got.Relations[0])
	}

	if len(got.Memories) != 1 {
		t.Fatalf("memories: got %d, want 1", len(got.Memories))
	}
	if got.Memories[0].ID != "mem_first_meeting" {
		t.Errorf("memory: got %+v", got.Memories[0])
	}
}

func TestLoreParser_ParseEmpty(t *testing.T) {
	n, err := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mock := &mockLoreAgent{
		err: errors.New("llm must not be contacted for empty text"),
	}
	parser := NewLoreParser(mock, n)
	doc := ingest.SourceDocument{ID: "doc-xiyou-01", Text: "   \n\t  "}

	got, err := parser.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse on whitespace-only text: unexpected error %v (short-circuit should have skipped the LLM)", err)
	}
	if mock.called {
		t.Fatal("Parse contacted the LLM on whitespace-only text; expected short-circuit")
	}
	if got.Canon != nil {
		t.Errorf("Canon should be nil, got %+v", got.Canon)
	}
	if len(got.Entities) != 0 {
		t.Errorf("Entities should be empty, got %+v", got.Entities)
	}
	if len(got.Relations) != 0 {
		t.Errorf("Relations should be empty, got %+v", got.Relations)
	}
	if len(got.Facts) != 0 {
		t.Errorf("Facts should be empty, got %+v", got.Facts)
	}
	if len(got.Threads) != 0 {
		t.Errorf("Threads should be empty, got %+v", got.Threads)
	}
	if len(got.Memories) != 0 {
		t.Errorf("Memories should be empty, got %+v", got.Memories)
	}
}

func TestLoreParser_EmptyContentReturnsError(t *testing.T) {
	n, err := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	parser := NewLoreParser(&mockLoreAgent{err: errors.New("empty content")}, n)
	_, err = parser.Parse(context.Background(), ingest.SourceDocument{ID: "doc-1", Text: "一些叙事。"})
	if err == nil {
		t.Fatal("expected error from Parse on empty content")
	}
	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("error should mention 'empty content', got: %v", err)
	}
}

func TestLoreParser_GenerateError(t *testing.T) {
	n, err := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	parser := NewLoreParser(&mockLoreAgent{err: errors.New("boom")}, n)
	_, err = parser.Parse(context.Background(), ingest.SourceDocument{ID: "doc-1", Text: "一些叙事。"})
	if err == nil {
		t.Fatal("expected error from Parse")
	}
	if !strings.Contains(err.Error(), "lorekeeper generate") {
		t.Errorf("error should wrap %q, got: %v", "lorekeeper generate", err)
	}
}

func TestLoreParser_ImplementsRoleLorekeeper(t *testing.T) {
	var _ role.Lorekeeper = (*LoreParser)(nil)
	t.Log("compile-time check")
}

func TestLoreParser_ImplementsIngestParser(t *testing.T) {
	var _ ingest.Parser = (*LoreParser)(nil)
	t.Log("compile-time check")
}
