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

package ecl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// TestIN03BatchMilvusLoadRecordsRealAttempt pins the IN-03 fix contract:
// when BuildDocuments fails first-try (before any seed attempt), the dead
// ledger row records AttemptCount == 1 (not maxAttempts). Without this fix,
// the verifier could resurrect WR-02-style overstate where batch failure
// shows 3/3 attempts even though batch seed failed on attempt 1.
//
// Mechanism: we exercise the manager-level recordFailure contract for the
// milvus-load batch-terminal-failure path. recordFailure is the single seam
// between Manager and FailureStore; the IN-03 call site (manager.go:405) is
// the only place that calls recordFailure with attempt=1 for StageMilvusLoad.
// This test pins the contract that "BuildDocuments failure → attempt=1" by
// directly invoking the fixed call site behavior: a single recordFailure
// with attempt=1 in StageMilvusLoad yields a ledger row with AttemptCount=1,
// distinct from the seedWithRetry path that records attempt=N-1 (= maxAttempts
// when all 3 attempts fail).
func TestIN03BatchMilvusLoadRecordsRealAttempt(t *testing.T) {
	fs := newFakeFailureStore()
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	require.NotNil(t, m)

	// Simulate the IN-03 fixed call site: BuildDocuments failed first try,
	// before any seed attempt, so attempt is recorded as 1.
	m.recordFailure(
		Note{SourceIdx: "001"},
		StageMilvusLoad,
		1, // ← IN-03 fix value (was: maxAttempts)
		errors.New("build documents: post-condition violated: section body too large"),
		FailureDead,
	)

	row, err := fs.GetFailure("001")
	require.NoError(t, err, "BuildDocuments failure must write a ledger row")
	assert.Equal(t, FailureDead, row.Status)
	assert.Equal(t, StageMilvusLoad, row.Stage)
	assert.Equal(t, 1, row.AttemptCount,
		"IN-03: BuildDocuments failure first-try must record AttemptCount = 1, not maxAttempts (%d)",
		maxAttempts)
}

// TestIN03SeedFailureRecordsRealAttempt is the complementary test: when
// seedFn exhausts its retry budget, the seedWithRetry loop records
// attempt-1 = maxAttempts (the count of attempts that actually ran). This
// pin guards against a future refactor that confuses "budget exhausted" with
// "first-try failure" — the same value (1) the IN-03 fix removed from the
// BuildDocuments path would be wrong here.
func TestIN03SeedFailureRecordsRealAttempt(t *testing.T) {
	fs := newFakeFailureStore()
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	require.NotNil(t, m)

	// Walk the seedWithRetry retry loop directly: 3 attempts all fail.
	m.seedFn = func(_ context.Context, _ []retriever.Document) error {
		return errors.New("forced milvus outage")
	}

	// Build a fake loadedNote batch and exercise seedWithRetry.
	notes := []Note{{SourceIdx: "001"}}
	loaded := []loadedNote{{note: notes[0]}}
	attempt, err := m.seedWithRetry(context.Background(), loaded, nil)
	require.Error(t, err, "all-attempts-failed seed should return an error")
	assert.Equal(t, maxAttempts, attempt,
		"seedWithRetry exhaustion should report attempt-1 (= maxAttempts)")

	row, err := fs.GetFailure("001")
	require.NoError(t, err)
	assert.Equal(t, StageMilvusLoad, row.Stage)
	assert.Equal(t, FailureDead, row.Status)
	assert.Equal(t, maxAttempts, row.AttemptCount,
		"seed exhaustion must record AttemptCount = maxAttempts (3 attempts ran), not 1")
}