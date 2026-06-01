package narrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/sizolity/worldline/agent/chat"
	"github.com/sizolity/worldline/agent/structured"
	"github.com/sizolity/worldline/internal/app/mod"
	"github.com/sizolity/worldline/rpg/role"
	"github.com/sizolity/worldline/world/ingest"
)

// LoreDraft is the JSON schema the LLM is asked to emit under json_object
// mode. Exported so external code (e.g. cmd/rpg-cli) can construct
// structured.Agent[LoreDraft] using eino adapters.
//
// All five list fields are the same Draft* element types defined in
// world/ingest/draft.go (no redefinition / no parallel hierarchy),
// so the conversion from LoreDraft → ingest.Draft is a plain field copy.
type LoreDraft struct {
	Entities  []ingest.DraftEntity   `json:"entities,omitempty" jsonschema:"description=Characters, locations, items, factions, or events appearing in the beat"`
	Relations []ingest.DraftRelation `json:"relations,omitempty" jsonschema:"description=Typed connections between entities (disciple_of, allied_with, located_in, etc.)"`
	Facts     []ingest.DraftFact     `json:"facts,omitempty" jsonschema:"description=(subject, predicate, value) triples capturing verifiable state from the beat"`
	Threads   []ingest.DraftThread   `json:"threads,omitempty" jsonschema:"description=Story threads opened or advanced this beat; status must be active or open"`
	Memories  []ingest.DraftMemory   `json:"memories,omitempty" jsonschema:"description=Persistent impressions worth retaining; owner_kind=world for shared memory"`
}

// lorekeeperEngineConstraints is the Go-hardcoded JSON schema layer
// appended to the lorekeeper persona at runtime. Per directive 2.6 mod
// authors only write the persona body; the schema + ID rules + field
// constraints stay in code so a mod cannot accidentally break the
// json_object contract with DeepSeek.
const lorekeeperEngineConstraints = `## 引擎合规层（不可编辑）

输出格式（DeepSeek json_object 模式硬约束）：
- 必须输出**合法的 JSON 字符串**，且仅包含 JSON 内容，不要任何前后解释、Markdown 包裹、` + "`" + `` + "`" + `` + "`" + ` 反引号或自然语言注释。
- 顶层必须是一个 JSON 对象，包含 5 个键：` + "`entities`" + `、` + "`relations`" + `、` + "`facts`" + `、` + "`threads`" + `、` + "`memories`" + `，每个值都是数组，可以为空 ` + "`[]`" + `。
- 没有可记录的内容时，对应数组留空即可，不要编造。
- 所有字符串字段如需换行，**必须用 \n 转义**，绝对不要直接输出裸换行字节。

JSON 输出样例（仅供格式参考，按实际剧情填写）：

` + "```json" + `
{
  "entities": [
    {"id": "ent_sun_wukong", "type": "character", "name": "孙悟空", "aliases": ["美猴王"], "confidence": 0.9, "source_refs": ["beat-xyz"]}
  ],
  "relations": [
    {"id": "rel_wukong_subudhi", "type": "disciple_of", "source_id": "ent_sun_wukong", "target_id": "ent_subudhi", "confidence": 0.8, "source_refs": ["beat-xyz"]}
  ],
  "facts": [],
  "threads": [],
  "memories": [
    {"id": "mem_first_meeting", "owner_kind": "world", "content": "悟空初见菩提祖师。", "scope": "canonical", "kind": "observation", "importance": 0.7, "confidence": 0.85, "source_refs": ["beat-xyz"]}
  ]
}
` + "```" + `

ID 规则（必须遵守）：
- 全部使用 lower_snake_case，仅含 [a-z0-9_]。
- 按类型加前缀：entity → ent_，relation → rel_，fact → fact_，thread → thr_，memory → mem_。
- 同一次返回中同一 ID 不要重复。

实体抽取（entities）：
- 范围：场景中出现的 NPC、地点、关键物品、势力、事件。
- Type 用简短 ASCII 单词：character / location / item / faction / event。
- Name 写最常用、最自然的人类可读名称。
- Aliases 写绰号、化名、敬称、别译等其他叫法。

关系抽取（relations）：
- 实体之间的连接（如 弟子—师父、敌对、同盟、位于、效忠）。
- Type 用 lower_snake_case，例如 disciple_of / master_of / allied_with / located_in / hostile_to。
- SourceID / TargetID 必须引用本次返回的 entities[].ID 或上下文中已存在的实体 ID。

事实抽取（facts）：
- 用 (subject_id, predicate, value) 三元组记录可验证的状态信息。
- Predicate 用 lower_snake_case，例如 has_weapon / is_at / faction_rank。

线索（threads）：
- 当前剧情中正在推进或被打开的事件线。
- Status 必须是 active 或 open。
- Priority 与 Tension 是 [0,1] 的浮点数，谨慎给值。

记忆（memories）：
- 本回合产生、需要长期保留的"印象"。
- OwnerKind 推荐 world（世界共享视角）；如选 character / faction / narrator，必须同时给 OwnerID。
- Scope 取值之一：canonical / factual / subjective / rumor / emotional / procedural。
- Kind 取值之一：observation / belief / rumor / summary。
- Content 或 Summary 至少填一项。

可信度与来源：
- Confidence 在 [0,1] 范围内，谨慎给值。文本里只出现一次的从属信息不要给高 confidence。
- 对话里的猜测、听到的传言：不要标 confidence=1.0；用 truth_status="unknown" 或 kind="rumor"。
- SourceRefs 的每一项填入用户消息中提供的"来源 ID"（doc.ID），用于追溯。`

// LorekeeperSystemPrompt is the legacy fully-rendered prompt string. It
// is preserved as a package-level constant for backward compatibility
// with external callers (e.g. tests, manual smoke scripts) that
// reference it directly. New code should call
// (*Narrator).LorekeeperSystemPrompt() instead, which honors any
// style-supplied persona override.
//
// Deprecated: use (*Narrator).LorekeeperSystemPrompt(). This constant
// will be removed once external callers migrate.
var LorekeeperSystemPrompt = func() string {
	doc, _ := mod.ParseDocument(defaultLorekeeperPersonaMD)
	return mod.RenderAuxiliaryPrompt(doc, lorekeeperEngineConstraints)
}()

// LorekeeperPrompt returns the fully-rendered system prompt for the
// Lorekeeper, combining the (possibly mod-supplied) persona document
// with the Go-hardcoded JSON schema constraints.
func (n *Narrator) LorekeeperPrompt() string {
	persona := n.lorekeeperPersona
	if persona == nil {
		persona = defaultLorekeeperPersona()
	}
	return mod.RenderAuxiliaryPrompt(persona, lorekeeperEngineConstraints)
}

// LoreParser is the LLM-driven implementation of role.Lorekeeper. It asks
// the structured agent (under json_object mode) to extract entities /
// relations / facts / threads / memories from a single beat narrative
// SourceDocument and returns an ingest.Draft.
//
// Empty input (whitespace-only doc.Text) short-circuits to ingest.Draft{}
// without calling the LLM, so callers can pass through trivial beats
// (e.g. silent setup) at zero cost.
type LoreParser struct {
	agent  structured.Agent[LoreDraft]
	prompt string
}

// Compile-time assertion that *LoreParser satisfies both ingest.Parser
// and role.Lorekeeper.
var _ role.Lorekeeper = (*LoreParser)(nil)

// NewLoreParser constructs a LoreParser bound to the given structured agent
// and using the legacy default lorekeeper system prompt.
func NewLoreParser(agent structured.Agent[LoreDraft]) *LoreParser {
	return &LoreParser{agent: agent, prompt: LorekeeperSystemPrompt}
}

// NewLoreParserWithStyle is the v1 mod-aware constructor: it pulls the
// lorekeeper persona from the given Narrator (which may have been
// configured with a mod.Style) and renders the full prompt with engine
// compliance trailer.
func NewLoreParserWithStyle(agent structured.Agent[LoreDraft], n *Narrator) *LoreParser {
	prompt := LorekeeperSystemPrompt
	if n != nil {
		prompt = n.LorekeeperPrompt()
	}
	return &LoreParser{agent: agent, prompt: prompt}
}

// Parse extracts an ingest.Draft from a single SourceDocument by asking
// the structured agent to return a parsed LoreDraft.
//
// Failure paths:
//   - whitespace-only doc.Text → short-circuit to ingest.Draft{} (zero cost).
//   - agent Call error → wrapped as "lorekeeper generate".
func (l *LoreParser) Parse(ctx context.Context, doc ingest.SourceDocument) (ingest.Draft, error) {
	if strings.TrimSpace(doc.Text) == "" {
		return ingest.Draft{}, nil
	}

	messages := []chat.Message{
		chat.SystemMessage(l.prompt),
		chat.UserMessage(buildLorePrompt(doc)),
	}

	ld, err := l.agent.Call(ctx, messages)
	if err != nil {
		if strings.Contains(err.Error(), "empty content") {
			return ingest.Draft{}, fmt.Errorf("lorekeeper parse: empty content (DeepSeek json_object mode returned no body — retry the beat)")
		}
		return ingest.Draft{}, fmt.Errorf("lorekeeper generate: %w", err)
	}

	return draftFromLoreDraft(ld), nil
}

// draftFromLoreDraft converts the LLM-extracted struct into the public
// ingest.Draft envelope.
func draftFromLoreDraft(ld LoreDraft) ingest.Draft {
	return ingest.Draft{
		Entities:  ld.Entities,
		Relations: ld.Relations,
		Facts:     ld.Facts,
		Threads:   ld.Threads,
		Memories:  ld.Memories,
	}
}

// buildLorePrompt assembles the LLM user-message: the narrative text plus
// the source document ID.
func buildLorePrompt(doc ingest.SourceDocument) string {
	var b strings.Builder
	b.WriteString("## 叙事文本\n\n")
	b.WriteString(doc.Text)
	b.WriteString("\n\n## 来源 ID\n")
	b.WriteString(doc.ID)
	return b.String()
}
