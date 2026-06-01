package view

import (
	"fmt"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/runtime"
)

// PreflightResult combines validation, budget, and readiness checks.
type PreflightResult struct {
	WorldID    string           `json:"world_id"`
	Validation runtime.ValidationReport `json:"validation"`
	Budget     BudgetEstimate   `json:"budget"`
	Readiness  []ReadinessCheck `json:"readiness"`
	Pass       bool             `json:"pass"`
}

// ReadinessCheck is a single go/no-go condition.
type ReadinessCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

const (
	ReadinessPass = "pass"
	ReadinessFail = "fail"
	ReadinessWarn = "warn"
)

// Preflight runs validation, budget estimation, and readiness checks.
func Preflight(w model.World, maxTokens int) PreflightResult {
	validation := runtime.DeepValidate(w)
	budget := EstimateBudget(w)

	var checks []ReadinessCheck

	if validation.ErrorCount() > 0 {
		checks = append(checks, ReadinessCheck{
			Name:    "validation",
			Status:  ReadinessFail,
			Message: fmt.Sprintf("%d validation error(s)", validation.ErrorCount()),
		})
	} else if !validation.IsClean() {
		checks = append(checks, ReadinessCheck{
			Name:    "validation",
			Status:  ReadinessWarn,
			Message: fmt.Sprintf("%d warning(s)", len(validation.Issues)),
		})
	} else {
		checks = append(checks, ReadinessCheck{
			Name:    "validation",
			Status:  ReadinessPass,
			Message: "no issues",
		})
	}

	if maxTokens > 0 && budget.TotalTokens > maxTokens {
		checks = append(checks, ReadinessCheck{
			Name:    "budget",
			Status:  ReadinessFail,
			Message: fmt.Sprintf("~%d tokens exceeds limit %d", budget.TotalTokens, maxTokens),
		})
	} else {
		checks = append(checks, ReadinessCheck{
			Name:    "budget",
			Status:  ReadinessPass,
			Message: fmt.Sprintf("~%d tokens", budget.TotalTokens),
		})
	}

	if len(w.Entities) == 0 {
		checks = append(checks, ReadinessCheck{
			Name:    "entities",
			Status:  ReadinessFail,
			Message: "world has no entities",
		})
	} else {
		checks = append(checks, ReadinessCheck{
			Name:    "entities",
			Status:  ReadinessPass,
			Message: fmt.Sprintf("%d entity(ies)", len(w.Entities)),
		})
	}

	hasActors := false
	for _, e := range w.Entities {
		if _, ok := e.ActorComponent(); ok {
			hasActors = true
			break
		}
	}
	if !hasActors && len(w.Entities) > 0 {
		checks = append(checks, ReadinessCheck{
			Name:    "actors",
			Status:  ReadinessWarn,
			Message: "no entities have an actor component",
		})
	} else if hasActors {
		checks = append(checks, ReadinessCheck{
			Name:    "actors",
			Status:  ReadinessPass,
			Message: "at least one actor found",
		})
	}

	activeThreads := 0
	for _, t := range w.Threads {
		if t.Status == model.ThreadStatusActive || t.Status == model.ThreadStatusOpen {
			activeThreads++
		}
	}
	if activeThreads == 0 {
		checks = append(checks, ReadinessCheck{
			Name:    "threads",
			Status:  ReadinessWarn,
			Message: "no active or open threads to drive narrative",
		})
	} else {
		checks = append(checks, ReadinessCheck{
			Name:    "threads",
			Status:  ReadinessPass,
			Message: fmt.Sprintf("%d active/open thread(s)", activeThreads),
		})
	}

	if w.Canon.Premise == "" && len(w.Canon.Genre) == 0 {
		checks = append(checks, ReadinessCheck{
			Name:    "canon",
			Status:  ReadinessWarn,
			Message: "no premise or genre set — LLM may lack context for tone/style",
		})
	} else {
		checks = append(checks, ReadinessCheck{
			Name:    "canon",
			Status:  ReadinessPass,
			Message: "premise/genre configured",
		})
	}

	pass := true
	for _, c := range checks {
		if c.Status == ReadinessFail {
			pass = false
			break
		}
	}

	return PreflightResult{
		WorldID:    string(w.ID),
		Validation: validation,
		Budget:     budget,
		Readiness:  checks,
		Pass:       pass,
	}
}

// FormatPreflight renders a human-readable preflight report.
func FormatPreflight(r PreflightResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Preflight: %s\n\n", r.WorldID))

	for _, c := range r.Readiness {
		icon := "✓"
		if c.Status == ReadinessFail {
			icon = "✗"
		} else if c.Status == ReadinessWarn {
			icon = "!"
		}
		sb.WriteString(fmt.Sprintf("  %s %-12s %s\n", icon, c.Name, c.Message))
	}

	sb.WriteString("\n")
	if r.Pass {
		sb.WriteString("Result: PASS — ready for beat execution.\n")
	} else {
		sb.WriteString("Result: FAIL — fix errors before running a beat.\n")
	}

	if !r.Validation.IsClean() {
		sb.WriteString("\n### Validation Issues\n\n")
		sb.WriteString(runtime.FormatValidationReport(r.Validation))
	}

	return sb.String()
}
