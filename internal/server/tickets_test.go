// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 29 / Plan 29-06 — tests for the ticket HTTP surface (TKT-02 + TKT-04).
//
// Three layers of guarantee live in this file:
//
//  1. Behavior matrix (TestTicketsHandler) — the POST /api/tickets
//     contract hermetically: idempotent insert (one ticket per message,
//     D-29-14), the derived acceptance number ("TKT-{row id}"), the
//     dispatch_status semantics (none = store-only, queued = handed to
//     the background dispatcher), the validation gates, the uniform 404,
//     and both nil-dependency 503s (provider / store).
//  2. Non-blocking dispatch (TestTicketDispatchNonBlocking + the audit
//     row matrix) — TKT-04 is a STRUCTURAL fact: the response returns
//     while the webhook is still in flight, the goroutine derives its
//     context from context.Background() (never the request's), a panicking
//     provider cannot take the server down, and the settled outcome lands
//     as an INSERT-only action_ticket_dispatch_* audit row. The ticket row
//     itself has no UPDATE path anywhere.
//  3. Structural greps — the route is registered exactly once and the
//     dispatch path stays INSERT-only at source level.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/ticket"
)

// ---------------------------------------------------------------------------
// fakes — the nil-embed pattern: every store method the handler under test
// must NOT call hits the nil embedded interface and panics, so an accidental
// extra dependency fails loudly instead of silently passing.
// ---------------------------------------------------------------------------

// ticketsFakeStore is a ticket-aware fake overriding ONLY the methods the
// tickets handler + the background dispatcher call. Everything else panics
// through the embedded nil store.Store.
type ticketsFakeStore struct {
	store.Store

	mu sync.Mutex

	// turn space: sessionID -> turnID -> row (GetSessionTurn).
	turns map[string]map[int64]store.SessionMessage
	// getSessionTurnCalls counts every GetSessionTurn — the validation
	// matrix asserts it stays ZERO for input rejected before any SQL.
	getSessionTurnCalls int

	// audit rows + the filters the handler issued (attribution probe).
	auditRows       []store.AuditRow
	queryAuditCalls int

	// tickets: the INSERT-only write surface + its single read path.
	ticketRows     []store.TicketRow
	putTicketCalls int
	failPutTicket  bool

	// audit rows written by the dispatch goroutine (captured).
	putAuditRows []store.AuditRow
}

func newTicketsFakeStore() *ticketsFakeStore {
	return &ticketsFakeStore{turns: make(map[string]map[int64]store.SessionMessage)}
}

var _ store.Store = (*ticketsFakeStore)(nil)

func (f *ticketsFakeStore) GetSessionTurn(_ context.Context, sessionID string, id int64) (store.SessionMessage, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getSessionTurnCalls++
	m, ok := f.turns[sessionID][id]
	return m, ok, nil
}

func (f *ticketsFakeStore) QueryAudit(_ context.Context, filter store.AuditFilter) ([]store.AuditRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryAuditCalls++
	var out []store.AuditRow
	for _, r := range f.auditRows {
		if filter.SessionID != "" && r.SessionID != filter.SessionID {
			continue
		}
		if !filter.Until.IsZero() && !r.Timestamp.Before(filter.Until) {
			continue // mirrors the store's `ts < ?` (Until) semantics
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[:1], nil // Limit 1: the most recent audit row before the turn
}

func (f *ticketsFakeStore) PutTicket(_ context.Context, row store.TicketRow) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putTicketCalls++
	if f.failPutTicket {
		return false, errors.New("simulated ticket write outage")
	}
	for _, existing := range f.ticketRows {
		if existing.SessionID == row.SessionID && existing.TurnID == row.TurnID {
			return false, nil // UNIQUE(session_id, turn_id) replay
		}
	}
	saved := row
	saved.ID = int64(len(f.ticketRows) + 1) // AUTOINCREMENT stand-in
	f.ticketRows = append(f.ticketRows, saved)
	return true, nil
}

func (f *ticketsFakeStore) TicketByTurn(_ context.Context, sessionID string, turnID int64) (store.TicketRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.ticketRows {
		if r.SessionID == sessionID && r.TurnID == turnID {
			return r, true, nil
		}
	}
	return store.TicketRow{}, false, nil
}

func (f *ticketsFakeStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putAuditRows = append(f.putAuditRows, row)
	return int64(len(f.putAuditRows)), nil
}

// ticketSnapshot / putAuditSnapshot are the test-facing read-backs
// (lock-captured copies).
func (f *ticketsFakeStore) ticketSnapshot() []store.TicketRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.TicketRow, len(f.ticketRows))
	copy(out, f.ticketRows)
	return out
}

func (f *ticketsFakeStore) putAuditSnapshot() []store.AuditRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.AuditRow, len(f.putAuditRows))
	copy(out, f.putAuditRows)
	return out
}

// tkSeedTurn installs one assistant turn + its preceding audit row (the
// writeAuditAndTurn shape: the audit row lands BEFORE the assistant turn,
// carrying the citations CSV + skill id the server-side attribution copies).
func (f *ticketsFakeStore) tkSeedTurn(sessionID string, turnID int64, role string, turnTs time.Time, skillID, chunkIDs string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.turns[sessionID] == nil {
		f.turns[sessionID] = make(map[int64]store.SessionMessage)
	}
	f.turns[sessionID][turnID] = store.SessionMessage{ID: turnID, Role: role, Content: "conclusion", Ts: turnTs}
	if skillID != "" || chunkIDs != "" {
		f.auditRows = append(f.auditRows, store.AuditRow{
			Timestamp: turnTs.Add(-2 * time.Second),
			SessionID: sessionID,
			SkillID:   skillID,
			ChunkIDs:  chunkIDs,
		})
	}
}

// ---------------------------------------------------------------------------
// provider stub — injectable latency / failure / panic so the dispatch
// goroutine can be observed without any network.
// ---------------------------------------------------------------------------

type ticketStubProvider struct {
	mu    sync.Mutex
	calls []ticket.Ticket

	delay     time.Duration // sleep before settling (the non-blocking probe)
	panicWith any           // non-nil → panic (the recover probe)
	err       error         // settled failure
	result    ticket.DispatchResult

	// deadlineSeen records whether Dispatch received a ctx carrying a
	// deadline — the budgeted background ctx, which a bare request ctx
	// never has.
	deadlineSeen bool
}

func (p *ticketStubProvider) Dispatch(ctx context.Context, t ticket.Ticket) (ticket.DispatchResult, error) {
	_, ok := ctx.Deadline()
	p.mu.Lock()
	p.calls = append(p.calls, t)
	p.deadlineSeen = p.deadlineSeen || ok
	p.mu.Unlock()
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	if p.panicWith != nil {
		panic(p.panicWith)
	}
	if p.err != nil {
		return ticket.DispatchResult{}, p.err
	}
	return p.result, nil
}

func (p *ticketStubProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *ticketStubProvider) callsSnapshot() []ticket.Ticket {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]ticket.Ticket, len(p.calls))
	copy(out, p.calls)
	return out
}

func (p *ticketStubProvider) sawDeadline() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.deadlineSeen
}

// waitForTicketCalls polls until the dispatcher reached n calls or the
// deadline passes (bounded — a test must never hang on a missing goroutine).
func waitForTicketCalls(t *testing.T, p *ticketStubProvider, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p.callCount() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("dispatch goroutine never reached %d call(s) (have %d)", n, p.callCount())
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

// tkTestSession matches sessionIDRegex ([a-zA-Z0-9-]{8,128}).
const tkTestSession = "tktest-session-01"

// tkTurnTs is the assistant turn instant; the audit row precedes it.
var tkTurnTs = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

func newTicketsHertz(deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(":0"))
	h.POST("/api/tickets", deps.ticketsHandler)
	return h
}

// tkRequest performs one POST /api/tickets with a JSON body.
func tkRequest(h *server.Hertz, jsonBody string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	var body *ut.Body
	if jsonBody != "" {
		body = &ut.Body{Body: strings.NewReader(jsonBody), Len: len(jsonBody)}
	}
	return ut.PerformRequest(h.Engine, http.MethodPost, "/api/tickets", body, headers...)
}

// tkDecode decodes a response body into the anonymous wire shape the FE
// consumes (asserting the JSON contract, not internal Go type names).
func tkDecode(t *testing.T, resp *ut.ResponseRecorder, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), out), "body: %s", resp.Body.String())
}

// ---------------------------------------------------------------------------
// Task 1 — POST /api/tickets behavior matrix
// ---------------------------------------------------------------------------

// TestTicketsHandler is the TKT-02 write-endpoint behavior matrix: seven
// sub-cases covering the store-only path, the queued path, the idempotent
// replay, the validation gates, the uniform 404, and both 503
// nil-dependency postures.
func TestTicketsHandler(t *testing.T) {
	const validBody = `{"session_id":"tktest-session-01","turn_id":42,"description":"第 3 条引用已过期，请复核"}`

	t.Run("creates ticket with the none provider (store-only)", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11,12")
		none, err := ticket.NewProvider(ticket.ProviderNone, ticket.Config{})
		require.NoError(t, err)
		deps := &Deps{Store: fake, TicketProvider: none, TicketProviderName: ticket.ProviderNone, TicketDispatchBudget: time.Second}
		h := newTicketsHertz(deps)

		resp := tkRequest(h, validBody)
		require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			TicketNumber   string `json:"ticket_number"`
			DispatchStatus string `json:"dispatch_status"`
		}
		tkDecode(t, resp, &body)
		assert.Equal(t, "TKT-1", body.TicketNumber, "the acceptance number derives from the persisted row id")
		assert.Equal(t, "none", body.DispatchStatus)

		rows := fake.ticketSnapshot()
		require.Len(t, rows, 1)
		assert.Equal(t, tkTestSession, rows[0].SessionID)
		assert.EqualValues(t, 42, rows[0].TurnID)
		assert.Equal(t, "第 3 条引用已过期，请复核", rows[0].Description)
		assert.Equal(t, "compliance", rows[0].SkillID, "skill_id is the audit-row copy, not a client claim")
		assert.Equal(t, "11,12", rows[0].ChunkIDs)
		assert.Equal(t, "none", rows[0].Provider)
		assert.Equal(t, "none", rows[0].DispatchStatus)

		// none = 仅落库不外发: no goroutine is ever started for this row.
		time.Sleep(50 * time.Millisecond)
		assert.Len(t, fake.ticketSnapshot(), 1, "exactly one row, and no second write after the settle window")
	})

	t.Run("queues dispatch with the webhook provider", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11,12")
		stub := &ticketStubProvider{result: ticket.DispatchResult{ExternalRef: "EXT-9", Attempts: 1}}
		deps := &Deps{Store: fake, TicketProvider: stub, TicketProviderName: ticket.ProviderWebhook, TicketDispatchBudget: 5 * time.Second}
		h := newTicketsHertz(deps)

		resp := tkRequest(h, validBody)
		require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			TicketNumber   string `json:"ticket_number"`
			DispatchStatus string `json:"dispatch_status"`
		}
		tkDecode(t, resp, &body)
		assert.Equal(t, "TKT-1", body.TicketNumber)
		assert.Equal(t, "queued", body.DispatchStatus)

		rows := fake.ticketSnapshot()
		require.Len(t, rows, 1)
		assert.Equal(t, "webhook", rows[0].Provider)
		assert.Equal(t, "queued", rows[0].DispatchStatus)

		// The queued row hands off to the background dispatcher.
		waitForTicketCalls(t, stub, 1)
		dispatched := stub.callsSnapshot()[0]
		assert.Equal(t, "TKT-1", dispatched.Number)
		assert.Equal(t, tkTestSession, dispatched.SessionID)
		assert.EqualValues(t, 42, dispatched.TurnID)
		assert.Equal(t, "第 3 条引用已过期，请复核", dispatched.Description)
		assert.Equal(t, "compliance", dispatched.SkillID)
		assert.Equal(t, []int64{11, 12}, dispatched.ChunkIDs, "the audit CSV becomes the chunk id list")
	})

	t.Run("replay returns already_exists with the existing number", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11")
		stub := &ticketStubProvider{}
		deps := &Deps{Store: fake, TicketProvider: stub, TicketProviderName: ticket.ProviderWebhook, TicketDispatchBudget: 5 * time.Second}
		h := newTicketsHertz(deps)

		first := tkRequest(h, validBody)
		require.Equal(t, http.StatusCreated, first.Code)
		waitForTicketCalls(t, stub, 1)

		// Same (session, turn), different description: the UNIQUE key
		// swallows the insert and the FIRST number is what comes back.
		replay := tkRequest(h,
			`{"session_id":"tktest-session-01","turn_id":42,"description":"second attempt at the same message"}`)
		require.Equal(t, http.StatusOK, replay.Code, "a replay is not a creation — 200, never a second 201")

		var body map[string]any
		tkDecode(t, replay, &body)
		assert.Equal(t, "already_exists", body["status"])
		assert.Equal(t, "TKT-1", body["ticket_number"], "the replay surfaces the FIRST ticket number")
		assert.NotContains(t, replay.Body.String(), "dispatch_status", "the replay envelope carries no dispatch half")

		assert.Len(t, fake.ticketSnapshot(), 1, "exactly one ticket row per (session, turn)")
		assert.Equal(t, 2, fake.putTicketCalls, "the replay still reached the store (DB constraint is the truth)")

		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 1, stub.callCount(), "a replay must not trigger a second dispatch")
	})

	t.Run("validation gates reject with zero writes", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11")
		deps := &Deps{Store: fake, TicketProviderName: ticket.ProviderNone}
		h := newTicketsHertz(deps)

		tooLong := strings.Repeat("细", 4001)
		payload, err := json.Marshal(map[string]any{
			"session_id": tkTestSession, "turn_id": 42, "description": tooLong,
		})
		require.NoError(t, err)

		for name, body := range map[string]string{
			"empty description":   `{"session_id":"tktest-session-01","turn_id":42,"description":""}`,
			"blank description":   `{"session_id":"tktest-session-01","turn_id":42,"description":"   "}`,
			"oversized body":      string(payload),
			"bad session shape":   `{"session_id":"bad_id!","turn_id":42,"description":"需要复核"}`,
			"zero turn id":        `{"session_id":"tktest-session-01","turn_id":0,"description":"需要复核"}`,
			"negative turn id":    `{"session_id":"tktest-session-01","turn_id":-5,"description":"需要复核"}`,
			"unparseable body":    `{not json`,
			"missing description": `{"session_id":"tktest-session-01","turn_id":42}`,
		} {
			resp := tkRequest(h, body)
			assert.Equal(t, http.StatusBadRequest, resp.Code, "%s must 400 (body: %s)", name, resp.Body.String())
		}

		assert.Empty(t, fake.ticketSnapshot(), "a rejected ticket writes nothing")
		assert.Equal(t, 0, fake.putTicketCalls, "zero PutTicket calls on every rejection above")
		assert.Equal(t, 0, fake.getSessionTurnCalls, "input rejected by shape must never issue a turn lookup")
	})

	t.Run("unknown, non-assistant and cross-session turns are a uniform 404", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "user", tkTurnTs, "", "")            // exists, wrong role
		fake.tkSeedTurn("other-session-9999", 7, "assistant", tkTurnTs, "", "") // exists, other session
		// A wired provider is required to REACH the turn gate: the dependency
		// 503s run before it, so an unwired deployment never leaks whether a
		// turn exists.
		none, err := ticket.NewProvider(ticket.ProviderNone, ticket.Config{})
		require.NoError(t, err)
		deps := &Deps{Store: fake, TicketProvider: none, TicketProviderName: ticket.ProviderNone}
		h := newTicketsHertz(deps)

		cases := map[string]string{
			"unknown turn":    `{"session_id":"tktest-session-01","turn_id":999,"description":"需要复核"}`,
			"user turn":       `{"session_id":"tktest-session-01","turn_id":42,"description":"需要复核"}`,
			"foreign session": `{"session_id":"tktest-session-01","turn_id":7,"description":"需要复核"}`,
		}
		var bodies []string
		for name, body := range cases {
			resp := tkRequest(h, body)
			require.Equal(t, http.StatusNotFound, resp.Code, "%s must 404 (body: %s)", name, resp.Body.String())
			bodies = append(bodies, resp.Body.String())
			assert.Empty(t, fake.ticketSnapshot(), "%s writes no ticket", name)
		}
		// 不区分原因: all three envelopes are byte-identical, so the status
		// code never leaks WHY a turn is not ticketable (T-29-06 posture).
		require.Len(t, bodies, 3)
		assert.Equal(t, bodies[0], bodies[1])
		assert.Equal(t, bodies[1], bodies[2])
	})

	t.Run("unwired provider answers 503 with the ProviderNotWiredError body", func(t *testing.T) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11")
		deps := &Deps{Store: fake} // TicketProvider nil — server started without a provider
		h := newTicketsHertz(deps)

		resp := tkRequest(h, validBody)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Error string `json:"error"`
		}
		tkDecode(t, resp, &body)
		assert.Equal(t, ticket.ProviderNotWiredError, body.Error)

		assert.Empty(t, fake.ticketSnapshot(), "an unwired provider never persists a ticket")
		assert.Equal(t, 0, fake.putTicketCalls)
	})

	t.Run("unwired store answers 503 fail-closed", func(t *testing.T) {
		stub := &ticketStubProvider{}
		deps := &Deps{TicketProvider: stub, TicketProviderName: ticket.ProviderWebhook} // Store nil
		h := newTicketsHertz(deps)

		resp := tkRequest(h, validBody)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body: %s", resp.Body.String())
		assert.Contains(t, resp.Body.String(), "not wired")
		assert.Equal(t, 0, stub.callCount(), "an unwired store never dispatches")
	})
}

// ---------------------------------------------------------------------------
// Task 2 — non-blocking dispatch + INSERT-only audit trail
// ---------------------------------------------------------------------------

// waitForAuditRows polls until the dispatcher wrote n audit rows or the
// deadline passes (bounded — a test must never hang on a missing goroutine).
func waitForAuditRows(t *testing.T, fake *ticketsFakeStore, n int) []store.AuditRow {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows := fake.putAuditSnapshot()
		if len(rows) >= n {
			return rows
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("dispatch goroutine never wrote %d audit row(s) (have %d)", n, len(fake.putAuditSnapshot()))
	return nil
}

// dispatchTestDeps is the webhook-wired Deps the dispatch tests share.
func dispatchTestDeps(fake *ticketsFakeStore, provider ticket.Provider) *Deps {
	return &Deps{
		Store:                fake,
		TicketProvider:       provider,
		TicketProviderName:   ticket.ProviderWebhook,
		TicketDispatchBudget: 5 * time.Second,
	}
}

const dispatchTestBody = `{"session_id":"tktest-session-01","turn_id":42,"description":"第 3 条引用已过期，请复核"}`

// TestTicketDispatchAuditRows locks the settled-outcome trail: every
// dispatch — success, failure, or panic — lands as an INSERT-only
// action_ticket_dispatch_* audit row with ActorType "admin" (the
// adminAuditHandler visibility filter) and UserID "system" (the real actor
// identity: the platform itself, not an admin user). The ticket row is
// never touched on any path — the store ships no UPDATE verb for it.
func TestTicketDispatchAuditRows(t *testing.T) {
	newFixture := func(provider ticket.Provider) (*ticketsFakeStore, *server.Hertz) {
		fake := newTicketsFakeStore()
		fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11,12")
		return fake, newTicketsHertz(dispatchTestDeps(fake, provider))
	}
	decodeFrozenArgs := func(t *testing.T, raw []byte) map[string]any {
		t.Helper()
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload), "FrozenArgs must be a JSON snapshot: %s", raw)
		return payload
	}

	t.Run("success writes action_ticket_dispatch_succeeded", func(t *testing.T) {
		fake, h := newFixture(&ticketStubProvider{result: ticket.DispatchResult{ExternalRef: "EXT-77", Attempts: 3}})

		resp := tkRequest(h, dispatchTestBody)
		require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())

		rows := waitForAuditRows(t, fake, 1)
		require.Len(t, rows, 1)
		assert.Equal(t, "action_ticket_dispatch_succeeded", rows[0].Action)
		assert.Equal(t, "admin", rows[0].ActorType, "ActorType=admin is the /api/admin/audit visibility filter")
		assert.Equal(t, "system", rows[0].UserID, "the platform dispatched it, not an admin user")

		payload := decodeFrozenArgs(t, rows[0].FrozenArgs)
		assert.EqualValues(t, 1, payload["ticket_id"])
		assert.Equal(t, "webhook", payload["provider"])
		assert.EqualValues(t, 3, payload["attempts"])
		assert.Equal(t, "EXT-77", payload["external_ref"])

		// INSERT-only discipline: the settled outcome lives in the audit
		// row, the ticket row keeps its insert-time shape forever.
		after := fake.ticketSnapshot()
		require.Len(t, after, 1)
		assert.Equal(t, "webhook", after[0].Provider)
		assert.Equal(t, "queued", after[0].DispatchStatus)
		assert.EqualValues(t, 1, after[0].ID)
	})

	t.Run("failure writes action_ticket_dispatch_failed and keeps the row", func(t *testing.T) {
		fake, h := newFixture(&ticketStubProvider{err: errors.New("connection refused")})

		resp := tkRequest(h, dispatchTestBody)
		require.Equal(t, http.StatusCreated, resp.Code, "an outbound failure must never fail the accepted ticket")

		rows := waitForAuditRows(t, fake, 1)
		require.Len(t, rows, 1)
		assert.Equal(t, "action_ticket_dispatch_failed", rows[0].Action)
		assert.Equal(t, "admin", rows[0].ActorType)
		assert.Equal(t, "system", rows[0].UserID)

		payload := decodeFrozenArgs(t, rows[0].FrozenArgs)
		assert.EqualValues(t, 1, payload["ticket_id"])
		assert.Equal(t, "webhook", payload["provider"])
		assert.Contains(t, payload["error"], "connection refused")

		after := fake.ticketSnapshot()
		require.Len(t, after, 1)
		assert.Equal(t, "queued", after[0].DispatchStatus, "no UPDATE path rewrites the ticket row on failure")
	})

	t.Run("panic is contained and recorded as a failed dispatch", func(t *testing.T) {
		fake, h := newFixture(&ticketStubProvider{panicWith: "webhook exploded"})

		resp := tkRequest(h, dispatchTestBody)
		require.Equal(t, http.StatusCreated, resp.Code, "the handler must already have returned before the panic")

		// If the goroutine's recover were missing, this process would be
		// dead before the assertion below could run — surviving here IS
		// the assertion (T-29-07).
		rows := waitForAuditRows(t, fake, 1)
		require.Len(t, rows, 1)
		assert.Equal(t, "action_ticket_dispatch_failed", rows[0].Action)
		assert.Equal(t, "admin", rows[0].ActorType)

		payload := decodeFrozenArgs(t, rows[0].FrozenArgs)
		assert.Contains(t, payload["error"], "panic")
		assert.Contains(t, payload["error"], "webhook exploded")
	})

	t.Run("dispatch context is budgeted and independent of the request", func(t *testing.T) {
		// A bare request context carries no deadline; the dispatcher's
		// context.Background()+WithTimeout budget always does. Observing a
		// deadline inside Dispatch proves the goroutine is not riding the
		// HTTP request's lifetime — abandoning the request cannot abort a
		// ticket the user was told exists.
		stub := &ticketStubProvider{delay: 50 * time.Millisecond}
		fake, h := newFixture(stub)

		resp := tkRequest(h, dispatchTestBody)
		require.Equal(t, http.StatusCreated, resp.Code)

		waitForTicketCalls(t, stub, 1)
		assert.True(t, stub.sawDeadline(), "the dispatch ctx must carry the budget deadline")
		assert.Equal(t, "queued", fake.ticketSnapshot()[0].DispatchStatus)
	})
}

// TestTicketDispatchNonBlocking is the TKT-04 structural proof: the 201 is
// on the wire long before a slow endpoint settles, and the dispatch still
// happens afterwards.
func TestTicketDispatchNonBlocking(t *testing.T) {
	fake := newTicketsFakeStore()
	fake.tkSeedTurn(tkTestSession, 42, "assistant", tkTurnTs, "compliance", "11,12")
	stub := &ticketStubProvider{delay: 300 * time.Millisecond, result: ticket.DispatchResult{ExternalRef: "EXT-1", Attempts: 1}}
	h := newTicketsHertz(dispatchTestDeps(fake, stub))

	start := time.Now()
	resp := tkRequest(h, dispatchTestBody)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())
	var body struct {
		DispatchStatus string `json:"dispatch_status"`
	}
	tkDecode(t, resp, &body)
	assert.Equal(t, "queued", body.DispatchStatus)

	assert.Less(t, elapsed, 150*time.Millisecond,
		"the 201 must not wait for the outbound dispatch (provider needs 300ms, response took %s)", elapsed)

	waitForTicketCalls(t, stub, 1)
	assert.True(t, stub.sawDeadline(), "the dispatch ctx must carry the budget deadline (background-derived, not the request ctx)")
}

// ---------------------------------------------------------------------------
// structural locks — dispatch surface + assembly wiring
// ---------------------------------------------------------------------------

// TestTicketsDispatchSourceDiscipline keeps the dispatch surface honest at
// source level: no row mutation, no direct SQL, the background-derived
// context and the panic containment present (comment lines are exempt —
// the lock is on statements, not prose).
func TestTicketsDispatchSourceDiscipline(t *testing.T) {
	src, err := os.ReadFile("tickets.go")
	require.NoError(t, err)
	lines := strings.Split(string(src), "\n")

	var offenders []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}
		for _, verb := range []string{"UPDATE ", "DELETE ", "Exec("} {
			if strings.Contains(line, verb) {
				offenders = append(offenders, fmt.Sprintf("line %d contains %q: %s", i+1, verb, trimmed))
			}
		}
	}
	assert.Empty(t, offenders, "tickets.go must stay INSERT-only and SQL-free:\n%s", strings.Join(offenders, "\n"))

	s := string(src)
	assert.Contains(t, s, "context.Background()", "the dispatch goroutine must not ride the request context")
	assert.Contains(t, s, "recover()", "a panicking provider must be contained (T-29-07)")
}

// TestTicketAssemblyWiring locks the env-driven assembly: five knobs, the
// registry constructor, and TWO fail-fast branches (webhook without a URL,
// unknown provider name) in cmd/server/main.go.
func TestTicketAssemblyWiring(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err, "the assembly lock reads the main package source relative to internal/server")
	s := string(src)

	for _, env := range []string{
		"SKILLFORGE_TICKET_PROVIDER",
		"SKILLFORGE_TICKET_WEBHOOK_URL",
		"SKILLFORGE_TICKET_WEBHOOK_TIMEOUT_MS",
		"SKILLFORGE_TICKET_WEBHOOK_RETRIES",
		"SKILLFORGE_TICKET_DISPATCH_BUDGET_MS",
	} {
		assert.Contains(t, s, env, "%s must be read at the assembly point (zero hardcoding)", env)
	}
	assert.Contains(t, s, "ticket.NewProvider", "the provider must come from the registry, never hand-rolled")

	fatals := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "log.Fatalf") && strings.Contains(strings.ToLower(line), "ticket") {
			fatals++
		}
	}
	assert.GreaterOrEqual(t, fatals, 2,
		"webhook-without-URL and unknown provider must both log.Fatalf (D16-08-D: no silent degradation)")
}

// TestTicketsRouteRegisteredOnce keeps the route table honest: POST
// /api/tickets is bound exactly once, as a POST, in server.go.
func TestTicketsRouteRegisteredOnce(t *testing.T) {
	src, err := os.ReadFile("server.go")
	require.NoError(t, err)
	s := string(src)
	assert.Equal(t, 1, strings.Count(s, `h.POST("/api/tickets"`),
		"POST /api/tickets must be registered exactly once in server.go")
	assert.NotContains(t, s, `h.GET("/api/tickets"`, "the ticket write endpoint must never be reachable via GET")
}
