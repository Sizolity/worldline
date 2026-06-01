package mod

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPlayConfig_MissingFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadPlayConfig(dir, "world-x")
	if err != nil {
		t.Fatalf("LoadPlayConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil cfg for missing file, got %+v", cfg)
	}
}

func TestSaveAndLoadPlayConfig(t *testing.T) {
	dir := t.TempDir()
	want := PlayConfig{
		ScenarioID:  "xiyou-changan",
		StyleID:     "default",
		Locale:      "zh-CN",
		PlayerID:    "player-1",
		PlayerName:  "孙悟空",
		CharacterID: "hero-sun_wukong",
	}
	if err := SavePlayConfig(dir, "world-x", want); err != nil {
		t.Fatalf("SavePlayConfig: %v", err)
	}

	path := PlayConfigPath(dir, "world-x")
	if path != filepath.Join(dir, "worlds", "world-x", "play.json") {
		t.Errorf("unexpected sidecar path %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sidecar not on disk: %v", err)
	}

	got, err := LoadPlayConfig(dir, "world-x")
	if err != nil {
		t.Fatalf("LoadPlayConfig: %v", err)
	}
	if got == nil {
		t.Fatal("got nil cfg after Save")
	}
	if *got != want {
		t.Errorf("round-trip mismatch: got %+v want %+v", *got, want)
	}
}
