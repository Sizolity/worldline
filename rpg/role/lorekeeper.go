package role

import (
	"github.com/sizolity/worldline/world/ingest"
)

// Lorekeeper is the role responsible for distilling beat narrative into a
// structured ingest.Draft (the "知识沉淀者" / keeper of world lore). It is
// deliberately separate from GM: the GM runs the game beat-by-beat, while
// the Lorekeeper records what just happened so the world's persistent lore
// can grow over time.
//
// The method set is exactly ingest.Parser, so any narrator that already
// implements ingest.Parser automatically satisfies Lorekeeper, and a
// Lorekeeper can be passed wherever an ingest.Parser is expected (no
// adapter needed).
//
// Failure semantics: a Lorekeeper call is a graceful side-channel. Callers
// must log and continue on error; a Lorekeeper failure must never abort the
// beat that produced the narrative.
type Lorekeeper interface {
	ingest.Parser
}
