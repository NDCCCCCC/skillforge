// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// snapshotState is the immutable (entry set, signature) pair. It exists
// so the two values can be swapped as ONE atomic pointer (WR-02,
// 26-REVIEW): entries and sig are two fields of one struct that is never
// mutated after publication, so a reader can never observe the new entry
// map with the previous signature (or vice versa). That pairing is
// load-bearing: audit rows carry Signature() so "which registry version
// produced this answer" stays replayable, and a torn read would let an
// audit row certify a signature that does not match the entry set that
// served the answer.
type snapshotState struct {
	entries map[string]Entry // immutable between swaps; copy-on-write
	sig     string           // content signature of exactly these entries
}

// snapshot is the in-memory Registry implementation: an immutable
// copy-on-write map of entries plus the content signature derived from
// them, optionally backed by a Store.
//
// Concurrency model (copy-on-write, mirrors the skill registry):
//
//   - Writers (Create/Deactivate) build a fresh snapshotState and
//     atomically swap it into place. The store write happens BEFORE the
//     swap, so a failed persistence never leaks into memory state.
//   - Readers load the current state ONCE per call and treat it as
//     read-only; every returned Entry/slice is a copy, so a caller can
//     never mutate registry state.
type snapshot struct {
	state atomic.Pointer[snapshotState] // immutable between swaps; copy-on-write
	store Store                         // nil = read-only: mutators fail closed
}

// newSnapshotState builds the immutable state for one entry set.
func newSnapshotState(entries []Entry) *snapshotState {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Slug] = e
	}
	return &snapshotState{entries: m, sig: computeSignature(entries)}
}

// newSnapshot builds a snapshot over entries. Duplicate slugs collapse
// last-wins; every production path (seed loader, Store adapter) enforces
// uniqueness before reaching here.
func newSnapshot(entries []Entry, st Store) *snapshot {
	s := &snapshot{store: st}
	s.state.Store(newSnapshotState(entries))
	return s
}

// New returns a read-only Registry over entries: the read surface is
// fully functional, while Create and Deactivate fail closed with
// ErrReadOnly because no Store is wired to persist them — a mutation that
// could not be persisted must not happen at all, since it would silently
// vanish on restart. It is the construction path for tests and for
// commands that only need the seed view of the world; the production
// sqlite-backed registry is assembled in 26-02.
func New(entries []Entry) Registry {
	return newSnapshot(entries, nil)
}

// compile-time proof that the snapshot carries the whole Registry contract.
var _ Registry = (*snapshot)(nil)

// current returns the live entry map. Readers must not mutate it.
func (s *snapshot) current() map[string]Entry {
	return s.state.Load().entries
}

// swap atomically replaces the entry map and its signature TOGETHER: one
// immutable snapshotState behind one pointer, so no reader can observe
// the entry set of one registry version with the signature of another
// (WR-02, 26-REVIEW).
func (s *snapshot) swap(entries []Entry) {
	s.state.Store(newSnapshotState(entries))
}

// Lookup returns the active entry for slug. A deactivated or unregistered
// slug is indistinguishable from absent — fail-closed by construction, so
// callers never have to remember to check a separate "active" flag.
func (s *snapshot) Lookup(slug string) (Entry, bool) {
	e, ok := s.current()[slug]
	if !ok || !e.RetrievalEnabled {
		return Entry{}, false
	}
	return e, true
}

// List returns every entry including deactivated ones (the admin view),
// sorted by slug ascending. The slice and its entries are copies.
func (s *snapshot) List() []Entry {
	m := s.current()
	out := make([]Entry, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ActiveSlugs returns the slugs of all active entries sorted ascending.
// A deactivated base disappears from this set, which is what makes skills
// referencing it stop loading (KBM-02).
func (s *snapshot) ActiveSlugs() []string {
	out := make([]string, 0)
	for _, e := range s.current() {
		if e.RetrievalEnabled {
			out = append(out, e.Slug)
		}
	}
	sort.Strings(out)
	return out
}

// SlugsByRole returns the slugs of active entries with role r, sorted
// ascending. An unknown role yields an empty slice (no entry can carry a
// role outside the closed vocabulary). Draft-role bases appear here when
// active: this answers "what exists", not "what is searchable".
func (s *snapshot) SlugsByRole(r Role) []string {
	out := make([]string, 0)
	for _, e := range s.current() {
		if e.Role == r && e.RetrievalEnabled {
			out = append(out, e.Slug)
		}
	}
	sort.Strings(out)
	return out
}

// RetrievalEligible returns the slugs that may enter a retrieval filter
// set: active, retrieval-enabled, and NOT draft-role. The draft exclusion
// is unconditional — role semantics outrank the entry switch, so no seed
// slip or admin toggle can make draft material searchable (A3). This is
// the single arbitration point for that rule.
func (s *snapshot) RetrievalEligible() []string {
	out := make([]string, 0)
	for _, e := range s.current() {
		if e.Role == RoleDraft || !e.RetrievalEnabled {
			continue
		}
		out = append(out, e.Slug)
	}
	sort.Strings(out)
	return out
}

// Signature returns the deterministic content hash of the entry set: the
// sha256 of every entry's identity fields, entries sorted by slug so
// insertion order never changes the value. Audit rows carry it, which is
// what makes "which registry version produced this answer" replayable.
// The value is read from the SAME snapshotState the entry accessors read,
// so it always describes the entry set a concurrent reader observes.
func (s *snapshot) Signature() string {
	return s.state.Load().sig
}

// computeSignature hashes the identity fields of every entry. Timestamps
// are deliberately excluded: a restart that only refreshes load times
// must not look like a registry change.
func computeSignature(entries []Entry) string {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })
	h := sha256.New()
	for _, e := range sorted {
		fmt.Fprintf(h, "%s|%s|%s|%s|%t\n", e.Slug, e.Role, e.DisplayName, e.Description, e.RetrievalEnabled)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ResolveReads expands a skill's reads declarations into the set of slugs
// the retriever may query (KBM-04). It is the single choke point between
// what a skill declares and what a search may touch.
//
// Accepted forms:
//
//   - "<slug>/**" or "<slug>" — an explicit base. An unknown or
//     deactivated slug is an error listing the active set (fail-closed:
//     silently resolving to nothing would hide a typo from the author).
//   - "role:<r>" — the wildcard: every retrieval-eligible base of that
//     role, ascending. An unknown role or a zero expansion is an error;
//     a wildcard that grants nothing must stop the load, not shrink to a
//     silent no-op.
//   - "data:..." — never accepted here. The data channel is validated by
//     the skill contract (D16-C2); this guard is defense-in-depth against
//     a future caller bypassing that validation.
//
// Draft-role bases are dropped from the resolved set rather than errored
// on: the author named a real base, the role simply never contributes to
// retrieval, so the honest result is an empty (default-deny) set for that
// read. Role semantics outrank the entry switch — see RetrievalEligible.
func (s *snapshot) ResolveReads(reads []string) ([]string, error) {
	if len(reads) == 0 {
		return nil, nil
	}
	for _, r := range reads {
		if strings.HasPrefix(r, "data:") {
			return nil, fmt.Errorf("reads %q: data channel values are validated by the skill contract (D16-C2), not by the knowledge-base registry — defense-in-depth guard", r)
		}
	}

	seen := make(map[string]bool, len(reads))
	out := make([]string, 0, len(reads))
	add := func(slug string) {
		if !seen[slug] {
			seen[slug] = true
			out = append(out, slug)
		}
	}

	for _, read := range reads {
		top := read
		if i := strings.Index(read, "/"); i >= 0 {
			top = read[:i]
		}

		if strings.HasPrefix(top, "role:") {
			role := Role(strings.TrimPrefix(top, "role:"))
			if !IsRole(string(role)) {
				return nil, fmt.Errorf("reads %q: unknown role %q (roles: %v)", read, role, Roles())
			}
			// Intersect the role membership with the retrieval-eligible
			// set: a wildcard expands to what can actually be searched.
			eligible := s.RetrievalEligible()
			members := s.SlugsByRole(role)
			expanded := make([]string, 0, len(members))
			for _, slug := range members {
				if containsString(eligible, slug) {
					expanded = append(expanded, slug)
				}
			}
			if len(expanded) == 0 {
				return nil, fmt.Errorf("reads %q: role %q matches zero active KBs (fail-closed)", read, role)
			}
			for _, slug := range expanded {
				add(slug)
			}
			continue
		}

		e, ok := s.Lookup(top)
		if !ok {
			return nil, fmt.Errorf("reads %q: unknown or deactivated KB %q (active: %v)", read, top, s.ActiveSlugs())
		}
		// draft role semantics outrank the explicit declaration: drop the
		// base from the resolved set so an explicit draft read yields an
		// empty (default-deny) set instead of a grant (A3).
		if e.Role == RoleDraft {
			continue
		}
		add(top)
	}
	return out, nil
}

// storeContext is the context carried into Store calls issued by the
// mutation verbs. The verbs take no context of their own (the admin
// handlers drive them synchronously at request scope); the store adapter
// owns its own deadlines and wraps what it needs.
func storeContext() context.Context { return context.Background() }

// containsString reports whether s is present in the slice. Registry sets
// are tiny (a handful of bases), so a linear scan beats a map for these
// intersections and membership probes.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// Create registers a new knowledge base (KBM-02). Validation fails closed
// on an invalid or reserved slug, an unknown role, or an empty display
// name; a slug that already exists — including a deactivated one — is a
// conflict, because re-creating would silently reset its identity. The
// store write happens before the in-memory swap, so a failed persistence
// never leaks into memory state.
func (s *snapshot) Create(in CreateInput) (Entry, error) {
	if s.store == nil {
		return Entry{}, ErrReadOnly
	}
	if err := ValidateNewSlug(in.Slug); err != nil {
		return Entry{}, err
	}
	if !IsRole(string(in.Role)) {
		return Entry{}, fmt.Errorf("role %q must be one of %v (roles are platform semantics, not configuration)", in.Role, Roles())
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		return Entry{}, fmt.Errorf("display_name must be non-empty (it is the UI badge text)")
	}
	if _, exists := s.current()[in.Slug]; exists {
		return Entry{}, fmt.Errorf("kb %q is already registered (deactivate it instead of re-creating)", in.Slug)
	}

	now := time.Now().UTC()
	e := Entry{
		Slug:             in.Slug,
		DisplayName:      in.DisplayName,
		Role:             in.Role,
		Description:      in.Description,
		RetrievalEnabled: true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.store.UpsertBase(storeContext(), e); err != nil {
		return Entry{}, fmt.Errorf("kb.Create: persist %q: %w", in.Slug, err)
	}
	s.swap(append(s.List(), e))
	return e, nil
}

// Deactivate flips the retrieval switch of slug to false (KBM-02). The
// entry stays listed for admins, vanishes from Lookup and from every
// retrieval-eligible set, and its data is untouched — there is no delete
// verb anywhere in this package. Already-deactivated bases deactivate
// idempotently. The store write happens before the in-memory swap.
func (s *snapshot) Deactivate(slug string) (Entry, error) {
	if s.store == nil {
		return Entry{}, ErrReadOnly
	}
	cur, ok := s.current()[slug]
	if !ok {
		return Entry{}, fmt.Errorf("kb %q is not registered (active: %v)", slug, s.ActiveSlugs())
	}
	if !cur.RetrievalEnabled {
		return cur, nil // already deactivated: idempotent no-op
	}

	now := time.Now().UTC()
	if err := s.store.SetRetrievalEnabled(storeContext(), slug, false, now); err != nil {
		return Entry{}, fmt.Errorf("kb.Deactivate: persist %q: %w", slug, err)
	}
	updated := cur
	updated.RetrievalEnabled = false
	updated.UpdatedAt = now

	entries := s.List()
	for i := range entries {
		if entries[i].Slug == slug {
			entries[i] = updated
		}
	}
	s.swap(entries)
	return updated, nil
}
