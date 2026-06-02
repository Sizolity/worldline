package narrator

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/role"
	rpgrule "github.com/sizolity/worldline/internal/rpg/rule"
	"github.com/sizolity/worldline/internal/rpg/tools"
	"github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/view"
)

// mockSuggestAgent is a typed.Agent[SuggestParams] for tests.
type mockSuggestAgent struct{}

func (m *mockSuggestAgent) Call(_ context.Context, _ []*schema.Message) (SuggestParams, error) {
	return SuggestParams{
		Options: []role.ActionOption{
			{Label: "调查密室", Type: "investigate"},
			{Label: "与守卫交谈", Type: "social"},
			{Label: "前往遗迹", Type: "explore"},
		},
	}, nil
}

func testWorld() model.World {
	combatRule := rpgrule.NarrativeRule{
		ID: "rule-combat-01", Category: "combat", Level: 0,
		Content: "Attack rolls use d20 + modifier", Source: rpgrule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}
	return model.World{
		ID:   "world-test-01",
		Name: "Crystal Caverns",
		Canon: model.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"mysterious", "dark"},
		},
		Entities: map[model.EntityID]model.Entity{
			"hero-arin": {
				ID: "hero-arin", Type: "character", Name: "Arin",
				Tags: []string{"player", "warrior"},
			},
			"loc-cavern": {
				ID: "loc-cavern", Type: "location", Name: "Ancient Cavern",
				Description: "A deep underground cavern with glowing crystals.",
			},
		},
		Rules: []model.Rule{rpgrule.ToModelRule(combatRule)},
	}
}

func testWorldCtx(w model.World) view.WorldDebugContext {
	return view.WorldDebugView{}.Render(w)
}

func testNarrativeCtx(w model.World) view.NarrativeContext {
	return view.NarrativeView{}.Render(w, view.NarrativeContextRequest{RecentEventLimit: 5})
}

func TestNarrator_Role(t *testing.T) {
	n, err := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Role(); got != "Narrator" {
		t.Errorf("Role() = %q, want %q", got, "Narrator")
	}
}

func TestNarrator_SystemPrompt(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	w := testWorld()
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	prompt := n.SystemPrompt(players, role.PromptOptions{
		WorldCtx:     testWorldCtx(w),
		NarrativeCtx: testNarrativeCtx(w),
	})
	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	// Layer 2 (data projection): world payload values are rendered into
	// the reserved `## 世界` / `## 规则` placeholders, so they surface
	// regardless of which persona is loaded.
	// Layer 1 (persona): "Test Narrator" comes from testdata so we can
	// confirm the persona body actually got rendered.
	// Layer 3 (engine): the "引擎合规层" trailer is appended in code.
	for _, want := range []string{
		"Crystal Caverns", "fantasy", "Attack rolls",
		"## 世界", "## 规则",
		"Test Narrator", "Minimal narrator persona",
		"引擎合规层",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
	if strings.Contains(prompt, "## 发现协议") {
		t.Error("fog disabled: discovery placeholder must be dropped")
	}
}

func TestNarrator_SystemPrompt_WithFog(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	w := testWorld()
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	prompt := n.SystemPrompt(players, role.PromptOptions{
		WorldCtx:     testWorldCtx(w),
		NarrativeCtx: testNarrativeCtx(w),
		FogEnabled:   true,
	})
	if !strings.Contains(prompt, "## 发现协议") {
		t.Error("fog enabled: should contain `## 发现协议` placeholder header")
	}
	if !strings.Contains(prompt, "Discovery Protocol") {
		t.Error("fog enabled: legacy marker sentinel must still surface for tests")
	}
	if !strings.Contains(prompt, "explore_knowledge") {
		t.Error("fog enabled: discovery section should mention explore_knowledge tool")
	}
}

// toolNames extracts the set of tool names from a BaseTool slice.
func toolNames(t *testing.T, tls []einotool.BaseTool) map[string]bool {
	t.Helper()
	ctx := context.Background()
	names := make(map[string]bool, len(tls))
	for _, tl := range tls {
		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("tool Info: %v", err)
		}
		names[info.Name] = true
	}
	return names
}

func TestNarrator_Tools_BaseOnly(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	w := testWorld()
	w.Rules = nil
	tc := &tools.ToolContext{World: w, Rng: rand.New(rand.NewPCG(1, 2))}
	invokable, err := n.Tools(tc)
	if err != nil {
		t.Fatalf("Tools(): %v", err)
	}
	// Base-only (no rules, no mutable state, no fog) still includes the
	// five always-on tools: get_entity_state plus the four internal
	// randomness tools (roll / random / chance / weighted_choice).
	if got, want := len(invokable), 5; got != want {
		t.Errorf("len(tools) = %d, want %d (get_entity_state + roll + random + chance + weighted_choice)", got, want)
	}
	names := toolNames(t, invokable)
	for _, want := range []string{"get_entity_state", "roll", "random", "chance", "weighted_choice"} {
		if !names[want] {
			t.Errorf("expected %q to be disclosed, got %v", want, names)
		}
	}
	for _, forbidden := range []string{"lookup_rules", "update_state", "explore_knowledge"} {
		if names[forbidden] {
			t.Errorf("%q must NOT be disclosed under base-only conditions, got %v", forbidden, names)
		}
	}
}

func TestNarrator_Tools_WithFog(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	disclosure := fog.DisclosureState{}
	tc := &tools.ToolContext{
		World:      testWorld(),
		Rng:        rand.New(rand.NewPCG(1, 2)),
		Disclosure: &disclosure,
	}
	invokable, err := n.Tools(tc)
	if err != nil {
		t.Fatalf("Tools(): %v", err)
	}
	names := toolNames(t, invokable)
	for _, want := range []string{
		"get_entity_state", "roll", "random", "chance", "weighted_choice",
		"lookup_rules", "explore_knowledge",
	} {
		if !names[want] {
			t.Errorf("expected %q to be disclosed, got %v", want, names)
		}
	}
	if names["update_state"] {
		t.Errorf("update_state must NOT be disclosed without mutable entities, got %v", names)
	}
}

func TestNarrator_Judge(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	action := role.PlayerAction{PlayerID: "p1", Content: "I open the door."}
	j, err := n.Judge(context.Background(), action, testWorld())
	if err != nil {
		t.Fatalf("Judge(): %v", err)
	}
	if j.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", j.Outcome, "success")
	}
}

func TestNarrator_SuggestActions(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	narrative := "The ancient door creaks open, revealing a dimly lit chamber."
	choices, err := n.SuggestActions(context.Background(), testWorld(), players, narrative)
	if err != nil {
		t.Fatalf("SuggestActions(): %v", err)
	}
	if got := len(choices.Options); got < 2 || got > 4 {
		t.Errorf("expected 2-4 options, got %d", got)
	}
	for i, opt := range choices.Options {
		if opt.Label == "" {
			t.Errorf("option[%d].Label is empty", i)
		}
		if opt.Type == "" {
			t.Errorf("option[%d].Type is empty", i)
		}
	}
}

func TestNarrator_ImplementsGM(t *testing.T) {
	var _ role.GM = (*Narrator)(nil)
}
