// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// repoSeedPath is the controlled seed file at the repository root, reached
// relative to this package directory.
const repoSeedPath = "../../knowledge_bases.yaml"

// Display names are asserted through code points so this package keeps
// zero CN literals: the controlled seed file is their single home, and
// the refusal-suggestion copy pins these exact bytes downstream.
var (
	seedDisplayNameAuthority  = string([]rune{0x6743, 0x5A01})
	seedDisplayNameHistorical = string([]rune{0x5386, 0x53F2})
	seedDisplayNamePersonal   = string([]rune{0x4E2A, 0x4EBA})
	seedDisplayNameDraft      = string([]rune{0x8349, 0x7A3F})
)

// seedBody builds a controlled seed file body from raw entry lines.
func seedBody(entries string) string {
	return "version: \"1\"\nknowledge_bases:\n" + entries
}

// seedEntryLine renders one well-formed entry line block.
func seedEntryLine(slug, role, display string) string {
	return fmt.Sprintf("  - slug: %s\n    display_name: %s\n    role: %s\n    description: test base\n    retrieval_enabled: true\n", slug, display, role)
}

// writeSeedFile writes body into a fresh temp file and returns its path.
func writeSeedFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "knowledge_bases.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	return path
}

// TestLoadSeedFileGolden loads the in-repo controlled seed and locks the
// four baseline entries: slugs, display names (byte-exact — the refusal
// copy and frozen evals depend on them), one role per entry in the closed
// vocabulary, every switch on, and load timestamps set.
func TestLoadSeedFileGolden(t *testing.T) {
	entries, err := LoadSeedFile(repoSeedPath)
	if err != nil {
		t.Fatalf("LoadSeedFile(%s) = %v, want nil", repoSeedPath, err)
	}
	wantSlugs := []string{"authority", "historical", "personal", "draft"}
	wantDisplay := []string{seedDisplayNameAuthority, seedDisplayNameHistorical, seedDisplayNamePersonal, seedDisplayNameDraft}
	wantRoles := []Role{RoleAuthority, RoleHistorical, RolePersonal, RoleDraft}
	if len(entries) != len(wantSlugs) {
		t.Fatalf("seed returned %d entries, want %d: %+v", len(entries), len(wantSlugs), entries)
	}
	for i, e := range entries {
		if e.Slug != wantSlugs[i] {
			t.Errorf("entries[%d].Slug = %q, want %q (declaration order is pinned)", i, e.Slug, wantSlugs[i])
		}
		if e.DisplayName != wantDisplay[i] {
			t.Errorf("entries[%d].DisplayName = %q, want %q (byte-exact display name)", i, e.DisplayName, wantDisplay[i])
		}
		if e.Role != wantRoles[i] {
			t.Errorf("entries[%d].Role = %q, want %q", i, e.Role, wantRoles[i])
		}
		if !e.RetrievalEnabled {
			t.Errorf("entries[%d] (%s) RetrievalEnabled = false, want true", i, e.Slug)
		}
		if e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
			t.Errorf("entries[%d] (%s) timestamps are zero, want load time", i, e.Slug)
		}
	}
}

// TestLoadSeedFileAbsent locks the single legal degraded state: a missing
// seed file yields no entries and no error.
func TestLoadSeedFileAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	entries, err := LoadSeedFile(path)
	if err != nil {
		t.Fatalf("LoadSeedFile(absent) = %v, want nil error", err)
	}
	if len(entries) != 0 {
		t.Fatalf("LoadSeedFile(absent) returned %d entries, want 0", len(entries))
	}
}

// TestLoadSeedFileMalformedYAML locks the fail-closed posture for a
// truncated/invalid controlled file: the error must name the file.
func TestLoadSeedFileMalformedYAML(t *testing.T) {
	path := writeSeedFile(t, "knowledge_bases: [this is: broken")
	_, err := LoadSeedFile(path)
	if err == nil {
		t.Fatal("LoadSeedFile(malformed) = nil error, want error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("malformed-seed error %q must name the file path", err)
	}
}

// TestLoadSeedFileFailClosedPerField locks per-entry validation: an
// invalid slug, an unknown or empty role, an empty display name and a
// duplicated slug each fail the whole load, and every error names the
// file, the entry index and the slug.
func TestLoadSeedFileFailClosedPerField(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string // substrings the error must contain
	}{
		{
			name: "invalid slug",
			body: seedBody(seedEntryLine("Bad_Slug", "authority", "Test Base")),
			want: []string{"entry 1", "Bad_Slug", "must match"},
		},
		{
			name: "unknown role",
			body: seedBody(seedEntryLine("kb-one", "admin", "Test Base")),
			want: []string{"entry 1", "kb-one", "admin"},
		},
		{
			name: "empty role",
			body: seedBody(`  - slug: kb-one
    display_name: Test Base
    role: ""
    description: test base
    retrieval_enabled: true
`),
			want: []string{"entry 1", "kb-one"},
		},
		{
			name: "empty display name",
			body: seedBody(seedEntryLine("kb-one", "authority", "")),
			want: []string{"entry 1", "kb-one", "display_name"},
		},
		{
			name: "duplicate slug",
			body: seedBody(seedEntryLine("kb-one", "authority", "First") + seedEntryLine("kb-one", "historical", "Second")),
			want: []string{"entry 2", "kb-one", "duplicate"},
		},
		{
			name: "missing retrieval switch",
			body: seedBody(`  - slug: kb-one
    display_name: Test Base
    role: authority
    description: test base
`),
			want: []string{"entry 1", "kb-one", "retrieval_enabled"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeSeedFile(t, tc.body)
			entries, err := LoadSeedFile(path)
			if err == nil {
				t.Fatalf("LoadSeedFile = %d entries, nil error; want fail-closed error", len(entries))
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error %q must name the file path", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q must contain %q", err, want)
				}
			}
		})
	}
}

// TestLoadSeedFileUnknownFieldStrict locks the strict decode: a typo'd
// top-level key must be an error, never a silently dropped field.
func TestLoadSeedFileUnknownFieldStrict(t *testing.T) {
	path := writeSeedFile(t, "version: \"1\"\nknowldge_bases:\n  - slug: authority\n")
	entries, err := LoadSeedFile(path)
	if err == nil {
		t.Fatalf("LoadSeedFile(typo'd key) = %d entries, nil error; want strict-decode error", len(entries))
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q must name the file path", err)
	}
}

// TestLoadSeedRegistry locks the convenience constructor plus the seed
// registry's core read surface: the draft base is active but never
// retrieval-eligible, and the signature is deterministic for a given seed
// and sensitive to any entry change.
func TestLoadSeedRegistry(t *testing.T) {
	reg, err := LoadSeedRegistry(repoSeedPath)
	if err != nil {
		t.Fatalf("LoadSeedRegistry = %v, want nil", err)
	}
	wantActive := []string{"authority", "draft", "historical", "personal"}
	if got := reg.ActiveSlugs(); !equalStrings(got, wantActive) {
		t.Errorf("ActiveSlugs() = %v, want %v", got, wantActive)
	}
	wantEligible := []string{"authority", "historical", "personal"}
	if got := reg.RetrievalEligible(); !equalStrings(got, wantEligible) {
		t.Errorf("RetrievalEligible() = %v, want %v (draft excluded by role)", got, wantEligible)
	}

	sig := reg.Signature()
	if sig == "" {
		t.Fatal("Signature() is empty, want a content hash")
	}
	reg2, err := LoadSeedRegistry(repoSeedPath)
	if err != nil {
		t.Fatalf("second LoadSeedRegistry = %v, want nil", err)
	}
	if reg2.Signature() != sig {
		t.Errorf("Signature() not stable for the same seed: %q vs %q", sig, reg2.Signature())
	}

	entries, err := LoadSeedFile(repoSeedPath)
	if err != nil {
		t.Fatalf("LoadSeedFile = %v, want nil", err)
	}
	entries[0].DisplayName = "Changed"
	if New(entries).Signature() == sig {
		t.Error("Signature() unchanged after an entry field changed, want a different hash")
	}
}

// TestDraftRoleNeverRetrievable locks the role-over-switch rule at the
// seed/registry boundary: a draft-role base never enters the retrieval
// set, no matter what its switch says (A3).
func TestDraftRoleNeverRetrievable(t *testing.T) {
	reg, err := LoadSeedRegistry(repoSeedPath)
	if err != nil {
		t.Fatalf("LoadSeedRegistry = %v, want nil", err)
	}
	if got := reg.RetrievalEligible(); containsString(got, "draft") {
		t.Errorf("RetrievalEligible() = %v contains draft, want it excluded (role gate)", got)
	}

	// synthetic double gate: switch on → active but not eligible;
	// switch off → neither.
	switchedOn := []Entry{{
		Slug:             "d-one",
		DisplayName:      "Draft One",
		Role:             RoleDraft,
		RetrievalEnabled: true,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}}
	on := New(switchedOn)
	if got := on.ActiveSlugs(); !containsString(got, "d-one") {
		t.Errorf("ActiveSlugs() = %v, want d-one present (entry switch is on)", got)
	}
	if got := on.RetrievalEligible(); containsString(got, "d-one") {
		t.Errorf("RetrievalEligible() = %v contains d-one, want it excluded (role gate outranks the switch)", got)
	}

	switchedOff := []Entry{{
		Slug:             "d-one",
		DisplayName:      "Draft One",
		Role:             RoleDraft,
		RetrievalEnabled: false,
	}}
	off := New(switchedOff)
	if got := off.ActiveSlugs(); containsString(got, "d-one") {
		t.Errorf("ActiveSlugs() = %v contains d-one, want it excluded (switch off)", got)
	}
	if got := off.RetrievalEligible(); containsString(got, "d-one") {
		t.Errorf("RetrievalEligible() = %v contains d-one, want it excluded", got)
	}
}

// equalStrings reports whether two string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containsString reports whether s is present in the slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
