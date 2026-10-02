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

// WSL snapshot tests (16-06 Task 3) — the third WSL lock.
//
// These two tests verify the post-call guarantees that close the loop
// on the executor's three-output single-source-of-truth contract:
//
//   - TestKPISnapshotFrozen    — KPI-02 replay: the audit row alone
//                                reproduces the call-time snapshot,
//                                even after the upstream dataset has
//                                been DETACHed + os.Remove-d + de-
//                                registered from sqlite.
//
//   - TestSameKPIAcrossSkills  — D16-C4: same KPI name + same params,
//                                across two distinct Skill contexts,
//                                must produce BYTE-IDENTICAL result
//                                rows and the same ASOf.Period. The
//                                executor's singleton Registry gives
//                                us the deterministic-answer property;
//                                ASOf.Ts is call-time metadata and
//                                intentionally differs across calls.

package wsl

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/kpi"
)

// loansFixtureRows returns the same three-bucket fixture used in
// TestKPIEngineIntegration so the snapshot tests assert on a stable
// shape (12 / 3 / 12 with one suppressed bucket).
func loansFixtureRows() []loanRow {
	rows := []loanRow{}
	for i := 0; i < 12; i++ {
		rows = append(rows, loanRow{Status: "正常", Balance: float64(1000 + i*100), OverdueDays: 0})
	}
	for i := 0; i < 3; i++ {
		rows = append(rows, loanRow{Status: "逾期", Balance: float64(5000 + i*100), OverdueDays: int64(10 + i)})
	}
	for i := 0; i < 12; i++ {
		rows = append(rows, loanRow{Status: "已结清", Balance: float64(800 + i*50), OverdueDays: 0})
	}
	return rows
}

// TestKPISnapshotFrozen is the 16-06 KPI-02 lock: a successful call
// writes a fully-frozen audit row. Even after the upstream DuckDB
// dataset is DETACHed + os.Remove-d + sqlite-de-registered, the audit
// row alone is sufficient to replay the call-time snapshot — byte-
// identical to what the agent saw.
//
// Sequence:
//  1. Build the fixture (real sqlite + staging→commit + LRU).
//  2. Call the KPI; capture the audit row and the in-hand Snapshot.
//  3. Simulate dataset cleanup: lru.Detach(composite) →
//     os.Remove(filePath) → store.DeleteDataset(name).
//  4. Unmarshall the audit row's ResultSnapshot and compare against
//     the original Snapshot (rows + k_min_applied + suppressed +
//     as_of, byte-by-byte).
//
// What this LOCKS:
//   - The audit row is the LONG-TERM answer; the upstream dataset can
//     be superseded or cleaned and the answer stays replayable.
//   - k_min suppression is IN the frozen snapshot (small buckets
//     survive as "suppressed" markers, not as hidden rows).
func TestKPISnapshotFrozen(t *testing.T) {
	ctx := context.Background()

	ef := newEngineFixture(t, 4)
	staging := ef.writeLoansStaging(t, "loans__2026-07", loansFixtureRows())
	committed := ef.commitLoansDataset(t, "loans", "2026-07", staging, 10)

	kpiDir := t.TempDir()
	writeLoansKPIYAML(t, kpiDir, false)
	reg := loadWSLRegistry(t, ef, kpiDir)

	audit := &captureAudit{}
	adapter := duckdb.NewEngineAdapter(ef.lru)
	meta := wsMeta{store: ef.store}
	fixedNow := time.Date(2026, 8, 27, 13, 0, 0, 0, time.UTC)
	exec := kpi.NewExecutor(reg, meta, adapter, audit, func() time.Time { return fixedNow })

	// 1) Make the call.
	outcome, err := exec.Call(ctx, kpi.CallRequest{
		Name:      "loan_overdue_rate_by_status",
		SessionID: "sess-frozen",
		SkillID:   "compliance",
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Snapshot, "snapshot must be present")
	require.Len(t, audit.rows, 1, "exactly one audit row")

	original := *outcome.Snapshot
	row := audit.rows[0]

	// 2) Simulate dataset cleanup: DETACH + os.Remove + sqlite delete.
	require.NoError(t, ef.lru.Detach(ctx, committed.Name),
		"DETACH the composite alias")
	require.NoError(t, os.Remove(committed.FilePath),
		"unlink the committed .duckdb file")
	require.NoError(t, ef.store.DeleteDataset(ctx, committed.Name),
		"delete the sqlite registry row")

	// Sanity: the dataset is truly gone.
	_, statErr := os.Stat(committed.FilePath)
	assert.True(t, os.IsNotExist(statErr), "dataset file must be gone after cleanup")
	rows, err := ef.store.ListDatasets(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows, "registry must be empty after cleanup")

	// 3) Replay from the audit row alone — no engine, no dataset, no LRU.
	var replayed kpi.Snapshot
	require.NoError(t, json.Unmarshal([]byte(row.ResultSnapshot), &replayed),
		"ResultSnapshot must be a valid JSON Snapshot")

	// 4) Byte-identity assertions: every leg matches the call-time Snapshot.
	origRowsJSON, err := json.Marshal(original.Rows)
	require.NoError(t, err)
	replayedRowsJSON, err := json.Marshal(replayed.Rows)
	require.NoError(t, err)
	assert.Equal(t, string(origRowsJSON), string(replayedRowsJSON),
		"replayed rows must be byte-identical to the call-time rows (KPI-02)")

	assert.Equal(t, original.KMinApplied, replayed.KMinApplied,
		"k_min_applied must replay identically")
	assert.Equal(t, original.Suppressed, replayed.Suppressed,
		"suppressed count must replay identically")
	assert.Equal(t, original.ASOf.Period, replayed.ASOf.Period,
		"as_of.period must replay identically")
	assert.Equal(t, original.ASOf.Ts, replayed.ASOf.Ts,
		"as_of.ts must replay identically (call instant is frozen, not recomputed)")
	assert.Equal(t, original.Func, replayed.Func, "func must replay identically")

	// 5) Audit row field locks (cross-check vs the executor's policy).
	assert.Equal(t, "loan_overdue_rate_by_status", row.Func)
	assert.Equal(t, "sess-frozen", row.SessionID)
	assert.Equal(t, "compliance", row.SkillID)
	assert.Equal(t, "2026-07", row.ASOfPeriod)
	assert.Equal(t, 10, row.KMinApplied)
	assert.Equal(t, reg.Signature(), row.Signature,
		"audit row carries the registry signature (definition version lock)")
}

// TestSameKPIAcrossSkills is the 16-06 D16-C4 lock: the executor's
// singleton Registry semantics give "same KPI + same params ⇒ same
// answer regardless of caller" (period-stable, skill-independent).
//
// Two calls differ ONLY in (SessionID, SkillID, TS). The marshaled
// Rows + ASOf.Period are byte-identical; the kpi_calls rows differ
// ONLY in (TS, SessionID, SkillID, ID); ResultSnapshot is byte-
// identical (modulo the JSON encoding of TS, which is normalized to
// RFC3339Nano so encoding is stable).
//
// Single Executor instance — the singleton-Registry invariant is
// asserted by sharing reg across BOTH calls (and via the registry
// signature row-locks below). The now() closure increments per call
// so ASOf.Ts deterministically differs (the consistency claim
// explicitly excludes Ts, which is call-time metadata).
func TestSameKPIAcrossSkills(t *testing.T) {
	ctx := context.Background()

	ef := newEngineFixture(t, 4)
	staging := ef.writeLoansStaging(t, "loans__2026-07", loansFixtureRows())
	ef.commitLoansDataset(t, "loans", "2026-07", staging, 5 /* KMinDefault */)

	kpiDir := t.TempDir()
	writeLoansKPIYAML(t, kpiDir, false)
	reg := loadWSLRegistry(t, ef, kpiDir)

	audit := &captureAudit{}
	adapter := duckdb.NewEngineAdapter(ef.lru)
	meta := wsMeta{store: ef.store}

	// Deterministic per-call timestamps: now() returns incrementing
	// instants so ASOf.Ts is reproducible across runs.
	now1 := time.Date(2026, 8, 27, 13, 0, 0, 0, time.UTC)
	now2 := now1.Add(15 * time.Second)
	var calls int
	stampedNow := func() time.Time {
		calls++
		if calls == 1 {
			return now1
		}
		return now2
	}

	// Single Executor — the singleton-Registry invariant (D16-C4).
	exec := kpi.NewExecutor(reg, meta, adapter, audit, stampedNow)

	// --- Call 1: SkillA / SessA
	audit.rows = nil
	outcome1, err := exec.Call(ctx, kpi.CallRequest{
		Name:      "loan_overdue_rate_by_status",
		Params:    map[string]any{"status": "正常"},
		SessionID: "sess-A",
		SkillID:   "skill-A",
	})
	require.NoError(t, err)
	require.NotNil(t, outcome1.Snapshot)

	// --- Call 2: SkillB / SessB (same Executor, same Registry)
	// Do NOT clear audit.rows here — the assertion below expects TWO audit
	// rows, one per call. Clearing between calls would leave only the
	// second row in the slice.
	outcome2, err := exec.Call(ctx, kpi.CallRequest{
		Name:      "loan_overdue_rate_by_status",
		Params:    map[string]any{"status": "正常"},
		SessionID: "sess-B",
		SkillID:   "skill-B",
	})
	require.NoError(t, err)
	require.NotNil(t, outcome2.Snapshot)
	require.Equal(t, 2, calls, "stampedNow called once per Call")

	// 1) Rows: byte-identical (singleton-Registry deterministic answer).
	rows1, err := json.Marshal(outcome1.Snapshot.Rows)
	require.NoError(t, err)
	rows2, err := json.Marshal(outcome2.Snapshot.Rows)
	require.NoError(t, err)
	assert.Equal(t, string(rows1), string(rows2),
		"same KPI + same params across Skills must produce byte-identical rows (D16-C4)")

	// 2) ASOf.Period: identical (system watermark, not call-time metadata).
	assert.Equal(t, outcome1.Snapshot.ASOf.Period, outcome2.Snapshot.ASOf.Period,
		"period must agree across Skills (registry watermark)")
	assert.Equal(t, "2026-07", outcome1.Snapshot.ASOf.Period)

	// 3) ASOf.Ts: intentionally independent (call-time metadata).
	assert.Equal(t, now1.UTC().Format(time.RFC3339Nano), outcome1.Snapshot.ASOf.Ts)
	assert.Equal(t, now2.UTC().Format(time.RFC3339Nano), outcome2.Snapshot.ASOf.Ts)
	assert.NotEqual(t, outcome1.Snapshot.ASOf.Ts, outcome2.Snapshot.ASOf.Ts,
		"call instants differ → ASOf.Ts differs (Ts is NOT part of the consistency claim)")

	// 4) KMinApplied + Suppressed: identical.
	assert.Equal(t, outcome1.Snapshot.KMinApplied, outcome2.Snapshot.KMinApplied)
	assert.Equal(t, outcome1.Snapshot.Suppressed, outcome2.Snapshot.Suppressed)

	// 5) Audit row locks: two rows, differ ONLY in ts/session/skill.
	require.Len(t, audit.rows, 2, "two calls → two audit rows")
	row1, row2 := audit.rows[0], audit.rows[1]
	assert.Equal(t, "sess-A", row1.SessionID)
	assert.Equal(t, "skill-A", row1.SkillID)
	assert.Equal(t, now1.UTC().Truncate(time.Nanosecond), row1.TS.UTC().Truncate(time.Nanosecond))
	assert.Equal(t, "sess-B", row2.SessionID)
	assert.Equal(t, "skill-B", row2.SkillID)
	assert.Equal(t, now2.UTC().Truncate(time.Nanosecond), row2.TS.UTC().Truncate(time.Nanosecond))
	assert.Equal(t, row1.Func, row2.Func)
	assert.Equal(t, row1.ASOfPeriod, row2.ASOfPeriod)
	assert.Equal(t, row1.KMinApplied, row2.KMinApplied)
	assert.Equal(t, row1.Signature, row2.Signature,
		"same registry → same signature across calls")
	// ResultSnapshot byte-identity (modulo TS which is normalized):
	// Both rows marshal Rows to the same JSON (singleton-Registry lock);
	// TS differs in the snapshot's ASOf.Ts field.
	var snap1, snap2 kpi.Snapshot
	require.NoError(t, json.Unmarshal([]byte(row1.ResultSnapshot), &snap1))
	require.NoError(t, json.Unmarshal([]byte(row2.ResultSnapshot), &snap2))
	s1Rows, _ := json.Marshal(snap1.Rows)
	s2Rows, _ := json.Marshal(snap2.Rows)
	assert.Equal(t, string(s1Rows), string(s2Rows),
		"ResultSnapshot rows must be byte-identical across Skills (KPI-02 + D16-C4)")
	assert.Equal(t, snap1.ASOf.Period, snap2.ASOf.Period)
	assert.NotEqual(t, snap1.ASOf.Ts, snap2.ASOf.Ts,
		"ResultSnapshot ASOf.Ts differs — call-time metadata, NOT part of consistency")
}
