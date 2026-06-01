package mod

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/rpg/rule"
	worldmodel "github.com/sizolity/worldline/world/model"
)

func TestCompileScenarioToWorld_XiyouChangan(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	world, err := CompileScenarioToWorld(sc, sc.ID)
	if err != nil {
		t.Fatalf("CompileScenarioToWorld: %v", err)
	}
	if err := world.Validate(); err != nil {
		t.Fatalf("compiled world fails Validate(): %v", err)
	}

	if string(world.ID) != "xiyou-changan" {
		t.Errorf("world.ID = %q", world.ID)
	}
	if !strings.Contains(world.Name, "西游") {
		t.Errorf("world.Name missing 西游: %q", world.Name)
	}

	// ID derivation invariants (directive 2.8).
	expectIDs := []worldmodel.EntityID{
		"hero-sun_wukong",
		"npc-tang_sanzang",
		"npc-zhu_bajie",
		"npc-sha_wujing",
		"npc-bai_longma",
		"npc-baigu_furen",
		"loc-changan_gate",
		"loc-liangjie_shan",
		"loc-baigu_ling",
	}
	for _, id := range expectIDs {
		if _, ok := world.Entities[id]; !ok {
			t.Errorf("missing expected entity %q", id)
		}
	}

	// Player must be tagged for prompt rendering / suggester filtering.
	player, ok := world.Entities["hero-sun_wukong"]
	if !ok {
		t.Fatal("hero-sun_wukong missing")
	}
	hasPlayerTag := false
	for _, tag := range player.Tags {
		if tag == "player" {
			hasPlayerTag = true
		}
	}
	if !hasPlayerTag {
		t.Errorf("hero-sun_wukong missing `player` tag, tags=%v", player.Tags)
	}

	// v1 red line — zero numerics on entity state.
	for id, e := range world.Entities {
		if len(e.State) != 0 {
			t.Errorf("entity %q has State payload but v1 requires empty State, got %v", id, e.State)
		}
	}

	// Threads use position-index IDs.
	wantThreadIDs := []worldmodel.ThreadID{"thread-1", "thread-2"}
	if len(world.Threads) != len(wantThreadIDs) {
		t.Fatalf("threads = %d, want %d", len(world.Threads), len(wantThreadIDs))
	}
	for i, want := range wantThreadIDs {
		if world.Threads[i].ID != want {
			t.Errorf("thread[%d].ID = %q, want %q", i, world.Threads[i].ID, want)
		}
	}
	// First thread must start active so SuggestActions has something to
	// anchor on at beat 0.
	if world.Threads[0].Status != worldmodel.ThreadStatusActive {
		t.Errorf("thread[0].status = %q, want active", world.Threads[0].Status)
	}

	// Rules use position-index IDs and contain no d20/DC/AC content.
	if len(world.Rules) != 4 {
		t.Fatalf("rules = %d, want 4", len(world.Rules))
	}
	rpgRules := rule.FromWorldRules(world.Rules)
	if len(rpgRules) != 4 {
		t.Fatalf("rule conversion = %d, want 4", len(rpgRules))
	}
	forbidden := []string{"d20", "DC", "AC", "HP", "modifier"}
	for _, r := range rpgRules {
		for _, bad := range forbidden {
			if strings.Contains(r.Content, bad) {
				t.Errorf("rule %q contains forbidden mechanic %q: %s", r.ID, bad, r.Content)
			}
		}
	}
	wantRuleIDs := []worldmodel.RuleID{"rule-1", "rule-2", "rule-3", "rule-4"}
	for i, want := range wantRuleIDs {
		if rpgRules[i].ID != want {
			t.Errorf("rule[%d].ID = %q, want %q", i, rpgRules[i].ID, want)
		}
	}

	// EventLog carries the qicheng starter event with `evt-` prefix.
	if len(world.EventLog) != 1 {
		t.Fatalf("event_log = %d, want 1", len(world.EventLog))
	}
	if world.EventLog[0].ID != "evt-qicheng" {
		t.Errorf("event[0].ID = %q, want evt-qicheng", world.EventLog[0].ID)
	}
	if world.EventLog[0].Type != worldmodel.EventTypeNote {
		t.Errorf("event[0].Type = %q, want note", world.EventLog[0].Type)
	}

	// Clock starts at scene=1, sequence=1.
	if world.Clock.Sequence != 1 {
		t.Errorf("clock.sequence = %d, want 1", world.Clock.Sequence)
	}
	if world.Clock.Current.Kind != worldmodel.WorldTimeScene {
		t.Errorf("clock.kind = %q, want scene", world.Clock.Current.Kind)
	}

	// Source provenance reaches Metadata.
	if world.Metadata.Source != "mod:xiyou-changan" {
		t.Errorf("metadata.source = %q, want mod:xiyou-changan", world.Metadata.Source)
	}
}

func TestPlayerEntityID_DerivesFromFileSlug(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if got := PlayerEntityID(sc); got != "hero-sun_wukong" {
		t.Errorf("PlayerEntityID = %q, want hero-sun_wukong", got)
	}
	if got := PlayerCharacterName(sc); got != "孙悟空" {
		t.Errorf("PlayerCharacterName = %q, want 孙悟空", got)
	}
}

func TestCompileScenarioToWorldLines_Xiyou(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	lines := CompileScenarioToWorldLines(sc)
	if len(lines) != 1 {
		t.Fatalf("worldlines = %d, want 1", len(lines))
	}
	l := lines[0]
	if l.ID != "wl_shitu" {
		t.Errorf("worldline.ID = %q, want wl_shitu", l.ID)
	}
	if l.ThreadID != "thread-2" {
		t.Errorf("worldline.ThreadID = %q, want thread-2 (师徒嫌隙)", l.ThreadID)
	}
	if err := l.Validate(); err != nil {
		t.Errorf("worldline.Validate(): %v", err)
	}
	// Both milestones must bind to npc-tang_sanzang under the new IDs.
	for _, m := range l.Milestones {
		for _, e := range m.Effects {
			if e.Kind == worldmodel.EffectUpdateEntityState && e.TargetID != "npc-tang_sanzang" {
				t.Errorf("effect targets %q, expected npc-tang_sanzang", e.TargetID)
			}
		}
	}
}

func TestCompileScenarioToWorldLines_UnknownScenario(t *testing.T) {
	sc := &Scenario{ID: "completely-unknown", PlayerIndex: -1}
	if got := CompileScenarioToWorldLines(sc); got != nil {
		t.Errorf("expected nil worldlines for unknown scenario, got %v", got)
	}
}
