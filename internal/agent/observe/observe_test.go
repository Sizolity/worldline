package observe_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/observe"
)

// syncBuf is a tiny goroutine-safe buffer; the streaming handler writes from a
// background goroutine, so the test reads must be race-free.
type syncBuf struct {
	mu  chan struct{}
	buf strings.Builder
}

func newSyncBuf() *syncBuf { return &syncBuf{mu: make(chan struct{}, 1)} }

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu <- struct{}{}
	defer func() { <-b.mu }()
	return b.buf.Write(p)
}

func (b *syncBuf) String() string {
	b.mu <- struct{}{}
	defer func() { <-b.mu }()
	return b.buf.String()
}

// TestHandlerObservesChatModelGenerate exercises the non-streaming path that
// the typed agents (Generate) take: the handler must tally token usage and,
// with LLM debug on, emit one stderr line tagged with the model name.
func TestHandlerObservesChatModelGenerate(t *testing.T) {
	buf := newSyncBuf()
	rec := &observe.Recorder{}
	h := observe.NewHandler(observe.WithOutput(buf), observe.WithRecorder(rec), observe.WithLLMDebug(true))

	info := &callbacks.RunInfo{Component: components.ComponentOfChatModel, Type: "FakeChatModel"}
	ctx := h.OnStart(context.Background(), info, &einomodel.CallbackInput{})
	h.OnEnd(ctx, info, &einomodel.CallbackOutput{
		Config:     &einomodel.Config{Model: "deepseek-chat"},
		TokenUsage: &einomodel.TokenUsage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20},
	})

	got := rec.Snapshot()
	if got.Calls != 1 || got.PromptTokens != 12 || got.CompletionTokens != 8 || got.TotalTokens != 20 {
		t.Fatalf("unexpected stats: %+v", got)
	}
	if out := buf.String(); !strings.Contains(out, "llm[deepseek-chat]") || !strings.Contains(out, "total=20") {
		t.Fatalf("expected stderr trace with model + tokens, got %q", out)
	}
}

// TestHandlerObservesChatModelStream exercises the streaming path that the
// ReAct beat agent (Stream) takes: token usage rides the final frame and is
// tallied from the handler's background drain goroutine.
func TestHandlerObservesChatModelStream(t *testing.T) {
	buf := newSyncBuf()
	rec := &observe.Recorder{}
	h := observe.NewHandler(observe.WithOutput(buf), observe.WithRecorder(rec), observe.WithLLMDebug(true))

	// eino hands the handler a stream of the generic callbacks.CallbackOutput
	// (each item is a *model.CallbackOutput); mirror that wire shape here.
	sr, sw := schema.Pipe[callbacks.CallbackOutput](4)
	sw.Send(&einomodel.CallbackOutput{Config: &einomodel.Config{Model: "deepseek-chat"}}, nil)
	sw.Send(&einomodel.CallbackOutput{
		Config:     &einomodel.Config{Model: "deepseek-chat"},
		TokenUsage: &einomodel.TokenUsage{PromptTokens: 30, CompletionTokens: 10, TotalTokens: 40},
	}, nil)
	sw.Close()

	info := &callbacks.RunInfo{Component: components.ComponentOfChatModel, Type: "FakeChatModel"}
	ctx := h.OnStart(context.Background(), info, &einomodel.CallbackInput{})
	h.OnEndWithStreamOutput(ctx, info, sr)

	// The drain runs in a background goroutine; poll until it lands.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if rec.Snapshot().Calls == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream handler never recorded: %+v", rec.Snapshot())
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := rec.Snapshot()
	if got.PromptTokens != 30 || got.CompletionTokens != 10 || got.TotalTokens != 40 {
		t.Fatalf("unexpected stream stats: %+v", got)
	}
	if out := buf.String(); !strings.Contains(out, "total=40") {
		t.Fatalf("expected stream stderr trace, got %q", out)
	}
}

// TestHandlerTracesTool confirms tool name + args + result reach stderr under
// the dice debug gate, mirroring how eino's ToolsNode dispatches Tool
// callbacks to the global handler.
func TestHandlerTracesTool(t *testing.T) {
	buf := newSyncBuf()
	h := observe.NewHandler(observe.WithOutput(buf), observe.WithRecorder(&observe.Recorder{}), observe.WithToolDebug(true))

	info := &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "roll"}
	ctx := h.OnStart(context.Background(), info, &einotool.CallbackInput{ArgumentsInJSON: `{"sides":20}`})
	h.OnEnd(ctx, info, &einotool.CallbackOutput{Response: `{"total":17}`})

	out := buf.String()
	if !strings.Contains(out, "tool[roll] args=") || !strings.Contains(out, `"sides":20`) {
		t.Fatalf("expected tool args trace, got %q", out)
	}
	if !strings.Contains(out, "tool[roll] -> ") || !strings.Contains(out, `"total":17`) {
		t.Fatalf("expected tool result trace, got %q", out)
	}
}

// TestHandlerSilentByDefault confirms the production default (no gate set):
// counters still accumulate but nothing is ever written to the output stream,
// protecting the stdout narrative red line.
func TestHandlerSilentByDefault(t *testing.T) {
	buf := newSyncBuf()
	rec := &observe.Recorder{}
	h := observe.NewHandler(observe.WithOutput(buf), observe.WithRecorder(rec),
		observe.WithLLMDebug(false), observe.WithToolDebug(false))

	info := &callbacks.RunInfo{Component: components.ComponentOfChatModel, Type: "X"}
	ctx := h.OnStart(context.Background(), info, &einomodel.CallbackInput{})
	h.OnEnd(ctx, info, &einomodel.CallbackOutput{TokenUsage: &einomodel.TokenUsage{TotalTokens: 7}})

	if out := buf.String(); out != "" {
		t.Fatalf("expected silent handler, got stderr %q", out)
	}
	if rec.Snapshot().TotalTokens != 7 {
		t.Fatalf("expected silent accumulation, got %+v", rec.Snapshot())
	}
}
