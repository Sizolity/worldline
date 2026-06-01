package seed

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sizolity/worldline/internal/app/mod"
	"github.com/sizolity/worldline/world/store"
)

func modRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "mod"))
	if err != nil {
		t.Fatalf("resolve mod root: %v", err)
	}
	return abs
}

// modEnv sets WORLDLINE_MOD_DIR so SeedFromMod's LocateRoot resolves the
// repository's mod/ regardless of test cwd. Restored on cleanup.
func modEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WORLDLINE_MOD_DIR", modRoot(t))
}

func TestSeedFromMod_XiyouChangan(t *testing.T) {
	modEnv(t)
	dir := t.TempDir()

	res, err := SeedFromMod(context.Background(), SeedRequest{
		Workspace:  dir,
		WorldID:    "xiyou-changan",
		ScenarioID: "xiyou-changan",
		StyleID:    "default",
	})
	if err != nil {
		t.Fatalf("SeedFromMod: %v", err)
	}
	if res.Scenario == nil || res.Style == nil {
		t.Fatal("nil scenario/style in result")
	}
	if res.WorldlinesCount != 1 {
		t.Errorf("worldlines = %d, want 1", res.WorldlinesCount)
	}
	if res.PlayConfig.CharacterID != "hero-sun_wukong" {
		t.Errorf("play.character_id = %q", res.PlayConfig.CharacterID)
	}
	if res.PlayConfig.StyleID != "default" {
		t.Errorf("play.style_id = %q", res.PlayConfig.StyleID)
	}

	// On-disk artifacts.
	for _, rel := range []string{
		"worlds/xiyou-changan/world.json",
		"worlds/xiyou-changan/threads.json",
		"worlds/xiyou-changan/worldlines.json",
		"worlds/xiyou-changan/play.json",
	} {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", rel, err)
		}
	}

	// Snapshot must round-trip through FileStore.
	fs := store.NewFileStore(dir)
	world, err := fs.LoadSnapshot(context.Background(), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if _, ok := world.Entities["hero-sun_wukong"]; !ok {
		t.Errorf("snapshot missing hero-sun_wukong")
	}

	// Sidecar must also load standalone.
	side, err := mod.LoadPlayConfig(dir, "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadPlayConfig: %v", err)
	}
	if side == nil || side.ScenarioID != "xiyou-changan" {
		t.Errorf("sidecar = %+v", side)
	}
}

func TestSeedFromMod_RefusesOverwriteWithoutForce(t *testing.T) {
	modEnv(t)
	dir := t.TempDir()
	req := SeedRequest{
		Workspace:  dir,
		WorldID:    "xiyou-changan",
		ScenarioID: "xiyou-changan",
		StyleID:    "default",
	}
	if _, err := SeedFromMod(context.Background(), req); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if _, err := SeedFromMod(context.Background(), req); err == nil {
		t.Fatal("expected overwrite refusal on second seed without --force")
	}
	req.Force = true
	if _, err := SeedFromMod(context.Background(), req); err != nil {
		t.Fatalf("forced re-seed: %v", err)
	}
}

func TestSeed_BackwardCompatibleDefault(t *testing.T) {
	modEnv(t)
	dir := t.TempDir()
	if err := Seed(dir, "xiyou-changan", false); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "worlds", "xiyou-changan", "world.json")); err != nil {
		t.Fatalf("world.json missing after Seed: %v", err)
	}
}
