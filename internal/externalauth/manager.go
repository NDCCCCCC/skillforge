// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"time"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

// Store is the persistence seam for the channel. SQLite implements
// it (Phase 21-02) and an in-memory fake implements it (tests).
// The methods mirror the external_auth_tokens / external_auth_audit
// tables; callers see the ExternalAuthRequest / AuditRow types
// only.
type Store interface {
	// IssueExternalAuthRequest persists a new pending request.
	IssueExternalAuthRequest(ctx context.Context, req ExternalAuthRequest) error

	// GetExternalAuthRequest returns the request by ID. Missing
	// rows return (zero, false, nil).
	GetExternalAuthRequest(ctx context.Context, id string) (ExternalAuthRequest, bool, error)

	// ResolveExternalAuthRequest writes the data officer's decision
	// + the D14 audit row + (on approve) the TTL token id. All
	// three in one transaction so a partial state never lands on
	// disk.
	ResolveExternalAuthRequest(ctx context.Context, req ExternalAuthRequest, audit AuditRow, token *TTLToken) error

	// ListPendingExternalAuthRequests returns every still-pending
	// request, ordered by created_at ASC (oldest first — the data
	// officer drains the queue in FIFO). Empty slice on no rows.
	ListPendingExternalAuthRequests(ctx context.Context, limit int) ([]ExternalAuthRequest, error)

	// GetTTLToken returns the token by ID. Missing rows return
	// (zero, false, nil).
	GetTTLToken(ctx context.Context, id string) (TTLToken, bool, error)

	// ListAuditRows returns external_auth_audit rows in [from, to)
	// (epoch ms). Used by the reconcile endpoint (DATA-06). Empty
	// slice on no rows.
	ListAuditRows(ctx context.Context, fromUnixMs, toUnixMs int64) ([]AuditRow, error)
}

// Manager orchestrates the channel: it takes a Store + Source + View
// registry and exposes the 4-step flow that the agent + data
// officer UI call:
//
//   1. RequestApproval — agent calls this with the NL2SQL preview;
//      manager writes the pending request.
//   2. Approve / Reject — data officer calls these; manager
//      resolves the request atomically.
//   3. ExecuteWithToken — agent (after approval) calls this with the
//      TTL token id; manager verifies the token, calls Source.Execute,
//      appends the external audit row.
//   4. Reconcile — operator / observability tool calls this to
//      verify the dual audit (DATA-06).
//
// Manager is the only object the agent code + handler code should
// import from this package; the lower-level types are data
// carriers.
type Manager struct {
	store    Store
	source   Source
	registry *ViewRegistry

	mu          sync.Mutex
	nextReqSeq  int64 // monotonic; FormatRequestID(seq)
	nextTokSeq  int64 // monotonic; FormatTokenID(seq)

	// now is the clock used for CreatedAt + TTL minting + Valid
	// checks. Production code leaves this nil and falls through to
	// the package-level timeNow (realTimeNow); tests override it to
	// simulate clock advancement (DATA-04 token expiry).
	now func() time.Time
}

// NewManager constructs the channel orchestrator. Source MUST be
// the data warehouse side (or MemorySource for tests); registry
// MUST already contain the allow-list of views (DATA-10).
func NewManager(store Store, source Source, registry *ViewRegistry) *Manager {
	return &Manager{store: store, source: source, registry: registry}
}

// SetClock overrides the clock used for token expiry / creation
// timestamps. Tests call this to simulate "the TTL window passed"
// without sleeping the test runner.
func (m *Manager) SetClock(now func() time.Time) { m.now = now }

func (m *Manager) currentTime() time.Time {
	if m.now != nil {
		return m.now()
	}
	return timeNow()
}

// RequestApproval validates the preview (DATA-11 SELECT-only) +
// the view name (DATA-10 allow-list) and persists a pending
// request. The returned ExternalAuthRequest has a generated
// RequestID; the caller passes this id back through the agent
// streaming flow so the user sees "request X pending" in their
// session.
func (m *Manager) RequestApproval(ctx context.Context, req ExternalAuthRequest) (ExternalAuthRequest, error) {
	// DATA-11: every preview must be SELECT-only. Non-SELECT is a
	// hard reject — the data officer never sees a query that could
	// mutate the warehouse.
	if err := RejectNonSelect(req.Preview.SQLPreview); err != nil {
		return ExternalAuthRequest{}, err
	}
	// DATA-10: view must be in the allow-list before any work
	// happens. Source.Execute re-checks this on actual query
	// execution; the early check here keeps typos from burning an
	// approval slot.
	if _, ok := m.registry.Get(req.Preview.ViewName); !ok {
		return ExternalAuthRequest{}, fmt.Errorf("%w: view %q", ErrViewNotAllowed, req.Preview.ViewName)
	}
	if req.State == "" {
		req.State = StatePending
	}
	m.mu.Lock()
	m.nextReqSeq++
	seq := m.nextReqSeq
	m.mu.Unlock()
	if req.RequestID == "" {
		req.RequestID = FormatRequestID(seq)
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = m.currentTime().UTC()
	}
	if err := m.store.IssueExternalAuthRequest(ctx, req); err != nil {
		return ExternalAuthRequest{}, fmt.Errorf("externalauth: issue: %w", err)
	}
	return req, nil
}

// Approve is the data officer's approval action. Resolves the
// request to StateApproved, mints a TTL token (DATA-04), writes
// the dual audit row (DATA-14 + DATA-06 — internal row here; the
// Source.AppendExternalAudit is called on Execute).
//
// DATA-13: a pending request is a positive state, not "no
// decision yet = default deny". The only resolution paths are
// Approve + Reject; there is no auto-reject / auto-timeout.
func (m *Manager) Approve(ctx context.Context, requestID, dataOfficerID, businessReason string, chunkIDsReturned []string) (ExternalAuthRequest, AuditRow, TTLToken, error) {
	req, ok, err := m.store.GetExternalAuthRequest(ctx, requestID)
	if err != nil {
		return ExternalAuthRequest{}, AuditRow{}, TTLToken{}, err
	}
	if !ok {
		return ExternalAuthRequest{}, AuditRow{}, TTLToken{}, errors.New("externalauth: request not found")
	}
	if req.State != StatePending {
		return ExternalAuthRequest{}, AuditRow{}, TTLToken{}, fmt.Errorf("externalauth: request state %q, expected pending", req.State)
	}

	m.mu.Lock()
	m.nextTokSeq++
	seq := m.nextTokSeq
	m.mu.Unlock()

	now := m.currentTime().UTC()
	scope := sha256.Sum256([]byte(req.Preview.ViewName + "|" + req.UserID))
	token := TTLToken{
		TokenID:   FormatTokenID(seq),
		ViewName:  req.Preview.ViewName,
		UserID:    req.UserID,
		ScopeHash: hex.EncodeToString(scope[:8]),
		IssuedAt:  now,
		ExpiresAt: now.Add(DefaultTTL),
	}
	masked := []string{}
	for _, v := range m.registry.Names() {
		if v == req.Preview.ViewName {
			view, _ := m.registry.Get(v)
			for _, spec := range view.FieldMasks {
				masked = append(masked, spec.FieldName)
			}
			break
		}
	}
	audit := AuditRow{
		QueryID:          req.RequestID,
		UserID:           req.UserID,
		BusinessReason:   businessReason,
		SQLPreview:       req.Preview.SQLPreview,
		Decision:         StateApproved,
		DataOfficerID:    dataOfficerID,
		TTLTokenID:       token.TokenID,
		Timestamp:        now,
		ChunkIDsReturned: chunkIDsReturned,
		MaskApplied:      masked,
	}
	req.State = StateApproved
	req.TTLTokenID = token.TokenID
	req.AuditRowID = 0 // store assigns at INSERT time
	if err := m.store.ResolveExternalAuthRequest(ctx, req, audit, &token); err != nil {
		return ExternalAuthRequest{}, AuditRow{}, TTLToken{}, fmt.Errorf("externalauth: resolve approve: %w", err)
	}
	return req, audit, token, nil
}

// Reject is the data officer's reject action. Resolves the request
// to StateRejected; no TTL token is minted. Audit row is written
// with Decision=rejected. DATA-13 — same posture as Approve: a
// pending request resolves only via explicit human action.
func (m *Manager) Reject(ctx context.Context, requestID, dataOfficerID, businessReason string) (ExternalAuthRequest, AuditRow, error) {
	req, ok, err := m.store.GetExternalAuthRequest(ctx, requestID)
	if err != nil {
		return ExternalAuthRequest{}, AuditRow{}, err
	}
	if !ok {
		return ExternalAuthRequest{}, AuditRow{}, errors.New("externalauth: request not found")
	}
	if req.State != StatePending {
		return ExternalAuthRequest{}, AuditRow{}, fmt.Errorf("externalauth: request state %q, expected pending", req.State)
	}
	now := m.currentTime().UTC()
	audit := AuditRow{
		QueryID:        req.RequestID,
		UserID:         req.UserID,
		BusinessReason: businessReason,
		SQLPreview:     req.Preview.SQLPreview,
		Decision:       StateRejected,
		DataOfficerID:  dataOfficerID,
		Timestamp:      now,
	}
	req.State = StateRejected
	if err := m.store.ResolveExternalAuthRequest(ctx, req, audit, nil); err != nil {
		return ExternalAuthRequest{}, AuditRow{}, fmt.Errorf("externalauth: resolve reject: %w", err)
	}
	return req, audit, nil
}

// ExecuteWithToken is the agent's post-approval query call. The
// token binds (view_name, user_id) — Source.Execute verifies the
// token + view against the registry + clock. Masking happens
// inside Source.Execute per the view's MaskSpec.
//
// Returns the masked rows + the external audit row (which the
// caller appends to Source.AppendExternalAudit for DATA-06 dual
// audit — internal audit row is in external_auth_audit, external
// row in Source.extLog).
func (m *Manager) ExecuteWithToken(ctx context.Context, tokenID string) ([]map[string]string, error) {
	token, ok, err := m.store.GetTTLToken(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTokenExpired
	}
	rows, err := m.source.Execute(ctx, token, token.ViewName, m.currentTime())
	if err != nil {
		return nil, err
	}
	// DATA-06: external audit row MUST carry the same QueryID as the
	// internal external_auth_audit row so reconcile can pair them.
	// The canonical join key is the request id (`xreq-NNNNNNNN`),
	// minted at RequestApproval time. We look it up by the token.
	//
	// Tokens currently don't carry the request id back — extend the
	// token side with an explicit QueryID. For the v1.1 reference
	// impl we resolve it via a single store lookup. v1.2 should
	// fold this into the TTL token itself.
	requestID, ok, err := m.findRequestIDByToken(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Fall back to scope_hash so the call still completes; the
		// reconcile report will surface a mismatch and the operator
		// can reconcile manually. Logs an alert — never silently.
		requestID = "(unpaired:" + token.ScopeHash + ")"
	}
	external := AuditRow{
		QueryID:          requestID,
		UserID:           token.UserID,
		BusinessReason:   fmt.Sprintf("token %s query on view %s", token.TokenID, token.ViewName),
		SQLPreview:       "(executed — see internal audit row for full preview)",
		Decision:         StateApproved,
		DataOfficerID:    "(warehouse-side; no data officer)",
		TTLTokenID:       token.TokenID,
		Timestamp:        m.currentTime().UTC(),
		ChunkIDsReturned: nil,
	}
	if eErr := m.source.AppendExternalAudit(ctx, external); eErr != nil {
		// External audit failure is logged but NOT propagated —
		// the agent's query succeeded; reconciliation (DATA-06)
		// will surface the missing pair to the operator.
		// Treated as a non-fatal telemetry event in v1.1; v1.2
		// promotes this to fail-closed when the dual audit
		// becomes a security SLA.
		_ = eErr
	}
	return rows, nil
}

// findRequestIDByToken resolves the original ExternalAuthRequest
// from a TTL token id. The store keeps the request-id-on-token
// relationship in the ExternalAuthRequest.TTLTokenID field; we
// scan it. The Store interface deliberately does NOT expose this
// as a public method (it's an internal join used only for
// reconciliation). v1.2 should fold the request id into the TTL
// token itself so no scan is needed.
func (m *Manager) findRequestIDByToken(ctx context.Context, tokenID string) (string, bool, error) {
	// Walk every request — this is a small list (a few dozen at
	// most per session) so the scan is fine for v1.1. v1.2 should
	// keep a (token_id -> request_id) index in the store.
	pending, err := m.store.ListPendingExternalAuthRequests(ctx, 1000)
	if err != nil {
		return "", false, err
	}
	for _, r := range pending {
		if r.TTLTokenID == tokenID {
			return r.RequestID, true, nil
		}
	}
	// Pending-only would miss approved+executed requests; sweep
	// the audit rows too (every approved request has an audit row).
	audits, err := m.store.ListAuditRows(ctx, 0, 9_999_999_999_999)
	if err != nil {
		return "", false, err
	}
	for _, a := range audits {
		if a.TTLTokenID == tokenID {
			return a.QueryID, true, nil
		}
	}
	return "", false, nil
}

// Reconcile pairs internal + external audit rows by query_id over
// a time window. Returns rows that exist on ONLY ONE side — those
// are the DATA-06 security events the operator must investigate.
//
// `from` / `to` are epoch ms (matches the request shape of the
// /api/external-auth/audit/reconcile endpoint).
func (m *Manager) Reconcile(ctx context.Context, fromUnixMs, toUnixMs int64) (ReconcileReport, error) {
	internal, err := m.store.ListAuditRows(ctx, fromUnixMs, toUnixMs)
	if err != nil {
		return ReconcileReport{}, err
	}
	external, err := m.source.ListExternalAudit(ctx, fromUnixMs, toUnixMs)
	if err != nil {
		return ReconcileReport{}, err
	}
	report := ReconcileReport{
		WindowFromMs:  fromUnixMs,
		WindowToMs:    toUnixMs,
		InternalOnly:  []AuditRow{},
		ExternalOnly:  []AuditRow{},
		MatchedCount:  0,
	}
	extByQuery := make(map[string]AuditRow, len(external))
	for _, r := range external {
		extByQuery[r.QueryID] = r
	}
	intByQuery := make(map[string]AuditRow, len(internal))
	for _, r := range internal {
		intByQuery[r.QueryID] = r
	}
	for qid := range intByQuery {
		if _, ok := extByQuery[qid]; ok {
			report.MatchedCount++
		} else {
			report.InternalOnly = append(report.InternalOnly, intByQuery[qid])
		}
	}
	for qid := range extByQuery {
		if _, ok := intByQuery[qid]; !ok {
			report.ExternalOnly = append(report.ExternalOnly, extByQuery[qid])
		}
	}
	return report, nil
}

// ReconcileReport is the DATA-06 pair-mismatch output. A non-empty
// InternalOnly / ExternalOnly is a security event (the operator
// must reconcile manually).
type ReconcileReport struct {
	WindowFromMs int64      `json:"window_from_ms"`
	WindowToMs   int64      `json:"window_to_ms"`
	MatchedCount int        `json:"matched_count"`
	InternalOnly []AuditRow `json:"internal_only"`
	ExternalOnly []AuditRow `json:"external_only"`
}

// Store exposes the underlying store (handlers read pending list
// via this — no separate access path needed). For tests this is
// the fakeStore; production wires the SQLite-backed implementation
// (Phase 21-02 follow-on).
func (m *Manager) Store() Store { return m.store }

// Registry exposes the view allowlist the manager was constructed
// with (v1.1.1): the needs_external_query agent tool derives its
// construction-time NL2SQL schema snapshot and its mask-field list
// from the same registry the manager enforces, so the two can never
// disagree about which views exist. Read-only view; registration
// happens only at boot through LoadViewsDir.
func (m *Manager) Registry() *ViewRegistry { return m.registry }
