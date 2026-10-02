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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validLoansRules mirrors datarules/loans.yaml (the data-staff sample
// shipped with the repo). Every mutation case below starts from this
// baseline and breaks exactly one field. The tier value is a registry
// slug since Phase 26 / Plan 26-04 (KBM-04: the controlled file speaks
// the registry vocabulary, never the retired display names).
func validLoansRules() *Rules {
	return &Rules{
		Dataset: "loans",
		Tier:    "authority",
		KMin:    10,
		Columns: map[string]ColumnRule{
			"customer_name": {Rule: "hash", Profile: "person_name"},
			"mobile":        {Rule: "hash", Profile: "phone_cn"},
			"cert_no":       {Rule: "hash", Profile: "id_card_cn"},
			"balance":       {Rule: "bucket_fixed", Width: 1000},
			"overdue_days":  {Rule: "bucket_fixed", Width: 30},
			"update_ts":     {Rule: "passthrough"},
			"status":        {Rule: "passthrough"},
		},
	}
}

// TestValidateRules locks the fail-closed rule-file validator
// (D15-A1/A2/D4, C2, D3): first-error-short-circuit, named fields,
// vocab allowlists, and the "__" composite-name separator reservation.
func TestValidateRules(t *testing.T) {
	require.NoError(t, ValidateRules(validLoansRules(), testKBReg()), "baseline loans sample must validate")

	tests := []struct {
		name    string
		mutate  func(*Rules)
		wantSub []string
	}{
		{
			name:    "rule value outside vocab names the column and the vocab",
			mutate:  func(r *Rules) { r.Columns["balance"] = ColumnRule{Rule: "encrypt"} },
			wantSub: []string{"balance", "encrypt", "hash", "passthrough", "bucket_fixed"},
		},
		{
			name:    "hash profile outside KnownProfiles points at normalize.go",
			mutate:  func(r *Rules) { r.Columns["mobile"] = ColumnRule{Rule: "hash", Profile: "phone"} },
			wantSub: []string{"mobile", "phone", "normalize.go"},
		},
		{
			name:    "hash with no profile is accepted (default NFC+trim chain)",
			mutate:  func(r *Rules) { r.Columns["remark2"] = ColumnRule{Rule: "hash"} },
			wantSub: nil,
		},
		{
			name:    "bucket_fixed missing width rejected",
			mutate:  func(r *Rules) { r.Columns["balance"] = ColumnRule{Rule: "bucket_fixed"} },
			wantSub: []string{"balance", "width"},
		},
		{
			name:    "bucket_fixed width <= 0 rejected",
			mutate:  func(r *Rules) { r.Columns["balance"] = ColumnRule{Rule: "bucket_fixed", Width: -1000} },
			wantSub: []string{"balance", "width"},
		},
		{
			name:    "tier outside the registry rejected naming the value and the active set",
			mutate:  func(r *Rules) { r.Tier = "classified" },
			wantSub: []string{"classified", "authority", "historical", "personal"},
		},
		{
			name:    "tier draft rejected (imports never produce draft state)",
			mutate:  func(r *Rules) { r.Tier = "draft" },
			wantSub: []string{"draft"},
		},
		{
			name:    "dataset containing __ rejected as reserved separator",
			mutate:  func(r *Rules) { r.Dataset = "loans__2026-07" },
			wantSub: []string{"__", "loans__2026-07"},
		},
		{
			name:    "empty dataset rejected",
			mutate:  func(r *Rules) { r.Dataset = "" },
			wantSub: []string{"dataset"},
		},
		{
			name:    "negative k_min rejected",
			mutate:  func(r *Rules) { r.KMin = -1 },
			wantSub: []string{"k_min"},
		},
		{
			name:    "k_min 0 accepted (0 = global KMinDefault)",
			mutate:  func(r *Rules) { r.KMin = 0 },
			wantSub: nil,
		},
		{
			name:    "duckdb_type outside vocab rejected",
			mutate:  func(r *Rules) { r.Columns["update_ts"] = ColumnRule{Rule: "passthrough", DuckDBType: "DATETIME"} },
			wantSub: []string{"update_ts", "DATETIME", "TIMESTAMP"},
		},
		{
			name:    "duckdb_type VARCHAR accepted",
			mutate:  func(r *Rules) { r.Columns["update_ts"] = ColumnRule{Rule: "passthrough", DuckDBType: "VARCHAR"} },
			wantSub: nil,
		},
		{
			name:    "empty columns map rejected (default-deny needs explicit decisions)",
			mutate:  func(r *Rules) { r.Columns = map[string]ColumnRule{} },
			wantSub: []string{"columns"},
		},
		{
			name:    "nil rules rejected",
			mutate:  func(r *Rules) {},
			wantSub: nil, // handled via direct nil call below
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validLoansRules()
			if tt.mutate != nil {
				tt.mutate(r)
			}
			err := ValidateRules(r, testKBReg())
			if tt.wantSub == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, sub := range tt.wantSub {
				assert.Contains(t, err.Error(), sub)
			}
		})
	}

	require.Error(t, ValidateRules(nil, testKBReg()), "nil rules must be rejected fail-closed")
}

// TestUnknownColumnFailClosed locks D15-A2: a CSV column that is neither
// ruled nor passthrough-declared is rejected, and the error lists ALL
// unknown columns (complete diff, not first-mismatch short-circuit).
func TestUnknownColumnFailClosed(t *testing.T) {
	r := &Rules{
		Dataset: "loans",
		Tier:    "authority",
		Columns: map[string]ColumnRule{
			"customer_name": {Rule: "passthrough"},
			"mobile":        {Rule: "passthrough"},
			"balance":       {Rule: "passthrough"},
			"update_ts":     {Rule: "passthrough"},
		},
	}
	csvCols := []string{"customer_name", "mobile", "balance", "update_ts", "id_card_no", "remark"}

	unknown := UnknownColumns(r, csvCols)
	assert.Equal(t, []string{"id_card_no", "remark"}, unknown, "unknown columns preserve CSV order")

	err := ValidateColumnSet(r, csvCols)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id_card_no", "the error must list EVERY unknown column")
	assert.Contains(t, err.Error(), "remark", "the error must list EVERY unknown column")

	assert.NoError(t, ValidateColumnSet(r, csvCols[:4]), "exact match must pass")
}

// TestRuleColumnMissingFromCSV locks the two-way schema check (Pitfall
// 21): a rule-file column absent from the CSV is ALSO a rejection —
// schema drift is caught in both directions.
func TestRuleColumnMissingFromCSV(t *testing.T) {
	r := validLoansRules()
	csvCols := []string{"customer_name", "mobile", "cert_no", "balance", "overdue_days", "update_ts"} // no status

	missing := MissingColumns(r, csvCols)
	assert.Equal(t, []string{"status"}, missing)

	err := ValidateColumnSet(r, csvCols)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

// TestLoadRulesComputesSHA256 locks the D15-A4 audit anchor: the rules
// digest returned by LoadRules is exactly sha256 of the file bytes, so
// an import-batch audit row can prove which rule version produced it.
// Also locks the V12-mirror symlink rejection.
func TestLoadRulesComputesSHA256(t *testing.T) {
	path := filepath.Join("..", "..", "datarules", "loans.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "datarules/loans.yaml must ship with the repo")
	sum := sha256.Sum256(raw)
	wantSHA := hex.EncodeToString(sum[:])

	rules, gotSHA, err := LoadRules(path, testKBReg())
	require.NoError(t, err)
	assert.Equal(t, wantSHA, gotSHA, "rules_sha256 must be sha256 of the raw file bytes")
	assert.Equal(t, "loans", rules.Dataset)
	assert.Equal(t, "historical", rules.Tier,
		"the shipped sample must speak the registry slug vocabulary (KBM-04)")

	// Same bytes → same digest on every load (audit determinism).
	_, gotSHA2, err := LoadRules(path, testKBReg())
	require.NoError(t, err)
	assert.Equal(t, gotSHA, gotSHA2)

	// Validation failure carries the file path (loader.go:202-215 form).
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("dataset: x\ntier: authority\ncolumns:\n  a: {rule: encrypt}\n"), 0o644))
	_, _, err = LoadRules(bad, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), bad, "load error must carry the file path")
	assert.Contains(t, err.Error(), "encrypt")

	// Symlinked rule file → rejected (V12 mirror; controlled files are
	// regular files only — an atomic symlink swap must not be followed).
	real := filepath.Join(dir, "real.yaml")
	require.NoError(t, os.WriteFile(real, raw, 0o644))
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation not permitted on this host: %v", err)
	}
	_, _, err = LoadRules(link, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

// --- Phase 26 / Plan 26-04 Task 2: the dataset tier contract is registry-driven ---

// testKBReg returns the read-only registry the rule-file validator
// resolves tiers against: the four baseline bases of the controlled seed
// (knowledge_bases.yaml), all active. Display names are ASCII here —
// they never participate in tier resolution.
func testKBReg() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	})
}

// TestValidateRulesTierViaRegistry — the tier field must name a LIVE
// registry knowledge base and must never resolve to the draft role
// (KBM-03: dataset imports never produce draft state). A nil registry
// fails closed: an import rule whose trust tier cannot be verified must
// not load at all.
func TestValidateRulesTierViaRegistry(t *testing.T) {
	r := validLoansRules()

	// tier: authority — a live non-draft base → valid.
	r.Tier = "authority"
	require.NoError(t, ValidateRules(r, testKBReg()), "a live slug must validate")

	// tier: ghost — unregistered → error listing the active slug set.
	r.Tier = "ghost"
	err := ValidateRules(r, testKBReg())
	require.Error(t, err, "an unregistered slug must fail closed")
	assert.Contains(t, err.Error(), "ghost")
	for _, active := range []string{"authority", "historical", "personal"} {
		assert.Contains(t, err.Error(), active,
			"the error MUST list the active slug set so the data steward can self-correct")
	}

	// tier: draft — a live base, but the draft role is forbidden here.
	r.Tier = "draft"
	err = ValidateRules(r, testKBReg())
	require.Error(t, err, "draft-role bases are excluded from dataset imports (KBM-03)")
	assert.Contains(t, err.Error(), "draft")

	// nil registry → fail closed.
	r.Tier = "authority"
	require.Error(t, ValidateRules(r, nil),
		"without a registry the trust tier cannot be verified — fail closed")
}

// TestLoansRulesFileLoads — the shipped controlled file and the validator
// migrate in the SAME plan: the sample must validate against the seed
// registry, so there is never a window where the code wants slugs and the
// file still speaks the retired display-name vocabulary (T-26-04-2).
func TestLoansRulesFileLoads(t *testing.T) {
	path := filepath.Join("..", "..", "datarules", "loans.yaml")
	rules, sha, err := LoadRules(path, testKBReg())
	require.NoError(t, err,
		"datarules/loans.yaml must validate against the seed registry (KBM-04 one-shot migration)")
	assert.Equal(t, "historical", rules.Tier,
		"the sample's tier is the historical-role slug (role vocabulary)")
	assert.NotEmpty(t, sha, "the D15-A4 rules digest must still be produced")
}
