package director

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"text/template"

	"github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/prompt"
	"github.com/sizolity/worldline/internal/world/textutil"
)

// FormatType specifies the template syntax for prompt templates.
type FormatType int

const (
	GoTemplate FormatType = iota
	FString
	Jinja2
)

// TextGenerator abstracts an LLM inference call. Implementations wrap
// provider-specific chat APIs (llama.cpp, OpenAI, etc.).
type TextGenerator interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ConversationGenerator extends TextGenerator with multi-turn support.
// Used by the repair loop to feed parse errors back to the LLM.
// Implementations that only support single-turn can omit this interface;
// the repair loop will fall back to a fresh Generate call with the error
// appended to the user prompt.
type ConversationGenerator interface {
	TextGenerator
	GenerateRepair(ctx context.Context, systemPrompt, originalUser, previousAssistant, repairUser string) (string, error)
}

// DefaultSystemPrompt documents the WorldEvent schema for LLM generators.
// Used when LLMDirectorConfig.SystemPrompt is empty.
const DefaultSystemPrompt = `You are a world director for a narrative simulation engine.
Given the current world state as JSON, propose one or more world events as a JSON array.

Each event MUST have:
- "id": unique snake_case identifier (e.g. "event_merchant_arrives")
- "type": one of "note", "world_fact_changed", "remember", "move", "thread_changed"
- "source": always "director"
- "description": one-sentence narrative description

Events MAY include "effects" to mutate world state. Each effect has:
- "kind": the mutation type
- "target_id": the target entity/fact/memory/thread ID
- "payload": key-value pairs where values are {"kind":"string|number|boolean|entity_ref","raw":<value>}

Supported effect kinds:
- "set_fact": payload needs "subject_id" (entity_ref), "predicate" (string), "value" (any)
- "update_entity_state": target_id is entity, payload keys become state entries
- "add_memory": target_id is new memory ID, payload needs "owner_kind" (string: "world"|"character"), "owner_id" (string, if character), "scope" (string: "factual"|"subjective"), "memory_kind" (string: "observation"|"belief"|"rumor"), "content" (string), "truth_status" (string: "true"|"unknown"|"disputed"|"secret")
- "open_thread": target_id is new thread ID, payload needs "title" (string), "kind" (string: "mystery"|"quest"|"conflict"|"theme")
- "close_thread": target_id is existing thread ID
- "add_relation": target_id is new relation ID, payload needs "type" (string), "source_id" (entity_ref), "relation_target_id" (entity_ref)
- "add_entity": target_id is new entity ID, payload needs "type" (string: "character"|"location"|"item"), "name" (string)

A simple narrative event with no world mutation:
[{"id":"event_dawn","type":"note","source":"director","description":"Dawn breaks."}]

An event that also sets a world fact:
[{"id":"event_gate_sealed","type":"world_fact_changed","source":"director","description":"The city gate is sealed.","effects":[{"kind":"set_fact","target_id":"fact_gate","payload":{"subject_id":{"kind":"entity_ref","raw":"city_gate"},"predicate":{"kind":"string","raw":"status"},"value":{"kind":"string","raw":"sealed"}}}]}]

Return ONLY a valid JSON array. No markdown, no explanation.`

const DefaultMaxRepairAttempts = 2

// PromptTemplate wraps a text/template that renders a system prompt
// using live world state. Variables are injected from PromptTemplateData.
type PromptTemplate struct {
	tpl *template.Template
}

// PromptTemplateData is the data available inside a system prompt template.
// Field names match prompt.Context for consistency.
type PromptTemplateData struct {
	WorldID     string
	Name        string
	Description string
	Clock       int64
	Entities    []prompt.EntitySummary
	Facts       []prompt.FactSummary
	Relations   []prompt.RelationSummary
	Memories    []prompt.MemorySummary
	Threads     []prompt.ThreadSummary
}

// ParsePromptTemplate parses a template string using Go text/template syntax
// ({{.Var}}). Use ParsePromptTemplateWithFormat for other syntaxes.
func ParsePromptTemplate(text string) (*PromptTemplate, error) {
	return ParsePromptTemplateWithFormat(text, GoTemplate)
}

// ParsePromptTemplateWithFormat parses a template string with the specified
// format type. Supported: GoTemplate ({{.Var}}), FString ({Var}),
// Jinja2 ({{Var}}).
func ParsePromptTemplateWithFormat(text string, ft FormatType) (*PromptTemplate, error) {
	goTpl := convertToGoTemplate(text, ft)
	tpl, err := template.New("prompt").Parse(goTpl)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, PromptTemplateData{}); err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}
	return &PromptTemplate{tpl: tpl}, nil
}

func convertToGoTemplate(text string, ft FormatType) string {
	switch ft {
	case FString:
		return fstringRe.ReplaceAllString(text, "{{.${1}}}")
	case Jinja2:
		return jinja2Re.ReplaceAllString(text, "{{.${1}}}")
	default:
		return text
	}
}

var (
	fstringRe = regexp.MustCompile(`\{(\w+)\}`)
	jinja2Re  = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)
)

// Render executes the template against the world state and returns the
// resulting system prompt string.
func (pt *PromptTemplate) Render(w model.World) (string, error) {
	pc := prompt.Render(w)
	data := PromptTemplateData{
		WorldID:     pc.WorldID,
		Name:        pc.Name,
		Description: pc.Description,
		Clock:       pc.Clock,
		Entities:    pc.Entities,
		Facts:       pc.Facts,
		Relations:   pc.Relations,
		Memories:    pc.Memories,
		Threads:     pc.Threads,
	}
	var buf bytes.Buffer
	if err := pt.tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt template: %w", err)
	}
	return buf.String(), nil
}

type LLMDirectorConfig struct {
	// SystemPrompt is a static system prompt string. Ignored when
	// PromptTemplate is set.
	SystemPrompt string

	// PromptTemplate is a dynamic system prompt rendered per Propose call
	// with live world state. Takes priority over SystemPrompt.
	PromptTemplate *PromptTemplate

	Generator TextGenerator

	// MaxRepairAttempts is the number of times the LLM is asked to fix its
	// response after a parse/validation failure. 0 means no retries (fail
	// immediately). Negative values are treated as 0. Defaults to
	// DefaultMaxRepairAttempts when left at 0 by the caller — use -1 to
	// explicitly disable retries.
	MaxRepairAttempts *int
}

type LLMDirector struct {
	id     string
	config LLMDirectorConfig
}

func NewLLMDirector(id string, config LLMDirectorConfig) LLMDirector {
	return LLMDirector{id: id, config: config}
}

func (d LLMDirector) ID() string {
	return d.id
}

func (d LLMDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	systemPrompt, err := d.resolveSystemPrompt(ctx.World)
	if err != nil {
		return nil, fmt.Errorf("system prompt: %w", err)
	}
	userPrompt := buildWorldPrompt(ctx.World)

	response, err := d.config.Generator.Generate(ctx.Ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	events, parseErr := parseEventResponse(response)
	if parseErr == nil {
		return events, nil
	}

	maxAttempts := d.repairAttempts()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		repairPrompt := fmt.Sprintf(
			"Your previous response could not be parsed:\n%s\n\nError: %s\n\nPlease return ONLY a valid JSON array of world events. No markdown, no explanation.",
			response, parseErr.Error(),
		)

		if cg, ok := d.config.Generator.(ConversationGenerator); ok {
			response, err = cg.GenerateRepair(ctx.Ctx, systemPrompt, userPrompt, response, repairPrompt)
		} else {
			response, err = d.config.Generator.Generate(ctx.Ctx, systemPrompt, repairPrompt)
		}
		if err != nil {
			return nil, fmt.Errorf("llm repair attempt %d: %w", attempt+1, err)
		}

		events, parseErr = parseEventResponse(response)
		if parseErr == nil {
			return events, nil
		}
	}

	return nil, fmt.Errorf("llm parse after %d repair attempt(s): %w", maxAttempts, parseErr)
}

// resolveSystemPrompt returns the system prompt for this call.
// Priority: PromptTemplate (dynamic) > SystemPrompt (static) > DefaultSystemPrompt.
func (d LLMDirector) resolveSystemPrompt(w model.World) (string, error) {
	if d.config.PromptTemplate != nil {
		return d.config.PromptTemplate.Render(w)
	}
	if d.config.SystemPrompt != "" {
		return d.config.SystemPrompt, nil
	}
	return DefaultSystemPrompt, nil
}

func (d LLMDirector) repairAttempts() int {
	if d.config.MaxRepairAttempts == nil {
		return DefaultMaxRepairAttempts
	}
	n := *d.config.MaxRepairAttempts
	if n < 0 {
		return 0
	}
	return n
}

func buildWorldPrompt(w model.World) string {
	pc := prompt.Render(w)
	data, err := json.Marshal(pc)
	if err != nil {
		return fmt.Sprintf(`{"world_id":%q,"name":%q}`, w.ID, w.Name)
	}
	return string(data)
}

func parseEventResponse(response string) ([]model.WorldEvent, error) {
	cleaned := textutil.StripMarkdownFences(response)
	var events []model.WorldEvent
	if err := json.Unmarshal([]byte(cleaned), &events); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("event[%d]: %w", i, err)
		}
	}
	return model.CloneEvents(events), nil
}
