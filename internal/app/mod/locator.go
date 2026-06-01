// Package mod loads mod-authored markdown content (scenarios + GM styles)
// from the on-disk `mod/` directory and compiles it into engine-side
// model.World values and prompt persona documents.
//
// Discovery order for the `mod/` root:
//  1. WORLDLINE_MOD_DIR environment variable (must point at a `mod/`)
//  2. current working directory's `mod/` subdir
//  3. executable directory's `mod/` subdir (for shipped binaries)
package mod

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnvVar names the environment variable that overrides the mod root.
const EnvVar = "WORLDLINE_MOD_DIR"

// LocateRoot resolves the on-disk `mod/` directory used for loading
// scenarios and styles. It returns the resolved absolute path.
//
// Resolution order:
//  1. $WORLDLINE_MOD_DIR if set and points at an existing directory.
//  2. <cwd>/mod
//  3. <exe-dir>/mod
//
// Returns a descriptive error if none of the candidates exist.
func LocateRoot() (string, error) {
	tried := []string{}

	if envDir := os.Getenv(EnvVar); envDir != "" {
		abs, _ := filepath.Abs(envDir)
		if isDir(abs) {
			return abs, nil
		}
		tried = append(tried, "$"+EnvVar+"="+envDir)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "mod")
		if isDir(candidate) {
			return candidate, nil
		}
		tried = append(tried, candidate)
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidate := filepath.Join(exeDir, "mod")
		if isDir(candidate) {
			return candidate, nil
		}
		tried = append(tried, candidate)
	}

	return "", fmt.Errorf("mod: cannot locate mod/ directory; tried: %v (set %s to override)", tried, EnvVar)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
