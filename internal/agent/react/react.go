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
//
// Result.ToolCalls additionally surfaces tool_calls that appear in the
// FINAL assistant message (the one routed to END because its first
// non-empty chunk was content, not a tool_call). This is the "inline
// schema" pattern: register a tool that the model is prompted to call at
// the end of its narrative without it actually being executed by the
// tools node; callers parse its arguments out of Result.ToolCalls as a
// structured output channel that piggybacks on the narrative stream and
// costs no extra LLM round-trip. See internal/rpg/session/pipeline for
// the canonical use case (set_choices replacing SuggestActions).
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
// calls). ToolCalls aggregates every tool_call delta that streamed back
// from the final assistant message — these are tool calls the model
// emitted alongside its narrative but that the React graph did NOT
// dispatch to the tools node (because routing decided END based on the
// first non-empty chunk being content). Callers that bind an "inline
// schema" tool (no real execution; the tool exists only to give the
// model a structured output channel) parse its arguments out of this
// slice. Tool calls that WERE dispatched to the tools node and looped
// back are not represented here — only the terminal message's
// tool_calls survive to END.
//
// Err is non-nil when the underlying eino agent or transport failed; it
// is independent of the narrative channel, which is always closed
// cleanly so range loops in callers terminate.
type Result struct {
	Narrative string
	ToolCalls []schema.ToolCall
	Err       error
}

// Stream is the two-channel handle returned by Run. Callers MUST consume
// Narrative to completion (or the producer will block on a buffered send)
// before reading Done, or they can call [Stream.Wait] which does both in
// the canonical order.
//
// Narrative close semantics: the channel is closed as soon as the model
// transitions from content streaming to tool_call streaming, NOT when
// the underlying stream fully ends. This lets downstream consumers
// react to "narrative complete" while tool_call args are still being
// streamed (e.g. start an LLM-bound extraction task that only depends
// on narrative text and gain ~1s of overlap with the tail of the
// stream). Wait still works unchanged because Done fires at the true
// stream end after Narrative has been closed; the existing
// `for chunk := range s.Narrative { ... }; r := <-s.Done` pattern is
// also unchanged because it never observed close timing.
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
//     never deadlocks. Per Stream's doc, "closed" here means content
//     streaming has ended — the underlying stream may still be
//     delivering tool_call deltas afterwards, which we keep
//     accumulating until the true end of stream.
func (a *einoAgent) Run(ctx context.Context, req Request) *Stream {
	narrativeCh := make(chan string, 32)
	doneCh := make(chan Result, 1)

	go func() {
		var (
			result            Result
			narrativeChClosed bool
		)
		closeNarrativeOnce := func() {
			if !narrativeChClosed {
				close(narrativeCh)
				narrativeChClosed = true
			}
		}
		defer func() {
			// Safety net: if runPipeline returned without ever closing
			// narrativeCh (e.g. error before the first tool_call chunk
			// arrived), close it now so range loops in callers
			// terminate.
			closeNarrativeOnce()
			doneCh <- result
		}()
		a.runPipeline(ctx, req, narrativeCh, closeNarrativeOnce, &result)
	}()

	return &Stream{Narrative: narrativeCh, Done: doneCh}
}

func (a *einoAgent) runPipeline(ctx context.Context, req Request, narrativeCh chan<- string, closeNarrative func(), result *Result) {
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

	// Accumulate every chunk so we can hand the merged final message to
	// schema.ConcatMessages and recover any tool_calls the model emitted
	// alongside its narrative (the "inline schema" pattern — see package
	// doc). Content is also forwarded chunk-by-chunk to narrativeCh for
	// the live streaming UX; the buffer here is for the post-stream
	// tool_call extraction, not for re-emitting text.
	//
	// closeNarrative is invoked at the FIRST chunk that carries any
	// tool_call delta — that is the model's transition from "still
	// writing narrative" to "writing tool_call args". Closing the
	// channel there (instead of at end-of-stream) lets the pipeline
	// kick off narrative-only work (lorekeeper extraction) in parallel
	// with the still-streaming tool_call tail (~1s for set_choices).
	// If the model never emits a tool_call, closeNarrative fires from
	// the outer goroutine's defer instead.
	//
	// Late content (a content chunk arriving AFTER tool_calls have
	// started, which the POC has not seen but is theoretically allowed
	// by some OpenAI-compatible streams) is still appended to
	// narrativeBuf so result.Narrative remains complete, but it is NOT
	// forwarded to the already-closed channel — callers that received
	// the close already moved on, and a panic on send-to-closed would
	// crash the goroutine.
	var (
		chunks            []*schema.Message
		narrativeBuf      []byte
		narrativeChClosed bool
	)
	for {
		chunk, err := sr.Recv()
		if err != nil {
			break
		}
		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			narrativeBuf = append(narrativeBuf, chunk.Content...)
			if !narrativeChClosed {
				select {
				case narrativeCh <- chunk.Content:
				case <-ctx.Done():
					result.Err = ctx.Err()
					return
				}
			}
		}
		if len(chunk.ToolCalls) > 0 && !narrativeChClosed {
			closeNarrative()
			narrativeChClosed = true
		}
	}
	result.Narrative = string(narrativeBuf)
	if len(chunks) > 0 {
		// ConcatMessages does the proper delta-merge: text content is
		// joined in order, and tool_calls are merged by Index/ID so
		// streaming tool_call arg deltas (which arrive one JSON token at
		// a time in OpenAI-compatible streams) are reassembled into the
		// final structured form. We deliberately swallow the error to
		// preserve the narrative-only result on partial / malformed
		// chunks: ToolCalls will simply be empty and callers fall back
		// to their non-inline path.
		if merged, mErr := schema.ConcatMessages(chunks); mErr == nil && merged != nil {
			result.ToolCalls = merged.ToolCalls
		}
	}
}
