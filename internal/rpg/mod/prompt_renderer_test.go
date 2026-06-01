package mod

import (
	"strings"
	"testing"
)

const testPersonaMD = `# 旁白

你是旁白，写故事。

## 风格

- 节制、留白

## 世界

## 规则

## 角色

## 地点

## NPC 记忆

## 最近事件

## 当前线索

## 发现协议

## 注意事项

自定义段落，引擎不动。
`

func TestRenderPersonaPrompt_ReplacesReservedH2(t *testing.T) {
	doc, err := ParseDocument(testPersonaMD)
	if err != nil {
		t.Fatalf("parse persona: %v", err)
	}
	out := RenderPersonaPrompt(doc, PromptSections{
		World:         "- Title: Test\n- Genre: fantasy",
		Rules:         "no rules",
		Characters:    "- Hero (id: hero-1)",
		Locations:     "- Place (id: loc-1)",
		NPCMemory:     "(no NPC memories yet)",
		RecentEvents:  "no recent events",
		ActiveThreads: "no active threads",
		Discovery:     "fog payload",
	}, "## TRAILER")

	for _, want := range []string{
		"# 旁白", "你是旁白", "## 风格", "节制、留白",
		"## 世界", "- Title: Test", "- Genre: fantasy",
		"## 规则", "no rules",
		"## 角色", "- Hero (id: hero-1)",
		"## 地点", "- Place (id: loc-1)",
		"## NPC 记忆", "(no NPC memories yet)",
		"## 最近事件", "no recent events",
		"## 当前线索", "no active threads",
		"## 发现协议", "fog payload",
		"## 注意事项", "自定义段落，引擎不动。",
		"## TRAILER",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q\nrendered:\n%s", want, out)
		}
	}
}

func TestRenderPersonaPrompt_FogDisabledDropsDiscovery(t *testing.T) {
	doc, _ := ParseDocument(testPersonaMD)
	out := RenderPersonaPrompt(doc, PromptSections{
		World:    "world body",
		Discovery: "", // fog disabled
	}, "")

	if strings.Contains(out, "## 发现协议") {
		t.Errorf("fog disabled: ## 发现协议 must be omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "## 注意事项") {
		t.Errorf("non-reserved section after discovery should still render")
	}
}

func TestRenderPersonaPrompt_NilPersona(t *testing.T) {
	out := RenderPersonaPrompt(nil, PromptSections{}, "trailer")
	if strings.TrimSpace(out) != "trailer" {
		t.Errorf("nil persona output = %q, want %q", out, "trailer")
	}
}

func TestRenderAuxiliaryPrompt_PassesThroughPersonaWithTrailer(t *testing.T) {
	src := `# Lorekeeper

I record the world.

## 任务

抽取实体、关系、事实。
`
	doc, _ := ParseDocument(src)
	out := RenderAuxiliaryPrompt(doc, "## ENGINE TRAILER\n\nschema rules here")
	if !strings.Contains(out, "# Lorekeeper") {
		t.Error("missing persona H1")
	}
	if !strings.Contains(out, "## 任务") {
		t.Error("missing persona section")
	}
	if !strings.Contains(out, "## ENGINE TRAILER") {
		t.Error("missing engine trailer")
	}
	// Trailer must come after persona content.
	idxPersona := strings.Index(out, "## 任务")
	idxTrailer := strings.Index(out, "## ENGINE TRAILER")
	if !(idxPersona < idxTrailer) {
		t.Errorf("trailer must follow persona; got persona=%d trailer=%d", idxPersona, idxTrailer)
	}
}
