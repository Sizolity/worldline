package narrator

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/view"
)

// charMem builds a minimally-valid character-owned MemoryRecord for tests.
func charMem(id, ownerID, kind, content string, importance float64, truth string) model.MemoryRecord {
	return model.MemoryRecord{
		ID:          model.MemoryID(id),
		Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: ownerID},
		Kind:        kind,
		Content:     content,
		Importance:  importance,
		TruthStatus: truth,
	}
}

func npcEntity(id, name string, tags ...string) model.Entity {
	return model.Entity{
		ID:   model.EntityID(id),
		Type: "character",
		Name: name,
		Tags: append([]string(nil), tags...),
	}
}

func TestBuildNPCMemorySection_Empty(t *testing.T) {
	got := buildNPCMemorySection(nil, nil)
	if got != npcSectionEmpty {
		t.Errorf("empty input: got %q, want %q", got, npcSectionEmpty)
	}
}

func TestBuildNPCMemorySection_NoNPCsWithMemories(t *testing.T) {
	entities := []model.Entity{
		npcEntity("npc-bone", "白骨夫人"),
		npcEntity("npc-baisu", "白素"),
	}
	memories := []model.MemoryRecord{
		// world-owned memory should not be attributed to any NPC
		{
			ID:      "mem-world-1",
			Owner:   model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Kind:    model.MemoryKindSummary,
			Content: "天降异象,白虹贯日",
		},
	}
	got := buildNPCMemorySection(entities, memories)
	if got != npcSectionEmpty {
		t.Errorf("no NPC memories: got %q, want %q", got, npcSectionEmpty)
	}
}

func TestBuildNPCMemorySection_SkipsPlayerCharacters(t *testing.T) {
	entities := []model.Entity{
		npcEntity("hero-arin", "Arin", "player", "warrior"),
		npcEntity("npc-monk", "老僧"),
	}
	memories := []model.MemoryRecord{
		charMem("mem-arin-1", "hero-arin", model.MemoryKindSummary, "Arin 自身的记忆", 0.9, ""),
		charMem("mem-monk-1", "npc-monk", model.MemoryKindSummary, "老僧在山门修行", 0.8, ""),
	}
	got := buildNPCMemorySection(entities, memories)
	if strings.Contains(got, "Arin") {
		t.Errorf("player character must be skipped, got:\n%s", got)
	}
	if strings.Contains(got, "Arin 自身的记忆") {
		t.Errorf("player memory must not be rendered, got:\n%s", got)
	}
	if !strings.Contains(got, "老僧") {
		t.Errorf("NPC name should appear, got:\n%s", got)
	}
	if !strings.Contains(got, "老僧在山门修行") {
		t.Errorf("NPC memory content should appear, got:\n%s", got)
	}
}

func TestBuildNPCMemorySection_GroupsByKind(t *testing.T) {
	entities := []model.Entity{npcEntity("npc-monk", "老僧")}
	memories := []model.MemoryRecord{
		charMem("mem-1", "npc-monk", model.MemoryKindRumor, "村中传言山门有妖", 0.4, ""),
		charMem("mem-2", "npc-monk", model.MemoryKindBelief, "妖物已被超度", 0.6, ""),
		charMem("mem-3", "npc-monk", model.MemoryKindObservation, "今日见一年轻施主登山", 0.5, ""),
		charMem("mem-4", "npc-monk", model.MemoryKindSummary, "在白马寺修行三十年", 0.9, ""),
	}
	got := buildNPCMemorySection(entities, memories)

	for _, want := range []string{npcLabelSummary, npcLabelObservation, npcLabelBelief, npcLabelRumor} {
		if !strings.Contains(got, want) {
			t.Errorf("expected group header %q, got:\n%s", want, got)
		}
	}

	idxLong := strings.Index(got, npcLabelSummary)
	idxShort := strings.Index(got, npcLabelObservation)
	idxBelief := strings.Index(got, npcLabelBelief)
	idxRumor := strings.Index(got, npcLabelRumor)
	if !(idxLong < idxShort && idxShort < idxBelief && idxBelief < idxRumor) {
		t.Errorf("group order should be %s → %s → %s → %s, got positions: long=%d short=%d belief=%d rumor=%d\nfull:\n%s",
			npcLabelSummary, npcLabelObservation, npcLabelBelief, npcLabelRumor,
			idxLong, idxShort, idxBelief, idxRumor, got)
	}
}

func TestBuildNPCMemorySection_TruthStatusMarker(t *testing.T) {
	entities := []model.Entity{npcEntity("npc-monk", "老僧")}
	memories := []model.MemoryRecord{
		charMem("mem-known", "npc-monk", model.MemoryKindBelief, "山门清净", 0.6, ""),
		charMem("mem-old", "npc-monk", model.MemoryKindBelief, "妖物已被超度", 0.5, model.TruthStatusOutdated),
		charMem("mem-false", "npc-monk", model.MemoryKindBelief, "白骨夫人是良善之人", 0.4, model.TruthStatusFalse),
		charMem("mem-disputed", "npc-monk", model.MemoryKindBelief, "山下井水不可饮", 0.3, model.TruthStatusDisputed),
	}
	got := buildNPCMemorySection(entities, memories)

	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.Contains(line, "山门清净"):
			if strings.Contains(line, npcMarkerUntrusted) || strings.Contains(line, npcMarkerDisputed) {
				t.Errorf("unknown-truth memory must not carry marker, got: %q", line)
			}
		case strings.Contains(line, "妖物已被超度"):
			if !strings.Contains(line, npcMarkerUntrusted) {
				t.Errorf("outdated memory must carry %s, got: %q", npcMarkerUntrusted, line)
			}
		case strings.Contains(line, "白骨夫人是良善之人"):
			if !strings.Contains(line, npcMarkerUntrusted) {
				t.Errorf("false memory must carry %s, got: %q", npcMarkerUntrusted, line)
			}
		case strings.Contains(line, "山下井水不可饮"):
			if !strings.Contains(line, npcMarkerDisputed) {
				t.Errorf("disputed memory must carry %s, got: %q", npcMarkerDisputed, line)
			}
		}
	}
}

func TestBuildNPCMemorySection_TruncatesPerNPC(t *testing.T) {
	entities := []model.Entity{npcEntity("npc-monk", "老僧")}
	var memories []model.MemoryRecord
	// 8 memories — all observation kind, varied importance and ID
	memories = append(memories,
		charMem("mem-a", "npc-monk", model.MemoryKindObservation, "记忆A", 0.1, ""),
		charMem("mem-b", "npc-monk", model.MemoryKindObservation, "记忆B", 0.9, ""),
		charMem("mem-c", "npc-monk", model.MemoryKindObservation, "记忆C", 0.5, ""),
		charMem("mem-d", "npc-monk", model.MemoryKindObservation, "记忆D", 0.7, ""),
		charMem("mem-e", "npc-monk", model.MemoryKindObservation, "记忆E", 0.7, ""),
		charMem("mem-f", "npc-monk", model.MemoryKindObservation, "记忆F", 0.3, ""),
		charMem("mem-g", "npc-monk", model.MemoryKindObservation, "记忆G", 0.8, ""),
		charMem("mem-h", "npc-monk", model.MemoryKindObservation, "记忆H", 0.6, ""),
	)
	got := buildNPCMemorySection(entities, memories)

	// Top-5 by importance desc, ID asc: B(0.9), G(0.8), D(0.7), E(0.7), H(0.6)
	expected := []string{"记忆B", "记忆G", "记忆D", "记忆E", "记忆H"}
	excluded := []string{"记忆A", "记忆C", "记忆F"}
	for _, want := range expected {
		if !strings.Contains(got, want) {
			t.Errorf("expected to retain %q, got:\n%s", want, got)
		}
	}
	for _, gone := range excluded {
		if strings.Contains(got, gone) {
			t.Errorf("expected to drop %q, got:\n%s", gone, got)
		}
	}

	// Verify ordering: B → G → D → E → H
	last := -1
	for _, want := range expected {
		idx := strings.Index(got, want)
		if idx < 0 {
			continue
		}
		if idx < last {
			t.Errorf("memory %q appeared out of order (idx=%d, last=%d) in:\n%s", want, idx, last, got)
		}
		last = idx
	}
}

func TestBuildNPCMemorySection_CapsNPCCount(t *testing.T) {
	var entities []model.Entity
	var memories []model.MemoryRecord
	// 8 NPCs, each with one memory
	for _, name := range []struct{ id, label string }{
		{"npc-a", "甲"}, {"npc-b", "乙"}, {"npc-c", "丙"}, {"npc-d", "丁"},
		{"npc-e", "戊"}, {"npc-f", "己"}, {"npc-g", "庚"}, {"npc-h", "辛"},
	} {
		entities = append(entities, npcEntity(name.id, name.label))
		memories = append(memories, charMem("mem-"+name.id, name.id, model.MemoryKindSummary, "记忆-"+name.label, 0.5, ""))
	}
	got := buildNPCMemorySection(entities, memories)

	got = strings.ReplaceAll(got, "\r", "")
	headerCount := strings.Count(got, "### ")
	if headerCount != npcSectionMaxNPCs {
		t.Errorf("expected exactly %d NPC headers, got %d in:\n%s", npcSectionMaxNPCs, headerCount, got)
	}

	// First 6 NPCs (input order) should appear; last 2 should not
	included := []string{"甲", "乙", "丙", "丁", "戊", "己"}
	excluded := []string{"庚", "辛"}
	for _, name := range included {
		if !strings.Contains(got, name) {
			t.Errorf("expected NPC %q included, got:\n%s", name, got)
		}
	}
	for _, name := range excluded {
		if strings.Contains(got, name) {
			t.Errorf("NPC %q exceeded cap and must be dropped, got:\n%s", name, got)
		}
	}
}

func TestBuildNPCMemorySection_OthersGroup(t *testing.T) {
	// Cover the (未分类) fallback group: memories whose Kind is empty
	// (validate-allowed) or non-canonical surface in a final bucket so the
	// narrator still sees them. Pin (a) the bucket appears, (b) it appears
	// AFTER the four canonical groups, (c) memories with empty-or-junk
	// Kind end up here (not in any canonical bucket).
	entities := []model.Entity{npcEntity("npc-monk", "老僧")}
	memories := []model.MemoryRecord{
		charMem("mem-canon", "npc-monk", model.MemoryKindSummary, "在白马寺修行三十年", 0.9, ""),
		charMem("mem-empty", "npc-monk", "", "无类别的记忆", 0.5, ""),
		charMem("mem-weird", "npc-monk", "weirdvariant", "奇异变种的记忆", 0.4, ""),
	}
	got := buildNPCMemorySection(entities, memories)

	if !strings.Contains(got, npcLabelSummary) {
		t.Errorf("expected canonical %s header, got:\n%s", npcLabelSummary, got)
	}
	if !strings.Contains(got, npcLabelOthers) {
		t.Errorf("expected %s fallback header, got:\n%s", npcLabelOthers, got)
	}

	idxSummary := strings.Index(got, npcLabelSummary)
	idxOthers := strings.Index(got, npcLabelOthers)
	if !(idxSummary < idxOthers) {
		t.Errorf("%s should appear AFTER %s; got idxSummary=%d idxOthers=%d\nfull:\n%s",
			npcLabelOthers, npcLabelSummary, idxSummary, idxOthers, got)
	}

	if !strings.Contains(got, "奇异变种的记忆") {
		t.Errorf("non-canonical-kind memory must surface in %s, got:\n%s", npcLabelOthers, got)
	}
	if !strings.Contains(got, "无类别的记忆") {
		t.Errorf("empty-kind memory must surface in %s, got:\n%s", npcLabelOthers, got)
	}

	// Each non-canonical memory should belong to the (未分类) bucket, not
	// to any canonical bucket above it.
	for _, content := range []string{"无类别的记忆", "奇异变种的记忆"} {
		if strings.Index(got, content) < idxOthers {
			t.Errorf("memory %q must appear under %s (after idx=%d), got idx=%d\nfull:\n%s",
				content, npcLabelOthers, idxOthers, strings.Index(got, content), got)
		}
	}
}

func TestSystemPrompt_IncludesNPCMemorySection(t *testing.T) {
	n, _ := New(&mockSuggestAgent{}, WithStyle(loadTestStyle(t)))
	w := model.World{
		ID:   "world-mem-01",
		Name: "测试世界",
		Canon: model.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"mysterious"},
		},
		Entities: map[model.EntityID]model.Entity{
			"npc-monk": {
				ID: "npc-monk", Type: "character", Name: "老僧",
			},
		},
		Memory: []model.MemoryRecord{
			{
				ID:         "mem-monk-1",
				Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "npc-monk"},
				Kind:       model.MemoryKindSummary,
				Content:    "在白马寺修行三十年",
				Importance: 0.9,
			},
		},
	}
	wc := view.WorldDebugView{}.Render(w)
	nc := view.NarrativeView{}.Render(w, view.NarrativeContextRequest{RecentEventLimit: 5})

	prompt := n.SystemPrompt(nil, role.PromptOptions{
		WorldCtx:     wc,
		NarrativeCtx: nc,
	})

	if !strings.Contains(prompt, "## NPC 记忆") {
		t.Errorf("expected `## NPC 记忆` header in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "老僧") {
		t.Errorf("expected NPC name `老僧` in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "在白马寺修行三十年") {
		t.Errorf("expected NPC memory content in prompt, got:\n%s", prompt)
	}

	// The testdata persona under testdata/styles/style/persona/narrator.md
	// orders the reserved placeholders as:
	// ## 角色 → ## 地点 → ## NPC 记忆 → ## 最近事件 → ## 当前线索 .
	idxChars := strings.Index(prompt, "## 角色")
	idxLocs := strings.Index(prompt, "## 地点")
	idxNPC := strings.Index(prompt, "## NPC 记忆")
	if !(idxChars < idxLocs && idxLocs < idxNPC) {
		t.Errorf("section placement wrong: chars=%d locs=%d npc=%d", idxChars, idxLocs, idxNPC)
	}
}
