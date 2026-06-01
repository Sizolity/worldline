package mod

import (
	"strings"
)

// Reserved H2 placeholder titles per mod/README.md "三层架构" §2.
// The renderer replaces each of these with engine-generated content
// (in source order). Any other H2 (e.g. "## 风格", "## 注意事项") is
// preserved verbatim from the persona document.
const (
	ReservedWorld         = "世界"
	ReservedRules         = "规则"
	ReservedCharacters    = "角色"
	ReservedLocations     = "地点"
	ReservedNPCMemory     = "NPC 记忆"
	ReservedRecentEvents  = "最近事件"
	ReservedActiveThreads = "当前线索"
	ReservedDiscovery     = "发现协议"
)

// ReservedH2 is the set of H2 titles the renderer treats as engine-filled
// placeholders.
var ReservedH2 = map[string]bool{
	ReservedWorld:         true,
	ReservedRules:         true,
	ReservedCharacters:    true,
	ReservedLocations:     true,
	ReservedNPCMemory:     true,
	ReservedRecentEvents:  true,
	ReservedActiveThreads: true,
	ReservedDiscovery:     true,
}

// PromptSections carries the engine-rendered content for each reserved
// placeholder. The renderer substitutes them into the persona doc in
// source order.
//
// All fields are pre-rendered strings; the renderer never iterates raw
// worldmodel state. Callers (narrator.SystemPrompt) own that projection.
type PromptSections struct {
	World         string
	Rules         string
	Characters    string
	Locations     string
	NPCMemory     string
	RecentEvents  string
	ActiveThreads string
	Discovery     string // empty when fog is disabled — Discovery H2 is removed entirely
}

// RenderPersonaPrompt assembles a final system prompt by walking the
// persona document and replacing reserved H2 placeholders with the
// pre-rendered sections. Compliance is the engine-managed trailer
// (anti-hallucination / tool nomenclature / cross-system contracts)
// appended to the end; pass "" to omit.
//
// Behavior summary:
//
//   - Lead (prose between H1 and first H2) is preserved verbatim.
//   - Each H2 in source order: if reserved, body is replaced; if reserved
//     and the section is empty (e.g. Discovery with fog disabled), the
//     entire H2 (header included) is dropped.
//   - Non-reserved H2 sections preserved verbatim.
//   - Compliance trailer appended last with a separating blank line.
func RenderPersonaPrompt(persona *Document, sections PromptSections, compliance string) string {
	if persona == nil {
		// Defensive: a missing persona renders as just the trailer.
		return strings.TrimSpace(compliance)
	}

	var b strings.Builder
	if persona.Title != "" {
		b.WriteString("# ")
		b.WriteString(persona.Title)
		b.WriteString("\n\n")
	}
	if persona.Lead != "" {
		b.WriteString(persona.Lead)
		b.WriteString("\n\n")
	}

	for _, sec := range persona.Sections {
		body, skip := resolveSection(sec, sections)
		if skip {
			continue
		}
		b.WriteString("## ")
		b.WriteString(sec.Title)
		b.WriteString("\n")
		if body != "" {
			b.WriteString(body)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if compliance != "" {
		b.WriteString(compliance)
		if !strings.HasSuffix(compliance, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func resolveSection(sec Section, sections PromptSections) (string, bool) {
	if !ReservedH2[sec.Title] {
		// Author-written section: keep body verbatim.
		return sec.Body, false
	}
	switch sec.Title {
	case ReservedWorld:
		return sections.World, false
	case ReservedRules:
		return sections.Rules, false
	case ReservedCharacters:
		return sections.Characters, false
	case ReservedLocations:
		return sections.Locations, false
	case ReservedNPCMemory:
		return sections.NPCMemory, false
	case ReservedRecentEvents:
		return sections.RecentEvents, false
	case ReservedActiveThreads:
		return sections.ActiveThreads, false
	case ReservedDiscovery:
		// Fog disabled → drop the entire H2 (header included) so the
		// player-facing prompt does not show an empty "## 发现协议".
		if sections.Discovery == "" {
			return "", true
		}
		return sections.Discovery, false
	}
	return sec.Body, false
}

// RenderAuxiliaryPrompt combines a persona document (lorekeeper /
// suggester) with an engine compliance trailer. No reserved H2
// substitution: those personas do not include world-state placeholders.
//
// The body is the persona document re-emitted as markdown (H1 + lead +
// each H2 verbatim); the trailer is appended after a blank line.
func RenderAuxiliaryPrompt(persona *Document, compliance string) string {
	if persona == nil {
		return strings.TrimSpace(compliance)
	}
	var b strings.Builder
	if persona.Title != "" {
		b.WriteString("# ")
		b.WriteString(persona.Title)
		b.WriteString("\n\n")
	}
	if persona.Lead != "" {
		b.WriteString(persona.Lead)
		b.WriteString("\n\n")
	}
	for _, sec := range persona.Sections {
		b.WriteString("## ")
		b.WriteString(sec.Title)
		b.WriteString("\n")
		if sec.Body != "" {
			b.WriteString(sec.Body)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if compliance != "" {
		b.WriteString(compliance)
		if !strings.HasSuffix(compliance, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
