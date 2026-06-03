package narrator

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/rpg/role"
)

// SetChoicesToolName is the eino tool name the beat agent uses to emit
// the next-step action list inline with its narrative. Centralized so
// the narrator (registration), pipeline (extraction), and tests share
// one source of truth — a string drift would silently break the inline
// extraction path and route every beat through the SuggestActions
// fallback without anyone noticing.
const SetChoicesToolName = "set_choices"

// setChoicesToolDesc is shown to the LLM as the tool description. We
// keep it focused on WHEN to call (end of every reply) and WHAT to emit
// (a small constrained list); the per-field schema (enum types, custom
// slot semantics) is carried by SetChoicesArgs's jsonschema tags below.
//
// IMPORTANT: kept in Chinese to match the conversation language. Live
// sessions in Chinese with an English tool description saw the model
// "forget" about this tool ~17% of the time after a long Chinese
// narrative; aligning the description language eliminates that
// language-switch drag and keeps the closing call salient.
const setChoicesToolDesc = `记录玩家在本回合结束后可选择的下一步行动。` +
	`【硬性合规】无论叙事如何结束、长度如何，本工具都必须在**叙事写完后、本次回复结束前**调用且仅调用一次。` +
	`不要在叙事中途调用本工具；调用本工具之后也不要再调用任何其他工具。` +
	`必须根据刚生成的叙事，给出 2-4 个紧贴当前情境的行动选项（label 用简短自然语；type 取固定枚举）。` +
	`非关键剧情节点的最后一个选项应是空 label、type=custom 的自定义槽，给玩家自由发挥的口子。`

// SetChoicesArgs is the argument schema the beat agent emits when it
// invokes set_choices. The Options shape mirrors role.ActionOption so
// the parser can map straight into role.ActionChoices without an
// adapter; keeping the struct narrow (Choices only) makes the JSON the
// model has to produce as small as possible.
//
// IMPORTANT: this struct is NEVER unmarshalled by the tool's handler
// (the handler is a no-op — see noopChoicesTool below). It exists
// purely to give utils.GoStruct2ToolInfo a Go shape from which to
// derive the JSON schema advertised to the model. The actual
// args-extraction happens in ExtractInlineChoices over the tool_call
// arguments string that streamed back in the assistant message.
type SetChoicesArgs struct {
	// Description is kept in Chinese for the same reason as
	// setChoicesToolDesc — the model is reading a Chinese narrative
	// and benefits from a same-language schema description right next
	// to the closing tool call.
	Options []role.ActionOption `json:"options" jsonschema:"required,description=玩家本回合结束后的 2-4 个下一步行动选项。每项的 label 是简短自然语，必须扎根于刚生成的叙事；type 必须取自固定枚举。最后一项可以是空 label + type=custom 的自定义槽（非关键剧情节点必须保留这个口子，关键剧情可省略）。"`
}

// NewSetChoicesTool returns an einotool.BaseTool the beat agent can be
// bound with so the model sees a set_choices schema in its tool list.
// The handler is intentionally a no-op (returns "") because the
// canonical execution path NEVER invokes it: the React graph's default
// firstChunkStreamToolCallChecker routes the model's text-first final
// message to END, and the tool_call delta rides along on the assistant
// message stream where pipeline extracts it via ExtractInlineChoices.
//
// The handler still has to exist (eino requires BaseTool.InvokableRun)
// in case the model — against its prompt — emits a tool_call as the
// very first non-empty chunk; in that defensive path the tool runs,
// returns "", and React loops back to the model with an empty result
// (the beat then degrades to the SuggestActions fallback in pipeline).
// We accept that cost as a graceful-degrade mode rather than a
// correctness concern; the POC confirmed DeepSeek consistently streams
// text first, so this branch is expected to be cold.
func NewSetChoicesTool() (einotool.BaseTool, error) {
	info, err := utils.GoStruct2ToolInfo[SetChoicesArgs](SetChoicesToolName, setChoicesToolDesc)
	if err != nil {
		return nil, fmt.Errorf("build set_choices schema: %w", err)
	}
	return &noopChoicesTool{info: info}, nil
}

// noopChoicesTool is the BaseTool implementation backing set_choices.
// Info publishes the schema; InvokableRun is a no-op. See NewSetChoicesTool's
// doc for why the handler is intentionally empty.
type noopChoicesTool struct {
	info *schema.ToolInfo
}

func (t *noopChoicesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *noopChoicesTool) InvokableRun(_ context.Context, _ string, _ ...einotool.Option) (string, error) {
	// Intentional no-op: see NewSetChoicesTool doc. The tool exists only
	// to advertise an output schema; the arguments are extracted from the
	// streamed assistant message by the pipeline, not by this handler.
	return "", nil
}

var _ einotool.InvokableTool = (*noopChoicesTool)(nil)

// ExtractInlineChoices is the role.InlineChoiceParser method on the
// Narrator GM, delegating to the package-level extractInlineChoices
// free function. We keep the parsing logic free-standing so external
// callers (probes, tests, future GMs that reuse the set_choices
// schema) can use it without going through a Narrator instance, while
// still satisfying the role.GM interface.
func (n *Narrator) ExtractInlineChoices(toolCalls []schema.ToolCall) (role.ActionChoices, bool, error) {
	return extractInlineChoices(toolCalls)
}

// ExtractInlineChoices is the package-level entry point preserved for
// callers that don't have a Narrator handle (probes, tests). New code
// inside the engine should call it through role.GM (the
// InlineChoiceParser method above) so layering is honored.
func ExtractInlineChoices(toolCalls []schema.ToolCall) (role.ActionChoices, bool, error) {
	return extractInlineChoices(toolCalls)
}

// extractInlineChoices scans toolCalls for a set_choices invocation
// and parses its arguments into role.ActionChoices. Returns (choices,
// true, nil) on success, (zero, false, nil) when no set_choices call
// is present (caller should fall back to SuggestActions), and (zero,
// true, err) when set_choices WAS called but its JSON arguments could
// not be unmarshalled (also a fallback signal; the err is for
// diagnostics).
//
// When multiple set_choices calls appear (the model getting too
// eager), the first one wins; subsequent calls are ignored because
// re-emitting choices mid-stream would only happen on prompt failure
// and a fresh "second opinion" is not useful to the player.
func extractInlineChoices(toolCalls []schema.ToolCall) (role.ActionChoices, bool, error) {
	for _, tc := range toolCalls {
		if tc.Function.Name != SetChoicesToolName {
			continue
		}
		args := tc.Function.Arguments
		if args == "" {
			return role.ActionChoices{}, true, fmt.Errorf("set_choices called with empty arguments")
		}
		var parsed SetChoicesArgs
		if err := sonic.UnmarshalString(args, &parsed); err != nil {
			return role.ActionChoices{}, true, fmt.Errorf("unmarshal set_choices args: %w", err)
		}
		return role.ActionChoices{Options: parsed.Options}, true, nil
	}
	return role.ActionChoices{}, false, nil
}
