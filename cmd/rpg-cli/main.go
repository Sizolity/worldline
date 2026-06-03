// rpg-cli is a manual smoke-test CLI for the rpg package. It wraps a
// DeepSeek-backed Narrator GM and a session.Session into three subcommands:
//
//	rpg-cli seed --workspace DIR --scenario ID --style ID [--world-id ID] [--force]
//	    Loads mod/scenarios/<scenario>/ + mod/styles/<style>/ and writes
//	    DIR/worlds/<world-id>/{world.json, worldlines.json, play.json}.
//	    Refuses to overwrite an existing snapshot unless --force is passed.
//	    --scenario and --style are required (the engine ships no defaults).
//	    --world-id falls back to --scenario when omitted.
//
//	rpg-cli play --workspace DIR --world-id ID [--style ID] [--no-story] [--prologue]
//	    Starts a REPL. The prologue (opening "醒木一拍" scene-setting beat)
//	    runs automatically only on the very first play (Clock.Sequence==1).
//	    Pass --prologue to force-replay it on a resumed game. The chosen
//	    style is persisted into play.json so subsequent plays do not need
//	    to re-pass --style.
//
//	rpg-cli status --workspace DIR --world-id ID
//	    Prints current Clock, thread tensions, last few EventLog entries.
//
// The DeepSeek API key is loaded from the DEEPSEEK_API_KEY env var or, if
// missing, from a .env file in (in order) the workspace, the current dir,
// or any ancestor up to /. Nothing is printed about the key itself.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sizolity/worldline/internal/rpg/app"
	"github.com/sizolity/worldline/internal/rpg/intent"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/seed"
	"github.com/sizolity/worldline/internal/rpg/session"
	"github.com/sizolity/worldline/internal/rpg/status"
	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/view"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch os.Args[1] {
	case "seed":
		os.Exit(cmdSeed(ctx, os.Args[2:]))
	case "play":
		os.Exit(cmdPlay(ctx, os.Args[2:]))
	case "status":
		os.Exit(cmdStatus(ctx, os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `rpg-cli — manual RPG smoke test

Usage:
  rpg-cli seed   --workspace DIR --scenario ID --style ID [--world-id ID] [--force]
  rpg-cli play   --workspace DIR --world-id ID [--style ID] [--no-story] [--prologue] [--max-step N]
  rpg-cli status --workspace DIR --world-id ID

Mod content lives under ./mod/. Set WORLDLINE_MOD_DIR to override.
The engine ships no default scenario/style — both are required for seed.

Environment:
  DEEPSEEK_API_KEY        Required for 'play'.
  WORLDLINE_MOD_DIR       Override mod/ root location.
  WORLDLINE_DEBUG_DICE=1     Trace internal-randomness tool calls to stderr.
  WORLDLINE_DEBUG_INTENT=1   Trace REPL intent-parser results to stderr.
  WORLDLINE_DEBUG_TIMING=1   Trace per-stage beat latencies as a single
                             [timing] line after each beat. Healthy:
                             lore_join + prep + beat_ttfc + beat +
                             inline_choices + effects + save. The
                             presence of "suggest=" in a trace means
                             the inline set_choices contract degraded
                             and the legacy SuggestActions fallback
                             fired — a signal that the prompt or model
                             output drifted; the beat itself is fine.
  WORLDLINE_DEBUG_LORE=1     Trace background lorekeeper task wall-time
                             breakdown as a single [lore] line per beat
                             (parse + wait + validate + compile + save
                             + draft_entities/inserted/notes). parse is
                             the LLM call; the other phases are
                             pure-Go and disk I/O. Combine with
                             WORLDLINE_DEBUG_TIMING to correlate this
                             with the next beat's lore_join.`)
}

// === seed ===

func cmdSeed(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "", "world id (defaults to --scenario when empty)")
	scenarioID := fs.String("scenario", "", "mod scenario id (under mod/scenarios/<id>/) — required")
	styleID := fs.String("style", "", "mod style id (under mod/styles/<id>/) — required")
	force := fs.Bool("force", false, "overwrite an existing world (destroys progress)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" {
		fmt.Fprintln(os.Stderr, "seed requires --workspace")
		return 2
	}
	if *scenarioID == "" {
		fmt.Fprintln(os.Stderr, "seed requires --scenario (engine ships no default; pick one from mod/scenarios/)")
		return 2
	}
	if *styleID == "" {
		fmt.Fprintln(os.Stderr, "seed requires --style (engine ships no default; pick one from mod/styles/)")
		return 2
	}
	if *worldID == "" {
		*worldID = *scenarioID
	}

	worldJSON := filepath.Join(*workspace, "worlds", *worldID, "world.json")
	if _, err := os.Stat(worldJSON); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "拒绝覆盖：%s 已存在。\n", worldJSON)
		fmt.Fprintf(os.Stderr, "  - 继续上次的进度: rpg-cli play --workspace %s --world-id %s\n", *workspace, *worldID)
		fmt.Fprintf(os.Stderr, "  - 查看当前进度  : rpg-cli status --workspace %s --world-id %s\n", *workspace, *worldID)
		fmt.Fprintf(os.Stderr, "  - 强制重新铺设  : 加 --force（会丢失进度）\n")
		return 1
	}

	result, err := seed.SeedFromMod(ctx, seed.SeedRequest{
		Workspace:  *workspace,
		WorldID:    *worldID,
		ScenarioID: *scenarioID,
		StyleID:    *styleID,
		Force:      *force,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		return 1
	}

	worldsDir := filepath.Join(*workspace, "worlds")
	fmt.Printf("已铺设世界于 %s\n", filepath.Join(worldsDir, *worldID))
	fmt.Printf("  scenario          — %s\n", result.Scenario.ID)
	fmt.Printf("  style             — %s\n", result.Style.ID)
	if result.Scenario.World != nil {
		fmt.Printf("  snapshot.json     — %s\n", result.Scenario.World.Title)
	}
	fmt.Printf("  worldlines.json   — %d 条隐线\n", result.WorldlinesCount)
	fmt.Printf("  play.json         — character=%s player_id=%s\n", result.PlayConfig.CharacterID, result.PlayConfig.PlayerID)
	if result.OverwroteExisting {
		fmt.Println("  (--force：已覆盖原有进度)")
	}
	fmt.Printf("\n下一步: rpg-cli play --workspace %s --world-id %s\n", *workspace, *worldID)
	return 0
}

// cmdStatus prints the current world state without entering the REPL —
// useful for "where did I leave off?" checks before resuming.
func cmdStatus(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "", "world id")
	tail := fs.Int("tail", 5, "show last N EventLog entries")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(os.Stderr, "status requires --workspace and --world-id")
		return 2
	}
	report, err := status.Build(ctx, *workspace, *worldID, *tail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load world: %v\n", err)
		return 1
	}
	renderStatusReport(report, *tail)
	return 0
}

func renderStatusReport(r *status.Report, tail int) {
	fmt.Printf("=== %s (%s) ===\n", r.WorldName, r.WorldID)
	fmt.Printf("回合 (Clock.Sequence): %d   时钟类型: %s\n",
		r.ClockSequence, r.ClockKind)
	fmt.Println()
	fmt.Println("Threads:")
	for _, t := range r.Threads {
		fmt.Printf("  - %-22s status=%-6s tension=%.2f — %s\n",
			t.ID, t.Status, t.Tension, t.Title)
	}

	if len(r.WorldLines) > 0 {
		fmt.Println("\nWorldLines:")
		for _, l := range r.WorldLines {
			fmt.Printf("  - %s → %s  visibility=%s  milestones=%d/%d triggered\n",
				l.ID, l.ThreadID, l.Visibility, l.MilestonesTrigged, l.MilestonesTotal)
		}
	}

	if len(r.RecentEvents) > 0 {
		fmt.Printf("\n最近 %d 条事件:\n", tail)
		for _, e := range r.RecentEvents {
			desc := view.TruncateRunes(strings.ReplaceAll(e.Description, "\n", " "), 60)
			fmt.Printf("  [%s/%s] %s\n", e.Type, e.Source, desc)
		}
	}

	if r.EntityCounts.Total == 0 && r.MemoryCounts.Total == 0 {
		return
	}

	fmt.Println("\n世界知识:")
	fmt.Printf("  实体: character=%d location=%d item=%d",
		r.EntityCounts.Character, r.EntityCounts.Location, r.EntityCounts.Item)
	if r.EntityCounts.Other > 0 {
		fmt.Printf(" (其他=%d)", r.EntityCounts.Other)
	}
	fmt.Printf(" 总计=%d\n", r.EntityCounts.Total)

	fmt.Printf("  记忆: world=%d character=%d",
		r.MemoryCounts.World, r.MemoryCounts.Character)
	if r.MemoryCounts.Other > 0 {
		fmt.Printf(" (其他=%d)", r.MemoryCounts.Other)
	}
	fmt.Printf(" 总计=%d\n", r.MemoryCounts.Total)

	if len(r.NPCMemoryTop) == 0 {
		return
	}
	fmt.Println("\nNPC 记忆Top（按条数）:")
	for _, s := range r.NPCMemoryTop {
		fmt.Printf("  - %s (%s): %d 条\n", s.Name, s.ID, s.Count)
	}
}

// === play ===

func cmdPlay(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "", "world id")
	styleFlag := fs.String("style", "", "mod style id (overrides sidecar)")
	noStory := fs.Bool("no-story", false, "disable WorldLine scheduler")
	maxStep := fs.Int("max-step", 8, "max tool-calling iterations per beat")
	forcePrologue := fs.Bool("prologue", false, "replay the opening prologue even on a resumed game")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(os.Stderr, "play requires --workspace and --world-id")
		return 2
	}

	rt, err := app.NewPlayRuntime(ctx, app.PlayOptions{
		Workspace: *workspace,
		WorldID:   *worldID,
		StyleID:   *styleFlag,
		NoStory:   *noStory,
		MaxStep:   *maxStep,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// The last beat's lorekeeper extraction runs in the background; flush it
	// before exiting so that beat's knowledge enrichment is durably saved.
	// (The world state itself is always saved synchronously per beat.)
	defer func() {
		if lore := rt.Session.AwaitPendingLore(); lore.Pending {
			if lore.SaveErr != nil {
				fmt.Fprintf(os.Stderr, "[lore] 末拍沉淀存档失败: %v（世界状态已同步保存，仅丢失本拍知识沉淀）\n", lore.SaveErr)
			} else if lore.LoreErr != nil && os.Getenv("WORLDLINE_DEBUG_LORE") != "" {
				fmt.Fprintf(os.Stderr, "[lore] 末拍提取失败: %v\n", lore.LoreErr)
			}
		}
	}()

	fmt.Printf("=== %s ===\n", *worldID)
	fmt.Printf("scenario=%s style=%s 世界线调度: %s   状态: %s\n",
		rt.PlayCfg.ScenarioID, rt.PlayCfg.StyleID, boolWord(!*noStory), modeWord(rt.Resuming))
	fmt.Println("输入提示：")
	fmt.Println("  - 自由文本：直接描述行动")
	fmt.Println("  - 单数字   N    ：选第 N 个推荐选项")
	fmt.Println("  - 多数字 / 混合 ：交给意图解释器自然组合（例 32、33、用2根手指掐诀念咒）")
	fmt.Println("  - q 或 Ctrl-D    ：退出")
	fmt.Println()

	stdin := bufio.NewReader(os.Stdin)
	var lastChoices role.ActionChoices
	var lastNarrative string

	switch {
	case rt.Resuming && !*forcePrologue:
		recap := "【续接场景】\n" + rt.RecapPrompt
		fmt.Println("[说书人接续场景…]")
		result := streamBeat(ctx, rt.Session, session.BeatInput{
			WorldID:      *worldID,
			Action:       role.PlayerAction{PlayerID: rt.Player.ID, Content: recap},
			RecentEvents: 8,
			// Recap scenes end on "a frozen moment for the player to
			// react to" by design — see mod/styles/shuoshu/scene/recap.md
			// ("末尾不列选项"). Suppressing choices here avoids the
			// ~2s SuggestActions fallback that would otherwise fire
			// because the narrator legitimately omits set_choices.
			SuppressChoices: true,
			// A recap just narrates context that is already in the system
			// prompt — it needs no tools. An empty toolset makes the beat
			// stream in one model round-trip instead of burning ~17s on
			// pre-narrative tool calls. See BeatInput.MinimalTools.
			MinimalTools: true,
		}, time.Time{})
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "接续失败：%v（仍可手动输入行动继续）\n", result.Err)
		} else {
			lastChoices = result.Choices
			lastNarrative = result.Narrative
		}
	default:
		prologue := "【开场】\n" + rt.ProloguePrompt
		fmt.Println("[说书人推演开场…]")
		result := streamBeat(ctx, rt.Session, session.BeatInput{
			WorldID:      *worldID,
			Action:       role.PlayerAction{PlayerID: rt.Player.ID, Content: prologue},
			RecentEvents: 8,
			// Prologue scenes close on "a scene tableau for the player
			// to react to" by design — see
			// mod/styles/shuoshu/scene/prologue.md ("结尾不要列举选项").
			// Suppressing choices here avoids the ~2s SuggestActions
			// fallback that would otherwise fire because the narrator
			// legitimately omits set_choices.
			SuppressChoices: true,
			// The opening scene is pure scene-setting narration — it needs
			// no tools, so an empty toolset keeps it to one model
			// round-trip. See BeatInput.MinimalTools.
			MinimalTools: true,
		}, time.Time{})
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "开场失败：%v（仍可手动输入行动继续）\n", result.Err)
		} else {
			lastChoices = result.Choices
			lastNarrative = result.Narrative
		}
	}

	for {
		fmt.Print("\n> ")
		line, err := stdin.ReadString('\n')
		if err == io.EOF {
			fmt.Println("\n[EOF — bye]")
			return 0
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "read input: %v\n", err)
			return 1
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "q" || line == "quit" || line == "exit" || line == "退出" {
			return 0
		}

		content, ok := resolveInput(ctx, line, lastChoices, lastNarrative, rt.IntentResolver, stdin)
		if !ok {
			continue
		}

		fmt.Println("\n[说书人推演中…]")
		started := time.Now()
		result := streamBeat(ctx, rt.Session, session.BeatInput{
			WorldID: *worldID,
			Action: role.PlayerAction{
				PlayerID: rt.Player.ID,
				Content:  session.WrapPlayerAction(content, rt.Player.Name),
			},
			RecentEvents: 8,
		}, started)
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "beat error: %v\n", result.Err)
			continue
		}
		lastChoices = result.Choices
		lastNarrative = result.Narrative
	}
}

// streamBeat runs one beat and streams the narrative live to stdout as it
// arrives, then prints the post-narrative status footer (turn count,
// tensions, optional timing) and the suggested action list. If started is
// the zero time the timing line is omitted (used for prologue/recap where
// elapsed framing would feel off).
//
// Returns the final BeatResult so callers can chain (e.g. cache choices
// for the next REPL iteration).
func streamBeat(ctx context.Context, sess *session.Session, in session.BeatInput, started time.Time) session.BeatResult {
	out := sess.RunBeat(ctx, in)
	fmt.Println("\n──────── 评话 ────────")
	for chunk := range out.NarrativeStream {
		fmt.Print(chunk)
	}
	fmt.Println()
	fmt.Println("──────────────────────")
	// react.go closes NarrativeStream at the content→tool_call transition,
	// so the range loop above returns the moment the prose ends while the
	// beat agent is still streaming its inline set_choices tool_call args
	// (the inline_choices tail — ~1s in live runs). Without a cue the screen
	// would sit silent on <-out.Done and read as a freeze. Surface a tiny
	// progress line so the wait feels intentional. Suppressed-choices beats
	// (recap / prologue) produce no options, so the hint is gated off there.
	if hint := optionProgressHint(in.SuppressChoices); hint != "" {
		fmt.Println(hint)
	}
	result := <-out.Done
	if result.Err != nil {
		return result
	}
	if !started.IsZero() {
		elapsed := time.Since(started).Round(100 * time.Millisecond)
		fmt.Printf("回合=%d 效果=%d 耗时=%s 张力=%s\n",
			result.World.Clock.Sequence, len(result.ToolEffects), elapsed, formatTensions(result.World))
	} else {
		fmt.Printf("回合=%d 效果=%d 张力=%s\n",
			result.World.Clock.Sequence, len(result.ToolEffects), formatTensions(result.World))
	}
	if result.SuggestErr != nil {
		fmt.Printf("(行动建议失败：%v — 请自由输入下一步行动)\n", result.SuggestErr)
	}
	// Lorekeeper extraction now runs in the background and is joined at the
	// start of the NEXT beat, so what we have here is the PREVIOUS beat's
	// outcome. Failures (dangling refs, empty JSON, schema drift, LLM noise,
	// or a failed enriched-snapshot save) are graceful-degrade signals — that
	// beat's world state was saved synchronously; only the knowledge
	// enrichment was affected. Never surface errors to the player; gate behind
	// WORLDLINE_DEBUG_LORE=1 for developers diagnosing extraction quality.
	if result.PrevLoreErr != nil {
		if os.Getenv("WORLDLINE_DEBUG_LORE") != "" {
			fmt.Fprintf(os.Stderr, "[lore] 上一拍沉淀失败: %v\n", result.PrevLoreErr)
		}
	} else if loreSummary := summarizeLoreReport(result.PrevLoreReport); loreSummary != "" {
		fmt.Println("（上一拍）" + loreSummary)
	}
	printChoices(result.Choices)
	// Per-stage latency trace (WORLDLINE_DEBUG_TIMING). Printed last, after
	// the narrative + footer + choices, so it never interleaves with the
	// streamed narrative the way a mid-pipeline stderr write would. The
	// background lorekeeper time is reported separately (it belongs to the
	// previous beat and is hidden under the player's read/think time).
	if result.TimingTrace != "" {
		fmt.Fprintln(os.Stderr, result.TimingTrace)
		if result.PrevLoreDur > 0 {
			fmt.Fprintf(os.Stderr, "[timing] prev_lore=%s (后台,上一拍)\n",
				result.PrevLoreDur.Round(time.Millisecond))
		}
	}
	return result
}

// optionProgressHint returns the lightweight progress line shown while the
// beat agent streams its inline set_choices tool_call args after the
// narrative text has finished (the inline_choices tail, ~1s in live runs).
// It returns "" for beats that suppress choices (recap / prologue), where
// no options are coming and the hint would mislead the player. Factored out
// of streamBeat so the suppression gating is unit-testable without driving
// a live beat.
func optionProgressHint(suppressChoices bool) string {
	if suppressChoices {
		return ""
	}
	return "（推演选项…）"
}

// summarizeLoreReport renders a one-line summary of what the Lorekeeper
// compiled this beat. Returns "" when the report is empty (no Inserted /
// Skipped / Rejected / Filtered counts and no Notes) so callers can omit
// the line entirely on quiet beats — that contract avoids spamming the
// REPL with "沉淀: 插入=0 ..." on every turn that yields no new lore.
//
// Format:
//
//	"沉淀: 插入=N 跳过=M 拒绝=K 过滤=L"
//
// followed by " (含 X 条提示)" when Notes is non-empty, to flag that the
// full report carries additional context (Validate findings, compile
// rejections) the streaming UI did not surface.
func summarizeLoreReport(r ingest.CompileReport) string {
	if r.Inserted == 0 && r.Skipped == 0 && r.Rejected == 0 && r.Filtered == 0 && len(r.Notes) == 0 {
		return ""
	}
	base := fmt.Sprintf("沉淀: 插入=%d 跳过=%d 拒绝=%d 过滤=%d",
		r.Inserted, r.Skipped, r.Rejected, r.Filtered)
	if len(r.Notes) > 0 {
		base = fmt.Sprintf("%s (含 %d 条提示)", base, len(r.Notes))
	}
	return base
}

func printChoices(choices role.ActionChoices) {
	if len(choices.Options) == 0 {
		fmt.Println("(无推荐选项，请自由发挥)")
		return
	}
	fmt.Println("\n可选行动:")
	for i, opt := range choices.Options {
		if opt.Type == role.ActionTypeCustom {
			fmt.Printf("  [%d] (自定义 — 自行描述)\n", i+1)
		} else {
			fmt.Printf("  [%d] %s — %s\n", i+1, opt.Label, opt.Type)
		}
	}
}

// intentResolverIface is the local mockable abstraction of
// *intent.Resolver. resolveInput depends on this minimal surface so
// unit tests can swap in a fake without spinning up a real LLM.
type intentResolverIface interface {
	Resolve(ctx context.Context, rawInput string, choices role.ActionChoices, recentNarrative string) (intent.Params, error)
}

// resolveInput interprets a REPL line and returns the action text the
// Narrator should execute this beat. The flow is:
//
//  1. Trim the line; empty input → ("", false).
//  2. Single-digit fast path: if the line parses as 1..len(choices) and
//     points at a non-custom slot, return that option's Label without
//     calling the LLM. Custom-slot single digit drops into
//     promptCustomSlot, and the resulting free-text is then passed
//     through the intent agent (step 3) for light polish + intent
//     extraction.
//  3. Anything else (multi-digit, mixed digits+text, free prose) is
//     handed to the intent.Resolver. Success → params.ActionText;
//     failure → silently fall back to the original line so the player
//     never sees a tracebacky error.
//
// WORLDLINE_DEBUG_INTENT=1 emits a one-line stderr trace per LLM call
// (action_text / is_destructive / notes). Per the v1 directive nothing
// is printed to stdout — the player should see only the narrator's
// reaction, not the engine's intent reasoning.
//
// resolver may be nil in tests of the single-digit fast path; in that
// case multi-digit / free-text input falls through to its raw form.
func resolveInput(
	ctx context.Context,
	line string,
	choices role.ActionChoices,
	recentNarrative string,
	resolver intentResolverIface,
	stdin *bufio.Reader,
) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	// Single-digit fast path: avoid an LLM round-trip for the most
	// common selector. Custom-slot still drops into the prompt flow.
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(choices.Options) {
		opt := choices.Options[n-1]
		if opt.Type != role.ActionTypeCustom {
			return opt.Label, true
		}
		text, ok := promptCustomSlot(n, choices, stdin)
		if !ok {
			return "", false
		}
		// The custom-slot text is free-form; route it through the
		// intent agent below so the narrator receives a polished
		// action_text consistent with the rest of the REPL flow.
		line = text
	}

	// A bare integer when there are NO numbered options on offer is a
	// mis-fire, not a selector: the player is reaching for an option this
	// beat deliberately did not produce. Recap / prologue scenes suppress
	// choices by design (BeatInput.SuppressChoices), so lastChoices is
	// empty on the first turn after a resume / new game. Routing "2" to
	// the intent resolver there would burn a ~2s LLM round-trip AND turn a
	// stray digit into a nonsense free-form action. Re-prompt with a hint
	// at zero LLM cost instead. This branch is gated on an empty option
	// list, so the normal (choices present) path — including out-of-range
	// digits like "9" with 4 options, which still defer to the resolver —
	// is unchanged. Placed before the resolver==nil check so it holds in
	// fast-path-only tests too.
	if len(choices.Options) == 0 {
		if _, err := strconv.Atoi(line); err == nil {
			fmt.Println("（本段无编号选项，请直接描述你的行动）")
			return "", false
		}
	}

	if resolver == nil {
		return line, true
	}

	intentStart := time.Now()
	params, err := resolver.Resolve(ctx, line, choices, recentNarrative)
	if os.Getenv("WORLDLINE_DEBUG_TIMING") != "" {
		fmt.Fprintf(os.Stderr, "[timing] intent=%s\n", time.Since(intentStart).Round(time.Millisecond))
	}
	if err != nil {
		if os.Getenv("WORLDLINE_DEBUG_INTENT") != "" {
			fmt.Fprintf(os.Stderr, "[intent] resolve failed (%v) — falling back to raw input %q\n", err, line)
		}
		return line, true
	}
	if os.Getenv("WORLDLINE_DEBUG_INTENT") != "" {
		fmt.Fprintf(os.Stderr, "[intent] action_text=%q is_destructive=%v notes=%q\n",
			params.ActionText, params.IsDestructive, params.Notes)
	}
	return params.ActionText, true
}

// promptCustomSlot handles the per-beat custom action prompt: read a line
// from stdin, fall back to [1] if empty.
func promptCustomSlot(n int, choices role.ActionChoices, stdin *bufio.Reader) (string, bool) {
	fmt.Printf("[%d] 自定义 — 请描述你的行动: ", n)
	text, err := stdin.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if len(choices.Options) > 0 && choices.Options[0].Type != role.ActionTypeCustom {
			fmt.Printf("(空输入 → 默认走 [1] %s)\n", choices.Options[0].Label)
			return choices.Options[0].Label, true
		}
		fmt.Println("(空自定义 — 请重试)")
		return "", false
	}
	return text, true
}

// === helpers ===

func boolWord(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func modeWord(resuming bool) string {
	if resuming {
		return "续玩（已跳过定场开场）"
	}
	return "新开"
}

func formatTensions(w worldmodel.World) string {
	if len(w.Threads) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(w.Threads))
	for _, th := range w.Threads {
		parts = append(parts, fmt.Sprintf("%s=%.2f", th.ID, th.Tension))
	}
	return strings.Join(parts, " ")
}
