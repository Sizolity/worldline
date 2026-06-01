//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"

	einobeat "github.com/sizolity/worldline/agent/eino/beat"
	einostructured "github.com/sizolity/worldline/agent/eino/structured"
	worldmodel "github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/world/store"
	"github.com/sizolity/worldline/rpg/narrator"
	"github.com/sizolity/worldline/rpg/role"
	"github.com/sizolity/worldline/rpg/session"
	"github.com/sizolity/worldline/rpg/story"
)

func TestBeat_DeepSeek_WorldLine_E2E(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Fatal("DEEPSEEK_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: "https://api.deepseek.com/v1",
		APIKey:  apiKey,
		Model:   "deepseek-chat",
		Timeout: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("create chat model: %v", err)
	}

	dir := t.TempDir()

	world := buildTestWorld()
	world.Clock.Current = worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: world.Clock.Sequence}
	for i := range world.Threads {
		if world.Threads[i].ID == "thread-seal" {
			world.Threads[i].Tension = 0.55
		}
	}

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("save world: %v", err)
	}

	worldsDir := dir + "/worlds"
	storyStore := story.NewStore(worldsDir)
	lines := []story.WorldLine{{
		ID:           "wl_seal",
		ThreadID:     worldmodel.ThreadID("thread-seal"),
		Visibility:   story.VisibilityHinted,
		CurrentStage: "tremors",
		Drift:        story.Drift{Scene: 0.20},
		Milestones: []story.Milestone{{
			ID: "m_seal_cracks",
			Condition: story.MilestoneCondition{
				Kind: story.CondThreadTensionGTE,
				Args: map[string]any{"thread_id": "thread-seal", "threshold": 0.70},
			},
			Effects: []worldmodel.Effect{{
				Kind:     worldmodel.EffectUpdateThread,
				TargetID: "thread-seal",
				Payload: map[string]worldmodel.Value{
					"tension": {Kind: worldmodel.ValueKindNumber, Raw: 0.95},
				},
			}},
		}},
	}}
	if err := storyStore.Save(string(world.ID), lines); err != nil {
		t.Fatalf("seed worldlines: %v", err)
	}

	suggestAgent := einostructured.NewToolCall[narrator.SuggestParams](chatModel, narrator.SuggestToolName, narrator.SuggestToolDesc)
	gm, err := narrator.New(suggestAgent)
	if err != nil {
		t.Fatalf("create narrator: %v", err)
	}

	beatAgent := einobeat.New(chatModel)
	player := role.Player{ID: "player-1", Name: "Kael", CharacterID: "hero-kael"}

	sess, err := session.New(session.Config{
		GM:            gm,
		Players:       []role.Player{player},
		WorkspacePath: dir,
		BeatAgent:     beatAgent,
		MaxStep:       5,
		StoryEnabled:  true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// --- Beat 1 ---
	t.Log("=== Beat 1 ===")
	stream1 := sess.RunBeat(ctx, session.BeatInput{
		WorldID: string(world.ID),
		Action: role.PlayerAction{
			PlayerID: player.ID,
			Content:  "I press my palm against the cold iron of the broken seal and listen for the heartbeat behind it.",
		},
		RecentEvents: 5,
	})
	out1 := drainBeat(t, 1, stream1)
	if out1.Err != nil {
		t.Fatalf("RunBeat #1: %v", out1.Err)
	}

	tensionAfter1 := findThreadTension(out1.World, "thread-seal")
	t.Logf("thread-seal tension after beat 1: %.3f", tensionAfter1)
	if tensionAfter1 <= 0.55 {
		t.Errorf("expected tension to drift above 0.55 after beat 1, got %.3f", tensionAfter1)
	}

	mid, err := storyStore.Load(string(world.ID))
	if err != nil {
		t.Fatalf("load worldlines after beat 1: %v", err)
	}
	if len(mid) != 1 {
		t.Fatalf("expected 1 line, got %d", len(mid))
	}
	if !mid[0].Milestones[0].Triggered {
		t.Errorf("expected milestone Triggered after beat 1 (tension crossed 0.70 via drift), got false")
	}

	// --- Beat 2 ---
	t.Log("=== Beat 2 ===")
	stream2 := sess.RunBeat(ctx, session.BeatInput{
		WorldID: string(world.ID),
		Action: role.PlayerAction{
			PlayerID: player.ID,
			Content:  "I step back and watch what the seal does next.",
		},
		RecentEvents: 5,
	})
	out2 := drainBeat(t, 2, stream2)
	if out2.Err != nil {
		t.Fatalf("RunBeat #2: %v", out2.Err)
	}

	tensionAfter2 := findThreadTension(out2.World, "thread-seal")
	t.Logf("thread-seal tension after beat 2: %.3f", tensionAfter2)
	if tensionAfter2 < tensionAfter1 {
		t.Errorf("expected tension non-decreasing across beats, got %.3f → %.3f", tensionAfter1, tensionAfter2)
	}
	if tensionAfter2 > 1.0+1e-9 {
		t.Errorf("expected tension clamped at 1.0, got %.3f", tensionAfter2)
	}

	final, err := storyStore.Load(string(world.ID))
	if err != nil {
		t.Fatalf("load worldlines after beat 2: %v", err)
	}
	if !final[0].Milestones[0].Triggered {
		t.Errorf("milestone Triggered should persist across beats")
	}
}

func drainBeat(t *testing.T, n int, out session.BeatOutput) session.BeatResult {
	t.Helper()
	fmt.Printf("\n──────── Beat %d Narrative (streaming) ────────\n", n)
	for chunk := range out.NarrativeStream {
		fmt.Print(chunk)
	}
	fmt.Println()
	fmt.Println("──────── End ────────")
	result := <-out.Done
	if result.Err == nil {
		fmt.Printf("Sequence: %d | Effects: %d | Choices: %d\n",
			result.World.Clock.Sequence, len(result.ToolEffects), len(result.Choices.Options))
	}
	return result
}

func findThreadTension(w worldmodel.World, id worldmodel.ThreadID) float64 {
	for _, th := range w.Threads {
		if th.ID == id {
			return th.Tension
		}
	}
	return -1
}
