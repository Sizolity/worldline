// Package httpretry provides an http.Client with automatic retry for
// transient transport-level errors.
package httpretry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config controls the exponential-backoff retry behaviour.
type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
}

// DefaultConfig returns a conservative default: 3 attempts with
// 500 ms → 1 s → 2 s back-off.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     2 * time.Second,
	}
}

// NewClient returns an *http.Client whose transport automatically
// retries on transient connection-level errors (EOF, connection reset,
// etc.) up to cfg.MaxAttempts times.
func NewClient(timeout time.Duration, cfg Config) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: &retryingTransport{inner: base, cfg: cfg},
		Timeout:   timeout,
	}
}

type retryingTransport struct {
	inner http.RoundTripper
	cfg   Config
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
		if !IsRetryable(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// IsRetryable reports whether err is a transient transport-layer
// failure that is safe to replay (the server never received or
// processed the request body).
func IsRetryable(err error) bool {
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
