package runtime

import "github.com/sizolity/worldline/world/director"

type RuntimeOption func(*Runtime)

func NewRuntime(options ...RuntimeOption) Runtime {
	rt := Runtime{Rules: DefaultRules()}
	for _, option := range options {
		option(&rt)
	}
	return rt
}

func DefaultRules() []Rule {
	return []Rule{
		EntityExistsRule{},
		ActorAliveRule{},
	}
}

func WithoutRules() RuntimeOption {
	return func(rt *Runtime) {
		rt.Rules = nil
	}
}

func WithRules(rules ...Rule) RuntimeOption {
	return func(rt *Runtime) {
		rt.Rules = append([]Rule(nil), rules...)
	}
}

func WithDirectors(directors ...director.Director) RuntimeOption {
	return func(rt *Runtime) {
		rt.Directors = append([]director.Director(nil), directors...)
	}
}

func WithEventQueueLimit(limit int) RuntimeOption {
	return func(rt *Runtime) {
		rt.EventQueueLimit = limit
	}
}
