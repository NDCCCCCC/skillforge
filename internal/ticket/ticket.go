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

// Package ticket is the TicketService reserved surface (TKT-01): a narrow
// outbound Provider interface plus a config-driven provider registry.
// Real ticket systems (Jira / ServiceNow / in-house) are explicitly out of
// scope for this milestone — this package IS the whole promise: the
// interface, the "none" store-only provider, and the webhook provider.
//
// Shape discipline mirrors internal/retriever/backend.go: a one-method
// interface plus a NotWired constant the server layer uses as its 503 body.
package ticket

import (
	"context"
	"time"
)

// Ticket is the dispatch payload the server layer assembles from a stored
// ticket row. It is data-only: the provider decides the wire shape.
type Ticket struct {
	// Number is the local ticket number minted at insert time ("TKT-<id>").
	Number string
	// SessionID is the chat session the ticketed turn belongs to.
	SessionID string
	// TurnID is the assistant session_turn row the ticket refers to.
	TurnID int64
	// Description is the operator-facing problem statement (user text). It
	// only ever travels inside the outbound JSON body — never in a URL and
	// never in a header (T-29-02).
	Description string
	// ChunkIDs are the cited chunk ids attached to the ticketed turn.
	ChunkIDs []int64
	// SkillID is the Skill that produced the ticketed turn.
	SkillID string
}

// DispatchResult reports one settled dispatch series. Dispatch is terminal:
// it returns only after the provider either succeeded or exhausted its
// bounded retries — callers never observe an in-flight outcome.
type DispatchResult struct {
	// ExternalRef is the external ticket id the remote system returned.
	// Empty = no external receipt (the none provider always leaves it
	// empty; a generic webhook has no agreed receipt shape).
	ExternalRef string
	// Attempts is the number of HTTP attempts actually made. The none
	// provider reports 0.
	Attempts int
}

// Provider is the narrow outbound interface of the TicketService. It is
// deliberately minimal — exactly one method, like retriever.IndexerBackend —
// so any future integration (Jira, ServiceNow, in-house) satisfies it with a
// single function.
//
// Dispatch is terminal: it returns only after the provider either succeeded
// or exhausted its bounded retries with a definite failure. Implementations
// must be safe for concurrent use and must honour ctx cancellation.
type Provider interface {
	// Dispatch sends the ticket outbound and reports the settled result.
	Dispatch(ctx context.Context, t Ticket) (DispatchResult, error)
}

// ProviderNotWiredError is the 503 body when Deps.TicketProvider is nil
// (server started without a ticket provider). The none provider is a WIRED
// provider — store-only, zero outbound traffic — and never surfaces this.
const ProviderNotWiredError = "ticket provider not wired (server started without a ticket provider)"

// Config carries the provider knobs resolved from operator env by the
// assembly point (cmd/server/main.go, 29-06). This package never reads env
// itself: a bad value surfaces as an error from NewProvider and the
// assembly point decides the failure mode (log.Fatalf, D16-08-D posture).
type Config struct {
	// WebhookURL is the outbound endpoint for the webhook provider. It MUST
	// come from operator env — user text never reaches the URL (T-29-02).
	WebhookURL string
	// Timeout is the per-attempt HTTP timeout. Zero → defaultWebhookTimeout.
	Timeout time.Duration
	// Retries is the number of retries after the first attempt, so a
	// dispatch makes at most Retries+1 HTTP attempts (T-29-07 bound).
	// Negative is clamped to 0.
	Retries int
	// Secret is the optional shared secret sent as X-SkillForge-Ticket-Secret.
	// Empty = the header is not sent at all.
	Secret string
}
