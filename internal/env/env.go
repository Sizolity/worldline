// Package env provides lightweight dotenv file loading.
package env

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadIfNeeded loads environment variables from .env files in the
// workspace directory and up to 6 ancestor directories from cwd.
// Already-set variables are never overwritten.
func LoadIfNeeded(workspace string) {
	cands := []string{filepath.Join(workspace, ".env")}
	cwd, _ := os.Getwd()
	d := cwd
	for i := 0; i < 6 && d != "/" && d != ""; i++ {
		cands = append(cands, filepath.Join(d, ".env"))
		d = filepath.Dir(d)
	}
	for _, p := range cands {
		LoadFile(p)
	}
}

// LoadFile reads a dotenv-format file and sets any variables that
// are not already present in the environment. Returns true if the file
// was opened and fully scanned without error.
func LoadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err() == nil
}
