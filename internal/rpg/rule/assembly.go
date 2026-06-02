package rule

import (
	"fmt"
	"strings"
)

func AssemblePromptSection(rules []NarrativeRule) string {
	var enabled []NarrativeRule
	for _, r := range rules {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}
	if len(enabled) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## World Rules\n\n")

	var coreRules []NarrativeRule
	categoryCount := make(map[string]int)

	for _, r := range enabled {
		if r.Level == 0 {
			coreRules = append(coreRules, r)
		} else {
			categoryCount[r.Category]++
		}
	}

	if len(coreRules) > 0 {
		b.WriteString("### Core Rules (always active)\n\n")
		for _, r := range coreRules {
			b.WriteString(fmt.Sprintf("- %s\n", r.Content))
		}
		b.WriteString("\n")
	}

	if len(categoryCount) > 0 {
		b.WriteString("### Available Rule Categories\n\n")
		for cat, count := range categoryCount {
			b.WriteString(fmt.Sprintf("- **%s** (%d rules)\n", cat, count))
		}
		b.WriteString("\n")
	}

	return b.String()
}
