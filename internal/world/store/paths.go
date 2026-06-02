package store

import "path/filepath"

// worldsDirName is the single source of truth for the on-disk directory
// that holds every persisted world under a workspace. It used to be an
// inline "worlds" string literal scattered across seed/session/status; all
// of those now route through WorldsDir / WorldDir so the layout is owned in
// one place (the store package, which actually writes the files).
const worldsDirName = "worlds"

// WorldsDir returns the directory under workspace that contains all worlds
// (workspace/worlds). It is the canonical join used by FileStore and by the
// sidecar stores (story, fog) that colocate their files under the same tree.
func WorldsDir(workspace string) string {
	return filepath.Join(workspace, worldsDirName)
}

// WorldDir returns the directory for a single world
// (workspace/worlds/<worldID>). Callers that need a specific file (world.json,
// play.json, …) join the filename onto this result.
func WorldDir(workspace, worldID string) string {
	return filepath.Join(WorldsDir(workspace), worldID)
}
