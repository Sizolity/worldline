package mod

import "github.com/sizolity/worldline/internal/markdown"

// The generic markdown/frontmatter parser was extracted to the neutral leaf
// package internal/markdown. These aliases keep the long-standing mod.*
// surface (mod.Document, mod.Section, mod.ParseDocument) working for the many
// callers inside and outside this package, while the parser itself no longer
// lives in this RPG-domain package.
type (
	// Document is an alias for markdown.Document. Because it is a type alias
	// (not a defined type), all methods declared in the markdown package —
	// SectionByTitle, FrontmatterString — are available on mod.Document too.
	Document = markdown.Document
	// Section is an alias for markdown.Section.
	Section = markdown.Section
)

// ParseDocument re-exports markdown.ParseDocument so existing mod.ParseDocument
// callers keep working unchanged.
func ParseDocument(src string) (*Document, error) {
	return markdown.ParseDocument(src)
}
