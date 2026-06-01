package narrator

import (
	"path/filepath"
	"testing"

	"github.com/sizolity/worldline/internal/rpg/mod"
)

// loadTestStyle loads the minimal persona fixture under testdata/ for
// unit tests. Avoids depending on the repo-root mod/ directory so tests
// can run from any cwd. The on-disk layout mirrors a real mod root:
// internal/rpg/narrator/testdata/styles/style/persona/*.md.
func loadTestStyle(t *testing.T) *mod.Style {
	t.Helper()
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolve testdata: %v", err)
	}
	st, err := mod.LoadStyle(root, "style")
	if err != nil {
		t.Fatalf("load test style: %v", err)
	}
	return st
}
