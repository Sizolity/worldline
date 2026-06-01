package model

type Value struct {
	Kind   string `json:"kind"`
	Raw    any    `json:"raw"`
	Unit   string `json:"unit,omitempty"`
	Source string `json:"source,omitempty"`
}

const (
	ValueKindString    = "string"
	ValueKindNumber    = "number"
	ValueKindBoolean   = "boolean"
	ValueKindEntityRef = "entity_ref"
	ValueKindObject    = "object"
)
