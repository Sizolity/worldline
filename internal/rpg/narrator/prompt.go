package narrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/worldline/internal/rpg/mod"
	"github.com/sizolity/worldline/internal/rpg/role"
	rpgrule "github.com/sizolity/worldline/internal/rpg/rule"
	"github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/view"
)

const (
	// npcSectionMaxNPCs caps how many NPCs the section lists in one
	// prompt to keep token usage bounded. Roughly 6 NPCs × 5 memories
	// × ~50 runes/memory ≈ 1500 runes worst case.
	npcSectionMaxNPCs           = 6
	npcSectionMaxMemoriesPerNPC = 5
	npcSectionMaxMemoryRunes    = 200

	// User-visible labels for the NPC memory section. Lifted out so tests
	// and production share one source of truth — a typo or full-width /
	// half-width drift would otherwise silently de-couple the assertion
	// from the rendered prompt.
	npcLabelSummary     = "长期记忆"
	npcLabelObservation = "短期记忆"
	npcLabelBelief      = "信念"
	npcLabelRumor       = "传言"
	npcLabelOthers      = "(未分类)"
	npcMarkerUntrusted  = " (可能有误)" // leading space — appears mid-bullet
	npcMarkerDisputed   = " (有争议)"
	npcSectionEmpty     = "(no NPC memories yet)"
)

// narratorComplianceTrailer is the engine-managed compliance layer
// appended to every narrator prompt regardless of style. v1 mod design
// keeps these constraints OUT of the mod author's reach because they
// underpin player UX contracts and anti-hallucination guarantees:
//
//   - tool nomenclature: random / chance / weighted_choice / roll are
//     internal randomness; results must NEVER surface as visible dice /
//     probabilities / numbers in the narrative.
//   - cross-system contracts: the action-suggester owns the menu, so the
//     narrator must NOT enumerate choices at the end of its prose.
//   - anti-hallucination: never invent entities that are not in the
//     world; narrate uncertainty instead.
const narratorComplianceTrailer = `## 引擎合规层（不可编辑）

工具调用纪律：
- ` + "`random`" + ` / ` + "`chance`" + ` / ` + "`weighted_choice`" + ` / ` + "`roll`" + ` 是内部随机性，
  用于不确定结果。**绝不**在叙述中暴露骰子、概率、数值或机制术语。
- ` + "`update_state`" + ` 用于持久化关键状态变化（仅当实体已有可变状态时可用）。
- ` + "`get_entity_state`" + ` 用于在叙述前查阅实体当下状态。
- ` + "`lookup_rules`" + ` 用于回顾世界观规则；不要把"机制公式"念给玩家听。

反幻觉约束：
- 不要编造世界里不存在的实体（角色、地点、物品、线索）。
- 玩家询问你看不到的东西时，用"不可知 / 未察觉"叙述，不要凭空补全。

玩家代理性原则：
- 玩家是主角，不替玩家做选择。
- 段尾不要列举行动选项——这一职责由独立的行动建议系统承担。
- 允许的收尾：一段感官定格、一个不带候选的悬念问句（例："但下一步会怎样，
  谁也说不准。"）。
- 禁止的收尾模式：编号列表 / 字母列表 / "是…还是…抑或…" / "你欲如何行事？" /
  "请选择行动 / 且看下回 / 且听下文" 等。`

const narratorDiscoveryPlaceholder = `世界对你而言是**部分可见**的：
- **Known** 实体：你只看得见名字与类型（存在性确认）
- **Explored** 实体：你看得见完整描述、状态、组件
- **Hidden** 实体：完全不可见——**严禁**引用、暗示或推论

当玩家行动触发新知识被披露时，使用 ` + "`explore_knowledge`" + ` 工具显式升级可见性。
绝不凭空捏造任何尚未存在于世界中的实体。`

// Discovery Protocol legacy marker — kept as a sentinel inside the
// generated discovery section so legacy tests asserting on the English
// label stay green. Mod-side authors never see this string.
const discoveryProtocolMarker = "Discovery Protocol"

// SystemPrompt assembles the Narrator's LLM system prompt from pre-rendered
// world projections in opts. WorldDebugContext supplies entities/rules/etc.
// over the visible (post-fog) world; NarrativeContext supplies the filtered
// event/thread slice.
//
// Implementation: the narrator persona document (from the loaded
// mod.Style) carries the prose framing + reserved H2 placeholders. The
// renderer slots engine-rendered sections into those placeholders and
// appends the compliance trailer.
func (n *Narrator) SystemPrompt(players []role.Player, opts role.PromptOptions) string {
	_ = players

	wc := opts.WorldCtx
	nc := opts.NarrativeCtx

	sections := mod.PromptSections{
		World:         buildWorldSection(wc.World),
		Rules:         buildRulesSection(wc.Rules),
		Characters:    buildCharactersSection(wc.Entities),
		Locations:     buildLocationsSection(wc.Entities),
		NPCMemory:     buildNPCMemorySection(wc.Entities, wc.Memories),
		RecentEvents:  buildEventsSection(nc.RecentEvents),
		ActiveThreads: buildThreadsSection(nc.ActiveThreads),
	}
	if opts.FogEnabled {
		sections.Discovery = narratorDiscoveryPlaceholder + "\n\n(" + discoveryProtocolMarker + ")"
	}
	return mod.RenderPersonaPrompt(n.narratorPersona, sections, narratorComplianceTrailer)
}

func buildWorldSection(w view.WorldSummary) string {
	genre := strings.Join(w.Canon.Genre, ", ")
	if genre == "" {
		genre = "unspecified"
	}
	tone := strings.Join(w.Canon.Tone, ", ")
	if tone == "" {
		tone = "unspecified"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- Title: %s\n", w.Name)
	fmt.Fprintf(&b, "- Genre: %s\n", genre)
	fmt.Fprintf(&b, "- Tone: %s\n", tone)
	if w.Description != "" {
		fmt.Fprintf(&b, "- Premise: %s\n", w.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildRulesSection(rules []model.Rule) string {
	rpgRules := rpgrule.FromWorldRules(rules)
	if len(rpgRules) == 0 {
		return "No specific rules defined."
	}
	section := rpgrule.AssemblePromptSection(rpgRules)
	if section == "" {
		return "No active rules."
	}
	return section
}

func buildCharactersSection(entities []model.Entity) string {
	var chars []model.Entity
	for _, e := range entities {
		if e.Type == "character" {
			chars = append(chars, e)
		}
	}
	if len(chars) == 0 {
		return "No characters present."
	}
	var b strings.Builder
	for _, e := range chars {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", e.Name, e.ID))
		if len(e.Tags) > 0 {
			b.WriteString(fmt.Sprintf(" [%s]", strings.Join(e.Tags, ", ")))
		}
		if actor, ok := e.ActorComponent(); ok && len(actor.Goals) > 0 {
			b.WriteString(fmt.Sprintf(" Goals: %s", strings.Join(actor.Goals, "; ")))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildLocationsSection(entities []model.Entity) string {
	var locs []model.Entity
	for _, e := range entities {
		if e.Type == "location" {
			locs = append(locs, e)
		}
	}
	if len(locs) == 0 {
		return "No locations defined."
	}
	var b strings.Builder
	for _, e := range locs {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", e.Name, e.ID))
		if e.Description != "" {
			b.WriteString(fmt.Sprintf(": %s", e.Description))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildEventsSection(events []model.WorldEvent) string {
	if len(events) == 0 {
		return "No recent events."
	}
	// Truncation lives in view.NarrativeView (RecentEventLimit). Re-truncating
	// here would silently override caller intent — trust the view layer.
	var b strings.Builder
	for _, e := range events {
		summary := e.Description
		if summary == "" {
			summary = e.Intent
		}
		if summary == "" {
			summary = string(e.Type)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, summary))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildThreadsSection(threads []model.WorldThread) string {
	if len(threads) == 0 {
		return "No active story threads."
	}
	var b strings.Builder
	for _, th := range threads {
		marker := " "
		if th.Status == model.ThreadStatusActive {
			marker = "→"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s: %s\n", marker, th.Status, th.Kind, th.Title))
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildNPCMemorySection renders each scene NPC's persisted memories
// grouped by category (long-term summary, short-term observation,
// belief/error, rumor). NPCs are characters that the world store
// remembers but that no Player is operating — distinguished by the
// "player" tag, which the seed and the Lorekeeper never apply to NPCs.
//
// Output is bounded: at most npcSectionMaxNPCs NPCs, each with at most
// npcSectionMaxMemoriesPerNPC memories, sorted by Importance desc then
// memory ID asc for determinism. If no NPCs have any memories, returns
// a single line "(no NPC memories yet)" so the section never collapses
// into a stray blank header in the rendered prompt.
//
// Memories with TruthStatus == "false" or "outdated" are rendered with
// a "(可能有误)" marker so the LLM does not treat them as canonical
// facts — they represent what the NPC believes, not what is true.
//
// Memories with Owner.Kind == "character" AND Owner.ID == NPC.ID are
// included. World-owned memories (Owner.Kind == "world") are NOT
// attributed to any individual NPC — they live in the global narrative
// context, not a per-character section.
func buildNPCMemorySection(entities []model.Entity, memories []model.MemoryRecord) string {
	npcMems := make(map[model.EntityID][]model.MemoryRecord)
	for _, m := range memories {
		if m.Owner.Kind != model.MemoryOwnerKindCharacter {
			continue
		}
		owner := model.EntityID(m.Owner.ID)
		npcMems[owner] = append(npcMems[owner], m)
	}

	var b strings.Builder
	rendered := 0
	for _, e := range entities {
		if rendered >= npcSectionMaxNPCs {
			break
		}
		if e.Type != "character" {
			continue
		}
		if hasPromptTag(e.Tags, "player") {
			continue
		}
		mems := npcMems[e.ID]
		if len(mems) == 0 {
			continue
		}

		sort.SliceStable(mems, func(i, j int) bool {
			if mems[i].Importance != mems[j].Importance {
				return mems[i].Importance > mems[j].Importance
			}
			return mems[i].ID < mems[j].ID
		})
		if len(mems) > npcSectionMaxMemoriesPerNPC {
			mems = mems[:npcSectionMaxMemoriesPerNPC]
		}

		b.WriteString(fmt.Sprintf("### %s (%s)\n", e.Name, e.ID))
		writeMemoryGroup(&b, mems, model.MemoryKindSummary, npcLabelSummary)
		writeMemoryGroup(&b, mems, model.MemoryKindObservation, npcLabelObservation)
		writeMemoryGroup(&b, mems, model.MemoryKindBelief, npcLabelBelief)
		writeMemoryGroup(&b, mems, model.MemoryKindRumor, npcLabelRumor)
		writeMemoryGroupOthers(&b, mems)
		rendered++
	}

	if rendered == 0 {
		return npcSectionEmpty
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeMemoryGroup renders one Kind bucket within a single NPC's block.
// Empty buckets are silently skipped so the prompt stays tight.
func writeMemoryGroup(b *strings.Builder, mems []model.MemoryRecord, kind, label string) {
	var bucket []model.MemoryRecord
	for _, m := range mems {
		if m.Kind == kind {
			bucket = append(bucket, m)
		}
	}
	if len(bucket) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("- %s:\n", label))
	for _, m := range bucket {
		b.WriteString("  - ")
		b.WriteString(renderMemoryLine(m))
		b.WriteString("\n")
	}
}

// writeMemoryGroupOthers catches memories whose Kind is empty or not one
// of the four canonical kinds, so they still surface (labelled 未分类)
// instead of vanishing silently.
func writeMemoryGroupOthers(b *strings.Builder, mems []model.MemoryRecord) {
	var bucket []model.MemoryRecord
	for _, m := range mems {
		switch m.Kind {
		case model.MemoryKindSummary, model.MemoryKindObservation, model.MemoryKindBelief, model.MemoryKindRumor:
			continue
		default:
			bucket = append(bucket, m)
		}
	}
	if len(bucket) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("- %s:\n", npcLabelOthers))
	for _, m := range bucket {
		b.WriteString("  - ")
		b.WriteString(renderMemoryLine(m))
		b.WriteString("\n")
	}
}

// renderMemoryLine formats one memory bullet body: content (truncated),
// optional truth marker, and importance suffix.
func renderMemoryLine(m model.MemoryRecord) string {
	content := m.Content
	if content == "" {
		content = m.Summary
	}
	content = view.TruncateRunes(content, npcSectionMaxMemoryRunes)

	var marker string
	switch m.TruthStatus {
	case model.TruthStatusFalse, model.TruthStatusOutdated:
		marker = npcMarkerUntrusted
	case model.TruthStatusDisputed:
		marker = npcMarkerDisputed
	}

	return fmt.Sprintf("%s%s (importance %.2f)", content, marker, m.Importance)
}

// hasPromptTag is a local helper kept package-private to avoid a layering
// dependency on rpg/session. Mirrors the same check used in devcli.
func hasPromptTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

