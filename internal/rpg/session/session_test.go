package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/sizolity/worldline/internal/agent/react"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/store"
	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/rule"
	"github.com/sizolity/worldline/internal/rpg/story"
	"github.com/sizolity/worldline/internal/rpg/tools"
)

// mockGM satisfies role.GM with deterministic, LLM-free behavior.
type mockGM struct{}

func (m *mockGM) Role() string { return "MockGM" }

func (m *mockGM) SystemPrompt(_ []role.Player, opts role.PromptOptions) string {
	return "You are a test GM. World: " + opts.WorldCtx.World.Name
}

func (m *mockGM) Tools(tc any) ([]einotool.BaseTool, error) {
	return tools.NewInvokableTools(tc.(*tools.ToolContext)), nil
}

func (m *mockGM) Judge(_ context.Context, _ role.PlayerAction, _ worldmodel.World) (role.Judgment, error) {
	return role.Judgment{Outcome: "success"}, nil
}

func (m *mockGM) SuggestActions(_ context.Context, _ worldmodel.World, _ []role.Player, _ string) (role.ActionChoices, error) {
	return role.ActionChoices{
		Options: []role.ActionOption{{Label: "test action", Type: role.ActionTypeExplore}},
	}, nil
}

// mockBeatAgent simulates a beat agent for tests. It performs one tool call
// (roll) and then produces a narrative.
type mockBeatAgent struct{}

func (m *mockBeatAgent) Run(_ context.Context, req react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)

	go func() {
		defer close(narrativeCh)

		narrative := "The ancient door creaks open, revealing a dimly lit chamber. Your torch flickers as cold air rushes past. In the center, a stone pedestal holds a glowing crystal."

		// Simulate a tool call (roll) if tools are available
		ctx := context.Background()
		for _, t := range req.Tools {
			info, err := t.Info(ctx)
			if err != nil || info.Name != "roll" {
				continue
			}
			if inv, ok := t.(einotool.InvokableTool); ok {
				_, _ = inv.InvokableRun(ctx, `{"sides":20,"count":1,"modifier":2}`)
			}
			break
		}

		narrativeCh <- narrative
		doneCh <- react.Result{Narrative: narrative}
	}()

	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

// mockExploreBeatAgent simulates a beat agent that calls explore_knowledge.
type mockExploreBeatAgent struct{}

func (m *mockExploreBeatAgent) Run(_ context.Context, req react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)

	go func() {
		defer close(narrativeCh)

		// Call explore_knowledge tool if available
		ctx := context.Background()
		for _, t := range req.Tools {
			info, err := t.Info(ctx)
			if err != nil || info.Name != "explore_knowledge" {
				continue
			}
			if inv, ok := t.(einotool.InvokableTool); ok {
				_, _ = inv.InvokableRun(ctx, `{"target_id":"loc-cavern","level":"explored"}`)
			}
			break
		}

		narrative := "You discover the Ancient Cavern in full detail."
		narrativeCh <- narrative
		doneCh <- react.Result{Narrative: narrative}
	}()

	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

func setupTestWorld(t *testing.T) (string, worldmodel.World) {
	t.Helper()
	dir := t.TempDir()

	combatRule := rule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 0,
		Content: "Attack rolls use d20 + modifier", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}

	world := worldmodel.World{
		ID:   "world-test-01",
		Name: "Crystal Caverns",
		Canon: worldmodel.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"mysterious", "dark"},
		},
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-arin": {
				ID: "hero-arin", Type: "character", Name: "Arin",
				Tags: []string{"player", "warrior"},
				State: map[string]worldmodel.Value{
					"hp":       {Kind: worldmodel.ValueKindNumber, Raw: float64(25)},
					"strength": {Kind: worldmodel.ValueKindNumber, Raw: float64(14)},
				},
			},
			"loc-cavern": {
				ID: "loc-cavern", Type: "location", Name: "Ancient Cavern",
				Description: "A deep underground cavern with glowing crystals.",
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-explore", Kind: worldmodel.ThreadKindQuest,
				Title:  "Explore the Crystal Caverns",
				Status: worldmodel.ThreadStatusActive,
			},
		},
		Rules: []worldmodel.Rule{
			rule.ToModelRule(combatRule),
		},
		Clock: worldmodel.WorldClock{Sequence: 5},
	}

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("save test world: %v", err)
	}

	return dir, world
}

func testPlayers() []role.Player {
	return []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Tester"},
	}
}

func TestRunBeat_FullPipeline(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I push open the ancient door.",
		},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}

	if result.Narrative == "" {
		t.Error("expected non-empty narrative")
	}
	if result.World.Clock.Sequence != 6 {
		t.Errorf("expected clock sequence 6, got %d", result.World.Clock.Sequence)
	}
	if len(result.Choices.Options) == 0 {
		t.Error("expected at least one suggested action option")
	}

	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if loaded.Clock.Sequence != 6 {
		t.Errorf("persisted clock sequence: expected 6, got %d", loaded.Clock.Sequence)
	}
}

func TestRunBeat_WithToolCalls(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I attack the stone golem.",
		},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}

	if result.Narrative == "" {
		t.Error("expected non-empty narrative")
	}
	if !strings.Contains(result.Narrative, "ancient door") {
		t.Errorf("unexpected narrative: %q", result.Narrative)
	}
}

func TestNew_Validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"empty config", Config{}},
		{"missing GM", Config{WorkspacePath: "/tmp/test", BeatAgent: &mockBeatAgent{}}},
		{"missing workspace", Config{GM: &mockGM{}, BeatAgent: &mockBeatAgent{}}},
		{"missing beat agent", Config{GM: &mockGM{}, WorkspacePath: "/tmp/test"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.cfg); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestWorldPersistence(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(dir)

	world := worldmodel.World{
		ID:   "persist-test",
		Name: "Persistence Test",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"e1": {ID: "e1", Name: "Entity1", Type: "character"},
		},
	}
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("save: %v", err)
	}

	files, _ := os.ReadDir(filepath.Join(dir, "worlds"))
	if len(files) == 0 {
		t.Fatal("expected world file to be created")
	}

	loaded, err := fs.LoadSnapshot(context.Background(), "persist-test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Name != "Persistence Test" {
		t.Errorf("expected name 'Persistence Test', got %q", loaded.Name)
	}
}

func TestRunBeat_WithFog(t *testing.T) {
	dir, _ := setupTestWorld(t)
	worldsDir := filepath.Join(dir, "worlds")

	fogStore := fog.NewStore(worldsDir)
	initState := fog.DisclosureState{
		Entities: map[worldmodel.EntityID]fog.EntityDisclosure{
			"hero-arin": {Level: fog.Explored},
		},
	}
	if err := fogStore.Save("world-test-01", initState); err != nil {
		t.Fatalf("seed disclosure: %v", err)
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockExploreBeatAgent{},
		MaxStep:       5,
		FogEnabled:    true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I explore the cavern entrance.",
		},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}

	if result.Narrative == "" {
		t.Error("expected narrative")
	}

	updated, err := fogStore.Load("world-test-01")
	if err != nil {
		t.Fatalf("load disclosure: %v", err)
	}
	level := updated.GetEntityLevel("loc-cavern")
	if level != fog.Explored {
		t.Errorf("expected loc-cavern to be explored, got %s", level)
	}
}

// === WorldLine integration ===

func setupTestWorldWithSceneClock(t *testing.T) string {
	t.Helper()
	dir, world := setupTestWorld(t)
	world.Clock.Current = worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: world.Clock.Sequence}
	if err := store.NewFileStore(dir).SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("re-save world with scene clock: %v", err)
	}
	return dir
}

func TestRunBeat_WorldLine_DriftAndMilestone(t *testing.T) {
	dir := setupTestWorldWithSceneClock(t)

	worldsDir := filepath.Join(dir, "worlds")
	fs := store.NewFileStore(dir)
	world, _ := fs.LoadSnapshot(context.Background(), "world-test-01")
	world.Threads[0].Tension = 0.5
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("seed thread tension: %v", err)
	}

	storyStore := story.NewStore(worldsDir)
	lines := []story.WorldLine{{
		ID:         "wl_explore",
		ThreadID:   worldmodel.ThreadID("thread-explore"),
		Visibility: story.VisibilityHinted,
		Drift:      story.Drift{Scene: 0.25},
		Milestones: []story.Milestone{{
			ID: "m_crisis",
			Condition: story.MilestoneCondition{
				Kind: story.CondThreadTensionGTE,
				Args: map[string]any{"thread_id": "thread-explore", "threshold": 0.70},
			},
			Effects: []worldmodel.Effect{{
				Kind:     worldmodel.EffectUpdateThread,
				TargetID: "thread-explore",
				Payload: map[string]worldmodel.Value{
					"status": {Kind: worldmodel.ValueKindString, Raw: worldmodel.ThreadStatusActive},
				},
			}},
		}},
	}}
	if err := storyStore.Save("world-test-01", lines); err != nil {
		t.Fatalf("seed worldlines: %v", err)
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
		StoryEnabled:  true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I peer deeper into the cavern.",
		},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}

	var th worldmodel.WorldThread
	for _, x := range result.World.Threads {
		if x.ID == "thread-explore" {
			th = x
		}
	}
	if th.Tension <= 0.5 {
		t.Errorf("expected drifted tension > 0.5, got %v", th.Tension)
	}

	updatedLines, err := storyStore.Load("world-test-01")
	if err != nil {
		t.Fatalf("load worldlines: %v", err)
	}
	if len(updatedLines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(updatedLines))
	}
	if !updatedLines[0].Milestones[0].Triggered {
		t.Errorf("expected milestone Triggered=true after crossing threshold")
	}

	result2 := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I press on.",
		},
	}).Wait()
	if result2.Err != nil {
		t.Fatalf("RunBeat #2: %v", result2.Err)
	}
	for _, x := range result2.World.Threads {
		if x.ID == "thread-explore" {
			if x.Tension < 0.99 {
				t.Errorf("expected tension near 1.0 after second drift, got %v", x.Tension)
			}
		}
	}
	finalLines, _ := storyStore.Load("world-test-01")
	if !finalLines[0].Milestones[0].Triggered {
		t.Errorf("milestone Triggered should persist as true across beats")
	}
}

func TestRunBeat_WorldLineDisabled_NoFile(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if result := (sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "x"},
	})).Wait(); result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}

	path := filepath.Join(dir, "worlds", "world-test-01", "worldlines.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no worldlines.json when StoryEnabled=false, stat=%v", err)
	}
}

// === Lorekeeper integration ===

type mockLorekeeper struct {
	draft   ingest.Draft
	err     error
	calls   int
	lastDoc ingest.SourceDocument
}

func (m *mockLorekeeper) Parse(_ context.Context, doc ingest.SourceDocument) (ingest.Draft, error) {
	m.calls++
	m.lastDoc = doc
	if m.err != nil {
		return ingest.Draft{}, m.err
	}
	return m.draft, nil
}

func TestRunBeat_LorekeeperDisabled(t *testing.T) {
	dir, baseWorld := setupTestWorld(t)

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "I look around."},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if result.LoreErr != nil {
		t.Errorf("expected nil LoreErr when Lorekeeper unset, got %v", result.LoreErr)
	}
	if !reflect.DeepEqual(result.LoreReport, ingest.CompileReport{}) {
		t.Errorf("expected zero-value LoreReport, got %+v", result.LoreReport)
	}

	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if len(loaded.Entities) != len(baseWorld.Entities) {
		t.Errorf("expected entity count unchanged (%d), got %d", len(baseWorld.Entities), len(loaded.Entities))
	}
}

func TestRunBeat_LorekeeperSuccess(t *testing.T) {
	dir, _ := setupTestWorld(t)
	lk := &mockLorekeeper{
		draft: ingest.Draft{
			Entities: []ingest.DraftEntity{{
				ID:         "ent_crystal_keeper",
				Type:       "character",
				Name:       "Crystal Keeper",
				Confidence: 0.9,
			}},
		},
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
		Lorekeeper:    lk,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "I greet the keeper."},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if result.LoreErr != nil {
		t.Errorf("expected nil LoreErr, got %v", result.LoreErr)
	}
	if result.LoreReport.Inserted != 1 {
		t.Errorf("expected Inserted=1, got %d (report=%+v)", result.LoreReport.Inserted, result.LoreReport)
	}

	if got, ok := result.World.Entities["ent_crystal_keeper"]; !ok {
		t.Error("expected ent_crystal_keeper in result.World, missing")
	} else if got.Name != "Crystal Keeper" {
		t.Errorf("expected Name=Crystal Keeper, got %q", got.Name)
	}

	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if got, ok := loaded.Entities["ent_crystal_keeper"]; !ok {
		t.Error("expected ent_crystal_keeper persisted in FileStore, missing")
	} else if got.Name != "Crystal Keeper" {
		t.Errorf("persisted Name=Crystal Keeper expected, got %q", got.Name)
	}

	if lk.calls != 1 {
		t.Errorf("expected mockLorekeeper.calls == 1, got %d", lk.calls)
	}
	wantID := "beat_world-test-01_5"
	if lk.lastDoc.ID != wantID {
		t.Errorf("expected SourceDocument.ID = %q, got %q", wantID, lk.lastDoc.ID)
	}
	if lk.lastDoc.Kind != "rpg_beat" {
		t.Errorf("expected SourceDocument.Kind = %q, got %q", "rpg_beat", lk.lastDoc.Kind)
	}
	if lk.lastDoc.Text == "" {
		t.Error("expected non-empty SourceDocument.Text")
	}
}

func TestRunBeat_LorekeeperParseFails(t *testing.T) {
	dir, _ := setupTestWorld(t)
	lk := &mockLorekeeper{err: errors.New("synthetic lore failure")}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
		Lorekeeper:    lk,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "Trigger a lore failure."},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("expected RunBeat to succeed despite lore failure, got %v", result.Err)
	}
	if result.LoreErr == nil {
		t.Fatal("expected LoreErr to be non-nil")
	}
	if !strings.Contains(result.LoreErr.Error(), "lorekeeper extract") {
		t.Errorf("expected LoreErr to wrap 'lorekeeper extract', got %v", result.LoreErr)
	}
	if !reflect.DeepEqual(result.LoreReport, ingest.CompileReport{}) {
		t.Errorf("expected zero-value LoreReport on parse failure, got %+v", result.LoreReport)
	}
	if result.Narrative == "" {
		t.Error("expected narrative to survive lore failure (graceful degrade)")
	}

	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if loaded.Clock.Sequence != 6 {
		t.Errorf("expected clock sequence advanced to 6, got %d", loaded.Clock.Sequence)
	}
	if _, ok := loaded.Entities["ent_crystal_keeper"]; ok {
		t.Error("expected NO new entities from failed extraction")
	}
	if lk.calls != 1 {
		t.Errorf("expected mockLorekeeper.calls == 1, got %d", lk.calls)
	}
}

func TestRunBeat_LorekeeperReportNotesIncludeValidate(t *testing.T) {
	dir, _ := setupTestWorld(t)
	lk := &mockLorekeeper{
		draft: ingest.Draft{
			Entities: []ingest.DraftEntity{
				{
					ID:         "ent_valid_npc",
					Type:       "character",
					Name:       "Valid NPC",
					Confidence: 0.9,
				},
				{
					ID:         "ent_no_name",
					Type:       "character",
					Name:       "",
					Confidence: 0.9,
				},
			},
		},
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgent{},
		MaxStep:       5,
		Lorekeeper:    lk,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "Stress test the validator."},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if result.LoreErr != nil {
		t.Fatalf("expected nil LoreErr, got %v", result.LoreErr)
	}
	if result.LoreReport.Inserted != 1 {
		t.Errorf("expected Inserted=1 (only the valid entity), got %d (report=%+v)", result.LoreReport.Inserted, result.LoreReport)
	}

	var hasValidateNote bool
	for _, n := range result.LoreReport.Notes {
		if strings.HasPrefix(n, "validate-error:") || strings.HasPrefix(n, "validate-warn:") {
			hasValidateNote = true
			break
		}
	}
	if !hasValidateNote {
		t.Errorf("expected at least one validate-{error,warn}: note, got Notes=%v", result.LoreReport.Notes)
	}
	if lk.calls != 1 {
		t.Errorf("expected mockLorekeeper.calls == 1, got %d", lk.calls)
	}
}
