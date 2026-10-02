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
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertHex64 asserts s is a full 64-character lowercase hex SHA-256
// digest — the storage contract is NO truncation (truncation lowers
// collision resistance; preview truncation is display-layer only).
func assertHex64(t *testing.T, s string) {
	t.Helper()
	require.Len(t, s, 64)
	for _, c := range s {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"hash must be lowercase hex, got %q in %q", string(c), s)
	}
}

// mustNormalize is the test-only fail-fast Normalize wrapper.
func mustNormalize(t *testing.T, profile, value string) string {
	t.Helper()
	out, err := Normalize(profile, value)
	require.NoError(t, err)
	return out
}

// TestHMACNormalizationJoinability is the DATA-02 join lifeline
// (15-CONTEXT locked test name): same real-world value in different
// surface forms, run through profile normalization then L1Hash, MUST
// produce one identical hash per group. 「张三」vs「张三␣(全角尾空格)」;
// 「+8613800000000」vs「138-0000-0000」vs「１３８００００００００」(全角).
func TestHMACNormalizationJoinability(t *testing.T) {
	salt, err := DeriveDatasetSalt("unit-test-site-salt", "loans")
	require.NoError(t, err)

	// Group 1 — person_name: trailing full-width space must not fork the hash.
	h1 := L1Hash(salt, "customer_name", mustNormalize(t, ProfilePersonName, "张三"))
	h2 := L1Hash(salt, "customer_name", mustNormalize(t, ProfilePersonName, "张三　"))
	assert.Equal(t, h1, h2, "张三 vs 张三+U+3000 must hash identically (person_name)")
	assertHex64(t, h1)
	assertHex64(t, h2)

	// Group 2 — phone_cn: three surface forms of the same number.
	forms := []string{"+8613800000000", "138-0000-0000", "１３８００００００００"}
	hashes := make([]string, 0, len(forms))
	for _, f := range forms {
		h := L1Hash(salt, "mobile", mustNormalize(t, ProfilePhoneCN, f))
		assertHex64(t, h)
		hashes = append(hashes, h)
	}
	for _, h := range hashes[1:] {
		assert.Equal(t, hashes[0], h, "all three phone surface forms must hash identically (phone_cn)")
	}
}

// TestHashStableAcrossPeriods locks Pitfall 4: the L1 hash input set
// contains NO period parameter, so the same (dataset, column, value)
// hashes identically across monthly batches — the precondition for
// Phase 16 period-over-period joins. The signature is compile-time
// locked; determinism, field-domain separation, and cross-dataset
// separation are behaviorally locked.
func TestHashStableAcrossPeriods(t *testing.T) {
	// Signature lock — if anyone adds a period/batch parameter this
	// assignment stops compiling, forcing a conscious contract change.
	var _ func(datasetSalt []byte, column, normalizedValue string) string = L1Hash
	var _ func(siteSalt, dataset string) ([]byte, error) = DeriveDatasetSalt

	saltLoans, err := DeriveDatasetSalt("unit-test-site-salt", "loans")
	require.NoError(t, err)

	// Deterministic: same (dataset, column, value) → byte-identical hash.
	first := L1Hash(saltLoans, "mobile", "13800000000")
	second := L1Hash(saltLoans, "mobile", "13800000000")
	assert.Equal(t, first, second, "L1Hash must be deterministic — no hidden per-call state")

	// Field-domain separation: same value in a different column →
	// different hash ("field:"+column domain tag, STACK-VERIFICATION §3.2).
	otherColumn := L1Hash(saltLoans, "backup_mobile", "13800000000")
	assert.NotEqual(t, first, otherColumn, "same value in a different column must NOT collide")

	// Cross-dataset link rejection: same (column, value) under a
	// different dataset salt → different hash. This is the cryptographic
	// landing of data iron-rule ④ (拒绝跨数据集推断个体) — a feature,
	// not a defect.
	saltDeposits, err := DeriveDatasetSalt("unit-test-site-salt", "deposits")
	require.NoError(t, err)
	crossDataset := L1Hash(saltDeposits, "mobile", "13800000000")
	assert.NotEqual(t, first, crossDataset, "cross-dataset joins must be refused by construction")
}

// TestBucketBoundaryStableAcrossPeriods locks D15-D1: the fixed-width
// bucket of a value depends ONLY on (value, width) — boundary stability
// across periods is what makes month-over-month KPI trends comparable.
func TestBucketBoundaryStableAcrossPeriods(t *testing.T) {
	// Signature lock — no period input exists on the bucket function.
	var _ func(value, width float64) float64 = FixedBucket

	assert.Equal(t, float64(1000), FixedBucket(1234.5, 1000), "1234.5 floors into the [1000,2000) bucket")
	assert.Equal(t, float64(0), FixedBucket(999.99, 1000), "999.99 stays in the [0,1000) bucket")
	assert.Equal(t, float64(1000), FixedBucket(1000, 1000), "the boundary itself belongs to the upper bucket")
	assert.Equal(t, FixedBucket(1234.5, 1000), FixedBucket(1234.5, 1000), "same (value, width) → same bucket on every call")
	// math.Floor contract: negatives floor DOWNWARD into the negative
	// bucket (documented behavior, not an accident).
	assert.Equal(t, float64(-1000), FixedBucket(-0.01, 1000))
}

// TestDeriveDatasetSalt locks the fail-closed site-salt gate (mirror of
// the isAdminRequest unset-env semantics) and the derivation properties:
// deterministic, per-dataset, and full 32-byte SHA-256 output.
func TestDeriveDatasetSalt(t *testing.T) {
	// Fail-closed: empty site salt is an error, never a degenerate key.
	_, err := DeriveDatasetSalt("", "loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SKILLFORGE_DATA_SALT")
	assert.Contains(t, err.Error(), "fail-closed")

	// Deterministic derivation (no storage, no migration).
	s1, err := DeriveDatasetSalt("unit-test-site-salt", "loans")
	require.NoError(t, err)
	s2, err := DeriveDatasetSalt("unit-test-site-salt", "loans")
	require.NoError(t, err)
	assert.True(t, bytes.Equal(s1, s2), "same (siteSalt, dataset) must derive the same salt")
	assert.Len(t, s1, sha256.Size)

	// Per-dataset: different dataset → different salt (cross-dataset
	// link rejection depends on this).
	s3, err := DeriveDatasetSalt("unit-test-site-salt", "deposits")
	require.NoError(t, err)
	assert.False(t, bytes.Equal(s1, s3), "different datasets must derive different salts")

	// Rotating the site salt rotates every derived salt — the documented
	// "full re-import required" operational warning.
	s4, err := DeriveDatasetSalt("rotated-site-salt", "loans")
	require.NoError(t, err)
	assert.False(t, bytes.Equal(s1, s4), "site-salt rotation must invalidate all derived salts")
}

// TestKMinDefault pins the D15-D3 global default literal. Source-grep
// locked externally (verify-phase-15.sh Stage 9); this assertion makes
// the value visible to the hermetic suite as well.
func TestKMinDefault(t *testing.T) {
	assert.Equal(t, 5, KMinDefault)
}
