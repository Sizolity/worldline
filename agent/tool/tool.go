package tool

import "context"

// Info describes a tool's metadata for LLM function calling.
type Info struct {
	Name        string
	Description string
	Parameters  any // JSON schema object
}

// Tool is a provider-agnostic callable tool.
type Tool interface {
	Info() Info
	Invoke(ctx context.Context, arguments string) (string, error)
}
