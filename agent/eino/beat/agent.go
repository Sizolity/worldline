package einobeat

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/agent/beat"
	einotooladapter "github.com/sizolity/worldline/agent/eino/tool"
	agenttool "github.com/sizolity/worldline/agent/tool"
)

// Agent implements beat.Agent using the eino react agent.
type Agent struct {
	CM einomodel.ToolCallingChatModel
}

// New creates an eino-backed beat agent.
func New(cm einomodel.ToolCallingChatModel) *Agent {
	return &Agent{CM: cm}
}

func (a *Agent) Run(ctx context.Context, req beat.Request) *beat.Stream {
	narrativeCh := make(chan string, 32)
	doneCh := make(chan beat.Result, 1)

	go func() {
		var result beat.Result
		defer func() {
			close(narrativeCh)
			doneCh <- result
		}()
		a.runPipeline(ctx, req, narrativeCh, &result)
	}()

	return &beat.Stream{Narrative: narrativeCh, Done: doneCh}
}

func (a *Agent) runPipeline(ctx context.Context, req beat.Request, narrativeCh chan<- string, result *beat.Result) {
	baseTools := toolsToEino(req.Tools)
	maxStep := req.MaxStep
	if maxStep <= 0 {
		maxStep = 10
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: a.CM,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: baseTools},
		MaxStep:          maxStep,
	})
	if err != nil {
		result.Err = fmt.Errorf("create agent: %w", err)
		return
	}

	messages := []*schema.Message{
		schema.SystemMessage(req.SystemPrompt),
		schema.UserMessage(req.UserMessage),
	}

	sr, err := agent.Stream(ctx, messages)
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
		if len(chunk.ToolCalls) > 0 {
			for _, tc := range chunk.ToolCalls {
				result.ToolCalls = append(result.ToolCalls, beat.ToolCallRecord{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
			continue
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

func toolsToEino(tools []agenttool.Tool) []einotool.BaseTool {
	etools := einotooladapter.ToEinoTools(tools)
	out := make([]einotool.BaseTool, len(etools))
	for i, t := range etools {
		out[i] = t
	}
	return out
}

var _ beat.Agent = (*Agent)(nil)
