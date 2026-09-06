// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package skill test — Phase 26 / Plan 26-04 Task 1 (TDD).
//
// ResolveTiers is now a thin delegate over kb.Registry.ResolveReads
// (KBM-01): the compiled-in TierEnum enumeration is deleted, so the
// reads contract is validated against the live knowledge-base registry
// instead of a 4-value map. These tests pin the delegation contract and
// the two semantics that are part of the Skill-author experience:
// the data:-prefix defense-in-depth guard and the fail-closed
// unknown-KB error that lists the active slug set.
//
// RED phase (this commit): the test file targets the two-argument
// ResolveTiers(reads, reg) signature, which does not exist yet.

package skill

import (
	"testing"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baselineKBEntries returns the four baseline knowledge bases of the
// controlled seed (knowledge_bases.yaml), all active. Only Slug / Role /
// RetrievalEnabled participate in reads resolution; the display names
// are ASCII here so this package stays free of CN data-layer literals
// (the structural gate greps the test files too).
func baselineKBEntries() []kb.Entry {
	return []kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	}
}

// testKBReg returns the read-only registry over the four baseline bases.
// The test-only registry constructor is shared by every test in this
// package that resolves or validates reads.
func testKBReg() kb.Registry {
	return kb.New(baselineKBEntries())
}

// testKBRegHistoricalDeactivated returns the baseline registry with the
// historical base deactivated — the KBM-02 deactivation posture that a
// skill referencing it must fail closed against.
func testKBRegHistoricalDeactivated() kb.Registry {
	entries := baselineKBEntries()
	for i := range entries {
		if entries[i].Slug == "historical" {
			entries[i].RetrievalEnabled = false
		}
	}
	return kb.New(entries)
}

// TestResolveTiersDelegatesToRegistry pins the KBM-01 delegation: the
// resolved set is the registry's ResolveReads output — slug values, not
// a compiled-in mapping. Slug patterns and bare slugs both resolve, and
// overlapping patterns dedupe.
func TestResolveTiersDelegatesToRegistry(t *testing.T) {
	reg := testKBReg()

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"slug_pattern", []string{"authority/**"}, []string{"authority"}},
		{"bare_slug", []string{"authority"}, []string{"authority"}},
		{"multi_slug", []string{"authority/**", "historical"}, []string{"authority", "historical"}},
		{"dedup_overlap", []string{"authority/**", "authority/policies/**"}, []string{"authority"}},
		{"deep_path", []string{"authority/policies/anti-bribery/v3"}, []string{"authority"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTiers(tc.in, reg)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			// The delegation is exact: ResolveTiers adds nothing of its
			// own beyond the data:-prefix guard below.
			want, err := reg.ResolveReads(tc.in)
			require.NoError(t, err)
			assert.Equal(t, want, got,
				"ResolveTiers MUST be a thin delegate of kb.Registry.ResolveReads (KBM-01)")
		})
	}
}

// TestResolveTiersDataPrefixGuard — the defense-in-depth guard keeps its
// position BEFORE the delegation: a data-channel value reaching
// ResolveTiers is a caller bug (Validate branches first, D16-C1/C2) and
// must surface as this guard's error, never as a registry lookup.
func TestResolveTiersDataPrefixGuard(t *testing.T) {
	_, err := ResolveTiers([]string{"data:aggregated_only"}, testKBReg())
	require.Error(t, err,
		"ResolveTiers MUST reject data: prefixed reads (defense-in-depth guard, D16-C2)")
	assert.Contains(t, err.Error(), "data:",
		"error MUST name the data: prefix")
	assert.Contains(t, err.Error(), "D16-C2",
		"error MUST cite D16-C2 for traceability")
}

// TestResolveTiersUnknownListsActive — the Skill-author error contract
// survives the delegation: an unknown slug fails closed and the message
// lists the active set (the KBM-04 wording of the old known-dirs list).
func TestResolveTiersUnknownListsActive(t *testing.T) {
	_, err := ResolveTiers([]string{"nope/**"}, testKBReg())
	require.Error(t, err,
		"unknown KB must fail closed (default-deny: silently resolving to nothing would mask a typo)")
	assert.Contains(t, err.Error(), "unknown or deactivated KB",
		"error MUST use the registry's unknown-or-deactivated wording")
	assert.Contains(t, err.Error(), "nope",
		"error MUST echo the offending top segment")
	for _, active := range []string{"authority", "historical", "personal"} {
		assert.Contains(t, err.Error(), active,
			"error MUST list the active slug set so the author can self-correct")
	}
}

// TestResolveTiersEmptyReadsReturnsNilNoError — ResolveTiers still does
// NOT enforce default-deny (that is Validate's job) and must answer the
// empty case before touching the registry.
func TestResolveTiersEmptyReadsReturnsNilNoError(t *testing.T) {
	got, err := ResolveTiers([]string{}, testKBReg())
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = ResolveTiers(nil, testKBReg())
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestResolveTiersNilRegistryFailsClosed — a nil registry is a wiring
// bug, never a silent pass. Non-empty reads cannot be resolved without
// the registry, so the fail-closed answer is an error (Validate turns
// it into a load failure, BuildSubAgent into a construction failure).
func TestResolveTiersNilRegistryFailsClosed(t *testing.T) {
	_, err := ResolveTiers([]string{"authority/**"}, nil)
	require.Error(t, err,
		"nil registry MUST fail closed — a silent empty set would be indistinguishable from default-deny")
	assert.Contains(t, err.Error(), "registry",
		"error MUST name the missing registry so the operator can fix the wiring")
}

// TestResolveTiersDeactivatedKBRejected — a deactivated base is
// indistinguishable from an unregistered one for reads resolution
// (KBM-02): the skill stops loading rather than silently narrowing.
func TestResolveTiersDeactivatedKBRejected(t *testing.T) {
	_, err := ResolveTiers([]string{"historical/**"}, testKBRegHistoricalDeactivated())
	require.Error(t, err,
		"a deactivated KB must fail closed at reads resolution (KBM-02)")
	assert.Contains(t, err.Error(), "historical")
}

// TestResolveTiersRoleWildcardAccepted — the KBM-04 wildcard expands to
// the active retrieval-eligible members of the role.
func TestResolveTiersRoleWildcardAccepted(t *testing.T) {
	got, err := ResolveTiers([]string{"role:authority"}, testKBReg())
	require.NoError(t, err)
	assert.Equal(t, []string{"authority"}, got)
}

// TestResolveTiersDraftReadResolvesEmpty — draft-role semantics outrank
// the explicit declaration: the base is dropped, not granted (A3), so
// an explicit draft read yields the empty default-deny set.
func TestResolveTiersDraftReadResolvesEmpty(t *testing.T) {
	got, err := ResolveTiers([]string{"draft/**"}, testKBReg())
	require.NoError(t, err)
	assert.Empty(t, got,
		"a draft-role KB contributes nothing to the resolved set (KBM-03: draft is never retrievable)")
}
