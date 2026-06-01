package chat

import "context"

// Option configures a Generate or Stream call.
type Option func(*Options)

// Options holds configuration for model calls.
type Options struct {
	Extra map[string]any
}

// WithExtra attaches an arbitrary key-value pair to the options.
func WithExtra(key string, val any) Option {
	return func(o *Options) {
		if o.Extra == nil {
			o.Extra = make(map[string]any)
		}
		o.Extra[key] = val
	}
}

// ApplyOptions builds an Options value from the given option functions.
func ApplyOptions(opts ...Option) Options {
	var o Options
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// StreamReader delivers streamed message chunks via a channel.
type StreamReader struct {
	Ch <-chan Message
}

// Model generates a single response from a list of messages.
type Model interface {
	Generate(ctx context.Context, messages []Message, opts ...Option) (Message, error)
}

// StreamModel extends Model with streaming support.
type StreamModel interface {
	Model
	Stream(ctx context.Context, messages []Message, opts ...Option) (*StreamReader, error)
}
