package director

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/agent/chat"
	worlddirector "github.com/sizolity/worldline/world/director"
	"github.com/sizolity/worldline/world/model"
)

// Config configures the LLM-driven Director.
type Config struct {
	SystemPrompt      string
	MaxRepairAttempts *int
}

// LLMDirector implements worlddirector.Director using the agent/chat abstraction.
type LLMDirector struct {
	id     string
	model  chat.Model
	config Config
}

// New creates an LLMDirector that uses the given chat.Model.
func New(id string, m chat.Model, cfg Config) *LLMDirector {
	return &LLMDirector{id: id, model: m, config: cfg}
}

func (d *LLMDirector) ID() string { return d.id }

func (d *LLMDirector) Propose(ctx worlddirector.Context) ([]model.WorldEvent, error) {
	systemPrompt := d.config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	userPrompt := buildWorldPrompt(ctx.World)

	resp, err := d.model.Generate(ctx.Ctx, []chat.Message{
		chat.SystemMessage(systemPrompt),
		chat.UserMessage(userPrompt),
	})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	events, parseErr := parseEventResponse(resp.Content)
	if parseErr == nil {
		return events, nil
	}

	maxAttempts := d.repairAttempts()
	for attempt := range maxAttempts {
		repairPrompt := fmt.Sprintf(
			"Your previous response could not be parsed:\n%s\n\nError: %s\n\nPlease return ONLY a valid JSON array of world events.",
			resp.Content, parseErr.Error(),
		)
		resp, err = d.model.Generate(context.WithoutCancel(ctx.Ctx), []chat.Message{
			chat.SystemMessage(systemPrompt),
			chat.UserMessage(repairPrompt),
		})
		if err != nil {
			return nil, fmt.Errorf("llm repair attempt %d: %w", attempt+1, err)
		}
		events, parseErr = parseEventResponse(resp.Content)
		if parseErr == nil {
			return events, nil
		}
	}
	return nil, fmt.Errorf("llm parse after %d repair attempt(s): %w", maxAttempts, parseErr)
}

func (d *LLMDirector) repairAttempts() int {
	if d.config.MaxRepairAttempts == nil {
		return DefaultMaxRepairAttempts
	}
	n := *d.config.MaxRepairAttempts
	if n < 0 {
		return 0
	}
	return n
}

var _ worlddirector.Director = (*LLMDirector)(nil)
