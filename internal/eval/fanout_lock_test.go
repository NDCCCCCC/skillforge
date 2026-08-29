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

package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fanoutLockPath / fanoutGoldenSetPath are the relative paths from
// internal/eval to the fan-out frozen lock and its pinned golden set
// (same posture as platformFloorLockPath).
var (
	fanoutLockPath      = filepath.Join("..", "..", "eval", "frozen", "fanout.lock.json")
	fanoutGoldenSetPath = filepath.Join("..", "..", "eval", "golden", "fanout.jsonl")
)

// fanoutLockStruct mirrors the CONSUMED subset of
// eval/frozen/fanout.lock.json. Fields this lock test does not assert
// are deliberately absent — the lock owns its own schema; this struct
// only pins what FAN-06 gate evidence depends on.
type fanoutLockStruct struct {
	LockID        string `json:"lock_id"`
	SchemaVersion string `json:"schema_version"`
	EvalSet       string `json:"eval_set"`
	EvalSetSHA256 string `json:"eval_set_sha256"`
	CaseCount     int    `json:"case_count"`
	CategoryCount struct {
		MultiHop       int `json:"multi_hop"`
		TierMismatch   int `json:"tier_mismatch_rejection"`
		PartialFailure int `json:"partial_failure_conservative_merge"`
		SingleVertex   int `json:"single_vertex_fallback"`
		MonthlyReport  int `json:"monthly_report"`
	} `json:"category_counts"`
	Thresholds struct {
		MonthlyReportFanoutRateMin float64 `json:"monthly_report_fanout_rate_min"`
	} `json:"thresholds"`
	Metrics struct {
		MonthlyReportFanoutRateActual *float64 `json:"monthly_report_fanout_rate_actual"`
	} `json:"metrics"`
	LockSHA256 string `json:"lock_sha256"`
}

// TestFanOutLockSelfConsistent — the shipped fanout.lock.json must be
// internally consistent (case_count == sum of category_counts) and
// byte-consistent with the golden set it pins (FAN-06 structural lock,
// 18-VERIFICATION GAP-2; plan 18-02/18-03 declared artifact).
func TestFanOutLockSelfConsistent(t *testing.T) {
	raw, err := os.ReadFile(fanoutLockPath)
	require.NoError(t, err, "read fanout lock")

	var lock fanoutLockStruct
	require.NoError(t, json.Unmarshal(raw, &lock), "decode fanout lock")

	assert.NotEmpty(t, lock.LockID, "lock_id must be present")
	assert.Equal(t, "v1", lock.SchemaVersion, "schema_version must stay v1")
	assert.Equal(t, "eval/golden/fanout.jsonl", lock.EvalSet,
		"eval_set must point at the shipped golden set")

	// Overall case count matches the lock's own category metadata.
	assert.Equal(t, 24, lock.CaseCount, "lock case_count must be 24")
	sum := lock.CategoryCount.MultiHop + lock.CategoryCount.TierMismatch +
		lock.CategoryCount.PartialFailure + lock.CategoryCount.SingleVertex +
		lock.CategoryCount.MonthlyReport
	assert.Equal(t, lock.CaseCount, sum,
		"case_count must equal the sum of category_counts (internal consistency)")

	// The monthly_report threshold row (FAN-06 gate) and its frozen
	// companions.
	assert.Equal(t, 4, lock.CategoryCount.MonthlyReport,
		"monthly_report category must carry exactly the 4 frozen rpt-fanout cases")
	assert.Equal(t, 0.95, lock.Thresholds.MonthlyReportFanoutRateMin,
		"monthly_report_fanout_rate_min threshold must stay 0.95")

	// The honest pending-live-rerun marker: monthly_report_fanout_rate_actual
	// is null until the operator's live rerun (see the lock's
	// _monthly_report_re_measure_required note). Filling it is an operator
	// re-freeze action and MUST update this test deliberately.
	assert.Nil(t, lock.Metrics.MonthlyReportFanoutRateActual,
		"monthly_report_fanout_rate_actual must stay null until the operator live rerun re-freezes the lock")

	// eval_set_sha256 pins the golden set's LF-normalized content (the
	// lock's _review_fix_note convention — git-blob semantics, robust to
	// CRLF checkouts).
	golden, err := os.ReadFile(fanoutGoldenSetPath)
	require.NoError(t, err, "read fanout golden set")
	normalized := strings.ReplaceAll(string(golden), "\r\n", "\n")
	goldenSum := sha256.Sum256([]byte(normalized))
	assert.Equal(t, hex.EncodeToString(goldenSum[:]), lock.EvalSetSHA256,
		"eval_set_sha256 must equal sha256(LF-normalized eval/golden/fanout.jsonl)")
}

// TestFanOutLockSHA256Recomputable — the lock's own sha256 convention is
// reproducible: sha256(UTF-8 body EXCLUDING the lock_sha256 field), the
// file content byte-for-byte with the single `"lock_sha256": "<value>"`
// line removed (documented in the lock's _lock_sha256_convention field).
func TestFanOutLockSHA256Recomputable(t *testing.T) {
	raw, err := os.ReadFile(fanoutLockPath)
	require.NoError(t, err, "read fanout lock")

	var lock fanoutLockStruct
	require.NoError(t, json.Unmarshal(raw, &lock), "decode fanout lock")
	require.NotEmpty(t, lock.LockSHA256, "lock_sha256 must be present")

	// Byte-faithful strip: drop exactly the line whose trimmed form is
	// the lock_sha256 KEY line (the _lock_sha256_convention line MERELY
	// CONTAINS the token — it must survive). Join with \n, no trailing
	// newline added.
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	dropped := 0
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `"lock_sha256":`) {
			dropped++
			continue
		}
		kept = append(kept, ln)
	}
	require.Equal(t, 1, dropped, "exactly one lock_sha256 key line must exist")
	body := strings.Join(kept, "\n")
	bodySum := sha256.Sum256([]byte(body))
	assert.Equal(t, lock.LockSHA256, hex.EncodeToString(bodySum[:]),
		"lock_sha256 must be reproducible via the strip-line convention")
}
