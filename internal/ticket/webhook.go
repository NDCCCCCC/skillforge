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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// defaultWebhookTimeout bounds a single HTTP attempt when Config.Timeout
	// is unset. Generous ceiling; the dispatch goroutine's total budget is
	// the assembly point's responsibility (29-06).
	defaultWebhookTimeout = 10 * time.Second
	// webhookRetryBackoff is the fixed pause between attempts (bge.go:121-128
	// precedent: ctx-aware sleep so cancellation is honoured mid-backoff).
	webhookRetryBackoff = 1 * time.Second
	// SecretHeader carries the optional shared secret. The secret comes from
	// operator env only and never contains user text (T-29-02).
	SecretHeader = "X-SkillForge-Ticket-Secret"
	// maxDrainBytes caps how much of a response body is drained so the
	// connection can be reused. Webhook receipts are tiny; a hostile or
	// broken endpoint must not be able to make us read forever.
	maxDrainBytes = 8 * 1024
)

// webhookProvider POSTs the ticket payload to a configured HTTP endpoint.
// The outbound dispatch is best-effort by contract: bounded attempts, fixed
// backoff, per-attempt timeout — a failure here never blocks the chat or
// feedback path (the caller records it and moves on, 29-06).
type webhookProvider struct {
	url     string
	secret  string
	timeout time.Duration
	retries int
	client  *http.Client
}

// Compile-time assertion: webhookProvider satisfies Provider.
var _ Provider = (*webhookProvider)(nil)

// ticketPayload is the outbound JSON body. The json field names are the wire
// contract. User text (Description) only ever travels inside this body —
// never in the URL, never in a header (T-29-02).
type ticketPayload struct {
	TicketNumber string  `json:"ticketNumber"`
	SessionID    string  `json:"sessionId"`
	TurnID       int64   `json:"turnId"`
	Description  string  `json:"description"`
	ChunkIDs     []int64 `json:"chunkIds"`
	SkillID      string  `json:"skillId"`
}

// newWebhookProvider builds the outbound provider. The client is
// per-instance: the transport is a Clone of DefaultTransport so proxy/TLS
// defaults survive while the connection pool stays private to this provider.
// http.DefaultClient is never used (bge.go:90-98 discipline) and the TLS
// posture is never skipped — intranet webhooks verify certificates like
// everything else.
func newWebhookProvider(cfg Config) *webhookProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultWebhookTimeout
	}
	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}
	return &webhookProvider{
		url:     cfg.WebhookURL,
		secret:  cfg.Secret,
		timeout: timeout,
		retries: retries,
		client: &http.Client{
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
}

// Dispatch POSTs the ticket to the webhook endpoint. Bounded: at most
// retries+1 attempts with a fixed 1s ctx-aware backoff between them, and a
// per-attempt timeout. Any 2xx status is success; anything else is retried
// and, once attempts are exhausted, returned as a wrapped error carrying the
// attempt count. Dispatch is terminal — it never reports an in-flight state.
func (w *webhookProvider) Dispatch(ctx context.Context, t Ticket) (DispatchResult, error) {
	ids := t.ChunkIDs
	if ids == nil {
		ids = []int64{} // marshal as [] not null — friendlier to generic consumers
	}
	body, err := json.Marshal(ticketPayload{
		TicketNumber: t.Number,
		SessionID:    t.SessionID,
		TurnID:       t.TurnID,
		Description:  t.Description,
		ChunkIDs:     ids,
		SkillID:      t.SkillID,
	})
	if err != nil {
		return DispatchResult{}, fmt.Errorf("ticket webhook: marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= w.retries+1; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return DispatchResult{Attempts: attempt - 1}, ctx.Err()
			case <-time.After(webhookRetryBackoff):
			}
		}
		err := w.postOnce(ctx, body)
		if err == nil {
			return DispatchResult{Attempts: attempt}, nil
		}
		lastErr = err
	}
	return DispatchResult{Attempts: w.retries + 1}, fmt.Errorf("ticket webhook dispatch to %s failed after %d attempts: %w", w.url, w.retries+1, lastErr)
}

// postOnce performs a single webhook POST. Any 2xx status is success.
func (w *webhookProvider) postOnce(ctx context.Context, body []byte) error {
	reqCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if w.secret != "" {
		req.Header.Set(SecretHeader, w.secret)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", w.url, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("POST %s: status %d", w.url, resp.StatusCode)
	}
	return nil
}
