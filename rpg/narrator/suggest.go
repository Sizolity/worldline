package narrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/sizolity/worldline/agent/chat"
	"github.com/sizolity/worldline/internal/app/mod"
	"github.com/sizolity/worldline/rpg/role"
	rpgrule "github.com/sizolity/worldline/rpg/rule"
	"github.com/sizolity/worldline/world/model"
)

// SuggestParams is the structured-output schema for SuggestActions. Exported
// so external code (e.g. cmd/rpg-cli) can construct structured.Agent[SuggestParams].
type SuggestParams struct {
	Options []role.ActionOption `json:"options" jsonschema:"required,description=2-4 contextual action options for the player"`
}

// SuggestToolName is the tool name used for the suggest structured agent.
const SuggestToolName = "suggest_actions"

// SuggestToolDesc is the tool description used for the suggest structured agent.
const SuggestToolDesc = "Suggest 2-4 meaningful action options for the player based on the current narrative, world rules, and entity state."

// suggesterEngineConstraints is the Go-hardcoded compliance layer for
// the action suggester: hard rules about tool nomenclature, option type
// enum, custom-slot trailer, and the "no free text outside tool call"
// contract. v1 keeps these in code so mod authors cannot break the
// suggest_actions tool schema.
const suggesterEngineConstraints = `## 引擎合规层（不可编辑）

工具调用纪律：
- 必须通过 ` + "`" + SuggestToolName + "`" + ` 工具返回选项；**严禁**在工具调用之外输出任何文本。
- 选项类型 ` + "`type`" + ` 必须取自固定枚举：
  ` + "`explore` / `social` / `combat` / `investigate` / `use_item` / `rest` / `custom`" + `。
- ` + "`custom`" + ` 槽的 ` + "`label`" + ` 必须为空字符串，其它类型的 ` + "`label`" + ` 必须非空。

选项数量约束：
- 总数 2-4 个。
- 非关键剧情节点：末尾**必须**追加一个 ` + "`type=custom`" + ` 的空槽。
- 关键剧情节点（玩家的选择不可逆地决定整条故事线走向）：可以省略 custom 槽。

反幻觉约束：
- 选项内容必须基于"## 场景实体"、"## 活跃线索"、"## 最新叙事"中已存在的世界事实，
  不要引用上下文未提供的实体或线索。`

// SuggesterPrompt returns the fully-rendered system prompt for the
// action suggester, combining the (possibly mod-supplied) persona doc
// with the Go-hardcoded engine constraints.
func (n *Narrator) SuggesterPrompt() string {
	persona := n.suggesterPersona
	if persona == nil {
		persona = defaultSuggesterPersona()
	}
	return mod.RenderAuxiliaryPrompt(persona, suggesterEngineConstraints)
}

// SuggestActions asks the LLM to propose 2-4 contextual ActionOptions given
// the current world state and the latest narrative.
func (n *Narrator) SuggestActions(ctx context.Context, w model.World, players []role.Player, narrative string) (role.ActionChoices, error) {
	if n.suggestAgent == nil {
		return role.ActionChoices{}, fmt.Errorf("suggest agent not configured")
	}

	messages := []chat.Message{
		chat.SystemMessage(n.SuggesterPrompt()),
		chat.UserMessage(buildSuggestPrompt(w, players, narrative)),
	}

	parsed, err := n.suggestAgent.Call(ctx, messages)
	if err != nil {
		return role.ActionChoices{}, fmt.Errorf("suggest generate: %w", err)
	}

	return role.ActionChoices{Options: parsed.Options}, nil
}

// buildSuggestPrompt assembles the LLM user-message context.
func buildSuggestPrompt(w model.World, players []role.Player, narrative string) string {
	var b strings.Builder

	b.WriteString("## 最新叙事\n")
	b.WriteString(narrative)

	if recent := recentPlayerActions(w.EventLog, 5); len(recent) > 0 {
		b.WriteString("\n\n## 近期玩家行动（避免重复变体）\n")
		for _, a := range recent {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}

	b.WriteString("\n\n## 场景实体\n")
	if len(w.Entities) == 0 {
		b.WriteString("(无)\n")
	}
	for _, e := range w.Entities {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", e.Type, e.Name, e.ID)
	}

	if active := activeThreads(w.Threads); len(active) > 0 {
		b.WriteString("\n## 活跃线索\n")
		for _, th := range active {
			fmt.Fprintf(&b, "- %s: %s\n", th.Kind, th.Title)
		}
	}

	if rules := enabledRules(w.Rules); len(rules) > 0 {
		b.WriteString("\n## 适用规则\n")
		for _, r := range rules {
			fmt.Fprintf(&b, "- [%s] %s\n", r.Category, r.Content)
		}
	}

	b.WriteString("\n## 玩家角色\n")
	for _, p := range players {
		if e, ok := w.Entities[p.CharacterID]; ok {
			fmt.Fprintf(&b, "- %s (操控 %s)\n", p.Name, e.Name)
		} else {
			fmt.Fprintf(&b, "- %s\n", p.Name)
		}
	}

	return b.String()
}

func activeThreads(threads []model.WorldThread) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, th := range threads {
		switch th.Status {
		case model.ThreadStatusActive, model.ThreadStatusOpen:
			out = append(out, th)
		}
	}
	return out
}

func recentPlayerActions(events []model.WorldEvent, limit int) []string {
	if limit <= 0 || len(events) == 0 {
		return nil
	}
	const prefix = "Player: "
	out := make([]string, 0, limit)
	for i := len(events) - 1; i >= 0 && len(out) < limit; i-- {
		desc := events[i].Description
		if !strings.HasPrefix(desc, prefix) {
			continue
		}
		line := desc[len(prefix):]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func enabledRules(rules []model.Rule) []rpgrule.Rule {
	all := rpgrule.FromWorldRules(rules)
	out := make([]rpgrule.Rule, 0, len(all))
	for _, r := range all {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out
}
