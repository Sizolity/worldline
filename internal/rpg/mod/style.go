package mod

// Style is the in-memory representation of one mod/styles/<id>/ directory.
//
// A style packages the GM-side "voice" — narrator, lorekeeper, action
// suggester, and intent-parser personas plus the two opening scripts
// (prologue, recap). Persona docs are kept as parsed Documents because
// the prompt renderer needs to walk reserved H2 placeholders; scenes
// are just trimmed body text since they are inlined into the player-
// facing first beat.
type Style struct {
	ID                string
	NarratorPersona   *Document
	LorekeeperPersona *Document
	SuggesterPersona  *Document
	IntentPersona     *Document
	ProloguePrompt    string // scene/prologue.md body (raw prose)
	RecapPrompt       string // scene/recap.md body (raw prose)
}
