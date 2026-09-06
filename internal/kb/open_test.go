// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

// Phase 26 / Plan 26-02 — the production assembly of the registry. These
// tests lock the seed/runtime merge semantics (Pitfall 6): an admin
// deactivation is never resurrected by the seed, a seed structure change
// does reach existing deployments, and an admin-created base is never
// recycled by the seed's absence. All of it runs against a map-backed
// Store — hermetic, no sqlite, no CGO.

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// memStore is the map-backed Store seam for the Open tests. Unlike the
// call-recording fake in registry_test.go, it APPLIES writes to its map,
// so a merge can be re-read the way a real persistence layer would serve
// it (Open re-reads the store after the seed upserts).
type memStore struct {
	bases     map[string]Entry
	upserts   []Entry
	switches  []switchCall
	loadErr   error
	upsertErr error
	switchErr error
}

// newMemStore seeds the map-backed store with the given rows.
func newMemStore(bases ...Entry) *memStore {
	m := make(map[string]Entry, len(bases))
	for _, e := range bases {
		m[e.Slug] = e
	}
	return &memStore{bases: m}
}

func (f *memStore) LoadBases(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err // AGNT-04: a cancelled context surfaces, never blocks
	}
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	out := make([]Entry, 0, len(f.bases))
	for _, e := range f.bases {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (f *memStore) UpsertBase(ctx context.Context, e Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserts = append(f.upserts, e)
	f.bases[e.Slug] = e
	return nil
}

func (f *memStore) SetRetrievalEnabled(ctx context.Context, slug string, enabled bool, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.switchErr != nil {
		return f.switchErr
	}
	f.switches = append(f.switches, switchCall{slug: slug, enabled: enabled, at: at})
	cur, ok := f.bases[slug]
	if !ok {
		return errors.New("memStore: unknown slug " + slug)
	}
	cur.RetrievalEnabled = enabled
	cur.UpdatedAt = at
	f.bases[slug] = cur
	return nil
}

// baselineSeedBody renders a controlled seed carrying the four baseline
// entries with structure matching testEntries() — so a merge test only
// sees writes for the rows it deliberately changes.
func baselineSeedBody() string {
	return seedBody(
		seedEntryLine("authority", "authority", "Authority") +
			seedEntryLine("historical", "historical", "Historical") +
			seedEntryLine("personal", "personal", "Personal") +
			seedEntryLine("draft", "draft", "Draft"))
}

// listedEntry fetches one entry from the admin view (Lookup would
// fail-closed on a deactivated base, which is exactly the row these tests
// need to inspect).
func listedEntry(t *testing.T, reg Registry, slug string) Entry {
	t.Helper()
	for _, e := range reg.List() {
		if e.Slug == slug {
			return e
		}
	}
	t.Fatalf("registry does not list %q; List() = %+v", slug, reg.List())
	return Entry{}
}

// TestOpenFreshClone — the Pitfall 6 direction (b) guard for a brand-new
// deployment: an empty store plus a legal seed produces the full seed
// registry, and every seed entry reaches the store as an upsert.
func TestOpenFreshClone(t *testing.T) {
	st := newMemStore()
	seed := writeSeedFile(t, baselineSeedBody())

	reg, err := Open(context.Background(), seed, st)
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}
	list := reg.List()
	if len(list) != 4 {
		t.Fatalf("Open returned %d entries, want the 4 seed entries: %+v", len(list), list)
	}
	for _, e := range list {
		if !e.RetrievalEnabled {
			t.Errorf("seed entry %s came back with its switch off; the seed's initial value must stand on a fresh clone", e.Slug)
		}
		if e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
			t.Errorf("seed entry %s has zero timestamps", e.Slug)
		}
	}
	if len(st.upserts) != 4 {
		t.Errorf("store received %d upserts, want 4 (one per seed entry): %+v", len(st.upserts), st.upserts)
	}
}

// TestOpenSeedUpsertStructureOnly — THE Pitfall 6 direction (a) lock:
// structure belongs to config (the seed's declared fields win), the switch
// belongs to ops (a persisted deactivation is never resurrected by a
// restart), and nothing is written for a row whose declared fields did not
// change.
func TestOpenSeedUpsertStructureOnly(t *testing.T) {
	bases := testEntries()
	for i := range bases {
		if bases[i].Slug == "historical" {
			bases[i].RetrievalEnabled = false // an admin deactivated this base
		}
	}
	st := newMemStore(bases...)
	seed := writeSeedFile(t, seedBody(
		seedEntryLine("authority", "authority", "Authority")+
			seedEntryLine("historical", "historical", "Historic Renamed")+
			seedEntryLine("personal", "personal", "Personal")+
			seedEntryLine("draft", "draft", "Draft")))

	reg, err := Open(context.Background(), seed, st)
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}

	got := listedEntry(t, reg, "historical")
	if got.DisplayName != "Historic Renamed" {
		t.Errorf("historical display name = %q, want the seed's declared %q (structure belongs to config)", got.DisplayName, "Historic Renamed")
	}
	if got.Role != RoleHistorical {
		t.Errorf("historical role = %q, want the seed's declared role", got.Role)
	}
	if got.RetrievalEnabled {
		t.Error("the seed resurrected a deactivated base — admin deactivation must survive a restart (Pitfall 6a, KBM-02)")
	}

	// exactly one write, and it carried the PERSISTED switch, never the
	// seed's initial true
	if len(st.upserts) != 1 {
		t.Fatalf("store received %d upserts, want 1 (unchanged rows must not be rewritten): %+v", len(st.upserts), st.upserts)
	}
	if st.upserts[0].Slug != "historical" {
		t.Errorf("the upsert touched %q, want historical only", st.upserts[0].Slug)
	}
	if st.upserts[0].RetrievalEnabled {
		t.Error("the structure upsert carried the seed's switch instead of the persisted deactivated value")
	}
}

// TestOpenNewSeedEntryReachesExistingDeploy — the Pitfall 6 direction (b)
// lock: a PR that adds a knowledge base to the seed reaches deployments
// that already have a persisted registry.
func TestOpenNewSeedEntryReachesExistingDeploy(t *testing.T) {
	st := newMemStore(testEntries()...)
	seed := writeSeedFile(t, baselineSeedBody()+seedEntryLine("kb-audit", "authority", "Audit"))

	reg, err := Open(context.Background(), seed, st)
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}
	if !containsString(reg.ActiveSlugs(), "kb-audit") {
		t.Errorf("ActiveSlugs() = %v, want it to contain the new seed entry kb-audit", reg.ActiveSlugs())
	}
	if got := len(reg.List()); got != 5 {
		t.Errorf("List() = %d entries, want 5", got)
	}
	if len(st.upserts) != 1 || st.upserts[0].Slug != "kb-audit" {
		t.Errorf("store received %+v upserts, want exactly the new seed entry", st.upserts)
	}
}

// TestOpenAdminCreatedRowSurvives — the seed is not a garbage collector: a
// base an admin created at runtime (absent from the seed file) stays
// registered across a restart.
func TestOpenAdminCreatedRowSurvives(t *testing.T) {
	bases := testEntries()
	bases = append(bases, Entry{
		Slug:             "ops-private",
		DisplayName:      "Ops Private",
		Role:             RolePersonal,
		Description:      "created by an admin at runtime",
		RetrievalEnabled: true,
		CreatedAt:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	st := newMemStore(bases...)
	seed := writeSeedFile(t, baselineSeedBody())

	reg, err := Open(context.Background(), seed, st)
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}
	if _, ok := reg.Lookup("ops-private"); !ok {
		t.Error("an admin-created base vanished because the seed does not declare it — the seed must never recycle runtime rows")
	}
	if got := len(reg.List()); got != 5 {
		t.Errorf("List() = %d entries, want the 5 including ops-private", got)
	}
}

// TestOpenSeedAbsent — the absent seed is the one legal degraded state:
// the persisted registry stands alone, and an empty store with no seed is
// an empty (not an error) registry — the boot's own zero-Skills guard is
// the backstop, not the assembly.
func TestOpenSeedAbsent(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	st := newMemStore(testEntries()...)
	reg, err := Open(context.Background(), absent, st)
	if err != nil {
		t.Fatalf("Open(absent seed, populated store) = %v, want a legal degraded pass", err)
	}
	if got := len(reg.List()); got != 4 {
		t.Errorf("List() = %d entries, want the 4 persisted rows", got)
	}
	if len(st.upserts) != 0 {
		t.Errorf("a degraded pass wrote %+v upserts, want none", st.upserts)
	}

	empty := newMemStore()
	reg2, err := Open(context.Background(), absent, empty)
	if err != nil {
		t.Fatalf("Open(absent seed, empty store) = %v, want an empty registry and no error", err)
	}
	if got := len(reg2.List()); got != 0 {
		t.Errorf("List() = %d entries, want 0", got)
	}
}

// TestOpenSeedMalformed — both failure directions fail closed: a broken
// controlled seed is an error (the caller log.Fatalf's it), and so is a
// store that cannot serve the persisted rows.
func TestOpenSeedMalformed(t *testing.T) {
	seed := writeSeedFile(t, "knowledge_bases: [this is: broken")
	if _, err := Open(context.Background(), seed, newMemStore(testEntries()...)); err == nil {
		t.Fatal("Open(malformed seed) = nil error, want a fail-closed error")
	} else if !strings.Contains(err.Error(), seed) {
		t.Errorf("error %q must name the seed file", err)
	}

	st := newMemStore(testEntries()...)
	st.loadErr = errors.New("db gone")
	absent := filepath.Join(t.TempDir(), "absent.yaml")
	if _, err := Open(context.Background(), absent, st); err == nil {
		t.Fatal("Open(store failure) = nil error, want the store error")
	} else if !strings.Contains(err.Error(), "db gone") {
		t.Errorf("error %q must carry the store failure", err)
	}
}

// TestOpenSignatureReflectsMerge — the signature is computed over the
// MERGED state, so a persisted deactivation moves it away from the pure
// seed view (the switch participates in the identity an audit row stamps).
func TestOpenSignatureReflectsMerge(t *testing.T) {
	bases := testEntries()
	for i := range bases {
		if bases[i].Slug == "historical" {
			bases[i].RetrievalEnabled = false
		}
	}
	st := newMemStore(bases...)
	seedPath := writeSeedFile(t, baselineSeedBody())

	reg, err := Open(context.Background(), seedPath, st)
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}
	seeds, err := LoadSeedFile(seedPath)
	if err != nil {
		t.Fatalf("LoadSeedFile = %v, want nil", err)
	}
	if reg.Signature() == New(seeds).Signature() {
		t.Error("Signature() ignored the persisted deactivation — the merged state must sign differently from the pure seed view")
	}
}

// TestOpenContextCancelled — a cancelled context surfaces as a context
// error (AGNT-04 posture), never as a partially assembled registry.
func TestOpenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	seed := writeSeedFile(t, baselineSeedBody())
	if _, err := Open(ctx, seed, newMemStore(testEntries()...)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open(cancelled ctx) = %v, want a context.Canceled error", err)
	}
}
