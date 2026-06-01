package template

import (
	"testing"
)

func TestTemplateNamesMatchKeys(t *testing.T) {
	t.Parallel()
	names := TemplateNames()
	if len(names) != len(Templates) {
		t.Fatalf("TemplateNames() = %d, Templates = %d", len(names), len(Templates))
	}
	for _, name := range names {
		if _, ok := Templates[name]; !ok {
			t.Errorf("TemplateNames includes %q but Templates does not", name)
		}
	}
}

func TestTemplateNamesSorted(t *testing.T) {
	t.Parallel()
	names := TemplateNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("TemplateNames not sorted: %v", names)
			break
		}
	}
}

func TestAllTemplatesProduceValidWorlds(t *testing.T) {
	t.Parallel()
	for name, tmpl := range Templates {
		w, err := ApplyTemplate(tmpl, "test_"+name, "Test "+name)
		if err != nil {
			t.Errorf("template %q: ApplyTemplate error: %v", name, err)
			continue
		}
		if len(w.Entities) == 0 {
			t.Errorf("template %q: no entities", name)
		}
		if w.Canon.Premise == "" {
			t.Errorf("template %q: missing premise", name)
		}
		if len(w.Canon.Genre) == 0 {
			t.Errorf("template %q: missing genre", name)
		}
		if len(w.Threads) == 0 {
			t.Errorf("template %q: no threads", name)
		}
	}
}

func TestApplyTemplateOverridesIDAndName(t *testing.T) {
	t.Parallel()
	tmpl := Templates["fantasy"]
	w, err := ApplyTemplate(tmpl, "custom_id", "Custom Name")
	if err != nil {
		t.Fatal(err)
	}
	if string(w.ID) != "custom_id" {
		t.Errorf("ID = %q", w.ID)
	}
	if w.Name != "Custom Name" {
		t.Errorf("Name = %q", w.Name)
	}
	if w.Description == "" {
		t.Error("Description should be set from template")
	}
}

func TestFantasyTemplateContent(t *testing.T) {
	t.Parallel()
	tmpl := Templates["fantasy"]
	w, _ := ApplyTemplate(tmpl, "f1", "Fantasy")
	if len(w.Entities) != 4 {
		t.Errorf("entities = %d, want 4", len(w.Entities))
	}
	if len(w.Relations) != 1 {
		t.Errorf("relations = %d, want 1", len(w.Relations))
	}
	if len(w.Facts) != 2 {
		t.Errorf("facts = %d, want 2", len(w.Facts))
	}
}

func TestScifiTemplateContent(t *testing.T) {
	t.Parallel()
	tmpl := Templates["scifi"]
	w, _ := ApplyTemplate(tmpl, "s1", "Scifi")
	if len(w.Entities) != 4 {
		t.Errorf("entities = %d, want 4", len(w.Entities))
	}
	if _, ok := w.Entities["char_captain"]; !ok {
		t.Error("missing char_captain")
	}
}
