package intent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sizolity/worldline/agent/chat"
	"github.com/sizolity/worldline/internal/app/mod"
	"github.com/sizolity/worldline/rpg/role"
)

// stubAgent is a structured.Agent[Params] for tests. It records the
// most recent message slice it was called with and replays canned
// outputs so we can assert (a) the system/user prompt content and (b)
// how the Resolver handles success / failure.
type stubAgent struct {
	calls    int
	gotMsgs  []chat.Message
	out      Params
	err      error
}

func (s *stubAgent) Call(_ context.Context, msgs []chat.Message) (Params, error) {
	s.calls++
	s.gotMsgs = msgs
	return s.out, s.err
}

func sampleChoices() role.ActionChoices {
	return role.ActionChoices{Options: []role.ActionOption{
		{Label: "勘察密室", Type: role.ActionTypeInvestigate},
		{Label: "与守卫攀谈", Type: role.ActionTypeSocial},
		{Label: "潜入后院", Type: role.ActionTypeExplore},
		{Type: role.ActionTypeCustom},
	}}
}

func TestNewResolver_RequiresAgent(t *testing.T) {
	if _, err := NewResolver(nil, nil); err == nil {
		t.Fatal("expected error when agent is nil")
	}
}

func TestNewResolver_FallsBackToDefaultPersona(t *testing.T) {
	r, err := NewResolver(&stubAgent{}, nil)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	prompt := r.SystemPrompt()
	for _, want := range []string{"意图解释", "## 解释原则", "引擎合规层", ResolveToolName} {
		if !strings.Contains(prompt, want) {
			t.Errorf("default-persona system prompt missing %q\n--- prompt ---\n%s", want, prompt)
		}
	}
}

func TestSystemPrompt_AppendsEngineConstraints(t *testing.T) {
	personaMD := "# 测试 Persona\n\n这是测试用 persona。\n\n## 规则\n\n- 别瞎说\n"
	doc, err := mod.ParseDocument(personaMD)
	if err != nil {
		t.Fatalf("parse persona: %v", err)
	}
	r, err := NewResolver(&stubAgent{}, doc)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	prompt := r.SystemPrompt()
	for _, want := range []string{
		"# 测试 Persona",
		"这是测试用 persona",
		"## 规则",
		"别瞎说",
		"引擎合规层",
		ResolveToolName,
		"反幻觉约束",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", want, prompt)
		}
	}
}

func TestUserPrompt_IncludesChoicesAndNarrative(t *testing.T) {
	r, err := NewResolver(&stubAgent{}, nil)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	choices := sampleChoices()
	out := r.UserPrompt("用2根手指掐诀念咒", choices, "前情：白虎岭风沙四起，悟空迟疑不动")

	for _, want := range []string{
		"## 玩家原始输入",
		"用2根手指掐诀念咒",
		"## 可选行动",
		"1. 勘察密室",
		"2. 与守卫攀谈",
		"3. 潜入后院",
		"4. (自定义槽",
		"## 最近叙事",
		"白虎岭风沙四起",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("user prompt missing %q\n--- prompt ---\n%s", want, out)
		}
	}
}

func TestUserPrompt_EmptyNarrativePlaceholder(t *testing.T) {
	r, _ := NewResolver(&stubAgent{}, nil)
	out := r.UserPrompt("调查地砖", role.ActionChoices{}, "")
	if !strings.Contains(out, "本局首个回合") {
		t.Errorf("expected first-beat fallback message in user prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "本回合无推荐选项") {
		t.Errorf("expected empty-choices fallback message in user prompt, got:\n%s", out)
	}
}

func TestResolve_Success(t *testing.T) {
	agent := &stubAgent{out: Params{
		ActionText:    "悟空腾云驾雾，俯瞰白虎岭",
		IsDestructive: false,
		Notes:         "通顺润色",
	}}
	r, err := NewResolver(agent, nil)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	got, err := r.Resolve(context.Background(), "腾云驾雾飞过去看看", sampleChoices(), "narr")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ActionText != "悟空腾云驾雾，俯瞰白虎岭" {
		t.Errorf("ActionText mismatch: %q", got.ActionText)
	}
	if agent.calls != 1 {
		t.Errorf("expected exactly one agent call, got %d", agent.calls)
	}
	if len(agent.gotMsgs) != 2 {
		t.Fatalf("expected [system, user] messages, got %d", len(agent.gotMsgs))
	}
	if agent.gotMsgs[0].Role != chat.RoleSystem {
		t.Errorf("first message must be system, got %s", agent.gotMsgs[0].Role)
	}
	if agent.gotMsgs[1].Role != chat.RoleUser {
		t.Errorf("second message must be user, got %s", agent.gotMsgs[1].Role)
	}
	if !strings.Contains(agent.gotMsgs[1].Content, "腾云驾雾飞过去看看") {
		t.Errorf("user message must contain raw input")
	}
}

func TestResolve_AgentErrorIsWrapped(t *testing.T) {
	wantErr := errors.New("transport blew up")
	agent := &stubAgent{err: wantErr}
	r, _ := NewResolver(agent, nil)

	_, err := r.Resolve(context.Background(), "随便走走", sampleChoices(), "narr")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestResolve_EmptyInputShortCircuits(t *testing.T) {
	agent := &stubAgent{out: Params{ActionText: "should-not-run"}}
	r, _ := NewResolver(agent, nil)

	_, err := r.Resolve(context.Background(), "   ", sampleChoices(), "narr")
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
	if agent.calls != 0 {
		t.Errorf("agent must not be called on empty input, got calls=%d", agent.calls)
	}
}

func TestResolve_EmptyActionTextRejected(t *testing.T) {
	agent := &stubAgent{out: Params{ActionText: "   "}}
	r, _ := NewResolver(agent, nil)
	_, err := r.Resolve(context.Background(), "腾云", sampleChoices(), "narr")
	if err == nil {
		t.Fatal("expected error when LLM returns empty action_text")
	}
}
