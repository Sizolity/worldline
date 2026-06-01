package story

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestStore_LoadMissing_ReturnsNil(t *testing.T) {
	s := NewStore(t.TempDir())
	lines, err := s.Load("nonexistent-world")
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil, got %v", lines)
	}
}

func TestStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	original := []WorldLine{
		{
			ID:           "wl_seal",
			ThreadID:     model.ThreadID("thread_seal"),
			Visibility:   VisibilityHinted,
			CurrentStage: "tremors",
			Drift:        Drift{Scene: 0.02, Day: 0.15, Chapter: 0.35},
			Milestones: []Milestone{
				{
					ID: "m_tower_explored",
					Condition: MilestoneCondition{
						Kind: "thread_tension_gte",
						Args: map[string]any{"thread_id": "thread_seal", "threshold": 0.6},
					},
					Effects: []model.Effect{
						{Kind: model.EffectUpdateThread, TargetID: "thread_seal", Payload: map[string]model.Value{
							"tension": {Kind: model.ValueKindNumber, Raw: 0.8},
						}},
					},
				},
			},
		},
	}

	if err := s.Save("w1", original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !s.Exists("w1") {
		t.Fatalf("Exists false after Save")
	}

	loaded, err := s.Load("w1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(loaded, original) {
		t.Errorf("round-trip mismatch:\noriginal=%+v\nloaded  =%+v", original, loaded)
	}

	// Verify documented on-disk path.
	want := filepath.Join(dir, "w1", "worldlines.json")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected file at %s: %v", want, err)
	}
}

func TestStore_SaveCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "deep", "nested"))
	if err := s.Save("w1", nil); err != nil {
		t.Fatalf("Save with nested dirs: %v", err)
	}
}

func TestWorldLine_Validate(t *testing.T) {
	tests := []struct {
		name    string
		line    WorldLine
		wantErr string
	}{
		{
			name:    "empty id",
			line:    WorldLine{ThreadID: "t"},
			wantErr: "id is required",
		},
		{
			name:    "empty thread id",
			line:    WorldLine{ID: "wl"},
			wantErr: "thread_id is required",
		},
		{
			name:    "invalid visibility",
			line:    WorldLine{ID: "wl", ThreadID: "t", Visibility: Visibility("bogus")},
			wantErr: `visibility "bogus" is invalid`,
		},
		{
			name: "milestone missing id",
			line: WorldLine{ID: "wl", ThreadID: "t", Milestones: []Milestone{
				{Condition: MilestoneCondition{Kind: "thread_tension_gte"}},
			}},
			wantErr: "milestones[0].id is required",
		},
		{
			name: "milestone missing condition kind",
			line: WorldLine{ID: "wl", ThreadID: "t", Milestones: []Milestone{
				{ID: "m1"},
			}},
			wantErr: "milestones[0].condition.kind is required",
		},
		{
			name: "valid empty visibility ok",
			line: WorldLine{ID: "wl", ThreadID: "t"},
		},
		{
			name: "valid full",
			line: WorldLine{
				ID: "wl", ThreadID: "t", Visibility: VisibilityActive,
				Milestones: []Milestone{
					{ID: "m1", Condition: MilestoneCondition{Kind: "thread_tension_gte"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.line.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestVisibility_IsValid(t *testing.T) {
	for _, v := range []Visibility{
		VisibilityHidden, VisibilityHinted, VisibilityKnown, VisibilityActive, VisibilityResolved,
	} {
		if !v.IsValid() {
			t.Errorf("expected %q valid", v)
		}
	}
	if Visibility("bogus").IsValid() {
		t.Errorf("expected bogus invalid")
	}
}
