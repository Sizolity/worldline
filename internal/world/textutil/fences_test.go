package textutil

import "testing"

func TestStripMarkdownFences(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain json", `[{"id":"x"}]`, `[{"id":"x"}]`},
		{"json fence", "```json\n[{\"id\":\"x\"}]\n```", `[{"id":"x"}]`},
		{"bare fence", "```\n[{\"id\":\"x\"}]\n```", `[{"id":"x"}]`},
		{"fence with trailing newline", "```json\n[{\"id\":\"x\"}]\n```\n", `[{"id":"x"}]`},
		{"whitespace around", "  \n```json\n[{\"id\":\"x\"}]\n```\n  ", `[{"id":"x"}]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripMarkdownFences(tc.input)
			if got != tc.want {
				t.Errorf("StripMarkdownFences(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
