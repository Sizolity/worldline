package narrator

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/rpg/role"
)

func TestNewSetChoicesTool_Info(t *testing.T) {
	tool, err := NewSetChoicesTool()
	if err != nil {
		t.Fatalf("NewSetChoicesTool: %v", err)
	}
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != SetChoicesToolName {
		t.Errorf("Name = %q, want %q", info.Name, SetChoicesToolName)
	}
	if info.Desc == "" {
		t.Error("Desc should not be empty")
	}
	if info.ParamsOneOf == nil {
		t.Error("ParamsOneOf should be derived from SetChoicesArgs schema")
	}
}

func TestExtractInlineChoices_HappyPath(t *testing.T) {
	calls := []schema.ToolCall{{
		ID: "call_xyz",
		Function: schema.FunctionCall{
			Name: SetChoicesToolName,
			Arguments: `{"options":[
				{"label":"前往柜台","type":"social"},
				{"label":"环视店堂","type":"investigate"},
				{"label":"","type":"custom"}
			]}`,
		},
	}}
	choices, found, err := ExtractInlineChoices(calls)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if got, want := len(choices.Options), 3; got != want {
		t.Fatalf("len(Options) = %d, want %d", got, want)
	}
	if choices.Options[0].Type != role.ActionTypeSocial {
		t.Errorf("Options[0].Type = %q", choices.Options[0].Type)
	}
	if !choices.HasCustomSlot() {
		t.Error("expected trailing custom slot")
	}
}

func TestExtractInlineChoices_Missing(t *testing.T) {
	calls := []schema.ToolCall{{
		ID:       "call_other",
		Function: schema.FunctionCall{Name: "advance_time", Arguments: `{"scenes":1}`},
	}}
	choices, found, err := ExtractInlineChoices(calls)
	if found {
		t.Error("expected found=false when set_choices not present")
	}
	if err != nil {
		t.Errorf("err should be nil when not found, got %v", err)
	}
	if len(choices.Options) != 0 {
		t.Errorf("Options should be empty, got %v", choices.Options)
	}
}

func TestExtractInlineChoices_EmptyToolCalls(t *testing.T) {
	_, found, err := ExtractInlineChoices(nil)
	if found || err != nil {
		t.Errorf("nil tool_calls: found=%v err=%v", found, err)
	}
}

func TestExtractInlineChoices_BadJSON(t *testing.T) {
	calls := []schema.ToolCall{{
		Function: schema.FunctionCall{
			Name:      SetChoicesToolName,
			Arguments: `{"options": [`, // truncated
		},
	}}
	_, found, err := ExtractInlineChoices(calls)
	if !found {
		t.Error("found should be true when the call IS present even if args parse fails")
	}
	if err == nil {
		t.Fatal("expected unmarshal error for truncated JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal set_choices args") {
		t.Errorf("err should wrap 'unmarshal set_choices args', got %v", err)
	}
}

func TestExtractInlineChoices_EmptyArgs(t *testing.T) {
	calls := []schema.ToolCall{{
		Function: schema.FunctionCall{Name: SetChoicesToolName, Arguments: ""},
	}}
	_, found, err := ExtractInlineChoices(calls)
	if !found {
		t.Error("found should be true when tool was called even with empty args")
	}
	if err == nil {
		t.Error("expected error for empty args (we can't parse nothing)")
	}
}

func TestExtractInlineChoices_FirstWins(t *testing.T) {
	calls := []schema.ToolCall{
		{Function: schema.FunctionCall{Name: SetChoicesToolName, Arguments: `{"options":[{"label":"first","type":"explore"}]}`}},
		{Function: schema.FunctionCall{Name: SetChoicesToolName, Arguments: `{"options":[{"label":"second","type":"social"}]}`}},
	}
	choices, found, err := ExtractInlineChoices(calls)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got := choices.Options[0].Label; got != "first" {
		t.Errorf("expected first set_choices call to win, got Label=%q", got)
	}
}
