package main

import (
	"context"
	"errors"
	"testing"

	"github.com/sizolity/worldline/internal/rpg/intent"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/status"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

// === resolveInput tests (intent-agent wiring replaces buildCombo) ===

// fakeResolver is the testing double for *intent.Resolver. It records
// the most recent Resolve call and returns canned values so tests can
// assert (a) when the agent IS called, (b) what context it received,
// and (c) how callers handle agent errors.
type fakeResolver struct {
	called        int
	lastInput     string
	lastChoices   role.ActionChoices
	lastNarrative string
	returnParams  intent.Params
	returnErr     error
}

func (f *fakeResolver) Resolve(_ context.Context, rawInput string, choices role.ActionChoices, recentNarrative string) (intent.Params, error) {
	f.called++
	f.lastInput = rawInput
	f.lastChoices = choices
	f.lastNarrative = recentNarrative
	return f.returnParams, f.returnErr
}

func sampleChoices() role.ActionChoices {
	return role.ActionChoices{Options: []role.ActionOption{
		{Label: "勘察密室", Type: role.ActionTypeInvestigate},
		{Label: "与守卫攀谈", Type: role.ActionTypeSocial},
		{Label: "潜入后院", Type: role.ActionTypeExplore},
		{Type: role.ActionTypeCustom},
	}}
}

func TestResolveInput_SingleDigitFastPath(t *testing.T) {
	choices := sampleChoices()
	resolver := &fakeResolver{returnParams: intent.Params{ActionText: "should-not-be-used"}}

	got, ok := resolveInput(context.Background(), "2", choices, "prev narrative", resolver, nil)
	if !ok {
		t.Fatalf("expected ok=true for single-digit selector")
	}
	if got != "与守卫攀谈" {
		t.Errorf("expected raw label, got %q", got)
	}
	if resolver.called != 0 {
		t.Errorf("intent agent should not be called on single-digit fast path (got called=%d)", resolver.called)
	}
}

func TestResolveInput_FreeTextCallsResolver(t *testing.T) {
	choices := sampleChoices()
	resolver := &fakeResolver{returnParams: intent.Params{
		ActionText:    "悟空腾云驾雾，俯瞰白虎岭",
		IsDestructive: false,
		Notes:         "通顺润色",
	}}

	got, ok := resolveInput(context.Background(), "腾云驾雾飞过去看看", choices, "上一段叙事", resolver, nil)
	if !ok {
		t.Fatalf("expected ok=true for free-text input")
	}
	if got != "悟空腾云驾雾，俯瞰白虎岭" {
		t.Errorf("expected resolved action_text, got %q", got)
	}
	if resolver.called != 1 {
		t.Fatalf("expected resolver to be called exactly once, got %d", resolver.called)
	}
	if resolver.lastInput != "腾云驾雾飞过去看看" {
		t.Errorf("resolver received raw input %q, want original line", resolver.lastInput)
	}
	if resolver.lastNarrative != "上一段叙事" {
		t.Errorf("resolver did not receive narrative slice, got %q", resolver.lastNarrative)
	}
	if len(resolver.lastChoices.Options) != len(choices.Options) {
		t.Errorf("resolver did not receive choices (len=%d, want %d)",
			len(resolver.lastChoices.Options), len(choices.Options))
	}
}

func TestResolveInput_MultiDigitGoesToResolver(t *testing.T) {
	// "32" / "33" / "231232" / "xxxx1xxx2xxxx" used to be combo-gated;
	// they must now defer entirely to the LLM resolver.
	cases := []string{"32", "33", "231232", "xxxx1xxx2xxxx"}
	for _, line := range cases {
		t.Run(line, func(t *testing.T) {
			choices := sampleChoices()
			resolver := &fakeResolver{returnParams: intent.Params{ActionText: "ok"}}

			got, ok := resolveInput(context.Background(), line, choices, "narr", resolver, nil)
			if !ok {
				t.Fatalf("expected ok=true")
			}
			if got != "ok" {
				t.Errorf("expected resolver action_text, got %q", got)
			}
			if resolver.called != 1 {
				t.Errorf("expected resolver called once, got %d", resolver.called)
			}
			if resolver.lastInput != line {
				t.Errorf("resolver got input %q, want %q (raw line must be passed verbatim)",
					resolver.lastInput, line)
			}
		})
	}
}

func TestResolveInput_FallbackOnAgentError(t *testing.T) {
	choices := sampleChoices()
	resolver := &fakeResolver{returnErr: errors.New("transport blew up")}

	got, ok := resolveInput(context.Background(), "腾云驾雾飞过去", choices, "narr", resolver, nil)
	if !ok {
		t.Fatalf("expected ok=true even on agent error (fallback path)")
	}
	if got != "腾云驾雾飞过去" {
		t.Errorf("expected fallback to raw input on agent error, got %q", got)
	}
}

func TestResolveInput_EmptyInput(t *testing.T) {
	choices := sampleChoices()
	resolver := &fakeResolver{}

	if _, ok := resolveInput(context.Background(), "   ", choices, "narr", resolver, nil); ok {
		t.Errorf("expected ok=false for whitespace-only input")
	}
	if resolver.called != 0 {
		t.Errorf("empty input must not call resolver")
	}
}

func TestResolveInput_OutOfRangeDigitGoesToResolver(t *testing.T) {
	// "9" with only 4 options is out of fast-path range — falls through
	// to the resolver as a free-form line.
	choices := sampleChoices()
	resolver := &fakeResolver{returnParams: intent.Params{ActionText: "归位"}}

	got, ok := resolveInput(context.Background(), "9", choices, "narr", resolver, nil)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "归位" {
		t.Errorf("expected resolver action_text, got %q", got)
	}
	if resolver.called != 1 {
		t.Errorf("expected resolver to be called for out-of-range digit, got %d", resolver.called)
	}
}

func TestResolveInput_NilResolverFallsBackToLine(t *testing.T) {
	// Defensive: callers may pass a nil resolver in tests that only
	// exercise the fast-path. resolveInput must not panic.
	choices := sampleChoices()
	got, ok := resolveInput(context.Background(), "用2根手指掐诀念咒", choices, "narr", nil, nil)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "用2根手指掐诀念咒" {
		t.Errorf("expected raw line on nil resolver, got %q", got)
	}
}

func TestResolveInput_NumericWithNoChoicesReprompts(t *testing.T) {
	// After a suppressed-choices beat (recap / prologue) lastChoices is
	// empty. A bare integer there is a mis-fired selector, not prose: it
	// must NOT reach the intent resolver (no wasted ~2s LLM round-trip) and
	// must re-prompt (ok=false) rather than become a nonsense action.
	for _, line := range []string{"2", "1", "42", "007"} {
		t.Run(line, func(t *testing.T) {
			resolver := &fakeResolver{returnParams: intent.Params{ActionText: "should-not-be-used"}}
			got, ok := resolveInput(context.Background(), line, role.ActionChoices{}, "recap narrative", resolver, nil)
			if ok {
				t.Errorf("expected ok=false (re-prompt) for bare integer with no choices, got ok=true text=%q", got)
			}
			if got != "" {
				t.Errorf("expected empty text on re-prompt, got %q", got)
			}
			if resolver.called != 0 {
				t.Errorf("resolver must NOT be called for numeric input with no choices (got called=%d)", resolver.called)
			}
		})
	}
}

func TestResolveInput_NumericWithNoChoicesRepromptsWithNilResolver(t *testing.T) {
	// Same mis-fire guard must hold even when no resolver is wired (the
	// branch sits before the resolver==nil fallthrough): a bare digit with
	// no options re-prompts rather than being passed through as a literal
	// "2" action.
	got, ok := resolveInput(context.Background(), "2", role.ActionChoices{}, "narr", nil, nil)
	if ok {
		t.Errorf("expected ok=false for bare integer with no choices and nil resolver, got ok=true text=%q", got)
	}
	if got != "" {
		t.Errorf("expected empty text, got %q", got)
	}
}

func TestResolveInput_FreeTextWithNoChoicesStillResolves(t *testing.T) {
	// Only bare integers are treated as mis-fired selectors. Non-numeric
	// free text with no choices is a legitimate action description and must
	// still reach the resolver, exactly as before.
	resolver := &fakeResolver{returnParams: intent.Params{ActionText: "推门而入"}}
	got, ok := resolveInput(context.Background(), "推门进去看看", role.ActionChoices{}, "narr", resolver, nil)
	if !ok {
		t.Fatalf("expected ok=true for free text with no choices")
	}
	if got != "推门而入" {
		t.Errorf("expected resolved action_text, got %q", got)
	}
	if resolver.called != 1 {
		t.Errorf("expected resolver called once for free text, got %d", resolver.called)
	}
}

// TestOptionProgressHint locks the gating of the inline_choices progress
// cue: shown for normal beats (options are coming after the prose ends),
// suppressed for recap / prologue beats (no options — the hint would
// mislead). See streamBeat / BeatInput.SuppressChoices.
func TestOptionProgressHint(t *testing.T) {
	if got := optionProgressHint(false); got == "" {
		t.Error("expected a non-empty progress hint when choices are NOT suppressed")
	}
	if got := optionProgressHint(true); got != "" {
		t.Errorf("expected empty hint when choices are suppressed (recap/prologue), got %q", got)
	}
}

// Compile-time: ensure *intent.Resolver actually satisfies the local
// interface so the production wiring keeps working.
var _ intentResolverIface = (*intent.Resolver)(nil)

// === Sub 5: Lorekeeper status footer + cmdStatus world-knowledge ===

func TestSummarizeLoreReport_Empty(t *testing.T) {
	var r ingest.CompileReport
	if got := summarizeLoreReport(r); got != "" {
		t.Fatalf("expected empty string for zero report, got %q", got)
	}
}

func TestSummarizeLoreReport_Basic(t *testing.T) {
	r := ingest.CompileReport{Inserted: 2, Skipped: 1}
	want := "沉淀: 插入=2 跳过=1 拒绝=0 过滤=0"
	if got := summarizeLoreReport(r); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestSummarizeLoreReport_WithNotes(t *testing.T) {
	r := ingest.CompileReport{
		Inserted: 1,
		Notes:    []string{"validate-warn: foo", "compile-reject: bar"},
	}
	want := "沉淀: 插入=1 跳过=0 拒绝=0 过滤=0 (含 2 条提示)"
	if got := summarizeLoreReport(r); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestTopNPCsByMemoryCount_Empty(t *testing.T) {
	got := status.TopNPCsByMemoryCount(worldmodel.World{}, 5)
	if len(got) != 0 {
		t.Fatalf("expected empty slice for empty world, got %v", got)
	}
}

func TestTopNPCsByMemoryCount_OrderAndCap(t *testing.T) {
	// Seven NPCs with memory counts {3, 1, 5, 5, 2, 4, 0}.
	// The zero-count NPC must be excluded; the rest sorted by count desc
	// then ID asc; truncated to top 5.
	specs := []struct {
		id    string
		count int
	}{
		{"npc-a", 3},
		{"npc-b", 1},
		{"npc-c", 5},
		{"npc-d", 5},
		{"npc-e", 2},
		{"npc-f", 4},
		{"npc-g", 0},
	}
	entities := map[worldmodel.EntityID]worldmodel.Entity{}
	var mem []worldmodel.MemoryRecord
	for _, s := range specs {
		eid := worldmodel.EntityID(s.id)
		entities[eid] = worldmodel.Entity{
			ID:   eid,
			Type: "character",
			Name: "name-" + s.id,
		}
		for i := 0; i < s.count; i++ {
			mem = append(mem, worldmodel.MemoryRecord{
				Owner: worldmodel.MemoryOwner{
					Kind: worldmodel.MemoryOwnerKindCharacter,
					ID:   s.id,
				},
			})
		}
	}
	world := worldmodel.World{Entities: entities, Memories: mem}

	got := status.TopNPCsByMemoryCount(world, 5)
	if len(got) != 5 {
		t.Fatalf("expected top 5, got %d (%+v)", len(got), got)
	}

	wantOrder := []struct {
		id    string
		count int
	}{
		{"npc-c", 5},
		{"npc-d", 5},
		{"npc-f", 4},
		{"npc-a", 3},
		{"npc-e", 2},
	}
	for i, want := range wantOrder {
		if string(got[i].ID) != want.id || got[i].Count != want.count {
			t.Errorf("rank %d: want id=%s count=%d, got id=%s count=%d",
				i, want.id, want.count, got[i].ID, got[i].Count)
		}
	}

	for _, s := range got {
		if string(s.ID) == "npc-g" {
			t.Errorf("zero-count NPC npc-g must not be included")
		}
		if string(s.ID) == "npc-b" {
			t.Errorf("npc-b (count=1) is outside top 5 but appeared")
		}
	}
}

func TestTopNPCsByMemoryCount_OnlyCharacterOwned(t *testing.T) {
	// Memories with non-character owners must NOT contribute to any NPC
	// counter. Only character-owned memories count.
	mem := []worldmodel.MemoryRecord{
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindWorld}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindWorld}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindNarrator, ID: "narr-1"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindFaction, ID: "fac-1"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindCharacter, ID: "npc-x"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindCharacter, ID: "npc-x"}},
	}
	entities := map[worldmodel.EntityID]worldmodel.Entity{
		"npc-x": {ID: "npc-x", Type: "character", Name: "X"},
	}
	world := worldmodel.World{Entities: entities, Memories: mem}

	got := status.TopNPCsByMemoryCount(world, 5)
	if len(got) != 1 {
		t.Fatalf("expected exactly one NPC (only character owner), got %d (%+v)", len(got), got)
	}
	if got[0].Count != 2 {
		t.Errorf("expected count=2 (two character-kind memories), got %d", got[0].Count)
	}
	if string(got[0].ID) != "npc-x" {
		t.Errorf("expected ID=npc-x, got %s", got[0].ID)
	}
	if got[0].Name != "X" {
		t.Errorf("expected Name=X, got %s", got[0].Name)
	}
}
