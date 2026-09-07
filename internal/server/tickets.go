// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// tickets.go — Phase 29 / Plan 29-06 (TKT-02 + TKT-04) — the ticket HTTP
// surface: one public write endpoint plus the background dispatcher that
// hands queued tickets to the wired provider.
//
// TRUST MODEL — a ticket is a user request that LEAVES the process (or
// would, with a real provider), so the load-bearing invariants here are:
//
//   - One ticket per message is the STORE's UNIQUE(session_id, turn_id)
//     constraint (D-29-14); the FE's "已建单" state is only a cache. A
//     replay surfaces the FIRST acceptance number back and writes
//     nothing — it never triggers a second dispatch either.
//   - The acceptance number is derived, not stored: "TKT-{row id}" from
//     the read-back TicketRow.ID. The tickets table is INSERT-only
//     (store/sqlite/tickets.go) — there is no number column to write
//     back, and the dispatch outcome NEVER lands in the ticket row; it
//     is recorded as an INSERT-only action_ticket_dispatch_* audit row.
//   - The outbound dispatch is STRUCTURALLY non-blocking (TKT-04): the
//     handler returns 201 while the webhook may still be in flight. The
//     dispatch runs in its own goroutine on a context.Background()-derived
//     budget — cancelling the HTTP request can never abort a ticket the
//     user was told was created — and a panicking provider is contained
//     by a defer recover so the chat / feedback main links survive.
//   - Attribution is SERVER-side (the D-29-05 shape): the cited chunk ids
//     + skill id are copied from the turn's own audit row, never from the
//     request body.
//
// Wire contract (consumed by the 29-07 FE — field names frozen):
//
//	POST /api/tickets  body {session_id, turn_id, description}
//	    → 201 {ticket_number:"TKT-17", dispatch_status:"none"|"queued"}
//	    → 200 {status:"already_exists", ticket_number:"TKT-17"}
//	    → 400 | 404 | 503 (provider / store unwired)

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/ticket"
)

// maxTicketDescriptionChars bounds the operator-facing problem statement.
// Mirrors maxFeedbackCommentChars (Plan 29-05): a hard constant for Phase 29
// (a local deployment has no reason to tune it; the body-size middleware
// caps the whole request anyway), so an abusive description is a readable
// 400 at the field level rather than a 413 at the transport.
const maxTicketDescriptionChars = 4000

// defaultTicketDispatchBudget is the whole-dispatch wall-clock fallback
// when Deps.TicketDispatchBudget is unset (production always wires the
// SKILLFORGE_TICKET_DISPATCH_BUDGET_MS value explicitly).
const defaultTicketDispatchBudget = 30 * time.Second

// ticketDispatchQueued is the insert-time dispatch_status that marks a row
// as handed to the background dispatcher. "none" (the store-only receipt)
// reuses the ticket.ProviderNone vocabulary — the same closed word list the
// tickets DDL CHECK enforces.
const ticketDispatchQueued = "queued"

// ticketStoreNotWiredError is the 503 body when Deps.Store is nil: a ticket
// can never be ACCEPTED without persistence (fail-closed) — "已建单" must
// mean a durable row with a durable number, not a promise.
const ticketStoreNotWiredError = "ticket store not wired (server started without a persistence store)"

// ticketTurnNotFoundError is the generic 404 body. A missing turn, a user
// turn, and a turn owned by another session all answer with THIS exact
// envelope — the status code must never leak WHY a turn is not ticketable
// (T-29-06 posture; the handler branch is deliberately merged).
const ticketTurnNotFoundError = "session turn not found"

// ticketBody is the wire shape of POST /api/tickets (the 29-07 FE contract
// — field names frozen, do not rename). There is deliberately NO skill_id
// and NO chunk_ids field: attribution is copied server-side from the audit
// row (D-29-05), and a client-supplied field would either be ignored
// (confusing) or trusted (dangerous).
type ticketBody struct {
	// SessionID is the conversation partition the ticket belongs to.
	SessionID string `json:"session_id"`
	// TurnID is session_turn.id (D-29-01), the assistant message being
	// ticketed. A client-invented id simply fails the existence check.
	TurnID int64 `json:"turn_id"`
	// Description is the required operator-facing problem statement.
	Description string `json:"description"`
}

// ticketResponse is the success envelope — and nothing else. The 201 (and
// only the 201) carries dispatch_status; the idempotent replay carries
// status="already_exists" plus the FIRST number, and its dispatch half
// serializes away (omitempty) because no new dispatch was started.
type ticketResponse struct {
	// Status is present only on the replay path ("already_exists").
	Status string `json:"status,omitempty"`
	// TicketNumber is the derived acceptance number "TKT-{row id}".
	TicketNumber string `json:"ticket_number"`
	// DispatchStatus is the insert-time state: "none" (store-only) or
	// "queued" (handed to the background dispatcher). Never updated later
	// — the settled outcome lives in the audit trail, not in this row.
	DispatchStatus string `json:"dispatch_status,omitempty"`
}

// ticketDispatchStatusFor maps the wired provider name onto the row's
// insert-time dispatch_status. Only the webhook provider produces work for
// the background dispatcher; everything else (including an empty name) is
// the store-only "none" receipt.
func ticketDispatchStatusFor(providerName string) string {
	if providerName == ticket.ProviderWebhook {
		return ticketDispatchQueued
	}
	return ticket.ProviderNone
}

// ticketsHandler serves POST /api/tickets — the TKT-02 ticket endpoint.
//
// Gate order is FIXED (mirrors feedbackHandler's CHECK ORDER discipline):
// body decode → session-id shape → turn-id positivity → description
// presence → description bound → provider nil 503 → store nil 503 → turn
// existence/role binding. The shape gates run BEFORE any SQL so malformed
// input never reaches the store (T-10-11 discipline); the dependency gates
// run BEFORE the turn lookup so an unwired deployment never leaks whether
// a turn exists.
//
// This is a PUBLIC endpoint by design (same trust shape as /api/chat and
// /api/feedback: a local deployment has no account system) — there is no
// 403 gate here. Abuse is bounded by the validation gates + the one
// ticket-per-message UNIQUE key + the 4000-rune description cap
// (T-29-01).
func (s *Deps) ticketsHandler(ctx context.Context, c *app.RequestContext) {
	var body ticketBody
	if err := c.Bind(&body); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}

	// SC-9 / T-10-11: the session-id shape gate runs BEFORE any SQL, so a
	// malformed id can never reach the store layer.
	if !sessionIDRegex.MatchString(body.SessionID) {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid session id"})
		return
	}
	if body.TurnID <= 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "turn_id must be a positive integer"})
		return
	}
	// A ticket without a problem statement is noise for the operator: the
	// description is the whole reason the ticket exists (D-29-13 form
	// input, required).
	if strings.TrimSpace(body.Description) == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "description is required"})
		return
	}
	// Rune count, not bytes: the cap is about CONTENT length, and the
	// product's descriptions are mostly Chinese (3 bytes per rune).
	if utf8.RuneCountInString(body.Description) > maxTicketDescriptionChars {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("description exceeds %d characters", maxTicketDescriptionChars),
		})
		return
	}

	// Dependency gates, then the turn lookup — a 503 must never answer the
	// implicit "does turn N exist" question.
	if s.TicketProvider == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: ticket.ProviderNotWiredError})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: ticketStoreNotWiredError})
		return
	}

	// Turn binding (T-29-06): the ticket must resolve to an ASSISTANT turn
	// of THIS session. All three failure shapes merge into the one generic
	// 404 — do not split them, the distinction is exactly the information
	// a probing caller wants and the operator does not.
	turn, ok, err := s.Store.GetSessionTurn(ctx, body.SessionID, body.TurnID)
	if err != nil {
		log.Printf("[WARN] server: ticket turn lookup (sess=%s turn=%d) failed: %v", body.SessionID, body.TurnID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "ticket lookup failed — see server logs"})
		return
	}
	if !ok || turn.Role != "assistant" {
		c.JSON(http.StatusNotFound, graphResponseError{Error: ticketTurnNotFoundError})
		return
	}

	// Server-side attribution (D-29-05 shape): copy the citations + skill
	// from the turn's own audit row — the same probe the feedback endpoint
	// uses, because it is the same fact about the same turn. The request
	// body has no say in this; the ticket stores the CSV snapshot directly
	// (no refs table on this surface).
	skillID, chunkIDs := s.feedbackAttribution(ctx, body.SessionID, turn.Ts)

	inserted, err := s.Store.PutTicket(ctx, store.TicketRow{
		SessionID:      body.SessionID,
		TurnID:         body.TurnID,
		Description:    body.Description,
		ChunkIDs:       chunkIDs,
		SkillID:        skillID,
		Provider:       s.TicketProviderName,
		DispatchStatus: ticketDispatchStatusFor(s.TicketProviderName),
	})
	if err != nil {
		log.Printf("[WARN] server: ticket write (sess=%s turn=%d) failed: %v", body.SessionID, body.TurnID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "ticket write failed — see server logs"})
		return
	}

	// The store assigns the row id; the read-back is the only way to name
	// it (and on the replay path it IS the first row — exactly what the
	// idempotent response must surface).
	row, ok, err := s.Store.TicketByTurn(ctx, body.SessionID, body.TurnID)
	if err != nil || !ok {
		log.Printf("[WARN] server: ticket read-back (sess=%s turn=%d) failed (ok=%v, err=%v)", body.SessionID, body.TurnID, ok, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "ticket write failed — see server logs"})
		return
	}

	if !inserted {
		// D-29-14: the UNIQUE key swallowed the replay. Surface the FIRST
		// acceptance number and write NOTHING — and start no second
		// dispatch for a ticket that was already handed off.
		c.JSON(http.StatusOK, ticketResponse{Status: "already_exists", TicketNumber: ticketNumberFor(row.ID)})
		return
	}

	// TKT-04: the outbound hand-off is asynchronous. The 201 below means
	// "accepted and durable", never "delivered" — the settled outcome
	// lands in the audit trail (see dispatchForTicket).
	s.dispatchForTicket(row, body.Description)

	c.JSON(http.StatusCreated, ticketResponse{
		TicketNumber:   ticketNumberFor(row.ID),
		DispatchStatus: row.DispatchStatus,
	})
}

// ticketNumberFor derives the acceptance number from the persisted row id.
// It is a derivation, not a column: the INSERT-only ticket row never
// carries (and never needs) a number field.
func ticketNumberFor(id int64) string {
	return fmt.Sprintf("TKT-%d", id)
}

// parseTicketChunkIDs expands an audit-style chunk_ids CSV into the id
// list the provider payload carries. Unparseable / non-positive segments
// are skipped — never guessed (the feedbackChunkRefs precedent).
func parseTicketChunkIDs(csv string) []int64 {
	if csv == "" {
		return nil
	}
	fields := strings.Split(csv, ",")
	ids := make([]int64, 0, len(fields))
	for _, field := range fields {
		id, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64)
		if err != nil || id <= 0 {
			continue // 不可解析的段跳过 — never guess a chunk id
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// actionTicketDispatchSucceeded / actionTicketDispatchFailed are the audit
// actions for one settled dispatch series (TKT-02). They are the ONLY
// record of the outbound outcome: the ticket row is INSERT-only, so a
// webhook result can never rewrite what the user was told at accept time —
// the trail appends, it never edits.
const (
	actionTicketDispatchSucceeded = "action_ticket_dispatch_succeeded"
	actionTicketDispatchFailed    = "action_ticket_dispatch_failed"
)

// ticketDispatchAudit is the FrozenArgs snapshot of one settled dispatch —
// the durable "what happened to ticket N" record inside the admin audit
// trail. ticket_number rides along so the trail is readable without a join
// back into the tickets table.
type ticketDispatchAudit struct {
	// TicketID is the persisted tickets row id (the "TKT-{id}" source).
	TicketID int64 `json:"ticket_id"`
	// TicketNumber is the derived acceptance number shown to the user.
	TicketNumber string `json:"ticket_number"`
	// Provider is the provider name configured at insert time.
	Provider string `json:"provider"`
	// Attempts is the number of HTTP attempts the provider actually made.
	Attempts int `json:"attempts"`
	// ExternalRef is the remote ticket id; empty = no external receipt.
	ExternalRef string `json:"external_ref,omitempty"`
	// Error is the settled failure; for a panicking provider it is the
	// recovered value prefixed with "panic:".
	Error string `json:"error,omitempty"`
}

// dispatchForTicket hands one persisted ticket row to the wired provider
// in a background goroutine (TKT-04). Structural isolation, not convention:
//
//   - only a "queued" row dispatches — the "none" provider means 仅落库不
//     外发, and no goroutine is started for it at all;
//   - the goroutine derives its context from context.Background() with the
//     whole-dispatch budget, NOT from the HTTP request: the user was told
//     the ticket exists, so an abandoned request must not abort it;
//   - a panicking provider is contained by defer recover — the chat and
//     feedback main links must survive a broken outbound integration
//     (T-29-07: no shared state, no crash path);
//   - the settled outcome — success, failure, or panic — is recorded as an
//     INSERT-only action_ticket_dispatch_* audit row; the ticket row itself
//     has no UPDATE path anywhere in this package.
func (s *Deps) dispatchForTicket(row store.TicketRow, description string) {
	if row.DispatchStatus != ticketDispatchQueued {
		return // "none" = store-only: no outbound traffic, no goroutine
	}
	if s.TicketProvider == nil {
		return // defensive: the handler gates this; never dispatch into nil
	}
	go func() {
		// Panic containment is ALSO an audit event: a provider that blows
		// up mid-dispatch produced no settled result, which is exactly the
		// failed-dispatch shape the operator needs to see.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WARN] server: ticket dispatch goroutine panicked (ticket %s, provider=%s): %v (server survives; recorded as a failed dispatch)",
					ticketNumberFor(row.ID), row.Provider, r)
				s.putTicketDispatchAuditRow(actionTicketDispatchFailed, ticketDispatchAudit{
					TicketID:     row.ID,
					TicketNumber: ticketNumberFor(row.ID),
					Provider:     row.Provider,
					Error:        fmt.Sprintf("panic: %v", r),
				})
			}
		}()

		budget := s.TicketDispatchBudget
		if budget <= 0 {
			budget = defaultTicketDispatchBudget
		}
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()

		t := ticket.Ticket{
			Number:      ticketNumberFor(row.ID),
			SessionID:   row.SessionID,
			TurnID:      row.TurnID,
			Description: description,
			ChunkIDs:    parseTicketChunkIDs(row.ChunkIDs),
			SkillID:     row.SkillID,
		}
		result, err := s.TicketProvider.Dispatch(ctx, t)
		if err != nil {
			log.Printf("[WARN] server: ticket dispatch failed (ticket %s, provider=%s): %v", t.Number, row.Provider, err)
			s.putTicketDispatchAuditRow(actionTicketDispatchFailed, ticketDispatchAudit{
				TicketID:     row.ID,
				TicketNumber: t.Number,
				Provider:     row.Provider,
				Error:        err.Error(),
			})
			return
		}
		log.Printf("[OK] server: ticket dispatched (ticket %s, provider=%s, attempts=%d, external_ref=%q)",
			t.Number, row.Provider, result.Attempts, result.ExternalRef)
		s.putTicketDispatchAuditRow(actionTicketDispatchSucceeded, ticketDispatchAudit{
			TicketID:     row.ID,
			TicketNumber: t.Number,
			Provider:     row.Provider,
			Attempts:     result.Attempts,
			ExternalRef:  result.ExternalRef,
		})
	}()
}

// putTicketDispatchAuditRow writes the INSERT-only audit row for one
// settled dispatch with the outcome snapshot in FrozenArgs.
//
// ActorType is "admin" so GET /api/admin/audit (ADM-06) surfaces the row —
// admin_audit.go filters on ActorType, and a dispatch trail invisible to
// the admin view would be a trail nobody can follow. UserID is "system":
// the real actor is the platform's background dispatcher, not an admin
// user, and that distinction must survive in the row itself.
//
// Best-effort, mirroring putFeedbackAuditRow / putKBAdminAuditRow: a failed
// write is logged at WARN and never panics the goroutine into a second
// recover round — the ticket row is already persisted and the dispatch has
// already settled; losing the trail row is an observability gap, not a
// reason to fabricate a failure the outbound system never reported.
func (s *Deps) putTicketDispatchAuditRow(action string, payload ticketDispatchAudit) {
	if s.Store == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WARN] server: ticket dispatch audit row marshal (%s ticket=%s): %v (row skipped)", action, payload.TicketNumber, err)
		return
	}
	if _, err := s.Store.PutAudit(context.Background(), store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "system",
		Action:     action,
		ActorType:  "admin",
		FrozenArgs: b,
	}); err != nil {
		log.Printf("[WARN] server: ticket dispatch audit row write (%s ticket=%s): %v (best-effort, dispatch already settled)", action, payload.TicketNumber, err)
	}
}
