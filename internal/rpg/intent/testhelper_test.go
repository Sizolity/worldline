package intent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sizolity/worldline/internal/rpg/mod"
)

// loadTestPersona parses the minimal intent persona fixture under
// testdata/persona/intent.md for unit tests. Avoids depending on the
// repo-root mod/ directory so tests can run from any cwd.
func loadTestPersona(t *testing.T) *mod.Document {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", "persona", "intent.md"))
	if err != nil {
		t.Fatalf("resolve testdata: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test persona: %v", err)
	}
	doc, err := mod.ParseDocument(string(data))
	if err != nil {
		t.Fatalf("parse test persona: %v", err)
	}
	return doc
}
