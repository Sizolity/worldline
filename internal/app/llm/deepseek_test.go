package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// TestIsRetryableHTTPErr pins which transport-layer errors are safe to
// replay. Only PRE-response errors (connection failed before any
// response bytes) are retryable; user-cancellation and post-response
// failures are not.
func TestIsRetryableHTTPErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"url.Error wrapping io.EOF (DeepSeek pattern)", &url.Error{Op: "Post", URL: "https://example/v1/chat", Err: io.EOF}, true},
		{"connection reset", errors.New("read tcp 1.2.3.4:443: connection reset by peer"), true},
		{"connection refused", errors.New("dial tcp 1.2.3.4:443: connection refused"), true},
		{"broken pipe", errors.New("write tcp: broken pipe"), true},
		{"TLS handshake EOF", errors.New("tls: handshake EOF"), true},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"plain 4xx-shaped error", errors.New("status 400 bad request"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableHTTPErr(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableHTTPErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestRetryingTransport_RetriesOnEOF simulates DeepSeek's "server closes
// the TCP connection before responding" pattern using a httptest server
// that hijacks and closes its first two connections, then succeeds on
// the third. The transport must retry transparently and surface the
// final 200 to the caller.
func TestRetryingTransport_RetriesOnEOF(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := newRetryingHTTPClient(5*time.Second, httpRetryConfig{
		MaxAttempts: 5,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     5 * time.Millisecond,
	})

	resp, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("server saw %d attempts, want exactly 3 (2 EOFs + 1 success)", got)
	}
}

// TestRetryingTransport_GivesUpAfterMaxAttempts ensures the retry
// budget is bounded — an endlessly-failing server eventually surfaces
// the last error to the caller instead of retrying forever.
func TestRetryingTransport_GivesUpAfterMaxAttempts(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer server.Close()

	client := newRetryingHTTPClient(2*time.Second, httpRetryConfig{
		MaxAttempts: 3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     2 * time.Millisecond,
	})

	_, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("server saw %d attempts, want exactly 3 (max budget)", got)
	}
}

// TestRetryingTransport_DoesNotRetryHTTP4xx confirms that a real HTTP
// error response (4xx) is NOT a transport-layer failure — RoundTrip
// returns a valid *Response and nil error, so retry must not engage.
func TestRetryingTransport_DoesNotRetryHTTP4xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newRetryingHTTPClient(2*time.Second, defaultRetryConfig())

	resp, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d want 400", resp.StatusCode)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("server saw %d attempts, want 1 (4xx is not transport-retryable)", got)
	}
}
