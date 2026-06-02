package director

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
	wprompt "github.com/sizolity/worldline/internal/world/prompt"
)

func TestLLMDirectorParsesGeneratedEvents(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:          "event_llm_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceDirector,
		Description: "A merchant arrives at the tavern.",
	}}
	responseJSON, _ := json.Marshal(events)

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		SystemPrompt: "You are a world director.",
		Generator: fakeGenerator{
			response: string(responseJSON),
		},
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "test_world", Name: "Test"},
	})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_llm_1" {
		t.Fatalf("proposal mismatch: %#v", got)
	}
	if got[0].Description != "A merchant arrives at the tavern." {
		t.Fatalf("description = %q", got[0].Description)
	}
}

func TestLLMDirectorReturnsEmptyOnEmptyResponse(t *testing.T) {
	t.Parallel()

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{response: "[]"},
	})

	got, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty proposals: %#v", got)
	}
}

func TestLLMDirectorReturnsErrorOnInvalidJSON(t *testing.T) {
	t.Parallel()

	noRetry := intPtr(-1)
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator:         fakeGenerator{response: "not json at all"},
		MaxRepairAttempts: noRetry,
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for invalid JSON")
	}
}

func TestLLMDirectorReturnsErrorOnGeneratorFailure(t *testing.T) {
	t.Parallel()

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{err: context.DeadlineExceeded},
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for generator error")
	}
}

func TestLLMDirectorValidatesProposedEvents(t *testing.T) {
	t.Parallel()

	badEvents := []model.WorldEvent{{ID: "event_1"}}
	responseJSON, _ := json.Marshal(badEvents)

	noRetry := intPtr(-1)
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator:         fakeGenerator{response: string(responseJSON)},
		MaxRepairAttempts: noRetry,
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for invalid event (missing type/source)")
	}
}

func TestLLMDirectorPromptIncludesWorldContext(t *testing.T) {
	t.Parallel()

	gen := &capturingGenerator{}
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		SystemPrompt: "You are the narrator.",
		Generator:    gen,
	})

	d.Propose(Context{
		Ctx: context.Background(),
		World: model.World{
			ID:   "test_world",
			Name: "Kingdom of Shadows",
		},
	})

	if gen.lastSystem != "You are the narrator." {
		t.Fatalf("system prompt = %q", gen.lastSystem)
	}
	if gen.lastUser == "" {
		t.Fatal("user prompt is empty")
	}
}

func TestLLMDirectorUsesDefaultSystemPromptWhenEmpty(t *testing.T) {
	t.Parallel()

	gen := &capturingGenerator{}
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: gen,
	})

	d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})

	if gen.lastSystem != DefaultSystemPrompt {
		t.Fatalf("expected DefaultSystemPrompt, got %q", gen.lastSystem[:80])
	}
}

func TestBuildWorldPromptIncludesFactsRelationsMemories(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:          "test_world",
		Name:        "Test",
		Description: "A dark kingdom.",
		Clock:       model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID: "char_alice", Type: "character", Name: "Alice",
				Description: "A curious adventurer.",
				State: map[string]model.Value{
					"mood": {Kind: model.ValueKindString, Raw: "cheerful"},
				},
			},
		},
		Facts: []model.Fact{{
			ID: "fact_gate", SubjectID: "city_gate",
			Predicate: "status",
			Value:     model.Value{Kind: model.ValueKindString, Raw: "open"},
		}},
		Relations: []model.Relation{{
			ID: "rel_1", Type: "ally",
			SourceID: "char_alice", TargetID: "char_bob",
		}},
		Memories: []model.MemoryRecord{{
			ID:          "mem_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Content:     "The war ended ten years ago.",
			TruthStatus: model.TruthStatusTrue,
		}},
		Threads: []model.WorldThread{{
			ID: "thread_1", Title: "Find the treasure", Status: "open",
		}},
	}

	prompt := buildWorldPrompt(w)

	var parsed wprompt.Context
	if err := json.Unmarshal([]byte(prompt), &parsed); err != nil {
		t.Fatalf("prompt is not valid JSON: %v", err)
	}

	if parsed.Description != "A dark kingdom." {
		t.Errorf("description = %q", parsed.Description)
	}
	if len(parsed.Entities) != 1 {
		t.Fatalf("entities count = %d", len(parsed.Entities))
	}
	if parsed.Entities[0].Description != "A curious adventurer." {
		t.Errorf("entity description = %q", parsed.Entities[0].Description)
	}
	if parsed.Entities[0].State["mood"] != "cheerful" {
		t.Errorf("entity state[mood] = %v", parsed.Entities[0].State["mood"])
	}
	if len(parsed.Facts) != 1 || parsed.Facts[0].Predicate != "status" {
		t.Errorf("facts = %+v", parsed.Facts)
	}
	if len(parsed.Relations) != 1 || parsed.Relations[0].Type != "ally" {
		t.Errorf("relations = %+v", parsed.Relations)
	}
	if len(parsed.Memories) != 1 || parsed.Memories[0].Content != "The war ended ten years ago." {
		t.Errorf("memories = %+v", parsed.Memories)
	}
	if len(parsed.Threads) != 1 || parsed.Threads[0].Title != "Find the treasure" {
		t.Errorf("threads = %+v", parsed.Threads)
	}
}

func TestParseEventResponseAcceptsEventsWithEffects(t *testing.T) {
	t.Parallel()

	raw := `[{"id":"event_gate","type":"world_fact_changed","source":"director","description":"Gate sealed.","effects":[{"kind":"set_fact","target_id":"fact_gate","payload":{"subject_id":{"kind":"entity_ref","raw":"city_gate"},"predicate":{"kind":"string","raw":"status"},"value":{"kind":"string","raw":"sealed"}}}]}]`

	events, err := parseEventResponse(raw)
	if err != nil {
		t.Fatalf("parseEventResponse error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d", len(events))
	}
	if len(events[0].Effects) != 1 {
		t.Fatalf("effects count = %d", len(events[0].Effects))
	}
	if events[0].Effects[0].Kind != model.EffectSetFact {
		t.Errorf("effect kind = %q", events[0].Effects[0].Kind)
	}
	if events[0].Effects[0].TargetID != "fact_gate" {
		t.Errorf("effect target = %q", events[0].Effects[0].TargetID)
	}
}

func TestLLMDirectorRepairsInvalidJSON(t *testing.T) {
	t.Parallel()

	goodJSON := `[{"id":"event_fix","type":"note","source":"director","description":"Fixed."}]`
	gen := &sequenceGenerator{
		responses: []string{"not valid json", goodJSON},
	}

	d := NewLLMDirector("llm_1", LLMDirectorConfig{Generator: gen})

	events, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if len(events) != 1 || events[0].ID != "event_fix" {
		t.Fatalf("unexpected events: %+v", events)
	}
	if gen.calls != 2 {
		t.Fatalf("expected 2 Generate calls, got %d", gen.calls)
	}
}

func TestLLMDirectorRepairsValidationError(t *testing.T) {
	t.Parallel()

	badEvent, _ := json.Marshal([]model.WorldEvent{{ID: "event_x"}})
	goodJSON := `[{"id":"event_ok","type":"note","source":"director","description":"OK."}]`
	gen := &sequenceGenerator{
		responses: []string{string(badEvent), goodJSON},
	}

	d := NewLLMDirector("llm_1", LLMDirectorConfig{Generator: gen})

	events, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if events[0].ID != "event_ok" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestLLMDirectorExhaustsRepairAttempts(t *testing.T) {
	t.Parallel()

	gen := &sequenceGenerator{
		responses: []string{"bad1", "bad2", "bad3"},
	}
	maxAttempts := intPtr(2)
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator:         gen,
		MaxRepairAttempts: maxAttempts,
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("expected error after exhausting repair attempts")
	}
	if gen.calls != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 repair), got %d", gen.calls)
	}
}

func TestLLMDirectorRepairUsesConversationGenerator(t *testing.T) {
	t.Parallel()

	goodJSON := `[{"id":"event_ok","type":"note","source":"director","description":"OK."}]`
	gen := &fakeConversationGenerator{
		initialResponse: "broken json",
		repairResponse:  goodJSON,
	}

	d := NewLLMDirector("llm_1", LLMDirectorConfig{Generator: gen})

	events, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err != nil {
		t.Fatalf("Propose error: %v", err)
	}
	if events[0].ID != "event_ok" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if !gen.repairCalled {
		t.Fatal("expected GenerateRepair to be called")
	}
}

func TestParseEventResponseStripsMarkdownFences(t *testing.T) {
	t.Parallel()

	wrapped := "```json\n[{\"id\":\"event_dawn\",\"type\":\"note\",\"source\":\"director\",\"description\":\"Dawn.\"}]\n```"
	events, err := parseEventResponse(wrapped)
	if err != nil {
		t.Fatalf("parseEventResponse error: %v", err)
	}
	if len(events) != 1 || events[0].ID != "event_dawn" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestPromptTemplateRendersWorldData(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplate(`You direct "{{.Name}}" ({{.WorldID}}). Clock={{.Clock}}. Entities={{len .Entities}}.`)
	if err != nil {
		t.Fatalf("ParsePromptTemplate error: %v", err)
	}

	w := model.World{
		ID:    "w_test",
		Name:  "Shadow Kingdom",
		Clock: model.WorldClock{Sequence: 7},
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
			"loc_b":  {ID: "loc_b", Type: "location", Name: "Tavern"},
		},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Shadow Kingdom") {
		t.Errorf("missing world name in %q", got)
	}
	if !strings.Contains(got, "w_test") {
		t.Errorf("missing world ID in %q", got)
	}
	if !strings.Contains(got, "Clock=7") {
		t.Errorf("missing clock in %q", got)
	}
	if !strings.Contains(got, "Entities=2") {
		t.Errorf("missing entity count in %q", got)
	}
}

func TestPromptTemplateCanIterateEntities(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplate(`{{range .Entities}}- {{.Name}} ({{.Type}})` + "\n" + `{{end}}`)
	if err != nil {
		t.Fatalf("ParsePromptTemplate error: %v", err)
	}

	w := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
		},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Alice (character)") {
		t.Errorf("entity not rendered: %q", got)
	}
}

func TestPromptTemplateFStringFormat(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplateWithFormat(`You direct "{Name}" ({WorldID}). Clock={Clock}.`, FString)
	if err != nil {
		t.Fatalf("ParsePromptTemplateWithFormat error: %v", err)
	}

	w := model.World{
		ID:    "w_test",
		Name:  "Shadow Kingdom",
		Clock: model.WorldClock{Sequence: 7},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Shadow Kingdom") {
		t.Errorf("missing world name in %q", got)
	}
	if !strings.Contains(got, "w_test") {
		t.Errorf("missing world ID in %q", got)
	}
	if !strings.Contains(got, "Clock=7") {
		t.Errorf("missing clock in %q", got)
	}
}

func TestParsePromptTemplateRejectsInvalid(t *testing.T) {
	t.Parallel()
	_, err := ParsePromptTemplate(`{{.Broken`)
	if err == nil {
		t.Fatal("expected error for broken template")
	}
}

func TestLLMDirectorPromptTemplateTakesPriority(t *testing.T) {
	t.Parallel()

	tpl, _ := ParsePromptTemplate(`World: {{.Name}}`)
	gen := &capturingGenerator{}
	d := NewLLMDirector("llm_tpl", LLMDirectorConfig{
		SystemPrompt:   "this should be ignored",
		PromptTemplate: tpl,
		Generator:      gen,
	})

	d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "TestWorld"},
	})

	if gen.lastSystem != "World: TestWorld" {
		t.Fatalf("system prompt = %q, want template-rendered", gen.lastSystem)
	}
}

func TestLLMDirectorFallsBackFromTemplateToStaticToDefault(t *testing.T) {
	t.Parallel()

	gen := &capturingGenerator{}

	// No template, no static → DefaultSystemPrompt
	d1 := NewLLMDirector("llm_1", LLMDirectorConfig{Generator: gen})
	d1.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if gen.lastSystem != DefaultSystemPrompt {
		t.Fatalf("expected DefaultSystemPrompt, got %q", gen.lastSystem[:40])
	}

	// Static prompt, no template → static
	d2 := NewLLMDirector("llm_2", LLMDirectorConfig{
		SystemPrompt: "custom static",
		Generator:    gen,
	})
	d2.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if gen.lastSystem != "custom static" {
		t.Fatalf("expected static prompt, got %q", gen.lastSystem)
	}
}

// --- test helpers ---

func intPtr(n int) *int { return &n }

type fakeGenerator struct {
	response string
	err      error
}

func (g fakeGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return g.response, g.err
}

type capturingGenerator struct {
	lastSystem string
	lastUser   string
}

func (g *capturingGenerator) Generate(_ context.Context, system, user string) (string, error) {
	g.lastSystem = system
	g.lastUser = user
	return "[]", nil
}

type sequenceGenerator struct {
	responses []string
	calls     int
}

func (g *sequenceGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	idx := g.calls
	g.calls++
	if idx < len(g.responses) {
		return g.responses[idx], nil
	}
	return g.responses[len(g.responses)-1], nil
}

type fakeConversationGenerator struct {
	initialResponse string
	repairResponse  string
	repairCalled    bool
}

func (g *fakeConversationGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return g.initialResponse, nil
}

func (g *fakeConversationGenerator) GenerateRepair(_ context.Context, _, _, _, _ string) (string, error) {
	g.repairCalled = true
	return g.repairResponse, nil
}
