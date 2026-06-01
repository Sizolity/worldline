package tools

type LookupRulesParams struct {
	Category string   `json:"category"`
	Tags     []string `json:"tags,omitempty"`
}

type UpdateStateParams struct {
	EntityID string `json:"entity_id"`
	Key      string `json:"key"`
	Value    any    `json:"value"`
}

type RollParams struct {
	Sides    int `json:"sides"`
	Count    int `json:"count,omitempty"`
	Modifier int `json:"modifier,omitempty"`
}

type GetEntityStateParams struct {
	EntityID string `json:"entity_id"`
}

type ExploreKnowledgeParams struct {
	TargetID string `json:"target_id"`
	Level    string `json:"level,omitempty"`
	Piece    string `json:"piece,omitempty"`
}

// RandomParams is intentionally empty: random() takes no inputs.
type RandomParams struct{}

// ChanceParams carries the probability for a chance() call.
type ChanceParams struct {
	Probability float64 `json:"probability"`
}

// WeightedChoiceParams is the input for weighted_choice().
type WeightedChoiceParams struct {
	Options []WeightedOption `json:"options"`
}

// WeightedOption is one entry in a weighted_choice option list.
type WeightedOption struct {
	Label  string  `json:"label"`
	Weight float64 `json:"weight"`
}
