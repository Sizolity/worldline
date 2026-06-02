// Package react drives a multi-step ReAct loop (Reason + Act) on top of
// cloudwego/eino's [flow/agent/react] agent. Callers supply an initial
// conversation (system + user, or whatever shape they need) plus a list
// of eino [einotool.BaseTool] values, and receive a streaming Stream
// that yields narrative chunks. Tool side effects do not flow back through
// this Stream — eino's ReAct graph only routes the ChatModel's
// no-more-tool-calls output to END, so the chunks reaching callers carry
// pure narrative. Tools surface their effects out-of-band via the shared
// [ToolContext] (see internal/rpg/tools), which the beat pipeline drains.
//
// Tools are passed in eino-native form ([]einotool.BaseTool); business
// implementations (see internal/rpg/tools) implement eino's tool
// interface directly so there is no worldline-side adapter layer.
// Messages are likewise native eino [*schema.Message] values so callers
// retain full control over role, multi-turn history, tool messages, etc.
package react

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// Request describes one ReAct invocation. Messages is the full initial
// conversation (typically system + user, but any role sequence is
// accepted). Tools is the eino tool list passed straight through to
// [compose.ToolsNodeConfig]. MaxStep caps the tool-call iterations; if
// non-positive the default cap (10) is applied.
type Request struct {
	Messages []*schema.Message
	Tools    []einotool.BaseTool
	MaxStep  int
}

// Result is the terminal value of a Run. Narrative is the concatenated
// assistant-message body (i.e. everything the model said outside of tool
// calls). Err is non-nil when the underlying eino agent or transport
// failed; it is independent of the narrative channel, which is always
// closed cleanly so range loops in callers terminate.
type Result struct {
	Narrative string
	Err       error
}

// Stream is the two-channel handle returned by Run. Callers MUST consume
// Narrative to completion (or the producer will block on a buffered send)
// before reading Done, or they can call [Stream.Wait] which does both in
// the canonical order.
type Stream struct {
	Narrative <-chan string
	Done      <-chan Result
}

// Wait drains Narrative and returns the final Result. Provided as a
// convenience for callers that do not need incremental narrative
// streaming.
func (s *Stream) Wait() Result {
	for range s.Narrative {
	}
	return <-s.Done
}

// Agent is the abstraction Run satisfies. It exists primarily so tests
// can substitute a deterministic, LLM-free implementation without
// pulling in an eino model.
type Agent interface {
	Run(ctx context.Context, req Request) *Stream
}

// New returns an Agent backed by an eino [model.ToolCallingChatModel].
// The returned agent is safe to share across goroutines: each Run
// builds its own eino agent + ToolsNode so concurrent calls do not
// race on bound tool state.
func New(cm model.ToolCallingChatModel) Agent {
	return &einoAgent{cm: cm}
}

type einoAgent struct {
	cm model.ToolCallingChatModel
}

var _ Agent = (*einoAgent)(nil)

// Run executes one ReAct loop. The goroutine guarantees:
//   - Narrative is always closed (even on early errors) so range loops
//     in callers terminate.
//   - Done receives exactly one Result.
//   - Narrative is closed BEFORE Done receives, so the canonical
//     `for chunk := range s.Narrative { ... }; r := <-s.Done` pattern
//     never deadlocks.
func (a *einoAgent) Run(ctx context.Context, req Request) *Stream {
	narrativeCh := make(chan string, 32)
	doneCh := make(chan Result, 1)

	go func() {
		var result Result
		defer func() {
			close(narrativeCh)
			doneCh <- result
		}()
		a.runPipeline(ctx, req, narrativeCh, &result)
	}()

	return &Stream{Narrative: narrativeCh, Done: doneCh}
}

func (a *einoAgent) runPipeline(ctx context.Context, req Request, narrativeCh chan<- string, result *Result) {
	maxStep := req.MaxStep
	if maxStep <= 0 {
		maxStep = 10
	}

	agent, err := einoreact.NewAgent(ctx, &einoreact.AgentConfig{
		ToolCallingModel: a.cm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: req.Tools},
		MaxStep:          maxStep,
	})
	if err != nil {
		result.Err = fmt.Errorf("create agent: %w", err)
		return
	}

	sr, err := agent.Stream(ctx, req.Messages)
	if err != nil {
		result.Err = fmt.Errorf("agent stream: %w", err)
		return
	}

	var narrativeBuf []byte
	for {
		chunk, err := sr.Recv()
		if err != nil {
			break
		}
		if chunk.Content != "" {
			narrativeBuf = append(narrativeBuf, chunk.Content...)
			select {
			case narrativeCh <- chunk.Content:
			case <-ctx.Done():
				result.Err = ctx.Err()
				return
			}
		}
	}
	result.Narrative = string(narrativeBuf)
}
