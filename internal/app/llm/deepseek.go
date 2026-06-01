package llm

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	openai "github.com/cloudwego/eino-ext/components/model/openai"
)

// NewDeepSeekClient constructs an eino OpenAI-compatible chat model backed by
// the DeepSeek API. The API key is resolved in order from:
//  1. DEEPSEEK_API_KEY environment variable
//  2. .env file in workspace dir
//  3. .env files in cwd and up to 6 ancestor directories
//
// Returns the model and nil on success, or a descriptive error.
func NewDeepSeekClient(workspace string) (einomodel.ToolCallingChatModel, error) {
	loadEnvIfNeeded(workspace)
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is not set (checked env and .env files)")
	}

	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:    "https://api.deepseek.com/v1",
		APIKey:     apiKey,
		Model:      "deepseek-chat",
		HTTPClient: newRetryingHTTPClient(90*time.Second, defaultRetryConfig()),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	return chatModel, nil
}

// --- .env loader ---

func loadEnvIfNeeded(workspace string) {
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return
	}
	cands := []string{filepath.Join(workspace, ".env")}
	cwd, _ := os.Getwd()
	d := cwd
	for i := 0; i < 6 && d != "/" && d != ""; i++ {
		cands = append(cands, filepath.Join(d, ".env"))
		d = filepath.Dir(d)
	}
	for _, p := range cands {
		if loadEnvFile(p) && os.Getenv("DEEPSEEK_API_KEY") != "" {
			return
		}
	}
}

func loadEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return true
}

// --- HTTP retry transport ---

type httpRetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
}

func defaultRetryConfig() httpRetryConfig {
	return httpRetryConfig{
		MaxAttempts: 3,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     2 * time.Second,
	}
}

func newRetryingHTTPClient(timeout time.Duration, cfg httpRetryConfig) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: &retryingTransport{inner: base, cfg: cfg},
		Timeout:   timeout,
	}
}

type retryingTransport struct {
	inner http.RoundTripper
	cfg   httpRetryConfig
}

func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	wait := t.cfg.InitialWait
	var lastErr error
	for attempt := 0; attempt < t.cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
			wait *= 2
			if wait > t.cfg.MaxWait {
				wait = t.cfg.MaxWait
			}
		}
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := t.inner.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableHTTPErr(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func isRetryableHTTPErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "TLS handshake")
}
