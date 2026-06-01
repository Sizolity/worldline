package narrator

import (
	"github.com/sizolity/worldline/rpg/role"
	"github.com/sizolity/worldline/rpg/template"
)

// AvailableTemplates returns the legacy world templates this narrator
// knows how to seed. It re-exports the fantasy/mystery/scifi/modern
// templates from rpg/template/ in the order produced by
// template.TemplateNames() so CLI output stays deterministic.
//
// Deprecated: v1 has migrated the default seed path to
// internal/app/mod (mod/scenarios/<id>/). These legacy templates are
// retained only so existing tests stay green; they are not wired into
// the CLI seed entry point and will be removed in v1.5+. Call
// internal/app/mod.LoadScenario directly for new code.
func AvailableTemplates() []role.WorldTemplate {
	names := template.TemplateNames()
	out := make([]role.WorldTemplate, 0, len(names))
	for _, name := range names {
		out = append(out, template.Templates[name])
	}
	return out
}
