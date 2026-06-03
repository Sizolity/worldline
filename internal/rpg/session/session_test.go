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
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/react"
	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/narrator"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/rule"
	"github.com/sizolity/worldline/internal/rpg/story"
	"github.com/sizolity/worldline/internal/rpg/tools"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/store"
)

// mockGM satisfies role.GM with deterministic, LLM-free behavior.
// suggestCalls is a recording channel for the inline-vs-fallback tests
// below; production code never touches it.
type mockGM struct {
	suggestCalls int
}

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

// suggestCalls is incremented on every SuggestActions invocation so
// tests can assert the inline-first / fallback policy: a healthy beat
// whose mock react agent emits set_choices must NOT exercise this
// path, while a beat whose mock omits the tool_call must.
func (m *mockGM) SuggestActions(_ context.Context, _ worldmodel.World, _ []role.Player, _ string) (role.ActionChoices, error) {
	m.suggestCalls++
	return role.ActionChoices{
		Options: []role.ActionOption{{Label: "fallback-suggest", Type: role.ActionTypeExplore}},
	}, nil
}

// ExtractInlineChoices satisfies role.InlineChoiceParser by delegating
// to the canonical narrator implementation — the same parser the
// production Narrator GM uses. This means tests cover the real
// extraction logic rather than a re-implementation that could drift
// silently.
func (m *mockGM) ExtractInlineChoices(toolCalls []schema.ToolCall) (role.ActionChoices, bool, error) {
	return narrator.ExtractInlineChoices(toolCalls)
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

// mockBeatAgentWithInlineChoices simulates the production happy-path
// where the beat agent streams narrative AND emits a set_choices
// tool_call in the same assistant message. Used to verify the
// pipeline's inline-first path: ExtractInlineChoices should consume
// the tool_call, no fallback SuggestActions LLM call should fire, and
// the resulting BeatResult.Choices should reflect the inline args.
//
// inlineArgs is the raw JSON string the mock places into the
// set_choices tool_call's Arguments field; callers parameterize it to
// test both happy parsing and (e.g.) malformed-args fallback.
type mockBeatAgentWithInlineChoices struct {
	inlineArgs string
}

func (m *mockBeatAgentWithInlineChoices) Run(_ context.Context, _ react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)
	go func() {
		defer close(narrativeCh)
		narrative := "He pauses at the threshold, listening to the muffled hum within."
		narrativeCh <- narrative
		doneCh <- react.Result{
			Narrative: narrative,
			ToolCalls: []schema.ToolCall{{
				ID: "call_mock_choices_1",
				Function: schema.FunctionCall{
					Name:      narrator.SetChoicesToolName,
					Arguments: m.inlineArgs,
				},
			}},
		}
	}()
	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

// mockBeatAgentErrorAfterStream emits narrative chunks (so the pipeline's
// startLoreExtraction fires Phase A on the streamed text) and THEN
// signals a stream-level error via beatResult.Err. This is the canonical
// path we need to test for the abort contract: the lore goroutine is
// alive and waiting on the post-effects world, but the pipeline will
// return early without ever calling attachLoreWorld — the deferred
// abortLoreTask must release the goroutine so it doesn't leak.
type mockBeatAgentErrorAfterStream struct{}

func (m *mockBeatAgentErrorAfterStream) Run(_ context.Context, _ react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)
	go func() {
		defer close(narrativeCh)
		narrativeCh <- "Some narrative chunk."
		doneCh <- react.Result{
			Narrative: "Some narrative chunk.",
			Err:       errors.New("synthetic beat-agent failure"),
		}
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

// mockRecordingBeatAgent records the react.Request the pipeline handed it
// so tests can assert HOW the beat agent was configured (e.g. an empty
// toolset + MaxStep=1 for MinimalTools beats, the full toolset + session
// MaxStep for normal beats). It otherwise behaves like a minimal
// narrative-only agent that emits no tool_calls. The recorded request is
// written inside Run (called from the beat goroutine) and read by the test
// only after Wait() returns, so the channel receive in Wait establishes a
// happens-before edge — no data race under -race.
type mockRecordingBeatAgent struct {
	gotReq react.Request
}

func (m *mockRecordingBeatAgent) Run(_ context.Context, req react.Request) *react.Stream {
	m.gotReq = req
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)
	go func() {
		defer close(narrativeCh)
		narrative := "A quiet recap of where you last stood."
		narrativeCh <- narrative
		doneCh <- react.Result{Narrative: narrative}
	}()
	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

func setupTestWorld(t *testing.T) (string, worldmodel.World) {
	t.Helper()
	dir := t.TempDir()

	combatRule := rule.NarrativeRule{
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

// TestWrapPlayerAction_IncludesSetChoicesReminder locks down the
// per-beat reminder added to fight model discipline drift: after a
// long Chinese narrative the model sometimes "forgot" the system
// prompt's set_choices mandate, so we reinforce it on every action
// beat's user message too. Quietly removing the reminder would
// regress fallback rate, hence the assertion here.
func TestWrapPlayerAction_IncludesSetChoicesReminder(t *testing.T) {
	wrapped := WrapPlayerAction("走到柜台前", "沈孤鸿")
	for _, want := range []string{
		"沈孤鸿本回合行动",
		"走到柜台前",
		"必须执行上述行动",
		// The load-bearing per-beat reminder that mirrors the
		// system-prompt trailer's mandate — locked down so a refactor
		// can't drop it silently.
		"收尾提醒",
		"必须在回复末尾调用一次 `set_choices`",
		"不要把选项写进叙事正文",
	} {
		if !strings.Contains(wrapped, want) {
			t.Errorf("wrapped action missing load-bearing phrase %q — was the per-beat reminder accidentally removed?", want)
		}
	}
}

// TestWrapPlayerAction_DefaultPlayerName covers the playerName=""
// fallback path. The fallback used to be "玩家" before the reminder
// was added; protect against an unrelated refactor breaking it.
func TestWrapPlayerAction_DefaultPlayerName(t *testing.T) {
	wrapped := WrapPlayerAction("看向门外", "")
	if !strings.Contains(wrapped, "玩家本回合行动") {
		t.Errorf("empty playerName should default to 玩家, got: %q", wrapped)
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
	if result.PrevLoreErr != nil {
		t.Errorf("expected nil PrevLoreErr on first beat, got %v", result.PrevLoreErr)
	}
	// No Lorekeeper configured → no background task should be spawned.
	if lore := sess.AwaitPendingLore(); lore.Pending {
		t.Errorf("expected no pending lore task when Lorekeeper unset, got %+v", lore)
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

	// Lorekeeper extraction now runs in the background; await it to observe
	// the outcome deterministically.
	lore := sess.AwaitPendingLore()
	if !lore.Pending {
		t.Fatal("expected a pending lore task when Lorekeeper is configured")
	}
	if lore.LoreErr != nil {
		t.Errorf("expected nil LoreErr, got %v", lore.LoreErr)
	}
	if lore.SaveErr != nil {
		t.Errorf("expected nil SaveErr, got %v", lore.SaveErr)
	}
	if lore.Report.Inserted != 1 {
		t.Errorf("expected Inserted=1, got %d (report=%+v)", lore.Report.Inserted, lore.Report)
	}

	// result.World is the post-effects snapshot (pre-enrichment); the extracted
	// entity lands only in the persisted enriched snapshot below.
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
	if result.Narrative == "" {
		t.Error("expected narrative to survive lore failure (graceful degrade)")
	}

	// The lore failure surfaces on the background task, not the beat result.
	lore := sess.AwaitPendingLore()
	if !lore.Pending {
		t.Fatal("expected a pending lore task")
	}
	if lore.LoreErr == nil {
		t.Fatal("expected background LoreErr to be non-nil")
	}
	if !strings.Contains(lore.LoreErr.Error(), "lorekeeper extract") {
		t.Errorf("expected LoreErr to wrap 'lorekeeper extract', got %v", lore.LoreErr)
	}
	if !reflect.DeepEqual(lore.Report, ingest.CompileReport{}) {
		t.Errorf("expected zero-value Report on parse failure, got %+v", lore.Report)
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

	lore := sess.AwaitPendingLore()
	if !lore.Pending {
		t.Fatal("expected a pending lore task")
	}
	if lore.LoreErr != nil {
		t.Fatalf("expected nil LoreErr, got %v", lore.LoreErr)
	}
	if lore.Report.Inserted != 1 {
		t.Errorf("expected Inserted=1 (only the valid entity), got %d (report=%+v)", lore.Report.Inserted, lore.Report)
	}

	var hasValidateNote bool
	for _, n := range lore.Report.Notes {
		if strings.HasPrefix(n, "validate-error:") || strings.HasPrefix(n, "validate-warn:") {
			hasValidateNote = true
			break
		}
	}
	if !hasValidateNote {
		t.Errorf("expected at least one validate-{error,warn}: note, got Notes=%v", lore.Report.Notes)
	}
	if lk.calls != 1 {
		t.Errorf("expected mockLorekeeper.calls == 1, got %d", lk.calls)
	}
}

// === Inline-choices contract ===

// TestRunBeat_InlineChoices_HappyPath verifies the production happy
// path: the beat agent emits a valid set_choices tool_call inline,
// ExtractInlineChoices succeeds, and the SuggestActions fallback is
// NEVER invoked — that absence is the optimization's whole point.
func TestRunBeat_InlineChoices_HappyPath(t *testing.T) {
	dir, _ := setupTestWorld(t)
	gm := &mockGM{}

	sess, err := New(Config{
		GM:            gm,
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent: &mockBeatAgentWithInlineChoices{
			inlineArgs: `{"options":[
				{"label":"step inside","type":"explore"},
				{"label":"hail the keeper","type":"social"},
				{"label":"","type":"custom"}
			]}`,
		},
		MaxStep: 5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "stand at the threshold"},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if result.SuggestErr != nil {
		t.Errorf("SuggestErr should be nil on inline happy path, got %v", result.SuggestErr)
	}
	if gm.suggestCalls != 0 {
		t.Errorf("SuggestActions fallback must NOT fire on inline happy path; called %d time(s)", gm.suggestCalls)
	}
	if got, want := len(result.Choices.Options), 3; got != want {
		t.Fatalf("expected %d inline choices, got %d", want, got)
	}
	if got := result.Choices.Options[0].Label; got != "step inside" {
		t.Errorf("Options[0].Label = %q, want %q", got, "step inside")
	}
	if !result.Choices.HasCustomSlot() {
		t.Error("inline choices should preserve the trailing custom slot")
	}
}

// TestRunBeat_InlineChoices_FallbackOnMissing verifies graceful
// degradation: when the beat agent omits set_choices (prompt drift,
// model regression, mod author misconfig), the pipeline falls back to
// the legacy SuggestActions LLM call instead of stranding the player
// with empty options.
func TestRunBeat_InlineChoices_FallbackOnMissing(t *testing.T) {
	dir, _ := setupTestWorld(t)
	gm := &mockGM{}

	// mockBeatAgent emits a narrative but NO set_choices tool_call —
	// exactly the "model forgot to call the tool" scenario.
	sess, err := New(Config{
		GM:            gm,
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
		Action:  role.PlayerAction{PlayerID: "p1", Content: "look around"},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if gm.suggestCalls != 1 {
		t.Errorf("SuggestActions fallback must fire exactly once when inline missing; got %d", gm.suggestCalls)
	}
	if got, want := len(result.Choices.Options), 1; got != want {
		t.Fatalf("expected %d fallback options, got %d", want, got)
	}
	if got := result.Choices.Options[0].Label; got != "fallback-suggest" {
		t.Errorf("expected fallback Label %q, got %q", "fallback-suggest", got)
	}
}

// TestRunBeat_SuppressChoices_NoSuggestCall verifies the
// BeatInput.SuppressChoices contract: when a beat declares its scene
// is choice-less by design (recap / prologue), the pipeline must NOT
// run inline extraction AND must NOT fall back to SuggestActions even
// though no set_choices tool_call was emitted. The whole point of the
// flag is to avoid the wasteful ~2s LLM call on these by-design
// no-options beats. result.Choices should also be the zero-value so
// downstream UI renders the "(无推荐选项)" branch correctly.
func TestRunBeat_SuppressChoices_NoSuggestCall(t *testing.T) {
	dir, _ := setupTestWorld(t)
	gm := &mockGM{}

	sess, err := New(Config{
		GM:            gm,
		Players:       testPlayers(),
		WorkspacePath: dir,
		// mockBeatAgent never emits a set_choices tool_call. Without
		// SuppressChoices the pipeline would fire the fallback (we
		// have separate tests covering that path). With it, neither
		// inline nor fallback should run.
		BeatAgent: &mockBeatAgent{},
		MaxStep:   5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID:         "world-test-01",
		Action:          role.PlayerAction{PlayerID: "p1", Content: "recap-style prompt"},
		SuppressChoices: true,
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if gm.suggestCalls != 0 {
		t.Errorf("SuggestActions must NOT fire when SuppressChoices=true; called %d time(s)", gm.suggestCalls)
	}
	if len(result.Choices.Options) != 0 {
		t.Errorf("Choices.Options must be empty when SuppressChoices=true, got %d options", len(result.Choices.Options))
	}
	if result.SuggestErr != nil {
		t.Errorf("SuggestErr must be nil when SuppressChoices=true (no fallback ran), got %v", result.SuggestErr)
	}
	// The timing trace must NOT contain a "suggest=" column — its
	// presence would mean the fallback ran despite the flag.
	if strings.Contains(result.TimingTrace, "suggest=") {
		t.Errorf("TimingTrace contains suggest= despite SuppressChoices=true: %q", result.TimingTrace)
	}
}

// TestRunBeat_MinimalTools_EmptyToolset verifies the recap/prologue
// latency fix: a MinimalTools beat must hand the beat agent an EMPTY
// toolset (so the model cannot emit pre-narrative tool_calls and streams
// in one round-trip) and cap MaxStep=1 as a belt-and-suspenders. The beat
// must otherwise complete normally (narrative + clock advance + save).
func TestRunBeat_MinimalTools_EmptyToolset(t *testing.T) {
	dir, _ := setupTestWorld(t)
	agent := &mockRecordingBeatAgent{}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     agent,
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "续接场景"},
		// Mirrors how the CLI uses the two flags together for recap/
		// prologue, but the assertions below isolate the MinimalTools
		// behavior (toolset + MaxStep).
		MinimalTools:    true,
		SuppressChoices: true,
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if len(agent.gotReq.Tools) != 0 {
		t.Errorf("MinimalTools beat must hand the agent an EMPTY toolset, got %d tools", len(agent.gotReq.Tools))
	}
	if agent.gotReq.MaxStep != 1 {
		t.Errorf("MinimalTools beat should cap MaxStep=1, got %d", agent.gotReq.MaxStep)
	}
	if result.Narrative == "" {
		t.Error("MinimalTools beat should still produce narrative")
	}
	if result.World.Clock.Sequence != 6 {
		t.Errorf("MinimalTools beat should still advance the clock (6), got %d", result.World.Clock.Sequence)
	}
}

// TestRunBeat_NormalBeat_FullToolsetAndMaxStep locks the other side of the
// MinimalTools branch: a normal beat (flag unset) must still receive the
// GM's full game toolset and the session-configured MaxStep, unchanged.
func TestRunBeat_NormalBeat_FullToolsetAndMaxStep(t *testing.T) {
	dir, _ := setupTestWorld(t)
	agent := &mockRecordingBeatAgent{}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     agent,
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "I push the door."},
		// MinimalTools defaults false.
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if len(agent.gotReq.Tools) == 0 {
		t.Error("normal beat must hand the agent the full game toolset, got 0 tools")
	}
	if agent.gotReq.MaxStep != 5 {
		t.Errorf("normal beat should use the session MaxStep (5), got %d", agent.gotReq.MaxStep)
	}
}

// TestRunBeat_LoreAbortsOnBeatError verifies the two-phase lore task's
// abort path: when the pipeline errors AFTER startLoreExtraction has
// already kicked off Phase A but BEFORE attachLoreWorld can hand off
// the post-effects world, the deferred abortLoreTask must close the
// task cleanly with errLoreAborted (no goroutine leak, no half-merged
// world on disk).
//
// We force this by having the beat agent emit narrative and then
// signal a stream-level error via beatResult.Err — the pipeline sees
// the error after <-stream.Done and bails before save, exercising the
// exact ordering the new optimization relies on.
func TestRunBeat_LoreAbortsOnBeatError(t *testing.T) {
	dir, _ := setupTestWorld(t)
	// The mockLorekeeper's Parse will succeed synchronously and return
	// a Draft with one entity. Phase B should NEVER run, so this entity
	// must NOT land on disk despite Parse being called.
	lk := &mockLorekeeper{
		draft: ingest.Draft{Entities: []ingest.DraftEntity{{
			ID:         "ent_must_not_persist",
			Type:       "character",
			Name:       "Phantom",
			Confidence: 0.9,
		}}},
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent:     &mockBeatAgentErrorAfterStream{},
		MaxStep:       5,
		Lorekeeper:    lk,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "trigger beat error"},
	}).Wait()
	if result.Err == nil {
		t.Fatal("expected RunBeat to surface the beat-agent error")
	}
	if !strings.Contains(result.Err.Error(), "synthetic beat-agent failure") {
		t.Errorf("expected wrapped beat-agent error, got %v", result.Err)
	}

	lore := sess.AwaitPendingLore()
	if !lore.Pending {
		t.Fatal("expected a pending lore task even on beat error (Phase A had already started)")
	}
	if lore.LoreErr == nil {
		t.Fatal("expected loreErr to indicate the task was aborted")
	}
	if !errors.Is(lore.LoreErr, errLoreAborted) {
		t.Errorf("expected errLoreAborted, got %v", lore.LoreErr)
	}
	if lore.SaveErr != nil {
		t.Errorf("SaveErr must be nil — Phase B (save) never runs on abort, got %v", lore.SaveErr)
	}
	if lore.Report.Inserted != 0 {
		t.Errorf("Report.Inserted must be 0 — Phase B (compile) never runs on abort, got %d", lore.Report.Inserted)
	}

	// Phase A still ran (it only depends on narrative + sourceID), so
	// Parse was called exactly once — confirming this test really
	// exercises the early-start path (otherwise it'd be 0).
	if lk.calls != 1 {
		t.Errorf("expected lk.calls == 1 (Phase A ran), got %d", lk.calls)
	}

	// The Phantom entity from the mock draft must NOT have landed on
	// disk because Phase B was aborted before CompileDraft and Save.
	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if _, ok := loaded.Entities["ent_must_not_persist"]; ok {
		t.Error("ent_must_not_persist landed on disk despite abort — Phase B leaked through")
	}
}

// TestRunBeat_InlineChoices_FallbackOnBadJSON verifies the second
// degraded mode: the model DID call set_choices but emitted args that
// won't unmarshal. The pipeline must still fall back rather than
// surface a hard error.
func TestRunBeat_InlineChoices_FallbackOnBadJSON(t *testing.T) {
	dir, _ := setupTestWorld(t)
	gm := &mockGM{}

	sess, err := New(Config{
		GM:            gm,
		Players:       testPlayers(),
		WorkspacePath: dir,
		BeatAgent: &mockBeatAgentWithInlineChoices{
			inlineArgs: `{"options": [`, // truncated — sonic will reject
		},
		MaxStep: 5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	result := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action:  role.PlayerAction{PlayerID: "p1", Content: "open the door"},
	}).Wait()
	if result.Err != nil {
		t.Fatalf("RunBeat: %v", result.Err)
	}
	if gm.suggestCalls != 1 {
		t.Errorf("SuggestActions fallback must fire when set_choices args fail to parse; got %d", gm.suggestCalls)
	}
	if got := result.Choices.Options[0].Label; got != "fallback-suggest" {
		t.Errorf("expected fallback Label %q, got %q", "fallback-suggest", got)
	}
}
