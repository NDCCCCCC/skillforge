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

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalize pins the three normalization profile chains
// (15-RESEARCH Pattern 2 — measured, locked implementation spec) plus
// the fail-closed unknown-profile rejection. These chains are the
// DATA-02 join lifeline: same real-world value in different surface
// forms MUST normalize to one identical string before hashing.
func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		in      string
		want    string
	}{
		// person_name: NFC → TrimSpace. NFKC is forbidden here —
		// compatibility folding merges distinct values (Pitfall 5).
		{"person_name trailing U+3000 full-width space trimmed", ProfilePersonName, "张三　", "张三"},
		{"person_name leading+trailing half-width spaces trimmed", ProfilePersonName, "  张三  ", "张三"},
		{"person_name NFC keeps circled digit (NFKC would fold to 1)", ProfilePersonName, "王①", "王①"},
		{"person_name NFC keeps full-width digits (NFKC would fold to 138)", ProfilePersonName, "陈１３８", "陈１３８"},

		// phone_cn: NFKC → strip separators → conditional +86 strip.
		{"phone_cn +86 prefix stripped", ProfilePhoneCN, "+8613800000000", "13800000000"},
		{"phone_cn bare 86 prefix stripped", ProfilePhoneCN, "8613800000000", "13800000000"},
		{"phone_cn hyphen separators stripped", ProfilePhoneCN, "138-0000-0000", "13800000000"},
		{"phone_cn full-width digits folded by NFKC", ProfilePhoneCN, "１３８００００００００", "13800000000"},
		{"phone_cn full-width +86 folded then stripped", ProfilePhoneCN, "＋８６１３８００００００００", "13800000000"},
		{"phone_cn inner spaces stripped", ProfilePhoneCN, "138 0000 0000", "13800000000"},
		{
			// Conditional strip: rest after "86" is NOT 11 digits → the
			// landline shape is preserved verbatim (never forcibly strip
			// a non-mobile value — join-safety reading of "validate 11 digits").
			"phone_cn landline 86-prefix NOT stripped (rest is 9 digits)", ProfilePhoneCN, "86-10-1234567", "86101234567",
		},

		// id_card_cn: NFKC → ToUpper → remove whitespace.
		{"id_card_cn lowercase x uppercased", ProfileIDCardCN, "11010119900307765x", "11010119900307765X"},
		{"id_card_cn full-width x folded then uppercased", ProfileIDCardCN, "11010119900307765ｘ", "11010119900307765X"},
		{"id_card_cn full-width digits folded", ProfileIDCardCN, "１１０１０１１９９００３０７７６５Ｘ", "11010119900307765X"},
		{"id_card_cn inner whitespace removed", ProfileIDCardCN, "110101 19900307 765X", "11010119900307765X"},

		// default chain (hash column with no explicit profile):
		// NFC → TrimSpace — identical to person_name's chain.
		{"default (empty profile) trims surrounding whitespace", "", "  张三　", "张三"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.profile, tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	// Unknown profile → fail-closed error pointing at KnownProfiles()
	// (the single source of truth in normalize.go — prevents "rule name
	// hallucination" in datarules/*.yaml).
	_, err := Normalize("nonexistent_profile", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_profile")
	for _, p := range KnownProfiles() {
		assert.Contains(t, err.Error(), p, "error must list every known profile")
	}
}

// TestKnownProfiles pins the single source of truth for profile names:
// exactly the three v1 profiles, deterministic order, and every entry
// actually normalizes (guards against a table/implementation drift).
func TestKnownProfiles(t *testing.T) {
	first := KnownProfiles()
	second := KnownProfiles()
	assert.Equal(t, []string{ProfileIDCardCN, ProfilePersonName, ProfilePhoneCN}, first)
	assert.Equal(t, first, second, "KnownProfiles must be deterministic across calls")
	for _, p := range first {
		_, err := Normalize(p, "张三")
		assert.NoError(t, err, "every KnownProfiles entry must be normalizable")
	}
}
