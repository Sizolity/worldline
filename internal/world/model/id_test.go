package model

import "testing"

func TestValidateIDAcceptsStoreSafeIDs(t *testing.T) {
	for _, id := range []string{"world1", "world_1", "world-1", "A123"} {
		if err := ValidateID(id); err != nil {
			t.Fatalf("ValidateID(%q) returned error: %v", id, err)
		}
	}
}

func TestValidateIDRejectsUnsafeIDs(t *testing.T) {
	for _, id := range []string{"", "../world", "world/id", " world", "world id", ".hidden"} {
		if err := ValidateID(id); err == nil {
			t.Fatalf("ValidateID(%q) returned nil error", id)
		}
	}
}
