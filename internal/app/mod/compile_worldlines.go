package mod

import (
	"github.com/sizolity/worldline/rpg/story"
	worldmodel "github.com/sizolity/worldline/world/model"
)

// CompileScenarioToWorldLines builds the starter hidden worldlines for a
// given scenario. WorldLines carry mechanic-layer data (drift /
// milestone conditions / effects) which is intentionally NOT exposed to
// mod authors in v1; per directive 2.1 the bindings are Go-side defaults
// keyed off the scenario ID, kept in sync with the natural-slug entity
// IDs that CompileScenarioToWorld produces.
//
// Unknown scenario IDs return nil — story.Tick treats that as "no lines"
// and is a no-op, so the rest of the seed/play pipeline degrades to
// "demo without a hidden thread engine" rather than failing hard.
func CompileScenarioToWorldLines(sc *Scenario) []story.WorldLine {
	if sc == nil {
		return nil
	}
	switch sc.ID {
	case "xiyou-changan":
		return xiyouChanganWorldLines()
	default:
		return nil
	}
}

// xiyouChanganWorldLines mirrors the legacy demo's `wl_shitu` line —
// hidden drift on the "师徒嫌隙" thread that gradually erodes Sanzang's
// trust in Wukong — but bound to the new naturally-derived IDs:
//
//	thread "师徒嫌隙" is the second H2 in threads.md → "thread-2"
//	唐三藏 file is tang_sanzang.md (role=npc)        → "npc-tang_sanzang"
func xiyouChanganWorldLines() []story.WorldLine {
	const threadShitu = "thread-2"
	const npcSanzang = "npc-tang_sanzang"

	return []story.WorldLine{{
		ID:           "wl_shitu",
		ThreadID:     worldmodel.ThreadID(threadShitu),
		Visibility:   story.VisibilityHidden,
		CurrentStage: "初行",
		Drift:        story.Drift{Scene: 0.05, Day: 0.20, Chapter: 0.40},
		Milestones: []story.Milestone{
			{
				ID: "m_xianxi",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": threadShitu, "threshold": 0.30},
				},
				Effects: []worldmodel.Effect{
					{
						Kind:     worldmodel.EffectUpdateEntityState,
						TargetID: npcSanzang,
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "微疑"},
						},
					},
				},
			},
			{
				ID: "m_jueche",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": threadShitu, "threshold": 0.60},
				},
				Effects: []worldmodel.Effect{
					{
						Kind:     worldmodel.EffectUpdateThread,
						TargetID: threadShitu,
						Payload: map[string]worldmodel.Value{
							"status": {Kind: worldmodel.ValueKindString, Raw: worldmodel.ThreadStatusActive},
						},
					},
					{
						Kind:     worldmodel.EffectUpdateEntityState,
						TargetID: npcSanzang,
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "心生芥蒂"},
							"trust":       {Kind: worldmodel.ValueKindNumber, Raw: 0.3},
						},
					},
				},
			},
		},
	}}
}
