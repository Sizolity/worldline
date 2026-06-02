package mod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"testing"

	"github.com/sizolity/worldline/internal/rpg/story"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

// TestCompileWorldlines_GoldenEquivalence pins the mod-compiled xiyou lines
// to the checked-in golden — the equivalence oracle frozen at P0 from the
// legacy hardcode, modulo the two approved deltas (see the next test).
func TestCompileWorldlines_GoldenEquivalence(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	lines, err := CompileScenarioToWorldLines(sc)
	if err != nil {
		t.Fatalf("CompileScenarioToWorldLines: %v", err)
	}
	got, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/xiyou_worldlines.golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("compiled worldlines diverge from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestWorldlineGolden_LegacyDeltaDocumented proves the only differences
// between the frozen legacy hardcode output and the final compiled output
// are the two user-approved deltas:
//
//  1. milestone IDs m_xianxi/m_jueche → positional m1/m2 (directive 2)
//  2. the write-only trust:0.3 number payload dropped (directive 1)
//
// It transforms the legacy golden by exactly those two rules and asserts
// the result deep-equals the final golden. If any OTHER field drifted, this
// fails — keeping the equivalence proof alive without referencing the
// deleted xiyouChanganWorldLines() Go function.
func TestWorldlineGolden_LegacyDeltaDocumented(t *testing.T) {
	legacy := readGoldenLines(t, "testdata/xiyou_worldlines.legacy.golden.json")
	final := readGoldenLines(t, "testdata/xiyou_worldlines.golden.json")

	for li := range legacy {
		for mi := range legacy[li].Milestones {
			m := &legacy[li].Milestones[mi]
			// Delta 2: rename milestone IDs positionally.
			m.ID = fmt.Sprintf("m%d", mi+1)
			// Delta 1: drop any number-valued payload entry (trust:0.3).
			for ei := range m.Effects {
				for key, v := range m.Effects[ei].Payload {
					if v.Kind == worldmodel.ValueKindNumber {
						delete(m.Effects[ei].Payload, key)
					}
				}
			}
		}
	}

	if !reflect.DeepEqual(legacy, final) {
		lb, _ := json.MarshalIndent(legacy, "", "  ")
		fb, _ := json.MarshalIndent(final, "", "  ")
		t.Errorf("legacy minus the two approved deltas != final golden.\n--- transformed legacy ---\n%s\n--- final ---\n%s", lb, fb)
	}
}

func readGoldenLines(t *testing.T, path string) []story.WorldLine {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var lines []story.WorldLine
	if err := json.Unmarshal(data, &lines); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return lines
}

// TestCompileWorldlines_TickBehavior feeds the compiled xiyou line into the
// deterministic story scheduler and asserts the migrated mechanic behaves
// exactly as the legacy line did: drift deltas from the 渐磨 tempo, and
// milestones firing in positional order with the right effect targets.
func TestCompileWorldlines_TickBehavior(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	lines, err := CompileScenarioToWorldLines(sc)
	if err != nil {
		t.Fatalf("CompileScenarioToWorldLines: %v", err)
	}

	// Drift: the 渐磨 tempo must yield {scene:0.05, day:0.20, chapter:0.40}.
	for _, tc := range []struct {
		scale worldmodel.WorldTimeKind
		want  float64
	}{
		{worldmodel.WorldTimeScene, 0.05},
		{worldmodel.WorldTimeDay, 0.20},
		{worldmodel.WorldTimeChapter, 0.40},
	} {
		world := worldWithThreadTension("thread-2", 0.0)
		out, err := story.Tick(story.TickInput{World: world, Lines: lines, TimeScale: tc.scale}, rng())
		if err != nil {
			t.Fatalf("Tick(%s): %v", tc.scale, err)
		}
		tension := driftedTension(out.Events, "thread-2")
		if !almostEqual(tension, tc.want) {
			t.Errorf("%s drift → tension %.3f, want %.3f", tc.scale, tension, tc.want)
		}
	}

	// Milestones: starting tension 0.60 + one scene of drift crosses both
	// the 初现(0.30) and 决裂(0.60) bands, so m1 then m2 fire in one tick.
	world := worldWithThreadTension("thread-2", 0.60)
	out, err := story.Tick(story.TickInput{World: world, Lines: lines, TimeScale: worldmodel.WorldTimeScene}, rng())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(out.UpdatedLines) != 1 || len(out.UpdatedLines[0].Milestones) != 2 {
		t.Fatalf("unexpected updated lines: %+v", out.UpdatedLines)
	}
	for i, m := range out.UpdatedLines[0].Milestones {
		if !m.Triggered {
			t.Errorf("milestone[%d] %q not triggered", i, m.ID)
		}
	}

	// Collect milestone events in order; assert m1 precedes m2 and effects
	// match the legacy outcome (entity disposition, then thread+entity).
	var milestoneOrder []string
	for _, ev := range out.Events {
		if ev.Type != worldmodel.EventTypeNote {
			continue
		}
		for _, eff := range ev.Effects {
			if eff.Kind == worldmodel.EffectUpdateEntityState && eff.TargetID != "npc-tang_sanzang" {
				t.Errorf("entity effect targets %q, want npc-tang_sanzang", eff.TargetID)
			}
			for key, v := range eff.Payload {
				if v.Kind == worldmodel.ValueKindNumber {
					t.Errorf("milestone effect payload %q is a number (%v); migration dropped numbers", key, v.Raw)
				}
			}
		}
		// Recover which milestone via its m1/m2 effect signature.
		milestoneOrder = append(milestoneOrder, classifyMilestoneEvent(ev))
	}
	if len(milestoneOrder) != 2 || milestoneOrder[0] != "m1" || milestoneOrder[1] != "m2" {
		t.Errorf("milestone fire order = %v, want [m1 m2]", milestoneOrder)
	}
}

func classifyMilestoneEvent(ev worldmodel.WorldEvent) string {
	for _, eff := range ev.Effects {
		if eff.Kind == worldmodel.EffectUpdateThread {
			return "m2" // only the 决裂 milestone touches the thread status
		}
	}
	return "m1"
}

func worldWithThreadTension(threadID string, tension float64) worldmodel.World {
	return worldmodel.World{
		Threads: []worldmodel.WorldThread{{
			ID:      worldmodel.ThreadID(threadID),
			Kind:    worldmodel.ThreadKindConflict,
			Title:   "师徒嫌隙",
			Status:  worldmodel.ThreadStatusOpen,
			Tension: tension,
		}},
		Clock: worldmodel.WorldClock{Sequence: 1},
	}
}

func driftedTension(events []worldmodel.WorldEvent, threadID string) float64 {
	for _, ev := range events {
		if ev.Type != worldmodel.EventTypeThreadChanged {
			continue
		}
		for _, eff := range ev.Effects {
			if eff.Kind == worldmodel.EffectUpdateThread && eff.TargetID == threadID {
				if v, ok := eff.Payload["tension"]; ok {
					if f, ok := v.Raw.(float64); ok {
						return f
					}
				}
			}
		}
	}
	return -1
}

func rng() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

func almostEqual(a, b float64) bool {
	d := a - b
	return d < 1e-9 && d > -1e-9
}
