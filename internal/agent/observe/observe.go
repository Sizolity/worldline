// Package observe provides a single eino callback Handler that instruments
// every LLM call in the process: ChatModel token usage + call latency are
// accumulated into a process-wide Recorder, and tool invocations are traced.
//
// The handler is meant to be installed ONCE at startup via
// callbacks.AppendGlobalHandlers (see internal/rpg/app). Because the eino-ext
// model implementations dispatch lifecycle callbacks to the process-wide
// global handler list, a single registration covers every ChatModel call —
// graph-based (ReAct Stream) and bare Generate alike — and every tool call
// executed inside an eino ToolsNode, with zero changes to business code.
//
// Output discipline is a product red line: all debug lines go ONLY to stderr
// so the player-facing stdout narrative stream is never polluted, and the
// tool tracer only *reads* tool arguments/results — it never mutates them, so
// hidden dice numbers cannot leak into the narrative.
package observe

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	utilcallbacks "github.com/cloudwego/eino/utils/callbacks"
)

// Environment gates. When unset the handler stays silent and only
// accumulates counters (readable via Snapshot); when set, each matching call
// emits one stderr trace line.
const (
	envDebugLLM  = "WORLDLINE_DEBUG_LLM"  // ChatModel token/latency traces
	envDebugDice = "WORLDLINE_DEBUG_DICE" // tool (incl. dice) traces
)

// Stats is a point-in-time snapshot of process-wide ChatModel counters.
type Stats struct {
	Calls            int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// Recorder accumulates ChatModel call statistics across the whole process.
// It is safe for concurrent use (counters are atomic), which matters because
// streaming token usage is tallied from a background goroutine.
type Recorder struct {
	calls            atomic.Int64
	promptTokens     atomic.Int64
	completionTokens atomic.Int64
	totalTokens      atomic.Int64
}

func (r *Recorder) record(u *einomodel.TokenUsage) {
	r.calls.Add(1)
	if u == nil {
		return
	}
	r.promptTokens.Add(int64(u.PromptTokens))
	r.completionTokens.Add(int64(u.CompletionTokens))
	r.totalTokens.Add(int64(u.TotalTokens))
}

// Snapshot returns the current counter values.
func (r *Recorder) Snapshot() Stats {
	return Stats{
		Calls:            r.calls.Load(),
		PromptTokens:     r.promptTokens.Load(),
		CompletionTokens: r.completionTokens.Load(),
		TotalTokens:      r.totalTokens.Load(),
	}
}

// defaultRecorder is the process-wide accumulator used by handlers built with
// NewHandler unless WithRecorder overrides it.
var defaultRecorder = &Recorder{}

// Snapshot returns the process-wide ChatModel statistics accumulated by the
// handler registered at startup (the default recorder).
func Snapshot() Stats { return defaultRecorder.Snapshot() }

type config struct {
	recorder  *Recorder
	out       io.Writer
	debugLLM  bool
	debugDice bool
}

// Option customizes a handler built by NewHandler. The defaults read the
// debug gates from the environment, write to stderr, and use the process-wide
// recorder; the options exist mainly so tests can inject a buffer/recorder and
// force the gates on.
type Option func(*config)

// WithRecorder routes accumulated stats into r instead of the package default.
func WithRecorder(r *Recorder) Option {
	return func(c *config) {
		if r != nil {
			c.recorder = r
		}
	}
}

// WithOutput redirects debug trace lines to w instead of os.Stderr.
func WithOutput(w io.Writer) Option {
	return func(c *config) {
		if w != nil {
			c.out = w
		}
	}
}

// WithLLMDebug forces ChatModel trace output on/off, overriding the env gate.
func WithLLMDebug(on bool) Option { return func(c *config) { c.debugLLM = on } }

// WithToolDebug forces tool trace output on/off, overriding the env gate.
func WithToolDebug(on bool) Option { return func(c *config) { c.debugDice = on } }

// NewHandler builds the eino callback Handler described in the package doc.
// Register the result exactly once via callbacks.AppendGlobalHandlers before
// any LLM call.
func NewHandler(opts ...Option) callbacks.Handler {
	cfg := config{
		recorder:  defaultRecorder,
		out:       os.Stderr,
		debugLLM:  os.Getenv(envDebugLLM) != "",
		debugDice: os.Getenv(envDebugDice) != "",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	modelHandler := &utilcallbacks.ModelCallbackHandler{
		// Stamp the start time so OnEnd / OnEndWithStreamOutput can report
		// elapsed latency. Always set (even when debug is off) so latency is
		// available the moment debugging is enabled.
		OnStart: func(ctx context.Context, _ *callbacks.RunInfo, _ *einomodel.CallbackInput) context.Context {
			return withStart(ctx, time.Now())
		},
		// Non-streaming path (typed agents call Generate).
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *einomodel.CallbackOutput) context.Context {
			var usage *einomodel.TokenUsage
			var cfgOut *einomodel.Config
			if output != nil {
				usage = output.TokenUsage
				cfgOut = output.Config
			}
			cfg.observeModel(modelLabel(info, cfgOut), elapsedSince(ctx), usage)
			return ctx
		},
		// Streaming path (ReAct beat agent calls Stream). The handler gets its
		// own stream copy and MUST close it; token usage typically rides the
		// final frame, so drain to the end in the background to avoid blocking
		// the real narrative consumer.
		OnEndWithStreamOutput: func(ctx context.Context, info *callbacks.RunInfo, stream *schema.StreamReader[*einomodel.CallbackOutput]) context.Context {
			start, hasStart := startFromCtx(ctx)
			go func() {
				defer stream.Close()
				var usage *einomodel.TokenUsage
				var cfgOut *einomodel.Config
				for {
					frame, err := stream.Recv()
					if err != nil {
						break
					}
					if frame == nil {
						continue
					}
					if frame.TokenUsage != nil {
						usage = frame.TokenUsage
					}
					if frame.Config != nil {
						cfgOut = frame.Config
					}
				}
				elapsed := time.Duration(0)
				if hasStart {
					elapsed = time.Since(start)
				}
				cfg.observeModel(modelLabel(info, cfgOut), elapsed, usage)
			}()
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if cfg.debugLLM {
				fmt.Fprintf(cfg.out, "llm[%s] error after %s: %v\n",
					modelLabel(info, nil), elapsedSince(ctx).Round(time.Millisecond), err)
			}
			return ctx
		},
	}

	// Tool tracer. Worldline tools (incl. the hidden dice tools) implement
	// eino's InvokableTool directly and do not manage their own callbacks, so
	// the ToolsNode wraps each call and dispatches Tool OnStart/OnEnd to this
	// global handler with the tool name + raw JSON args + string result. We
	// only read those values; the tool's return value is untouched.
	toolHandler := &utilcallbacks.ToolCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *einotool.CallbackInput) context.Context {
			if cfg.debugDice && input != nil {
				fmt.Fprintf(cfg.out, "tool[%s] args=%s\n", toolLabel(info), compact(input.ArgumentsInJSON))
			}
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *einotool.CallbackOutput) context.Context {
			if cfg.debugDice && output != nil {
				fmt.Fprintf(cfg.out, "tool[%s] -> %s\n", toolLabel(info), compact(output.Response))
			}
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if cfg.debugDice {
				fmt.Fprintf(cfg.out, "tool[%s] error: %v\n", toolLabel(info), err)
			}
			return ctx
		},
	}

	return utilcallbacks.NewHandlerHelper().
		ChatModel(modelHandler).
		Tool(toolHandler).
		Handler()
}

// observeModel records one ChatModel call and, when LLM debug is on, prints a
// single stderr line.
func (c config) observeModel(name string, elapsed time.Duration, usage *einomodel.TokenUsage) {
	c.recorder.record(usage)
	if !c.debugLLM {
		return
	}
	if usage != nil {
		fmt.Fprintf(c.out, "llm[%s] %s prompt=%d completion=%d total=%d\n",
			name, elapsed.Round(time.Millisecond),
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	} else {
		fmt.Fprintf(c.out, "llm[%s] %s tokens=n/a\n", name, elapsed.Round(time.Millisecond))
	}
}

type startKey struct{}

func withStart(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startKey{}, t)
}

func startFromCtx(ctx context.Context) (time.Time, bool) {
	t, ok := ctx.Value(startKey{}).(time.Time)
	return t, ok
}

func elapsedSince(ctx context.Context) time.Duration {
	if t, ok := startFromCtx(ctx); ok {
		return time.Since(t)
	}
	return 0
}

// modelLabel prefers the concrete model name from the call's resolved config
// (e.g. "deepseek-chat"), falling back to the graph node name or eino's
// component impl type.
func modelLabel(info *callbacks.RunInfo, cfg *einomodel.Config) string {
	if cfg != nil && cfg.Model != "" {
		return cfg.Model
	}
	if info != nil {
		if info.Name != "" {
			return info.Name
		}
		if info.Type != "" {
			return info.Type
		}
	}
	return "chatmodel"
}

func toolLabel(info *callbacks.RunInfo) string {
	if info != nil {
		if info.Name != "" {
			return info.Name
		}
		if info.Type != "" {
			return info.Type
		}
	}
	return "tool"
}

// compact collapses a payload to a single trimmed line, capped to a sane
// length so a runaway argument/result blob cannot flood stderr.
func compact(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	const maxRunes = 200
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
