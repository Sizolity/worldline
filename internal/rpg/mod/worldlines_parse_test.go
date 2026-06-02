package mod

import (
	"testing"

	"github.com/sizolity/worldline/internal/rpg/story"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

const shituWorldlineMD = `---
thread: 师徒嫌隙
visibility: hidden
stage: 初行
tempo: 渐磨
---

# 师徒嫌隙

悟空性烈，三藏慈悲，信任正悄然消磨。

## 微疑

触发：师徒张力·初现
结果：
- 唐三藏：微疑

## 决裂

触发：师徒张力·决裂
结果：
- 师徒嫌隙：浮上台面
- 唐三藏：心生芥蒂
`

func TestParseWorldLineDoc_Shitu(t *testing.T) {
	doc, err := ParseDocument(shituWorldlineMD)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	wl, err := parseWorldLineDoc("shitu", doc)
	if err != nil {
		t.Fatalf("parseWorldLineDoc: %v", err)
	}
	if wl.FileSlug != "shitu" {
		t.Errorf("FileSlug = %q, want shitu", wl.FileSlug)
	}
	if wl.ThreadName != "师徒嫌隙" {
		t.Errorf("ThreadName = %q", wl.ThreadName)
	}
	if wl.Visibility != "hidden" {
		t.Errorf("Visibility = %q", wl.Visibility)
	}
	if wl.Stage != "初行" {
		t.Errorf("Stage = %q", wl.Stage)
	}
	if wl.Tempo != "渐磨" {
		t.Errorf("Tempo = %q", wl.Tempo)
	}
	if len(wl.Milestones) != 2 {
		t.Fatalf("milestones = %d, want 2", len(wl.Milestones))
	}

	m1 := wl.Milestones[0]
	if m1.Title != "微疑" || m1.Band != "初现" {
		t.Errorf("m1 = {%q, band %q}", m1.Title, m1.Band)
	}
	if len(m1.Outcomes) != 1 || m1.Outcomes[0].Target != "唐三藏" || m1.Outcomes[0].Word != "微疑" {
		t.Errorf("m1 outcomes = %+v", m1.Outcomes)
	}

	m2 := wl.Milestones[1]
	if m2.Title != "决裂" || m2.Band != "决裂" {
		t.Errorf("m2 = {%q, band %q}", m2.Title, m2.Band)
	}
	want := []OutcomeSpec{
		{Target: "师徒嫌隙", Word: "浮上台面"},
		{Target: "唐三藏", Word: "心生芥蒂"},
	}
	if len(m2.Outcomes) != len(want) {
		t.Fatalf("m2 outcomes = %d, want %d", len(m2.Outcomes), len(want))
	}
	for i, o := range want {
		if m2.Outcomes[i] != o {
			t.Errorf("m2 outcome[%d] = %+v, want %+v", i, m2.Outcomes[i], o)
		}
	}
}

func TestParseWorldLineDoc_MissingThread(t *testing.T) {
	doc, err := ParseDocument("# 无线索\n\n## 微疑\n触发：x·初现\n")
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := parseWorldLineDoc("bad", doc); err == nil {
		t.Fatal("expected error for missing `thread` frontmatter")
	}
}

func TestParseMilestoneSection_Errors(t *testing.T) {
	// Missing 触发 line.
	if _, err := parseMilestoneSection(Section{Title: "x", Body: "结果：\n- 唐三藏：微疑"}); err == nil {
		t.Error("expected error for missing 触发 line")
	}
	// Bullet without a colon.
	if _, err := parseMilestoneSection(Section{Title: "x", Body: "触发：力·初现\n结果：\n- 唐三藏微疑"}); err == nil {
		t.Error("expected error for malformed bullet")
	}
}

func TestParseTriggerBand(t *testing.T) {
	cases := map[string]string{
		"触发：师徒张力·初现":    "初现", // full-width colon + interpunct
		"触发:tension·决裂": "决裂", // ascii colon + interpunct
		"触发：临界":         "临界", // band alone, no interpunct
		"触发：":           "",   // empty
		"没有冒号":          "",   // no colon at all
	}
	for in, want := range cases {
		if got := parseTriggerBand(in); got != want {
			t.Errorf("parseTriggerBand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitColon(t *testing.T) {
	for _, tc := range []struct {
		in          string
		left, right string
		ok          bool
	}{
		{"唐三藏：微疑", "唐三藏", "微疑", true},
		{"thread: active", "thread", "active", true},
		{"nocolon", "", "", false},
	} {
		l, r, ok := splitColon(tc.in)
		if l != tc.left || r != tc.right || ok != tc.ok {
			t.Errorf("splitColon(%q) = (%q,%q,%v), want (%q,%q,%v)", tc.in, l, r, ok, tc.left, tc.right, tc.ok)
		}
	}
}

// TestCompileWorldlines_FailLoud verifies bad references and unknown
// keywords return an error rather than silently dropping data.
func TestCompileWorldlines_FailLoud(t *testing.T) {
	base := func(wl WorldLineDoc) *Scenario {
		return &Scenario{
			ID:          "t",
			PlayerIndex: -1,
			Threads:     []ThreadEntry{{Title: "师徒嫌隙"}},
			Characters:  []CharacterDoc{{FileSlug: "tang", Title: "唐三藏"}},
			WorldLines:  []WorldLineDoc{wl},
		}
	}
	cases := map[string]WorldLineDoc{
		"unknown thread": {FileSlug: "a", ThreadName: "不存在", Tempo: "渐磨"},
		"unknown tempo":  {FileSlug: "a", ThreadName: "师徒嫌隙", Tempo: "疾风"},
		"unknown band": {FileSlug: "a", ThreadName: "师徒嫌隙",
			Milestones: []MilestoneSpec{{Title: "x", Band: "爆发"}}},
		"unknown status word": {FileSlug: "a", ThreadName: "师徒嫌隙",
			Milestones: []MilestoneSpec{{Title: "x", Band: "初现", Outcomes: []OutcomeSpec{{Target: "师徒嫌隙", Word: "胡来"}}}}},
		"unresolvable target": {FileSlug: "a", ThreadName: "师徒嫌隙",
			Milestones: []MilestoneSpec{{Title: "x", Band: "初现", Outcomes: []OutcomeSpec{{Target: "玉皇大帝", Word: "微疑"}}}}},
		"bad visibility": {FileSlug: "a", ThreadName: "师徒嫌隙", Visibility: "公开", Tempo: "渐磨"},
	}
	for name, wl := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := CompileScenarioToWorldLines(base(wl)); err == nil {
				t.Errorf("expected fail-loud error for %s, got nil", name)
			}
		})
	}
}

// TestCompileWorldlines_AmbiguousThread fails loud when two threads share a
// title that a worldline references.
func TestCompileWorldlines_AmbiguousThread(t *testing.T) {
	sc := &Scenario{
		ID:          "t",
		PlayerIndex: -1,
		Threads:     []ThreadEntry{{Title: "夙怨"}, {Title: "夙怨"}},
		WorldLines:  []WorldLineDoc{{FileSlug: "a", ThreadName: "夙怨", Tempo: "渐磨"}},
	}
	if _, err := CompileScenarioToWorldLines(sc); err == nil {
		t.Error("expected fail-loud error for ambiguous thread title, got nil")
	}
}

// TestKeywordTables_LegacySeed pins the keyword table values to the legacy
// xiyou-changan constants so the migration stays behaviour-preserving.
func TestKeywordTables_LegacySeed(t *testing.T) {
	if got := tempoTable["渐磨"]; got != (story.Drift{Scene: 0.05, Day: 0.20, Chapter: 0.40}) {
		t.Errorf("tempoTable[渐磨] = %+v, want legacy {0.05,0.20,0.40}", got)
	}
	if got := bandTable["初现"]; got != 0.30 {
		t.Errorf("bandTable[初现] = %v, want 0.30", got)
	}
	if got := bandTable["决裂"]; got != 0.60 {
		t.Errorf("bandTable[决裂] = %v, want 0.60", got)
	}
	if got := statusTable["浮上台面"]; got != worldmodel.ThreadStatusActive {
		t.Errorf("statusTable[浮上台面] = %q, want active", got)
	}
}
