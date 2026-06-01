package director

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sizolity/worldline/internal/world/model"
)

const (
	DirectorKindScript     = "script"
	DirectorKindReconcile  = "reconcile"
	DirectorKindEventTable = "event_table"
	DirectorKindRandom     = "random"
	DirectorKindLLM        = "llm"
)

type File struct {
	Directors []DirectorConfig `json:"directors"`
}

type DirectorConfig struct {
	ID           string                     `json:"id"`
	Kind         string                     `json:"kind"`
	Events       []model.WorldEvent         `json:"events,omitempty"`
	Cases        []ReconcileCase   `json:"cases,omitempty"`
	Entries      []EventTableEntry `json:"entries,omitempty"`
	Seed         *int64                     `json:"seed,omitempty"`
	SystemPrompt         string                     `json:"system_prompt,omitempty"`
	SystemPromptTemplate string                     `json:"system_prompt_template,omitempty"`
	TemplateFormat       string                     `json:"template_format,omitempty"`
	Provider             string                     `json:"provider,omitempty"`
	Model                string                     `json:"model,omitempty"`
	MaxRepairAttempts    *int                       `json:"max_repair_attempts,omitempty"`
}

// GeneratorFactory builds a TextGenerator from the provider and model
// specified in a director config entry. The config package does not import
// any inference provider; callers inject this factory to wire provider-
// specific generators (DeepSeek, OpenAI, etc.).
type GeneratorFactory func(provider, model string) (TextGenerator, error)

// LoadOptions configures optional dependencies for LoadDirectors.
type LoadOptions struct {
	// GeneratorFactory is required when any director uses kind "llm".
	// It receives the provider and model from the JSON config and returns
	// the corresponding TextGenerator.
	GeneratorFactory GeneratorFactory
}

// LoadDirectorsFromFile loads directors from a JSON or YAML file. The format
// is auto-detected by file extension (.yaml, .yml → YAML; otherwise JSON).
func LoadDirectorsFromFile(path string, opts ...LoadOptions) ([]Director, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err = yamlToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("yaml conversion: %w", err)
		}
	}
	return LoadDirectors(data, opts...)
}

func yamlToJSON(data []byte) ([]byte, error) {
	var generic any
	if err := yaml.Unmarshal(data, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}

func LoadDirectors(data []byte, opts ...LoadOptions) ([]Director, error) {
	var opt LoadOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	directors := make([]Director, 0, len(file.Directors))
	for i, cfg := range file.Directors {
		d, err := buildDirector(cfg, opt)
		if err != nil {
			return nil, fmt.Errorf("directors[%d]: %w", i, err)
		}
		directors = append(directors, d)
	}
	return directors, nil
}

func buildDirector(cfg DirectorConfig, opt LoadOptions) (Director, error) {
	if err := model.ValidateID(cfg.ID); err != nil {
		return nil, fmt.Errorf("id: %w", err)
	}
	if cfg.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}
	switch cfg.Kind {
	case DirectorKindScript:
		if err := validateEvents(cfg.Events); err != nil {
			return nil, err
		}
		return NewScriptDirector(cfg.ID, cfg.Events), nil
	case DirectorKindReconcile:
		if err := validateReconcileCases(cfg.Cases); err != nil {
			return nil, err
		}
		return NewReconcileDirector(cfg.ID, cfg.Cases), nil
	case DirectorKindEventTable:
		if err := validateEventTableEntries(cfg.Entries); err != nil {
			return nil, err
		}
		return NewEventTableDirector(cfg.ID, cfg.Entries), nil
	case DirectorKindRandom:
		if err := validateEventTableEntries(cfg.Entries); err != nil {
			return nil, err
		}
		var rng *rand.Rand
		if cfg.Seed != nil {
			rng = rand.New(rand.NewSource(*cfg.Seed))
		}
		return NewRandomDirector(cfg.ID, cfg.Entries, rng), nil
	case DirectorKindLLM:
		if opt.GeneratorFactory == nil {
			return nil, fmt.Errorf("llm director requires a GeneratorFactory in LoadOptions")
		}
		provider := cfg.Provider
		if provider == "" {
			provider = "deepseek"
		}
		gen, err := opt.GeneratorFactory(provider, cfg.Model)
		if err != nil {
			return nil, fmt.Errorf("generator factory: %w", err)
		}
		var promptTpl *PromptTemplate
		if cfg.SystemPromptTemplate != "" {
			ft, ftErr := resolveTemplateFormat(cfg.TemplateFormat)
			if ftErr != nil {
				return nil, ftErr
			}
			var parseErr error
			promptTpl, parseErr = ParsePromptTemplateWithFormat(cfg.SystemPromptTemplate, ft)
			if parseErr != nil {
				return nil, fmt.Errorf("system_prompt_template: %w", parseErr)
			}
		}
		return NewLLMDirector(cfg.ID, LLMDirectorConfig{
			SystemPrompt:      cfg.SystemPrompt,
			PromptTemplate:    promptTpl,
			Generator:         gen,
			MaxRepairAttempts: cfg.MaxRepairAttempts,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported director kind %q", cfg.Kind)
	}
}

func resolveTemplateFormat(s string) (FormatType, error) {
	switch s {
	case "", "go_template":
		return GoTemplate, nil
	case "fstring":
		return FString, nil
	case "jinja2":
		return Jinja2, nil
	default:
		return 0, fmt.Errorf("unsupported template_format %q (use go_template, fstring, or jinja2)", s)
	}
}

func validateEvents(events []model.WorldEvent) error {
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	return nil
}

func validateReconcileCases(cases []ReconcileCase) error {
	for i, c := range cases {
		if err := model.ValidateID(string(c.EventID)); err != nil {
			return fmt.Errorf("cases[%d].event_id: %w", i, err)
		}
		if err := model.ValidateID(string(c.TargetMemoryID)); err != nil {
			return fmt.Errorf("cases[%d].target_memory_id: %w", i, err)
		}
	}
	return nil
}

func validateEventTableEntries(entries []EventTableEntry) error {
	for i, entry := range entries {
		if entry.Weight <= 0 {
			return fmt.Errorf("entries[%d].weight must be positive", i)
		}
		if err := entry.Event.Validate(); err != nil {
			return fmt.Errorf("entries[%d].event: %w", i, err)
		}
	}
	return nil
}
