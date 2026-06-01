package runtime

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/internal/world/director"
	"github.com/sizolity/worldline/internal/world/model"
)

type Runtime struct {
	Rules           []Rule
	Directors       []director.Director
	EventQueueLimit int
}

type StepResult struct {
	World         model.World        `json:"world"`
	Proposals     []model.WorldEvent `json:"proposals"`
	AppliedEvents []model.WorldEvent `json:"applied_events"`
	SkippedEvents []model.WorldEvent `json:"skipped_events,omitempty"`
}

func (r Runtime) Step(ctx context.Context, world model.World) (StepResult, error) {
	result := StepResult{
		World:         world,
		Proposals:     []model.WorldEvent{},
		AppliedEvents: []model.WorldEvent{},
		SkippedEvents: []model.WorldEvent{},
	}
	for _, d := range r.Directors {
		proposals, err := d.Propose(director.Context{Ctx: ctx, World: cloneWorldForMutation(result.World)})
		if err != nil {
			return result, fmt.Errorf("director %q: %w", d.ID(), err)
		}
		result.Proposals = append(result.Proposals, proposals...)
	}
	for i, proposal := range result.Proposals {
		next, err := r.ApplyEvent(result.World, proposal)
		if err != nil {
			return result, fmt.Errorf("proposal %d: %w", i, err)
		}
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, proposal)
	}
	retriedThisStep := map[model.EventID]bool{}
	for i := 0; i < r.EventQueueLimit && len(result.World.EventQueue) > 0; i++ {
		queueIndex, ok := nextReadyQueueIndexExcluding(result.World, retriedThisStep)
		if !ok {
			break
		}
		item := result.World.EventQueue[queueIndex]
		result.World.EventQueue = removeQueueItem(result.World.EventQueue, queueIndex)
		next, err := r.ApplyEvent(result.World, item.Event)
		if err != nil {
			switch item.ErrorPolicy {
			case model.QueueErrorPolicySkip:
				result.SkippedEvents = append(result.SkippedEvents, item.Event)
				continue
			case model.QueueErrorPolicyRetry:
				item.Attempts++
				if item.MaxAttempts > 0 && item.Attempts >= item.MaxAttempts {
					result.SkippedEvents = append(result.SkippedEvents, item.Event)
					continue
				}
				retriedThisStep[item.Event.ID] = true
				result.World.EventQueue = append(result.World.EventQueue, item)
				continue
			default:
				return result, fmt.Errorf("queued event %d: %w", i, err)
			}
		}
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, item.Event)
	}
	result.World.Clock = advanceClock(result.World.Clock)
	return result, nil
}

type RunResult struct {
	World            model.World        `json:"world"`
	StepsCompleted   int                `json:"steps_completed"`
	AllAppliedEvents []model.WorldEvent `json:"all_applied_events"`
}

func (r Runtime) Run(ctx context.Context, world model.World, steps int) (RunResult, error) {
	result := RunResult{
		World:            world,
		AllAppliedEvents: []model.WorldEvent{},
	}
	for i := 0; i < steps; i++ {
		step, err := r.Step(ctx, result.World)
		if err != nil {
			return result, fmt.Errorf("step %d: %w", i, err)
		}
		result.World = step.World
		result.AllAppliedEvents = append(result.AllAppliedEvents, step.AppliedEvents...)
		result.StepsCompleted++
	}
	return result, nil
}

func cloneWorldForMutation(world model.World) model.World {
	return world.Clone()
}

func nextReadyQueueIndexExcluding(world model.World, exclude map[model.EventID]bool) (int, bool) {
	bestIndex := -1
	for i, item := range world.EventQueue {
		if exclude[item.Event.ID] {
			continue
		}
		if !queueItemReady(world.Clock.Current, item) {
			continue
		}
		if bestIndex == -1 || item.Priority > world.EventQueue[bestIndex].Priority {
			bestIndex = i
		}
	}
	return bestIndex, bestIndex != -1
}

func queueItemReady(now model.WorldTime, item model.EventQueueItem) bool {
	if item.NotBefore.Kind == "" {
		return true
	}
	if item.NotBefore.Kind != now.Kind {
		return false
	}
	switch now.Kind {
	case model.WorldTimeTick, model.WorldTimeTurn, model.WorldTimeScene, model.WorldTimeChapter, model.WorldTimeDay:
		return item.NotBefore.Tick <= now.Tick
	default:
		return false
	}
}

func removeQueueItem(queue []model.EventQueueItem, index int) []model.EventQueueItem {
	out := make([]model.EventQueueItem, 0, len(queue)-1)
	out = append(out, queue[:index]...)
	out = append(out, queue[index+1:]...)
	return out
}

func advanceClock(clock model.WorldClock) model.WorldClock {
	clock.Sequence++
	if clock.Current.Kind == model.WorldTimeTick {
		clock.Current.Tick++
	}
	return clock
}

func (r Runtime) evaluateRules(world model.World, event model.WorldEvent) error {
	ctx := RuleContext{World: world}
	for _, rule := range r.Rules {
		decision := rule.Evaluate(ctx, event)
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID(), err)
		}
		if decision.Status == RuleDecisionReject {
			if decision.Reason == "" {
				return fmt.Errorf("rule %q rejected event", rule.ID())
			}
			return fmt.Errorf("rule %q rejected event: %s", rule.ID(), decision.Reason)
		}
	}
	return nil
}
