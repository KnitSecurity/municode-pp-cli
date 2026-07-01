// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: transient network failures must be retried with exponential
// backoff (not a tight immediate loop), so a brief blip mid-walk doesn't abort a
// long clone. See the network-error branch in do().

package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"municode-pp-cli/internal/config"
)

// TestNetworkErrorRetriesWithBackoff drops the first connection (a network-level
// failure, not an HTTP status) and serves 200 on the retry. The client must
// recover, and must wait the backoff interval before retrying rather than
// hammering immediately.
func TestNetworkErrorRetriesWithBackoff(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			// Force HTTPClient.Do to return an error: hijack and close the
			// connection without writing a response.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatalf("test server does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.(net.Conn).Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
	c.NoCache = true

	start := time.Now()
	body, err := c.Get(context.Background(), "/thing", nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after a network-error retry, got: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body after retry: %s", body)
	}
	if n := atomic.LoadInt32(&attempts); n < 2 {
		t.Fatalf("expected at least one retry, server saw %d attempt(s)", n)
	}
	// The first retry backs off 2^0 = 1s. Without backoff the retry is immediate
	// (well under a second), so this interval is the regression guard.
	if elapsed < time.Second {
		t.Fatalf("expected ~1s backoff before the network retry, took only %v", elapsed)
	}
}
