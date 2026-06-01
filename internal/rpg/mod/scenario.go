package mod

// Scenario is the in-memory representation of one mod/scenarios/<id>/
// directory after loading.
//
// All field types are mod-local: the compile step turns this into the
// engine's worldmodel.World. Authors never see worldmodel types.
type Scenario struct {
	// ID is the scenario directory name (e.g. "xiyou-changan").
	ID string

	// World carries world.md's content + frontmatter.
	World *WorldDoc

	// Rules is one entry per H2 in rules.md, in source order.
	Rules []RuleEntry

	// Threads is one entry per H2 in threads.md, in source order.
	Threads []ThreadEntry

	// Characters lists each characters/*.md in lexicographic filename order.
	Characters []CharacterDoc

	// Locations lists each locations/*.md in lexicographic filename order.
	Locations []LocationDoc

	// Events lists each events/*.md in lexicographic filename order.
	Events []EventDoc

	// PlayerIndex is the index into Characters of the lone `role: player`
	// entry. -1 if loading rejected (no player) — Load returns an error in
	// that case so callers can assume PlayerIndex >= 0.
	PlayerIndex int
}

// WorldDoc is world.md's parsed form. v1 only recognizes the
// `start_location` frontmatter field; everything else is body prose used
// for the world Description and (best-effort) Canon Genre / Tone.
type WorldDoc struct {
	Title         string   // H1
	Lead          string   // body between H1 and first H2
	Genre         []string // parsed from "## 类型" H2 if present (best-effort)
	Tone          []string // parsed from "## 基调" H2 if present (best-effort)
	StartLocation string   // frontmatter `start_location`
}

// RuleEntry is one H2 block from rules.md.
type RuleEntry struct {
	Title string
	Body  string
}

// ThreadEntry is one H2 block from threads.md.
type ThreadEntry struct {
	Title string
	Body  string
}

// CharacterDoc is one characters/<slug>.md after parsing.
type CharacterDoc struct {
	FileSlug string // filename without ".md"
	Role     string // frontmatter `role`; "" defaults to npc on consumption
	Title    string // H1
	Body     string // full body prose (lead + each H2 reflowed)
}

// LocationDoc is one locations/<slug>.md.
type LocationDoc struct {
	FileSlug string
	Title    string
	Body     string
}

// EventDoc is one events/<slug>.md.
type EventDoc struct {
	FileSlug string
	Title    string
	Body     string
}

// Role constants mirror the README's `role:` values.
const (
	RolePlayer = "player"
	RoleNPC    = "npc"
)
