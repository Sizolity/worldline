package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sizolity/worldline/internal/world/ingest"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

// loreProfileEnabled is read once at package init so the per-task hot
// path doesn't hit os.Getenv on every beat. WORLDLINE_DEBUG_LORE=1
// turns on a one-line stderr breakdown of where each lorekeeper task's
// wall-time goes (Parse / wait / Validate / Compile / Save). Disabled
// by default — when off, the timing helper is a few zero-cost
// time.Now() calls per phase and nothing is printed.
var loreProfileEnabled = os.Getenv("WORLDLINE_DEBUG_LORE") != ""

// loreProfile records the per-phase durations and report counters for
// one lore task. It is built up across runLoreTaskTwoPhase and emitted
// (when loreProfileEnabled) just before the task's done channel
// closes. We aggregate per-phase rather than fire individual stderr
// lines because a single line per task is far easier to grep / diff
// across beats.
type loreProfile struct {
	worldID     string
	sourceID    string
	parseDur    time.Duration
	waitDur     time.Duration
	validateDur time.Duration
	compileDur  time.Duration
	saveDur     time.Duration
	totalDur    time.Duration
	draftEnts   int
	inserted    int
	notes       int
	parseErr    bool
	otherErr    bool
}

// emit writes the one-line profile to stderr in a single Fprintf so
// the line doesn't interleave with other concurrent stderr writers
// (e.g. the [timing] and [choices] lines emitted around the same wall
// clock). Always include each phase even when zero so a column-wise
// diff across beats is straightforward.
func (p *loreProfile) emit() {
	status := "ok"
	switch {
	case p.parseErr:
		status = "parse_err"
	case p.otherErr:
		status = "other_err"
	}
	fmt.Fprintf(os.Stderr,
		"[lore] world=%s src=%s status=%s parse=%s wait=%s validate=%s compile=%s save=%s total=%s draft_entities=%d inserted=%d notes=%d\n",
		p.worldID, p.sourceID, status,
		p.parseDur.Round(time.Millisecond),
		p.waitDur.Round(time.Millisecond),
		p.validateDur.Round(time.Millisecond),
		p.compileDur.Round(time.Millisecond),
		p.saveDur.Round(time.Millisecond),
		p.totalDur.Round(time.Millisecond),
		p.draftEnts, p.inserted, p.notes,
	)
}

// pendingLore tracks a background lorekeeper extraction + enriched-snapshot
// save kicked off DURING a beat (at narrative-end, before the beat's
// final stream tail and effects/save are settled). The task runs in two
// phases so the slow LLM call overlaps with the rest of the pipeline:
//
//	Phase A — Parse: LLM extract from narrative (~6-8s). Starts the moment
//	         the narrative content phase ends, well before the beat's
//	         critical path completes. Only depends on narrative text +
//	         a stable source-doc ID (which is computable from pre-effects
//	         world.Clock.Sequence).
//	Phase B — Compile + Save: pure-Go merge into the post-effects world,
//	         then re-persist the enriched snapshot if new lore was
//	         inserted. Gated on the pipeline pushing the post-effects
//	         world via attachLoreWorld; if Phase A is still running when
//	         the world arrives, this phase queues behind it naturally.
//
// The next beat (or AwaitPendingLore) joins the whole task via done,
// guaranteeing there is never a concurrent writer of world.json and
// that the next LoadSnapshot observes the enriched world.
//
// Fields other than done and worldCh are written only by the task
// goroutine and read only after done is closed, so they need no
// additional synchronization. worldCh is a buffered (cap 1) channel
// the pipeline sends the post-effects world into; the task closes
// done after Phase B completes (success or any error).
type pendingLore struct {
	done    chan struct{}
	worldID string
	report  ingest.CompileReport
	loreErr error
	saveErr error
	dur     time.Duration

	// worldCh carries the post-effects world from the pipeline to the
	// task's Phase B. Buffered (cap 1) so attachLoreWorld never blocks.
	// Closed by attachLoreWorld after sending (so the receive sees the
	// value, no further sends) OR by abortLoreTask without a send (so
	// the receive sees ok=false and the task short-circuits).
	worldCh chan worldmodel.World

	// attached records whether attachLoreWorld has consumed this task's
	// worldCh, so an abortLoreTask on the same task can detect that
	// it's too late to abort and silently no-op. Guarded by the
	// session's loreMu.
	attached bool
}

// LoreOutcome is the result of a background lorekeeper task, returned by
// AwaitPendingLore.
type LoreOutcome struct {
	// Pending is true when a task was actually in flight and awaited; false
	// when there was nothing to await (no Lorekeeper configured, or already
	// joined by the next beat).
	Pending bool
	Report  ingest.CompileReport
	LoreErr error
	SaveErr error
	Dur     time.Duration
}

// errLoreAborted is the soft-fail signal Phase B records when the pipeline
// closes worldCh without delivering a world (i.e. the beat aborted before
// reaching effects/save and the lore enrichment cannot proceed). It is
// distinct from "Parse failed" so test/CLI surfaces can distinguish a
// truly degraded LLM extraction from a normal pipeline rollback.
var errLoreAborted = errors.New("lore task aborted: pipeline did not deliver post-effects world")

// runLoreTaskTwoPhase is the background body for one beat's lorekeeper
// extraction. Phase A (LLM Parse) runs immediately on the goroutine's
// own time. Phase B (Compile + Save) waits for the pipeline to push
// the post-effects world via worldCh. When the pipeline aborts before
// reaching effects (any early-return error in runBeatPipeline),
// abortLoreTask closes worldCh empty and Phase B short-circuits with
// errLoreAborted — no half-merged world ever lands on disk.
//
// task.dur is the WALL-CLOCK time from goroutine start to done (i.e.
// it includes both phases AND any time Phase B spent waiting on
// worldCh). This is the right number for "how long was the task in
// flight"; lore_join on the next beat measures the same wall-clock
// interval from the joining side.
func (s *Session) runLoreTaskTwoPhase(ctx context.Context, task *pendingLore, sourceID, narrative string) {
	start := time.Now()
	var prof *loreProfile
	if loreProfileEnabled {
		prof = &loreProfile{worldID: task.worldID, sourceID: sourceID}
	}
	defer func() {
		task.dur = time.Since(start)
		if prof != nil {
			prof.totalDur = task.dur
			prof.emit()
		}
		close(task.done)
	}()

	// Phase A: LLM-bound extraction. This is the ~6-8s heavy step that
	// the early-start optimization exists to overlap with the beat's
	// stream tail + effects + save.
	doc := ingest.SourceDocument{ID: sourceID, Kind: "rpg_beat", Text: narrative}
	parseStart := time.Now()
	draft, err := s.lorekeeper.Parse(ctx, doc)
	if prof != nil {
		prof.parseDur = time.Since(parseStart)
		prof.draftEnts = len(draft.Entities)
	}
	if err != nil {
		task.loreErr = fmt.Errorf("lorekeeper extract: %w", err)
		if prof != nil {
			prof.parseErr = true
		}
		// Drain worldCh in the background so an attachLoreWorld send
		// (or close from abort) never piles up. Receive-and-discard is
		// safe: cap is 1 and we don't care about the value here.
		go func() {
			select {
			case <-task.worldCh:
			case <-ctx.Done():
			}
		}()
		return
	}

	// Phase B: wait for the post-effects world. If the pipeline calls
	// abortLoreTask before pushing one (because beat itself errored),
	// worldCh closes without a value — short-circuit with the abort
	// sentinel so callers / tests can tell that apart from a Parse or
	// Compile failure.
	waitStart := time.Now()
	var (
		world worldmodel.World
		ok    bool
	)
	select {
	case world, ok = <-task.worldCh:
		if prof != nil {
			prof.waitDur = time.Since(waitStart)
		}
		if !ok {
			task.loreErr = errLoreAborted
			if prof != nil {
				prof.otherErr = true
			}
			return
		}
	case <-ctx.Done():
		if prof != nil {
			prof.waitDur = time.Since(waitStart)
			prof.otherErr = true
		}
		task.loreErr = ctx.Err()
		return
	}

	// Phase B continued: validate + compile + (conditionally) save.
	// Note the report-Notes ordering mirrors the pre-refactor behavior:
	// CompileDraft errors short-circuit (zero report), but validate
	// warnings/errors are appended as Notes on the otherwise-successful
	// report so the CLI can still surface them.
	validateStart := time.Now()
	validation := ingest.ValidateDraft(draft)
	if prof != nil {
		prof.validateDur = time.Since(validateStart)
	}
	compileStart := time.Now()
	newWorld, report, err := ingest.CompileDraft(world, draft, ingest.CompileOptions{
		ConflictPolicy: ingest.ConflictPolicySkip,
		// Resolver left nil: CompileDraft falls back to NoopAliasResolver.
		// A narrator-driven AliasResolver for NPC name dedup is a future sub.
	})
	if prof != nil {
		prof.compileDur = time.Since(compileStart)
	}
	if err != nil {
		task.loreErr = fmt.Errorf("lorekeeper compile: %w", err)
		if prof != nil {
			prof.otherErr = true
		}
		return
	}
	for _, e := range validation.Errors {
		report.Notes = append(report.Notes, "validate-error: "+e)
	}
	for _, w := range validation.Warnings {
		report.Notes = append(report.Notes, "validate-warn: "+w)
	}
	task.report = report
	if prof != nil {
		prof.inserted = report.Inserted
		prof.notes = len(report.Notes)
	}

	if report.Inserted == 0 {
		// Nothing new merged (ConflictPolicySkip never mutates existing
		// records), so the post-effects snapshot saved by the pipeline
		// is already current — skip a redundant write.
		return
	}
	saveStart := time.Now()
	if err := s.store.SaveSnapshot(ctx, newWorld); err != nil {
		task.saveErr = fmt.Errorf("save enriched snapshot: %w", err)
		if prof != nil {
			prof.otherErr = true
		}
	}
	if prof != nil {
		prof.saveDur = time.Since(saveStart)
	}
}

// startLoreExtraction kicks off Phase A (LLM extract) immediately and
// returns a handle the pipeline must later settle via attachLoreWorld
// (success) or abortLoreTask (early error). Failing to settle leaks
// the goroutine waiting on worldCh — the pipeline's defer is the
// canonical place to ensure exactly-once settlement.
//
// The handle is also stored on s.loreTask so the next beat's
// joinPendingLore can find it. Calling startLoreExtraction with a
// task already in flight is unsupported (sessions run beats serially);
// it overwrites the previous handle and the orphaned task will leak
// if no one joins it. The serial-beat assumption matches the existing
// canonical usage pattern.
func (s *Session) startLoreExtraction(ctx context.Context, worldID, sourceID, narrative string) *pendingLore {
	task := &pendingLore{
		done:    make(chan struct{}),
		worldID: worldID,
		worldCh: make(chan worldmodel.World, 1),
	}
	s.loreMu.Lock()
	s.loreTask = task
	s.loreMu.Unlock()
	go s.runLoreTaskTwoPhase(ctx, task, sourceID, narrative)
	return task
}

// attachLoreWorld hands the pipeline's post-effects world to the task
// so Phase B (Compile + Save) can run. Safe to call exactly once per
// task; the buffered channel guarantees the send never blocks. The
// close after send signals "no more worlds coming" so the task's
// select knows the channel is final. attached=true is recorded so a
// subsequent abortLoreTask call (e.g. from a misbehaving defer)
// becomes a no-op instead of double-closing.
func (s *Session) attachLoreWorld(task *pendingLore, world worldmodel.World) {
	if task == nil {
		return
	}
	s.loreMu.Lock()
	if task.attached {
		s.loreMu.Unlock()
		return
	}
	task.attached = true
	s.loreMu.Unlock()
	task.worldCh <- world
	close(task.worldCh)
}

// abortLoreTask releases a started-but-not-attached task when the
// pipeline failed before producing a post-effects world. Closes
// worldCh without a value so Phase B's receive sees ok=false and
// short-circuits with errLoreAborted. No-op when already attached.
//
// Intended use: defer abortLoreTask in the pipeline right after
// startLoreExtraction; on the success path, attachLoreWorld flips
// attached=true so the deferred abort becomes a no-op.
func (s *Session) abortLoreTask(task *pendingLore) {
	if task == nil {
		return
	}
	s.loreMu.Lock()
	if task.attached {
		s.loreMu.Unlock()
		return
	}
	task.attached = true
	s.loreMu.Unlock()
	close(task.worldCh)
}

// joinPendingLore takes ownership of the most recent background lore task (if
// any), clearing it, and blocks until it finishes. Returns nil when no task is
// pending. Idempotent: only one caller can take a given task.
func (s *Session) joinPendingLore() *pendingLore {
	s.loreMu.Lock()
	task := s.loreTask
	s.loreTask = nil
	s.loreMu.Unlock()
	if task == nil {
		return nil
	}
	<-task.done
	return task
}

// AwaitPendingLore blocks until the background lorekeeper task from the most
// recent beat (if any) has merged and persisted its enrichment, returning its
// outcome. It is idempotent and returns LoreOutcome{Pending:false} when
// nothing is in flight.
//
// Call this before shutting down a session so the last beat's extracted lore
// is durably saved — the world *state* is always saved synchronously by the
// beat, but the knowledge enrichment lands here. Tests use it to observe the
// async result deterministically.
func (s *Session) AwaitPendingLore() LoreOutcome {
	task := s.joinPendingLore()
	if task == nil {
		return LoreOutcome{}
	}
	return LoreOutcome{
		Pending: true,
		Report:  task.report,
		LoreErr: task.loreErr,
		SaveErr: task.saveErr,
		Dur:     task.dur,
	}
}
