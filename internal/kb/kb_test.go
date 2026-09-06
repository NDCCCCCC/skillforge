// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import "testing"

// TestRolesClosedSet locks the closed role vocabulary: exactly four
// values, in the fixed deterministic order, with IsRole accepting only
// exact lowercase members (KBM-03 "choose one of four"; roles are
// platform semantics, not configuration).
func TestRolesClosedSet(t *testing.T) {
	got := Roles()
	want := []Role{RoleAuthority, RoleHistorical, RolePersonal, RoleDraft}
	if len(got) != len(want) {
		t.Fatalf("Roles() returned %d values, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Roles()[%d] = %q, want %q (order is part of the contract)", i, got[i], want[i])
		}
	}
	for _, r := range want {
		if !IsRole(string(r)) {
			t.Errorf("IsRole(%q) = false, want true", r)
		}
	}
	for _, bad := range []string{"", "Authority", "ADMIN", "admin", "write"} {
		if IsRole(bad) {
			t.Errorf("IsRole(%q) = true, want false (closed set, case-sensitive)", bad)
		}
	}
}

// TestCitableRoleIsAuthority pins the code-level single source of the
// citation whitelist (KBM-03): CitableRole must equal RoleAuthority, so
// consumers compare against the constant and never against a literal.
func TestCitableRoleIsAuthority(t *testing.T) {
	if CitableRole != RoleAuthority {
		t.Fatalf("CitableRole = %q, want %q (citation whitelist accepts the authority role only)", CitableRole, RoleAuthority)
	}
	found := false
	for _, r := range Roles() {
		if r == CitableRole {
			found = true
		}
	}
	if !found {
		t.Fatalf("CitableRole %q is not a member of Roles() — the whitelist would cite a role that cannot exist", CitableRole)
	}
}
