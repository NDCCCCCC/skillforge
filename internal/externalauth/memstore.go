// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// MemoryStore is the v1.1 reference implementation of Store: an
// in-process, non-durable persistence seam for the External
// Authorized Channel. It exists because the only Store
// implementation that shipped with Phase 21 lived inside a _test.go
// file, which made the channel unconstructable from cmd/server
// (MILESTONE_SUMMARY-v1.1 Gap 1) — the manager needs a Store at
// assembly time, and a test file cannot be imported.
//
// Honest boundaries:
//
//   - In-process and non-durable. A process restart loses every
//     pending request, TTL token, and audit row. There is no
//     crash-safety story here; the durable storage story is a v1.2
//     candidate (sqlite-backed Store and/or a token vault).
//
//   - Insert-only discipline is preserved by construction: audit
//     rows are only ever appended (the slice is never reordered or
//     rewritten), and a request is resolved exactly once — the
//     pending -> approved/rejected transition is a single write of
//     the resolved row plus one appended audit row. No row is ever
//     mutated in place after its append, and no row is ever removed.
//
//   - The audit-row id counter (auditSeq) is monotonically
//     increasing and stamps ExternalAuthRequest.AuditRowID at
//     resolution time, which is the DATA-14 join key back to the
//     appended audit row.
//
// Production deployments that need durability swap this type for a
// driver-backed Store; the Manager + Source + tool call sites are
// unchanged (v1.1-D10 reference-impl posture).
type MemoryStore struct {
	mu sync.RWMutex

	// requests is the pending/resolved request index by RequestID.
	// A resolved request keeps its entry (the state field records the
	// decision); only the pending subset is surfaced by the FIFO list.
	requests map[string]ExternalAuthRequest

	// tokens holds the minted TTL tokens by TokenID. Entries appear
	// only on an approve resolution and are never removed here —
	// expiry is enforced by the Source at execution time, not by
	// deletion.
	tokens map[string]TTLToken

	// audits is the append-only internal audit log (DATA-14 rows).
	// Rows are appended at resolution time and read by ListAuditRows;
	// nothing ever rewrites or drops an entry.
	audits []AuditRow

	// auditSeq is the monotonic audit-row counter. It is bumped
	// exactly once per ResolveExternalAuthRequest so the stamped
	// AuditRowID is unique per decision.
	auditSeq int64
}

// NewMemoryStore constructs an empty in-process Store with all index
// maps initialized. The zero MemoryStore is NOT usable (nil maps
// panic on write); always construct through this constructor.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		requests: make(map[string]ExternalAuthRequest),
		tokens:   make(map[string]TTLToken),
	}
}

// Compile-time assertion: MemoryStore satisfies the production Store
// seam the Manager is constructed with.
var _ Store = (*MemoryStore)(nil)

// IssueExternalAuthRequest persists a new pending request. A
// duplicate RequestID is an error — request ids are minted by the
// Manager's monotonic sequence, so a collision is a caller bug.
func (s *MemoryStore) IssueExternalAuthRequest(_ context.Context, r ExternalAuthRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[r.RequestID]; ok {
		return fmt.Errorf("externalauth: MemoryStore: duplicate request id %q", r.RequestID)
	}
	s.requests[r.RequestID] = r
	return nil
}

// GetExternalAuthRequest returns the request by ID. Missing rows
// return (zero, false, nil) per the Store contract.
func (s *MemoryStore) GetExternalAuthRequest(_ context.Context, id string) (ExternalAuthRequest, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.requests[id]
	return r, ok, nil
}

// ResolveExternalAuthRequest writes the data officer's decision: the
// resolved request row (state + AuditRowID stamp), the D14 audit row
// (append-only), and — on approve only — the minted TTL token. The
// three effects land together or not at all (the Manager already
// validated the transition; the store keeps the single-write
// discipline by refusing to touch a request that is no longer
// pending, so history is never rewritten after the fact).
func (s *MemoryStore) ResolveExternalAuthRequest(_ context.Context, r ExternalAuthRequest, a AuditRow, t *TTLToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.requests[r.RequestID]
	if !ok {
		return fmt.Errorf("externalauth: MemoryStore: resolve of unknown request %q", r.RequestID)
	}
	if cur.State != StatePending {
		return fmt.Errorf("externalauth: MemoryStore: request %q already resolved (%s) — a decision is written once",
			r.RequestID, cur.State)
	}
	s.auditSeq++
	r.AuditRowID = s.auditSeq
	s.requests[r.RequestID] = r
	if t != nil {
		s.tokens[t.TokenID] = *t
	}
	s.audits = append(s.audits, a)
	return nil
}

// ListPendingExternalAuthRequests returns the still-pending requests
// ordered oldest-first (CreatedAt ASC, RequestID as the deterministic
// tiebreak) — the data officer drains the queue in FIFO. A positive
// limit truncates the head of that order. Empty slice on no rows.
func (s *MemoryStore) ListPendingExternalAuthRequests(_ context.Context, limit int) ([]ExternalAuthRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ExternalAuthRequest, 0)
	for _, r := range s.requests {
		if r.State == StatePending {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].RequestID < out[j].RequestID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetTTLToken returns the token by ID. Missing rows return
// (zero, false, nil) per the Store contract.
func (s *MemoryStore) GetTTLToken(_ context.Context, id string) (TTLToken, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tokens[id]
	return t, ok, nil
}

// ListAuditRows returns the appended audit rows whose timestamp falls
// in [fromMs, toMs) (epoch ms) — the window the reconcile endpoint
// (DATA-06) queries over. Empty slice on no rows.
func (s *MemoryStore) ListAuditRows(_ context.Context, fromMs, toMs int64) ([]AuditRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditRow, 0)
	for _, a := range s.audits {
		ms := a.Timestamp.UnixMilli()
		if ms >= fromMs && ms < toMs {
			out = append(out, a)
		}
	}
	return out, nil
}
