package model

type Relation struct {
	ID       RelationID `json:"id"`
	Type     string     `json:"type"`
	SourceID EntityID   `json:"source_id"`
	TargetID EntityID   `json:"target_id"`
}

type Fact struct {
	ID        FactID   `json:"id"`
	SubjectID EntityID `json:"subject_id"`
	Predicate string   `json:"predicate"`
	Value     Value    `json:"value"`
}
