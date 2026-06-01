package rule

import (
	"fmt"
	"strings"
)

type LookupFilter struct {
	Category string
	Tags     []string
}

func Lookup(rules []Rule, filter LookupFilter) []Rule {
	var out []Rule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if filter.Category != "" && r.Category != filter.Category {
			continue
		}
		if len(filter.Tags) > 0 && !hasAnyTag(r.Tags, filter.Tags) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func hasAnyTag(ruleTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, rt := range ruleTags {
			if rt == ft {
				return true
			}
		}
	}
	return false
}

func FormatRules(rules []Rule) string {
	if len(rules) == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range rules {
		b.WriteString(fmt.Sprintf("%d. %s", i+1, r.Content))
		if len(r.Tags) > 0 {
			b.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.Tags, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}
