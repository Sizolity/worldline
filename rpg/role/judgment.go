package role

// Judgment is the purely numeric / categorical output of GM.Judge.
// It MUST NOT contain LLM-generated language — narrative around the ruling
// is produced by the main ReAct loop based on these values.
type Judgment struct {
	Outcome    string // "success" | "failure" | "partial" | "critical"
	Difficulty int    // DC or equivalent

	// Roll is the actual roll result. The Narrator (no rule system) leaves it 0.
	// TODO: when DM/KP wire in real dice, replace this with *int or a paired
	// RollApplied bool — a legitimate roll of 0 must be distinguishable from "N/A".
	Roll int

	Modifiers []string // applied modifiers (e.g. "+2 cover", "-1 wounded")
}
