// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeStore is the in-memory Store seam used to observe the mutation
// verbs without touching a real persistence layer.
type fakeStore struct {
	bases       []Entry
	upserts     []Entry
	switchCalls []switchCall
	loadErr     error
	upsertErr   error
	switchErr   error
}

type switchCall struct {
	slug    string
	enabled bool
	at      time.Time
}

func (f *fakeStore) LoadBases(ctx context.Context) ([]Entry, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return f.bases, nil
}

func (f *fakeStore) UpsertBase(ctx context.Context, e Entry) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserts = append(f.upserts, e)
	return nil
}

func (f *fakeStore) SetRetrievalEnabled(ctx context.Context, slug string, enabled bool, at time.Time) error {
	if f.switchErr != nil {
		return f.switchErr
	}
	f.switchCalls = append(f.switchCalls, switchCall{slug: slug, enabled: enabled, at: at})
	return nil
}

// testEntries returns four baseline-shaped entries (ascii display names —
// this package carries no CN literals) in an intentionally unsorted order.
func testEntries() []Entry {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	mk := func(slug, display string, role Role) Entry {
		return Entry{
			Slug:             slug,
			DisplayName:      display,
			Role:             role,
			Description:      "test base",
			RetrievalEnabled: true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}
	return []Entry{
		mk("personal", "Personal", RolePersonal),
		mk("authority", "Authority", RoleAuthority),
		mk("draft", "Draft", RoleDraft),
		mk("historical", "Historical", RoleHistorical),
	}
}

// testRegistry returns a read-only registry over the baseline entries.
func testRegistry() Registry {
	return New(testEntries())
}

// TestLookupActiveOnly locks the fail-closed lookup: a deactivated entry
// is indistinguishable from an unregistered one, while the admin view
// still sees it.
func TestLookupActiveOnly(t *testing.T) {
	entries := testEntries()
	reg := newSnapshot(entries, nil)
	if _, err := reg.Deactivate("historical"); err != nil {
		t.Fatalf("Deactivate = %v, want nil", err)
	}
	if got, ok := reg.Lookup("historical"); ok {
		t.Errorf("Lookup(deactivated) = (%+v, true), want false", got)
	}
	list := reg.List()
	found := false
	for _, e := range list {
		if e.Slug == "historical" {
			found = true
			if e.RetrievalEnabled {
				t.Errorf("deactivated entry still listed with RetrievalEnabled = true: %+v", e)
			}
		}
	}
	if !found {
		t.Error("List() must still contain a deactivated entry (deactivate, never destroy)")
	}
	if containsString(reg.ActiveSlugs(), "historical") {
		t.Errorf("ActiveSlugs() = %v contains a deactivated slug", reg.ActiveSlugs())
	}
}

// TestListSortedAndDefensiveCopy locks deterministic ordering and the
// defensive-copy discipline: mutating a returned slice must never leak
// into registry state.
func TestListSortedAndDefensiveCopy(t *testing.T) {
	reg := testRegistry()
	want := []string{"authority", "draft", "historical", "personal"}
	if got := reg.ActiveSlugs(); !equalStrings(got, want) {
		t.Errorf("ActiveSlugs() = %v, want ascending %v", got, want)
	}
	list := reg.List()
	for i := 1; i < len(list); i++ {
		if list[i-1].Slug >= list[i].Slug {
			t.Errorf("List() not sorted ascending: %v", list)
		}
	}
	// mutate the returned slice and its entries
	list[0].DisplayName = "MUTATED"
	list = append(list, Entry{Slug: "zzz-injected"})
	fresh := reg.List()
	if len(fresh) != len(want) {
		t.Fatalf("List() leaked a mutation: %d entries, want %d", len(fresh), len(want))
	}
	for _, e := range fresh {
		if e.Slug == "authority" && e.DisplayName == "MUTATED" {
			t.Error("List() handed out registry-owned entry state (no defensive copy)")
		}
		if e.Slug == "zzz-injected" {
			t.Error("an appended slice element reached the registry")
		}
	}
	// same discipline for the derived slug sets
	slugs := reg.ActiveSlugs()
	slugs[0] = "MUTATED"
	if reg.ActiveSlugs()[0] == "MUTATED" {
		t.Error("ActiveSlugs() handed out registry-owned state")
	}
	byRole := reg.SlugsByRole(RoleAuthority)
	if len(byRole) > 0 {
		byRole[0] = "MUTATED"
		if reg.SlugsByRole(RoleAuthority)[0] == "MUTATED" {
			t.Error("SlugsByRole() handed out registry-owned state")
		}
	}
}

// TestRetrievalEligibleExcludesDraft locks the A3 assumption in code: the
// draft role never enters the retrieval set, and turning the entry switch
// off removes it from the active set too (double gate).
func TestRetrievalEligibleExcludesDraft(t *testing.T) {
	reg := testRegistry()
	want := []string{"authority", "historical", "personal"}
	if got := reg.RetrievalEligible(); !equalStrings(got, want) {
		t.Errorf("RetrievalEligible() = %v, want %v (draft excluded by role)", got, want)
	}

	entries := testEntries()
	for i := range entries {
		if entries[i].Role == RoleDraft {
			entries[i].RetrievalEnabled = false
		}
	}
	off := newSnapshot(entries, nil)
	if got := off.ActiveSlugs(); containsString(got, "draft") {
		t.Errorf("ActiveSlugs() = %v contains draft with its switch off", got)
	}
	if got := off.RetrievalEligible(); containsString(got, "draft") {
		t.Errorf("RetrievalEligible() = %v contains draft with its switch off", got)
	}
}

// TestSlugsByRole locks the per-role projection: active entries of the
// requested role, ascending; an unknown role yields an empty slice; a
// draft-role base is active so it shows up here even though it is never
// retrieval-eligible.
func TestSlugsByRole(t *testing.T) {
	reg := testRegistry()
	cases := []struct {
		role Role
		want []string
	}{
		{RoleAuthority, []string{"authority"}},
		{RoleHistorical, []string{"historical"}},
		{RolePersonal, []string{"personal"}},
		{RoleDraft, []string{"draft"}},
		{Role("admin"), []string{}},
	}
	for _, tc := range cases {
		got := reg.SlugsByRole(tc.role)
		if len(got) != len(tc.want) {
			t.Errorf("SlugsByRole(%q) = %v, want %v", tc.role, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SlugsByRole(%q) = %v, want %v", tc.role, got, tc.want)
			}
		}
	}
}

// TestResolveReadsSlugForms locks the slug reads forms: the top segment
// is extracted, bare slugs pass through, duplicates collapse, and empty
// input stays a no-op (default-deny remains the caller's job).
func TestResolveReadsSlugForms(t *testing.T) {
	reg := testRegistry()
	got, err := reg.ResolveReads([]string{"authority/**", "historical"})
	if err != nil {
		t.Fatalf("ResolveReads = %v, want nil", err)
	}
	if want := []string{"authority", "historical"}; !equalStrings(got, want) {
		t.Errorf("ResolveReads = %v, want %v", got, want)
	}
	got, err = reg.ResolveReads([]string{"authority/**", "authority/deep/path"})
	if err != nil {
		t.Fatalf("ResolveReads = %v, want nil", err)
	}
	if want := []string{"authority"}; !equalStrings(got, want) {
		t.Errorf("ResolveReads = %v, want deduplicated %v", got, want)
	}
	got, err = reg.ResolveReads(nil)
	if err != nil {
		t.Fatalf("ResolveReads(nil) = %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Errorf("ResolveReads(nil) = %v, want empty", got)
	}
}

// TestResolveReadsUnknownSlug locks the fail-closed unknown-slug path and
// the error form that lists the active slug set.
func TestResolveReadsUnknownSlug(t *testing.T) {
	reg := testRegistry()
	_, err := reg.ResolveReads([]string{"kb-nope/**"})
	if err == nil {
		t.Fatal("ResolveReads(unknown) = nil error, want fail-closed error")
	}
	if !strings.Contains(err.Error(), "unknown or deactivated KB") {
		t.Errorf("error %q must name the unknown-KB condition", err)
	}
	for _, active := range reg.ActiveSlugs() {
		if !strings.Contains(err.Error(), active) {
			t.Errorf("error %q must list the active slug set (missing %q)", err, active)
		}
	}
}

// TestResolveReadsDeactivatedSlug locks the KBM-02 semantics: a
// deactivated base disappears from skill validation entirely.
func TestResolveReadsDeactivatedSlug(t *testing.T) {
	entries := testEntries()
	reg := newSnapshot(entries, nil)
	if _, err := reg.Deactivate("historical"); err != nil {
		t.Fatalf("Deactivate = %v, want nil", err)
	}
	_, err := reg.ResolveReads([]string{"historical/**"})
	if err == nil {
		t.Fatal("ResolveReads(deactivated) = nil error, want fail-closed error")
	}
	if !strings.Contains(err.Error(), "unknown or deactivated KB") {
		t.Errorf("error %q must name the deactivated-KB condition", err)
	}
}

// TestResolveReadsRoleWildcard locks the role wildcard: it expands to the
// retrieval-eligible bases of that role, ascending; unknown roles and
// empty expansions fail closed.
func TestResolveReadsRoleWildcard(t *testing.T) {
	reg := testRegistry()
	got, err := reg.ResolveReads([]string{"role:authority"})
	if err != nil {
		t.Fatalf("ResolveReads(role:authority) = %v, want nil", err)
	}
	if want := []string{"authority"}; !equalStrings(got, want) {
		t.Errorf("ResolveReads(role:authority) = %v, want %v", got, want)
	}

	// a second base of the same role joins the expansion, ascending
	entries := testEntries()
	entries = append(entries, Entry{
		Slug:             "kb-audit",
		DisplayName:      "Audit",
		Role:             RoleAuthority,
		RetrievalEnabled: true,
	})
	two := newSnapshot(entries, nil)
	got, err = two.ResolveReads([]string{"role:authority"})
	if err != nil {
		t.Fatalf("ResolveReads(role:authority, two bases) = %v, want nil", err)
	}
	if want := []string{"authority", "kb-audit"}; !equalStrings(got, want) {
		t.Errorf("ResolveReads(role:authority, two bases) = %v, want %v", got, want)
	}

	// an unknown role name fails closed and names the closed vocabulary
	if _, err := two.ResolveReads([]string{"role:kb-nope"}); err == nil {
		t.Error("ResolveReads(role:kb-nope) = nil error, want unknown-role error")
	}

	// a draft-role wildcard expands to nothing retrievable: fail closed,
	// never silently grant the non-retrievable role
	if _, err := two.ResolveReads([]string{"role:draft"}); err == nil {
		t.Error("ResolveReads(role:draft) = nil error, want zero-expansion error")
	}

	// deactivating every base of a role empties the expansion → error
	only := newSnapshot(testEntries(), nil)
	if _, err := only.Deactivate("authority"); err != nil {
		t.Fatalf("Deactivate = %v, want nil", err)
	}
	_, err = only.ResolveReads([]string{"role:authority"})
	if err == nil {
		t.Fatal("ResolveReads(role:authority, all deactivated) = nil error, want zero-expansion error")
	}
	if !strings.Contains(err.Error(), "matches zero active") {
		t.Errorf("error %q must name the zero-active-expansion condition", err)
	}
}

// TestResolveReadsDraftDropped locks the sole choke point of the
// draft-never-retrievable rule: an explicit draft read resolves to an
// empty (default-deny) set instead of an error, and never contributes a
// slug to a mixed set.
func TestResolveReadsDraftDropped(t *testing.T) {
	reg := testRegistry()
	got, err := reg.ResolveReads([]string{"draft/**"})
	if err != nil {
		t.Fatalf("ResolveReads(draft) = %v, want nil (drop, not error)", err)
	}
	if len(got) != 0 {
		t.Errorf("ResolveReads(draft) = %v, want an empty default-deny set", got)
	}
	got, err = reg.ResolveReads([]string{"authority/**", "draft/**"})
	if err != nil {
		t.Fatalf("ResolveReads(mixed) = %v, want nil", err)
	}
	if want := []string{"authority"}; !equalStrings(got, want) {
		t.Errorf("ResolveReads(mixed) = %v, want %v (draft dropped)", got, want)
	}
}

// TestResolveReadsDataPrefixGuard keeps the defense-in-depth guard: data
// channel reads are validated by the skill contract and must never reach
// the knowledge-base resolver.
func TestResolveReadsDataPrefixGuard(t *testing.T) {
	reg := testRegistry()
	_, err := reg.ResolveReads([]string{"data:aggregated_only"})
	if err == nil {
		t.Fatal("ResolveReads(data:) = nil error, want the defense-in-depth guard error")
	}
	if !strings.Contains(err.Error(), "data:") {
		t.Errorf("error %q must name the data channel", err)
	}
}

// TestSignatureDeterministic locks the signature: insensitive to entry
// order and to load timestamps, sensitive to every identity field.
func TestSignatureDeterministic(t *testing.T) {
	a := testRegistry()
	reversed := testEntries()
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	b := New(reversed)
	if a.Signature() == "" {
		t.Fatal("Signature() is empty, want a content hash")
	}
	if a.Signature() != b.Signature() {
		t.Errorf("Signature() changed with insertion order: %q vs %q", a.Signature(), b.Signature())
	}

	// timestamps are not identity: same entries, different times
	restamped := testEntries()
	for i := range restamped {
		restamped[i].CreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		restamped[i].UpdatedAt = restamped[i].CreatedAt
	}
	if New(restamped).Signature() != a.Signature() {
		t.Error("Signature() changed when only timestamps changed, want it stable")
	}

	// every identity field moves the hash
	switched := testEntries()
	for i := range switched {
		if switched[i].Slug == "historical" {
			switched[i].RetrievalEnabled = false
		}
	}
	if New(switched).Signature() == a.Signature() {
		t.Error("Signature() unchanged after RetrievalEnabled changed")
	}
	renamed := testEntries()
	renamed[0].DisplayName = "Renamed"
	if New(renamed).Signature() == a.Signature() {
		t.Error("Signature() unchanged after DisplayName changed")
	}
}

// TestCreate locks the admin create path: validated input, store write
// before the in-memory swap, uniqueness including deactivated slugs, and
// closed-vocabulary role checking.
func TestCreate(t *testing.T) {
	store := &fakeStore{}
	reg := newSnapshot(testEntries(), store)

	e, err := reg.Create(CreateInput{
		Slug:        "kb-new",
		DisplayName: "New Base",
		Role:        RolePersonal,
		Description: "fresh base",
	})
	if err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}
	if !e.RetrievalEnabled {
		t.Error("created entry RetrievalEnabled = false, want true")
	}
	if e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
		t.Error("created entry timestamps are zero, want now")
	}
	if len(store.upserts) != 1 || store.upserts[0].Slug != "kb-new" {
		t.Errorf("store received %+v upserts, want exactly kb-new", store.upserts)
	}
	if !containsString(reg.ActiveSlugs(), "kb-new") {
		t.Errorf("ActiveSlugs() = %v, want it to contain kb-new after create", reg.ActiveSlugs())
	}
	if got, ok := reg.Lookup("kb-new"); !ok || got.DisplayName != "New Base" {
		t.Errorf("Lookup(kb-new) = (%+v, %v), want the created entry", got, ok)
	}

	// duplicate slug, active or deactivated: 409 semantics
	if _, err := reg.Create(CreateInput{Slug: "kb-new", DisplayName: "Again", Role: RolePersonal}); err == nil {
		t.Error("Create(duplicate active slug) = nil error, want conflict error")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("duplicate-slug error %q must name the conflict", err)
	}
	deactivated := testEntries()
	deactivated = append(deactivated, Entry{Slug: "kb-off", DisplayName: "Off", Role: RoleHistorical, RetrievalEnabled: false})
	withOff := newSnapshot(deactivated, store)
	if _, err := withOff.Create(CreateInput{Slug: "kb-off", DisplayName: "Again", Role: RoleHistorical}); err == nil {
		t.Error("Create(deactivated slug) = nil error, want conflict error")
	}

	// invalid input fails closed
	if _, err := reg.Create(CreateInput{Slug: "Bad_Slug", DisplayName: "X", Role: RolePersonal}); err == nil {
		t.Error("Create(invalid slug) = nil error, want validation error")
	}
	if _, err := reg.Create(CreateInput{Slug: "draft", DisplayName: "X", Role: RolePersonal}); err == nil {
		t.Error("Create(role-named slug) = nil error, want reserved-slug error")
	}
	if _, err := reg.Create(CreateInput{Slug: "kb-x", DisplayName: "X", Role: Role("admin")}); err == nil {
		t.Error("Create(unknown role) = nil error, want closed-vocabulary error")
	}
	if _, err := reg.Create(CreateInput{Slug: "kb-x", DisplayName: "  ", Role: RolePersonal}); err == nil {
		t.Error("Create(empty display name) = nil error, want validation error")
	}
}

// TestDeactivate locks the admin deactivate path: store switch first,
// then the in-memory swap; the base stays listed but stops resolving.
func TestDeactivate(t *testing.T) {
	store := &fakeStore{}
	reg := newSnapshot(testEntries(), store)

	e, err := reg.Deactivate("authority")
	if err != nil {
		t.Fatalf("Deactivate = %v, want nil", err)
	}
	if e.RetrievalEnabled {
		t.Error("deactivated entry RetrievalEnabled = true, want false")
	}
	if len(store.switchCalls) != 1 || store.switchCalls[0].slug != "authority" || store.switchCalls[0].enabled {
		t.Errorf("store received %+v switch calls, want exactly one disable of authority", store.switchCalls)
	}
	if _, ok := reg.Lookup("authority"); ok {
		t.Error("Lookup(deactivated) resolved, want invisible")
	}
	found := false
	for _, listed := range reg.List() {
		if listed.Slug == "authority" {
			found = true
			if listed.RetrievalEnabled {
				t.Error("List() shows the deactivated base as enabled")
			}
		}
	}
	if !found {
		t.Error("List() lost the deactivated base (deactivate, never destroy)")
	}
	if containsString(reg.RetrievalEligible(), "authority") {
		t.Error("RetrievalEligible() still contains a deactivated base")
	}

	if _, err := reg.Deactivate("kb-nope"); err == nil {
		t.Error("Deactivate(unknown) = nil error, want fail-closed error")
	}
}

// TestReadOnlyRegistry locks the read-only posture: without a Store the
// mutation verbs fail closed instead of mutating memory that would vanish
// on restart.
func TestReadOnlyRegistry(t *testing.T) {
	reg := testRegistry()
	if _, err := reg.Create(CreateInput{Slug: "kb-new", DisplayName: "X", Role: RolePersonal}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Create on a read-only registry = %v, want ErrReadOnly", err)
	}
	if _, err := reg.Deactivate("authority"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Deactivate on a read-only registry = %v, want ErrReadOnly", err)
	}
}

// TestCreateStoreFailureKeepsSnapshot locks the write-before-swap
// ordering: a failed persistence must never leak into memory state.
func TestCreateStoreFailureKeepsSnapshot(t *testing.T) {
	store := &fakeStore{upsertErr: errors.New("disk full")}
	reg := newSnapshot(testEntries(), store)
	before := reg.Signature()
	if _, err := reg.Create(CreateInput{Slug: "kb-new", DisplayName: "X", Role: RolePersonal}); err == nil {
		t.Fatal("Create(store failure) = nil error, want the store error")
	} else if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error %q must carry the store failure", err)
	}
	if containsString(reg.ActiveSlugs(), "kb-new") {
		t.Error("a failed create leaked into the in-memory snapshot")
	}
	if reg.Signature() != before {
		t.Error("a failed create changed the signature")
	}
}

// TestRestartDeactivateStaysDeactivated simulates a restart: the persisted
// rows come back with a deactivated switch and the rebuilt registry must
// honor it — the switch belongs to operations, not to the seed.
func TestRestartDeactivateStaysDeactivated(t *testing.T) {
	bases := testEntries()
	for i := range bases {
		if bases[i].Slug == "historical" {
			bases[i].RetrievalEnabled = false
		}
	}
	store := &fakeStore{bases: bases}
	reg := newSnapshot(store.bases, store)
	if _, ok := reg.Lookup("historical"); ok {
		t.Error("a persisted deactivation was resurrected on rebuild")
	}
	found := false
	for _, e := range reg.List() {
		if e.Slug == "historical" {
			found = true
			if e.RetrievalEnabled {
				t.Error("persisted row came back enabled")
			}
		}
	}
	if !found {
		t.Error("persisted deactivated row vanished from the admin view")
	}
	if containsString(reg.RetrievalEligible(), "historical") {
		t.Error("a persisted deactivation re-entered the retrieval set")
	}
}
