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

package kpi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// ---------- hermetic fixture infrastructure ----------

// testASTEnvelope mimics the probe-recorded json_serialize_sql output for a
// SELECT whose only BASE_TABLE reference is ds_loans.data (16-01 A3 split
// form). The fake serializer ignores the statement text — ValidateSQLAST
// compares against the FILE's declared dataset ("loans").
const testASTEnvelope = `{"error":false,"error_type":"","error_message":"","statements":[{"node":{"type":"SELECT_NODE","from_table":{"type":"BASE_TABLE","schema_name":"ds_loans","table_name":"data"},"select_list":[]}}]}`

// fakeSerializer stands in for the CGO DuckDB serializer (hermetic).
type fakeSerializer struct{}

func (fakeSerializer) SerializeSQL(_ context.Context, _ string) (string, error) {
	return testASTEnvelope, nil
}

// emptyLister models the bootstrap window (no datasets registered → existence
// check skipped by LoadKPIs).
type emptyLister struct{}

func (emptyLister) ListDatasets(context.Context) ([]string, error) {
	return nil, nil
}

// stubMeta returns a fixed DatasetVersion (or an error for the fallback path).
type stubMeta struct {
	version DatasetVersion
	err     error
}

func (m stubMeta) LatestDatasetVersion(context.Context, string) (DatasetVersion, error) {
	return m.version, m.err
}

// stubEngine records the last EngineRequest and replays canned rows/errors.
type stubEngine struct {
	rows    []map[string]any
	err     error
	lastReq EngineRequest
	calls   int
}

func (e *stubEngine) Query(_ context.Context, req EngineRequest) ([]map[string]any, error) {
	e.calls++
	e.lastReq = req
	if e.err != nil {
		return nil, e.err
	}
	return e.rows, nil
}

// captureAudit records every KPICallRow (or fails with canned err).
type captureAudit struct {
	rows []store.KPICallRow
	err  error
}

func (a *captureAudit) PutKPICall(_ context.Context, row store.KPICallRow) error {
	if a.err != nil {
		return a.err
	}
	a.rows = append(a.rows, row)
	return nil
}

var fixedNow = time.Date(2026, 8, 27, 12, 30, 15, 250_000_000, time.UTC)

// writeFixtureDirs writes the loans-shaped KPI fixture in two variants
// (status optional vs required) into fresh temp dirs. Returns dir paths.
func writeLoansFixture(t *testing.T, statusRequired bool) string {
	t.Helper()
	dir := t.TempDir()
	required := "false"
	if statusRequired {
		required = "true"
	}
	yaml := `
dataset: loans
description: 测试用贷款指标集（16-06 hermetic fixture）
kpis:
  - name: loan_overdue_rate_by_status
    description: 各状态贷款逾期率
    sql: |
      SELECT status AS status,
             count(*) AS row_count,
             round(sum(CASE WHEN overdue_days > 0 THEN 1 ELSE 0 END) / count(*), 2) AS overdue_rate
      FROM ds_loans.data
      WHERE (:status IS NULL OR status = :status)
      GROUP BY status
    count_column: row_count
    max_rows: 1000
    params:
      status:
        type: enum
        values: [正常, 逾期]
        required: ` + required + `
  - name: loan_bucket_overview
    description: 桶分布概览（无参数）
    sql: |
      SELECT balance AS balance_bucket,
             count(*) AS row_count
      FROM ds_loans.data
      GROUP BY balance
    count_column: row_count
    params: {}
`
	path := filepath.Join(dir, "loans.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

// loadTestRegistry runs the full two-layer LoadKPIs pipeline against the
// fixture dir using the hermetic fake serializer (CGO-free).
func loadTestRegistry(t *testing.T, dir string) *Registry {
	t.Helper()
	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	if err != nil {
		t.Fatalf("LoadKPIs fixture failed: %v", err)
	}
	if reg == nil {
		t.Fatal("LoadKPIs returned nil registry without error")
	}
	return reg
}

// defaultStubMeta returns the standard available-dataset version row.
func defaultStubMeta() stubMeta {
	return stubMeta{version: DatasetVersion{
		Period:   "2026-07",
		FilePath: "/tmp/nonexistent/data/datasets/loans__2026-07.duckdb",
	}}
}

// sampleEngineRows returns the canned 2-bucket result set (one healthy, one
// below any plausible kmin) used across the four locks.
func sampleEngineRows() []map[string]any {
	return []map[string]any{
		{"status": "正常", "row_count": int64(120), "overdue_rate": 5.5},
		{"status": "逾期", "row_count": int64(3), "overdue_rate": 66.6},
	}
}

// ---------- Lock ① TestNoMatchingKPIRefusal ----------

// TestNoMatchingKPIRefusal: unknown name → structured tool RESULT (nil Go
// error) carrying reason_code=no_matching_kpi + constructive Chinese
// explanation + the SORTED full catalog of registered names (D16-B2/DATA-12).
func TestNoMatchingKPIRefusal(t *testing.T) {
	reg := loadTestRegistry(t, writeLoansFixture(t, false))
	exec := NewExecutor(reg, defaultStubMeta(), &stubEngine{}, &captureAudit{}, func() time.Time { return fixedNow })

	outcome, err := exec.Call(context.Background(), CallRequest{
		Name:      "total_gdp_forecast",
		SessionID: "sess-1",
		SkillID:   "compliance",
	})

	if err != nil {
		t.Fatalf("no_matching_kpi must be a TOOL RESULT, not a Go error (D16-B2); got err=%v", err)
	}
	if outcome.Refusal == nil || outcome.Snapshot != nil {
		t.Fatalf("expected Refusal-only outcome, got refusal=%v snapshot=%v", outcome.Refusal, outcome.Snapshot)
	}
	r := outcome.Refusal
	if r.ReasonCode != ReasonNoMatchingKPI && r.ReasonCode != "no_matching_kpi" {
		t.Errorf("ReasonCode = %q, want %q", r.ReasonCode, "no_matching_kpi")
	}
	if strings.TrimSpace(r.HumanExplanation) == "" {
		t.Error("human_explanation must be non-empty (DATA-12)")
	} else if !strings.Contains(r.HumanExplanation, "available_kpis") {
		t.Errorf("human_explanation %q should point the agent at available_kpis (constructive narration)", r.HumanExplanation)
	}

	wantNames := []string{"loan_bucket_overview", "loan_overdue_rate_by_status"}
	got := r.AvailableKPIs
	if len(got) != len(wantNames) {
		t.Fatalf("AvailableKPIs = %v, want sorted full catalog %v", got, wantNames)
	}
	for i := range wantNames {
		if got[i] != wantNames[i] {
			t.Errorf("AvailableKPIs[%d] = %q, want %q (sorted)", i, got[i], wantNames[i])
		}
	}
}

// ---------- Lock ② TestEnumParamFailClosed ----------

// TestEnumParamFailClosed: param-domain enforcement BEFORE binding — enum
// out-of-domain / required-missing / reserved as_of all yield
// invalid_params refusals (tool results); a legal value reaches the engine.
func TestEnumParamFailClosed(t *testing.T) {
	ctx := context.Background()

	newExec := func(t *testing.T, required bool) (*Executor, *stubEngine, *captureAudit) {
		reg := loadTestRegistry(t, writeLoansFixture(t, required))
		engine := &stubEngine{}
		audit := &captureAudit{}
		return NewExecutor(reg, defaultStubMeta(), engine, audit, func() time.Time { return fixedNow }), engine, audit
	}

	t.Run("enum_out_of_domain", func(t *testing.T) {
		exec, engine, _ := newExec(t, false)
		outcome, err := exec.Call(ctx, CallRequest{Name: "loan_overdue_rate_by_status", Params: map[string]any{"status": "全部"}})
		if err != nil {
			t.Fatalf("invalid_params must be a tool result, got err=%v", err)
		}
		if outcome.Refusal == nil || outcome.Refusal.ReasonCode != ReasonInvalidParams {
			t.Fatalf("want invalid_params refusal, got %+v", outcome)
		}
		expl := outcome.Refusal.HumanExplanation
		if !strings.Contains(expl, "status") {
			t.Errorf("explanation must name the offending param: %q", expl)
		}
		if !strings.Contains(expl, "逾期") || !strings.Contains(expl, "正常") {
			t.Errorf("explanation must list the legal enum domain: %q", expl)
		}
		if engine.calls != 0 {
			t.Errorf("refused call must NEVER reach the engine, engine.calls=%d", engine.calls)
		}
	})

	t.Run("legal_enum_reaches_engine", func(t *testing.T) {
		exec, engine, _ := newExec(t, false)
		engine.rows = []map[string]any{{"status": "逾期", "row_count": int64(45), "overdue_rate": 88.8}}
		outcome, err := exec.Call(ctx, CallRequest{Name: "loan_overdue_rate_by_status", Params: map[string]any{"status": "逾期"}})
		if err != nil {
			t.Fatalf("legal call failed: %v", err)
		}
		if outcome.Refusal != nil || outcome.Snapshot == nil {
			t.Fatalf("expected snapshot, got %+v", outcome)
		}
		if engine.calls != 1 {
			t.Fatalf("legal value must reach the engine exactly once, calls=%d", engine.calls)
		}
		if len(engine.lastReq.Args) != 1 || engine.lastReq.Args[0].Value != "逾期" {
			t.Errorf("engine should receive sql.Named(status=逾期), got %+v", engine.lastReq.Args)
		}
		if n := len(outcome.Snapshot.Rows); n != 1 {
			t.Errorf("snapshot rows = %d, want 1 (only the queried status)", n)
		}
	})

	t.Run("required_missing", func(t *testing.T) {
		exec, _, _ := newExec(t, true) // fixture variant declares status REQUIRED
		outcome, err := exec.Call(ctx, CallRequest{Name: "loan_overdue_rate_by_status", Params: map[string]any{}})
		if err != nil {
			t.Fatalf("required-missing must be a tool result, got err=%v", err)
		}
		if outcome.Refusal == nil || outcome.Refusal.ReasonCode != ReasonInvalidParams {
			t.Fatalf("want invalid_params refusal, got %+v", outcome)
		}
		if !strings.Contains(outcome.Refusal.HumanExplanation, "status") {
			t.Errorf("explanation must name the missing required param: %q", outcome.Refusal.HumanExplanation)
		}
	})

	t.Run("reserved_as_of_param_rejected", func(t *testing.T) {
		exec, engine, _ := newExec(t, false)
		outcome, err := exec.Call(ctx, CallRequest{
			Name:   "loan_overdue_rate_by_status",
			Params: map[string]any{"as_of": "1970-01-01T00:00:00Z"},
		})
		if err != nil {
			t.Fatalf("as_of injection must be a tool-result refusal, got err=%v", err)
		}
		if outcome.Refusal == nil || outcome.Refusal.ReasonCode != ReasonInvalidParams {
			t.Fatalf("want invalid_params for as_of, got %+v", outcome)
		}
		if !strings.Contains(outcome.Refusal.HumanExplanation, "系统") {
			t.Errorf("explanation must state as_of is system-generated (D16-B4): %q", outcome.Refusal.HumanExplanation)
		}
		if engine.calls != 0 {
			t.Errorf("as_of refusal must never reach the engine, calls=%d", engine.calls)
		}
	})
}

// ---------- Lock ③ TestASofAutoGenerated ----------

// TestASofAutoGenerated: as_of arrives ONLY from the system side — the
// snapshot carries the registry watermark + parsable call instant, the param
// schema has NO as_of field, and supplying one is refused (D16-B4 triple).
func TestASofAutoGenerated(t *testing.T) {
	reg := loadTestRegistry(t, writeLoansFixture(t, false))

	compiled, ok := reg.Lookup("loan_overdue_rate_by_status")
	if !ok {
		t.Fatal("fixture KPI not found")
	}
	for name := range compiled.Params {
		if name == ReservedParamAsOf {
			t.Fatalf("param schema MUST NOT declare an as_of field (D16-B4): found %q", name)
		}
	}

	engine := &stubEngine{rows: sampleEngineRows()}
	exec := NewExecutor(reg, defaultStubMeta(), engine, &captureAudit{}, func() time.Time { return fixedNow })

	outcome, err := exec.Call(context.Background(), CallRequest{Name: "loan_overdue_rate_by_status"})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	snap := outcome.Snapshot
	if snap == nil {
		t.Fatalf("expected snapshot, got refusal %+v", outcome.Refusal)
	}
	if snap.ASOf.Period != "2026-07" {
		t.Errorf("ASOf.Period = %q, want registry watermark 2026-07", snap.ASOf.Period)
	}
	ts, perr := time.Parse(time.RFC3339, snap.ASOf.Ts)
	if perr != nil {
		t.Fatalf("ASOf.Ts %q is not parsable as RFC3339: %v", snap.ASOf.Ts, perr)
	}
	if want := fixedNow.UTC(); !ts.Equal(want) {
		t.Errorf("ASOf.Ts instant = %v, want system clock %v", ts, want)
	}
	// File k_min unset + dataset override 0 → global floor flows into the snapshot.
	if snap.KMinApplied != KMinDefault {
		t.Errorf("KMinApplied = %d, want chain default %d", snap.KMinApplied, KMinDefault)
	}
	// Suppression happened inside the same call (small bucket marked).
	if snap.Suppressed != 1 {
		t.Errorf("Suppressed = %d, want 1 (row_count=3 < k_min=5)", snap.Suppressed)
	}
	if got := snap.Rows[1]["overdue_rate"]; got != "suppressed" {
		t.Errorf("small-bucket overdue_rate = %v, want \"suppressed\"", got)
	}
	if got := snap.Rows[1]["row_count"]; got != int64(3) {
		t.Errorf("small-bucket row_count = %v, want original preserved", got)
	}
}

// ---------- Lock ④ TestCallOutcomeAuditWrite ----------

// TestCallOutcomeAuditWrite: the audit leg freezes the FULL snapshot — field-
// by-field row assertions plus byte-identical ResultSnapshot — and an audit
// failure turns the whole call into a Go error with NO outcome legs (审计失败
// 不放行, T-16-06-05).
func TestCallOutcomeAuditWrite(t *testing.T) {
	ctx := context.Background()
	reg := loadTestRegistry(t, writeLoansFixture(t, false))

	meta := stubMeta{version: DatasetVersion{
		Period:       "2026-07",
		FilePath:     "/data/datasets/loans__2026-07.duckdb",
		KMinOverride: 10,
	}}

	t.Run("captured_row_field_locks", func(t *testing.T) {
		engine := &stubEngine{rows: sampleEngineRows()}
		audit := &captureAudit{}
		exec := NewExecutor(reg, meta, engine, audit, func() time.Time { return fixedNow })

		outcome, err := exec.Call(ctx, CallRequest{
			Name:      "loan_overdue_rate_by_status",
			Params:    map[string]any{},
			SessionID: "sess-A",
			SkillID:   "compliance",
		})
		if err != nil {
			t.Fatalf("call failed: %v", err)
		}
		if len(audit.rows) != 1 {
			t.Fatalf("audit rows = %d, want exactly 1", len(audit.rows))
		}
		row := audit.rows[0]
		snap := outcome.Snapshot

		if row.TS.Equal(fixedNow) == false {
			t.Errorf("row.TS = %v, want %v", row.TS, fixedNow)
		}
		if row.SessionID != "sess-A" || row.SkillID != "compliance" {
			t.Errorf("session/skill context drifted: %q/%q", row.SessionID, row.SkillID)
		}
		if row.Func != "loan_overdue_rate_by_status" {
			t.Errorf("row.Func = %q", row.Func)
		}
		if row.ASOfPeriod != snap.ASOf.Period {
			t.Errorf("row.ASOfPeriod = %q, snapshot %q", row.ASOfPeriod, snap.ASOf.Period)
		}
		if row.KMinApplied != 10 {
			t.Errorf("row.KMinApplied = %d, want 10 (dataset override chain)", row.KMinApplied)
		}
		if row.Signature != reg.Signature() {
			t.Errorf("row.Signature = %q, want registry signature %q", row.Signature, reg.Signature())
		}

		// Byte-identity of the frozen payload vs the tool JSON (三路输出同源):
		wantSnapJSON, mErr := json.Marshal(snap)
		if mErr != nil {
			t.Fatalf("marshal snapshot: %v", mErr)
		}
		if row.ResultSnapshot != string(wantSnapJSON) {
			t.Errorf("ResultSnapshot drift:\n got %s\nwant %s", row.ResultSnapshot, wantSnapJSON)
		}

		// Inspect the FROZEN copy (independent unmarshal — replay discipline):
		var replay Snapshot
		if uErr := json.Unmarshal([]byte(row.ResultSnapshot), &replay); uErr != nil {
			t.Fatalf("unmarshal ResultSnapshot: %v", uErr)
		}
		if len(replay.Rows) != 2 {
			t.Errorf("frozen rows = %d, want full result set 2 (incl. suppressed marker row)", len(replay.Rows))
		}
		var hasSuppressed bool
		for _, r := range replay.Rows {
			if v, ok := r["overdue_rate"].(string); ok && v == "suppressed" {
				hasSuppressed = true
				if r["status"] != "逾期" || r["row_count"] != float64(3) {
					t.Errorf("frozen suppressed row lost dimension/count: %+v", r)
				}
			}
		}
		if !hasSuppressed {
			t.Error("frozen snapshot lacks a \"suppressed\" value marker row")
		}

		argsJSON, mErr := json.Marshal(snap.Args)
		if mErr != nil {
			t.Fatalf("marshal args: %v", mErr)
		}
		if row.ArgsJSON != string(argsJSON) {
			t.Errorf("ArgsJSON = %q, want %q (actual provided args)", row.ArgsJSON, argsJSON)
		}
	})

	t.Run("audit_failure_blocks_call", func(t *testing.T) {
		engine := &stubEngine{rows: sampleEngineRows()}
		audit := &captureAudit{err: errors.New("disk full")}
		exec := NewExecutor(reg, meta, engine, audit, func() time.Time { return fixedNow })

		outcome, err := exec.Call(ctx, CallRequest{Name: "loan_overdue_rate_by_status", SessionID: "sess-B"})
		if err == nil {
			t.Fatal("audit failure MUST return a Go error (不放行) — silent degradation breaks KPI-02")
		}
		if !strings.Contains(err.Error(), "PutKPICall") {
			t.Errorf("error should identify the failing audit leg: %v", err)
		}
		if outcome.Snapshot != nil || outcome.Refusal != nil {
			t.Errorf("both legs must be nil on infra failure, got %+v", outcome)
		}
	})
}
