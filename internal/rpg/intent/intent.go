// Package intent implements the LLM-driven REPL input interpreter that
// turns a single line of player text (which may be a digit selector, a
// digit combo, mixed digits+free-text, or pure free-form prose) into one
// coherent natural-language action instruction for the Narrator.
//
// It replaces the hand-rolled "先 X，再 Y" combo template that lived in
// cmd/rpg-cli/main.go and could not handle:
//
//   - repeated digits ("33"), where the player is emphasizing one option
//   - mixed input ("用2根手指掐诀念咒"), where digits are sentence noise
//   - long noisy sequences ("231232"), which are mistypes, not combos
//   - free-text that needs light copy-editing while preserving intent
//
// The package follows the same "persona md + Go-side engine compliance
// trailer" layering used by narrator.SuggesterPrompt and
// narrator.LorekeeperPrompt: mod authors edit the persona body to shape
// voice and interpretation rules, while the JSON tool-call schema and
// "no free text outside tool call" contract stay hardcoded so a mod
// cannot accidentally break the structured-output contract.
package intent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/typed"
	"github.com/sizolity/worldline/internal/rpg/mod"
	"github.com/sizolity/worldline/internal/rpg/role"
)

// Params is the structured-output schema the LLM must emit through the
// `resolve_intent` tool call. It is exported so external callers (e.g.
// cmd/rpg-cli) can construct typed.Agent[Params].
type Params struct {
	// ActionText is the resolved natural-language action instruction
	// passed to the Narrator. Required, non-empty after trimming.
	// Must reflect ALL player intents (e.g. when the raw input is a
	// multi-digit combo, every referenced option must appear in the
	// composed prose); single-action inputs naturally render as one
	// sentence.
	ActionText string `json:"action_text" jsonschema:"required,description=A coherent Chinese natural-language action instruction (third-person limited POV) the narrator must execute this beat. When the player references multiple options or mixes digits with custom text, every intent MUST be reflected in the prose."`

	// IsDestructive marks intents that may yield irreversible damage
	// (kill master, suicide, destroy artifact, etc.). v1 records only;
	// no UI confirm is implemented.
	IsDestructive bool `json:"is_destructive,omitempty" jsonschema:"description=Set true when the intent could cause irreversible destructive consequences. v1 only logs this flag."`

	// Notes is a debug-only explanation of how the LLM interpreted the
	// raw input. Never shown to the player.
	Notes string `json:"notes,omitempty" jsonschema:"description=Short rationale for how the raw input was interpreted. Debug-only; not shown to the player."`
}

// ResolveToolName is the tool name the structured agent must call.
const ResolveToolName = "resolve_intent"

// ResolveToolDesc is the tool description for the structured agent.
const ResolveToolDesc = "Resolve player REPL input into a coherent action instruction (prose; one or multiple sentences) for the narrator. When the player references multiple options or mixes digits with custom text, every intent must be reflected in the composed prose."

// engineConstraints is the Go-hardcoded compliance trailer appended to
// the intent persona at runtime. Mod authors edit the persona body for
// voice and interpretation rules; the schema, tool name, and "no free
// text outside the tool call" contract live here so a mod cannot break
// the structured-output contract.
const engineConstraints = `## 引擎合规层（不可编辑）

工具调用纪律：
- 必须通过 ` + "`" + ResolveToolName + "`" + ` 工具返回结果；**严禁**在工具调用之外输出任何文本、解释或 Markdown 包装。
- ` + "`action_text`" + ` 必须为非空中文自然语言；玩家若指定多个动作（多数字组合 / 数字+自定义文本混合 / 自定义文本里描述多步），必须**完整反映**所有动作，但要用**流畅散文**串接，不得用编号列表或 "1. … 2. …" 格式。
- 单一选项或纯短句输入时，` + "`action_text`" + ` 自然写成一句即可；多动作组合时，写成一段连贯散文（可多句）。
- ` + "`action_text`" + ` 风格遵循 persona 中的"## 风格"约束（第三人称限知、避免"先…再…"模板腔）。
- ` + "`is_destructive`" + ` 默认 false；只有判定为不可逆破坏性后果时设 true，引擎仅记录不阻止。
- ` + "`notes`" + ` 仅用于 debug，可省略；不要把玩家可见信息塞进 notes。

反幻觉约束：
- 不要引用 ## 可选行动 / ## 最近叙事 之外的实体或线索。
- 数字选项里没有的内容，不要强行编造为玩家行动。`

// Agent is the structured-output adapter the Resolver depends on. It is
// satisfied by typed.Agent[Params] and by package-local mocks in
// tests.
type Agent = typed.Agent[Params]

// Resolver wraps a typed.Agent[Params] with the persona-aware
// system prompt and a helper to assemble the user prompt from REPL
// context (raw input + action choices + recent narrative slice).
type Resolver struct {
	agent   Agent
	persona *mod.Document
}

// NewResolver constructs a Resolver. Both agent and persona are
// required: persona must be loaded from a mod style (see
// mod.LoadStyle) so the engine ships no hardcoded fallback copy.
func NewResolver(agent Agent, persona *mod.Document) (*Resolver, error) {
	if agent == nil {
		return nil, fmt.Errorf("intent: agent is required")
	}
	if persona == nil {
		return nil, fmt.Errorf("intent: persona required (load via mod.LoadStyle and pass style.IntentPersona)")
	}
	return &Resolver{agent: agent, persona: persona}, nil
}

// SystemPrompt renders the full intent system prompt: the persona body
// (lead + H2 sections, verbatim) followed by the Go-hardcoded engine
// compliance trailer.
func (r *Resolver) SystemPrompt() string {
	return mod.RenderAuxiliaryPrompt(r.persona, engineConstraints)
}

// UserPrompt assembles the per-call user message: the raw player line
// + the most recent suggested ActionChoices (numbered list) + a short
// slice of the previous beat's narrative to give the LLM context for
// "what just happened" and therefore what numbers reference.
//
// recentNarrative may be empty (very first beat); choices may be empty
// (legacy or fallback path). Both are simply elided from the prompt
// when missing.
func (r *Resolver) UserPrompt(rawInput string, choices role.ActionChoices, recentNarrative string) string {
	return buildUserPrompt(rawInput, choices, recentNarrative)
}

// Resolve calls the structured agent with the rendered system prompt
// and a user prompt assembled from the REPL context. Returns the parsed
// Params on success, or a wrapped error on transport / parse failure.
//
// rawInput is required; an empty (whitespace-only) raw input short-
// circuits to a "fail" error so callers can fall back to their original
// line — no point spending an LLM call on nothing.
func (r *Resolver) Resolve(ctx context.Context, rawInput string, choices role.ActionChoices, recentNarrative string) (Params, error) {
	if strings.TrimSpace(rawInput) == "" {
		return Params{}, fmt.Errorf("intent resolve: empty input")
	}
	messages := []*schema.Message{
		schema.SystemMessage(r.SystemPrompt()),
		schema.UserMessage(buildUserPrompt(rawInput, choices, recentNarrative)),
	}
	out, err := r.agent.Call(ctx, messages)
	if err != nil {
		return Params{}, fmt.Errorf("intent resolve: %w", err)
	}
	if strings.TrimSpace(out.ActionText) == "" {
		return out, fmt.Errorf("intent resolve: empty action_text")
	}
	return out, nil
}

// buildUserPrompt assembles the LLM user-message body. Layout is fixed
// so prompt-engineering changes (or unit tests) can rely on the H2
// titles "## 玩家原始输入", "## 可选行动", "## 最近叙事".
func buildUserPrompt(rawInput string, choices role.ActionChoices, recentNarrative string) string {
	var b strings.Builder
	b.WriteString("## 玩家原始输入\n")
	b.WriteString(strings.TrimRight(rawInput, "\n"))
	b.WriteString("\n\n## 可选行动\n")
	if len(choices.Options) == 0 {
		b.WriteString("(本回合无推荐选项，请按玩家自由文本理解)\n")
	} else {
		for i, opt := range choices.Options {
			if opt.Type == role.ActionTypeCustom {
				fmt.Fprintf(&b, "%d. (自定义槽 — 玩家可自由描述)\n", i+1)
				continue
			}
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				label = "(空)"
			}
			if opt.Type != "" {
				fmt.Fprintf(&b, "%d. %s [%s]\n", i+1, label, opt.Type)
			} else {
				fmt.Fprintf(&b, "%d. %s\n", i+1, label)
			}
		}
	}
	b.WriteString("\n## 最近叙事\n")
	if narrative := strings.TrimSpace(recentNarrative); narrative != "" {
		b.WriteString(narrative)
		b.WriteString("\n")
	} else {
		b.WriteString("(无 — 这是本局首个回合，请按玩家文字字面理解)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
