// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync/atomic"
)

// snapshot is the in-memory Registry implementation: an immutable
// copy-on-write map of entries plus the content signature derived from
// them, optionally backed by a Store.
//
// Concurrency model (copy-on-write, mirrors the skill registry):
//
//   - Writers (Create/Deactivate) build a fresh map and atomically swap
//     it into place. The store write happens BEFORE the swap, so a failed
//     persistence never leaks into memory state.
//   - Readers resolve the current map once per call and treat it as
//     read-only; every returned Entry/slice is a copy, so a caller can
//     never mutate registry state.
type snapshot struct {
	entries atomic.Pointer[map[string]Entry] // immutable between swaps; copy-on-write
	sig     atomic.Pointer[string]           // content signature; swapped together with entries
	store   Store                            // nil = read-only: mutators fail closed
}

// newSnapshot builds a snapshot over entries. Duplicate slugs collapse
// last-wins; every production path (seed loader, Store adapter) enforces
// uniqueness before reaching here.
func newSnapshot(entries []Entry, st Store) *snapshot {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Slug] = e
	}
	s := &snapshot{store: st}
	s.entries.Store(&m)
	sig := computeSignature(entries)
	s.sig.Store(&sig)
	return s
}

// New returns a read-only registry over entries: Lookup/List and the
// derived slug sets are fully functional, while the mutation verbs stay
// unavailable because no Store is wired to persist them. It is the
// construction path for tests and for commands that only need the seed
// view of the world; the production sqlite-backed registry is assembled
// in 26-02.
func New(entries []Entry) *snapshot {
	return newSnapshot(entries, nil)
}

// current returns the live entry map. Readers must not mutate it.
func (s *snapshot) current() map[string]Entry {
	return *s.entries.Load()
}

// swap atomically replaces the entry map and recomputes the signature.
func (s *snapshot) swap(entries []Entry) {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Slug] = e
	}
	s.entries.Store(&m)
	sig := computeSignature(entries)
	s.sig.Store(&sig)
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
func (s *snapshot) Signature() string {
	return *s.sig.Load()
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
