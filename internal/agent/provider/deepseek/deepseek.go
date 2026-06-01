// Package deepseek constructs an Eino ToolCallingChatModel backed by the
// DeepSeek API. The API key MUST already be present in the process
// environment (DEEPSEEK_API_KEY); this package does not perform any
// .env loading or other startup-time initialization — that is the
// caller's responsibility (e.g. internal/env.LoadIfNeeded invoked from
// the application entry point).
package deepseek

import (
	"context"
	"fmt"
	"os"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"

	"github.com/sizolity/worldline/internal/agent/provider/internal/httpretry"
)

// NewChatModel constructs a DeepSeek Eino ToolCallingChatModel. It is a
// pure factory: the caller is responsible for ensuring DEEPSEEK_API_KEY
// is set in the process environment before calling.
func NewChatModel() (einomodel.ToolCallingChatModel, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}

	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:    "https://api.deepseek.com/v1",
		APIKey:     apiKey,
		Model:      "deepseek-chat",
		HTTPClient: httpretry.NewClient(90*time.Second, httpretry.DefaultConfig()),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	return chatModel, nil
}
