// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordedRequest is what the stub webhook observed for one attempt.
type recordedRequest struct {
	rawQuery string
	path     string
	secret   string
	body     string
}

// stubWebhook is an httptest server that records every request and lets each
// test decide the status code (and how slow the response is).
type stubWebhook struct {
	server *httptest.Server

	mu   sync.Mutex
	reqs []recordedRequest
}

// startStubWebhook spins up the stub. handler returns the status code to
// respond with (0 → 200) after the request has been recorded.
func startStubWebhook(t *testing.T, handler func(r *http.Request) int) *stubWebhook {
	t.Helper()
	s := &stubWebhook{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.reqs = append(s.reqs, recordedRequest{
			rawQuery: r.URL.RawQuery,
			path:     r.URL.Path,
			secret:   r.Header.Get(SecretHeader),
			body:     string(body),
		})
		s.mu.Unlock()
		code := handler(r)
		if code == 0 {
			code = http.StatusOK
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *stubWebhook) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}

func (s *stubWebhook) at(i int) recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reqs[i]
}

// testTicket is a payload whose every field is distinctive enough to assert
// on the wire (including a user-text description that must never leak into
// the URL).
func testTicket() Ticket {
	return Ticket{
		Number:      "TKT-17",
		SessionID:   "sess-abc123",
		TurnID:      42,
		Description: "季度报销口径与差旅制度冲突，请人工复核",
		ChunkIDs:    []int64{101, 102},
		SkillID:     "compliance",
	}
}

// TestWebhookDispatch locks the webhook provider behavior matrix: 2xx
// success, bounded retry, exhaustion error, per-attempt timeout, optional
// secret header, body-only payload, and ctx cancellation mid-backoff.
func TestWebhookDispatch(t *testing.T) {
	t.Run("first_2xx_succeeds_with_single_attempt", func(t *testing.T) {
		s := startStubWebhook(t, func(*http.Request) int { return http.StatusOK })
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: 2 * time.Second, Retries: 2, Secret: "ops-secret"})

		res, err := p.Dispatch(context.Background(), testTicket())
		if err != nil {
			t.Fatalf("Dispatch returned error: %v", err)
		}
		if res.Attempts != 1 {
			t.Errorf("Attempts = %d, want 1 (2xx on the first try)", res.Attempts)
		}
		if res.ExternalRef != "" {
			t.Errorf("ExternalRef = %q, want empty (generic webhook has no receipt shape)", res.ExternalRef)
		}
		if got := s.count(); got != 1 {
			t.Errorf("server saw %d requests, want 1", got)
		}
	})

	t.Run("retry_then_success_counts_attempts", func(t *testing.T) {
		var calls int32
		s := startStubWebhook(t, func(*http.Request) int {
			if atomic.AddInt32(&calls, 1) == 1 {
				return http.StatusInternalServerError
			}
			return http.StatusOK
		})
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: 2 * time.Second, Retries: 2})

		res, err := p.Dispatch(context.Background(), testTicket())
		if err != nil {
			t.Fatalf("Dispatch returned error after first 500: %v", err)
		}
		if res.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2 (500 then 200)", res.Attempts)
		}
		if got := s.count(); got != 2 {
			t.Errorf("server saw %d requests, want 2", got)
		}
	})

	t.Run("exhausted_retries_return_error_with_attempt_count", func(t *testing.T) {
		s := startStubWebhook(t, func(*http.Request) int { return http.StatusInternalServerError })
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: 2 * time.Second, Retries: 2})

		res, err := p.Dispatch(context.Background(), testTicket())
		if err == nil {
			t.Fatal("Dispatch with a persistently failing endpoint: want error, got nil")
		}
		if res.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3 (retries+1)", res.Attempts)
		}
		if got := s.count(); got != 3 {
			t.Errorf("server saw %d requests, want 3", got)
		}
		if !strings.Contains(err.Error(), "3 attempts") {
			t.Errorf("error should report the exhausted attempt count, got: %v", err)
		}
	})

	t.Run("slow_endpoint_times_out_per_attempt", func(t *testing.T) {
		// Handler responds long after the per-attempt timeout, so both
		// attempts must die on the timeout, not on the response.
		s := startStubWebhook(t, func(*http.Request) int {
			time.Sleep(1500 * time.Millisecond)
			return http.StatusOK
		})
		const timeout = 150 * time.Millisecond
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: timeout, Retries: 1})

		start := time.Now()
		res, err := p.Dispatch(context.Background(), testTicket())
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("Dispatch against a slow endpoint: want error, got nil")
		}
		if res.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2 (retries+1, each cut by the per-attempt timeout)", res.Attempts)
		}
		// Bound: (retries+1)*timeout + retries*backoff ≈ 1.3s. The
		// no-timeout path would need ≥ 4s (2×1.5s handler + 1s backoff).
		if elapsed > 3*time.Second {
			t.Errorf("elapsed = %v, want ≤ ~1.3s (per-attempt timeout must bound each try)", elapsed)
		}
		if got := s.count(); got != 2 {
			t.Errorf("server saw %d requests, want 2", got)
		}
	})

	t.Run("secret_header_present_iff_configured", func(t *testing.T) {
		withSecret := startStubWebhook(t, func(*http.Request) int { return http.StatusOK })
		p := newWebhookProvider(Config{WebhookURL: withSecret.server.URL, Timeout: 2 * time.Second, Secret: "ops-secret"})
		if _, err := p.Dispatch(context.Background(), testTicket()); err != nil {
			t.Fatalf("Dispatch (with secret) returned error: %v", err)
		}
		if got := withSecret.at(0).secret; got != "ops-secret" {
			t.Errorf("secret header = %q, want %q", got, "ops-secret")
		}

		withoutSecret := startStubWebhook(t, func(*http.Request) int { return http.StatusOK })
		p2 := newWebhookProvider(Config{WebhookURL: withoutSecret.server.URL, Timeout: 2 * time.Second})
		if _, err := p2.Dispatch(context.Background(), testTicket()); err != nil {
			t.Fatalf("Dispatch (without secret) returned error: %v", err)
		}
		if got := withoutSecret.at(0).secret; got != "" {
			t.Errorf("secret header = %q, want absent (empty Secret must not send the header)", got)
		}
	})

	t.Run("payload_is_body_only_and_url_stays_clean", func(t *testing.T) {
		s := startStubWebhook(t, func(*http.Request) int { return http.StatusOK })
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: 2 * time.Second})
		if _, err := p.Dispatch(context.Background(), testTicket()); err != nil {
			t.Fatalf("Dispatch returned error: %v", err)
		}

		req := s.at(0)
		// User text must never travel in the URL (T-29-02): the endpoint is
		// operator config, so both the path and the query must be untouched.
		if req.rawQuery != "" {
			t.Errorf("URL query = %q, want empty (user text must never reach the URL)", req.rawQuery)
		}
		if req.path != "/" {
			t.Errorf("URL path = %q, want the bare endpoint path %q", req.path, "/")
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(req.body), &payload); err != nil {
			t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, req.body)
		}
		want := map[string]any{
			"ticketNumber": "TKT-17",
			"sessionId":    "sess-abc123",
			"turnId":       float64(42),
			"description":  "季度报销口径与差旅制度冲突，请人工复核",
			"skillId":      "compliance",
		}
		for key, wantVal := range want {
			gotVal, ok := payload[key]
			if !ok {
				t.Errorf("payload missing key %q (body: %s)", key, req.body)
				continue
			}
			if gotVal != wantVal {
				t.Errorf("payload[%q] = %v, want %v", key, gotVal, wantVal)
			}
		}
		ids, ok := payload["chunkIds"].([]any)
		if !ok {
			t.Fatalf("payload chunkIds = %#v, want an int64 array (body: %s)", payload["chunkIds"], req.body)
		}
		if len(ids) != 2 || ids[0] != float64(101) || ids[1] != float64(102) {
			t.Errorf("payload chunkIds = %v, want [101 102]", ids)
		}
	})

	t.Run("parent_ctx_cancel_during_backoff_returns_ctx_err", func(t *testing.T) {
		hit := make(chan struct{}, 4)
		s := startStubWebhook(t, func(*http.Request) int {
			hit <- struct{}{}
			return http.StatusInternalServerError
		})
		p := newWebhookProvider(Config{WebhookURL: s.server.URL, Timeout: 2 * time.Second, Retries: 3})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() {
			_, err := p.Dispatch(ctx, testTicket())
			errCh <- err
		}()

		<-hit // first attempt completed (500 recorded)
		cancel()

		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled (cancellation must cut the backoff short)", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Dispatch did not return promptly after ctx cancellation during backoff")
		}
	})
}

// TestCheckRedirect verifies the http.Client denies cross-host redirects and
// caps redirect depth to prevent secret-header leakage and SSRF pivoting.
func TestCheckRedirect(t *testing.T) {
	t.Run("cross_host_redirect_is_rejected", func(t *testing.T) {
		// The final server records which headers it received.
		var secretReceived string
		final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secretReceived = r.Header.Get(SecretHeader)
			w.WriteHeader(http.StatusOK)
		}))
		defer final.Close()

		// The redirector sends a 301 to the final server on a different host.
		// httptest.NewServer gives each server a different port, so the redirect
		// is cross-host and CheckRedirect should block it before any secret leaks.
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, final.URL, http.StatusMovedPermanently)
		}))
		defer redirector.Close()

		p := newWebhookProvider(Config{WebhookURL: redirector.URL, Timeout: 2 * time.Second, Secret: "ops-secret"})
		_, err := p.Dispatch(context.Background(), testTicket())

		if err == nil {
			t.Fatal("Dispatch with cross-host redirect: want error, got nil")
		}
		if !strings.Contains(err.Error(), "cross-host redirect") {
			t.Errorf("error = %v, want error mentioning 'cross-host redirect'", err)
		}
		// The secret must never reach the final server.
		if secretReceived != "" {
			t.Errorf("secret header reached the redirect target = %q, want empty (leak blocked)", secretReceived)
		}
	})

	t.Run("too_many_redirects_is_rejected", func(t *testing.T) {
		// A single server that loops back to itself 4 times (exceeds cap of 2).
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/next", http.StatusMovedPermanently)
		}))
		redirectorURL := redirector.URL

		// Override handler to chain: /next -> /next2 -> /next3 -> 200.
		redirector.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/next" {
				http.Redirect(w, r, "/next2", http.StatusMovedPermanently)
				return
			}
			if r.URL.Path == "/next2" {
				http.Redirect(w, r, "/next3", http.StatusMovedPermanently)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		p := newWebhookProvider(Config{WebhookURL: redirectorURL, Timeout: 2 * time.Second})
		_, err := p.Dispatch(context.Background(), testTicket())

		if err == nil {
			t.Fatal("Dispatch with redirect chain > 2: want error, got nil")
		}
		if !strings.Contains(err.Error(), "too many redirects") {
			t.Errorf("error = %v, want error mentioning 'too many redirects'", err)
		}
	})

	t.Run("same_host_redirect_is_allowed", func(t *testing.T) {
		// A single server that redirects to /ok on the same host.
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/ok", http.StatusMovedPermanently)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(s.Close)

		p := newWebhookProvider(Config{WebhookURL: s.URL, Timeout: 2 * time.Second})
		res, err := p.Dispatch(context.Background(), testTicket())
		if err != nil {
			t.Fatalf("Dispatch with same-host redirect: want success, got error: %v", err)
		}
		if res.Attempts < 1 {
			t.Errorf("Attempts = %d, want >= 1", res.Attempts)
		}
	})
}
