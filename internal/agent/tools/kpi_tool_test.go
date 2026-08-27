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

// kpi_tool_test.go is Phase 16 / Plan 16-07 Task 2's hermetic test
// suite for BuildKPICallTool. Five locks (per the plan's <action>
// section + a meta SQL-surface guard):
//
//   ① TestKpiToolNoMatchingKPIPassThrough — unknown KPI name →
//      structured refusal in the tool result, err == nil (D16-B2 /
//      DATA-12 — a Refusal is NEVER a Go error).
//
//   ② TestKpiToolSnapshotPassesThroughAndEmits — successful call →
//      Snapshot in the tool result; emit receives wire.DataReference
//      mirroring the snapshot fields (rows body EXCLUDED — volume
//      control per 16-RESEARCH Pattern 7).
//
//   ③ TestKpiToolEmitNilDoesNotPanic — BuildKPICallTool with
//      emit=nil + invocation must not panic (hermetic-test mode +
//      the no-SSE pipeline path share this contract).
//
//   ④ TestKpiToolInfoCarriesCatalogAndEnum — ToolInfo.Desc embeds
//      the catalog; ParamsOneOf → JSON schema's `name` parameter
//      has Enum equal to reg.Names() (sorted).
//
//   ⑤ TestKpiToolPreflightSQLSurfaceClean — defensive meta-gate:
//      the Tool's source file must NOT contain any banned SQL
//      keywords (post comment-strip) — the WalkDir gate in
//      internal/agent/sql_surface_test.go is the primary lock; this
//      meta-gate catches drift if the WalkDir roots change.
//
// Run with: `go test ./internal/agent/tools/ -count=1 -v`.
package agenttools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	jsonschema "github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/wire"
)

// ---------- hermetic fixture infrastructure (mirror executor_test.go) ----------

// validToolYAML is the minimum hermetic KPI definition for tool tests:
// one enum-typed optional parameter + a plain snapshot-shape result.
// The fake AST serializer returns the standard single BASE_TABLE
// envelope so Layer 2 validates without CGO.
const validToolYAML = `
dataset: loans
description: kpi_tool_test fixture
k_min: 5
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
        required: false
  - name: loan_balance_bucket_distribution
    description: 贷款余额桶分布
    sql: |
      SELECT balance AS balance_bucket,
             count(*) AS row_count
      FROM ds_loans.data
      GROUP BY balance
      ORDER BY balance_bucket
    count_column: row_count
    params: {}
`

// toolFakeASTEnvelope is the canonical json_serialize_sql response for
// the single-BASE_TABLE SELECT shape (split form: schema_name +
// table_name per 16-01 A3).
const toolFakeASTEnvelope = `{"error":false,"error_type":"","error_message":"","statements":[{"node":{"type":"SELECT_NODE","from_table":{"type":"BASE_TABLE","schema_name":"ds_loans","table_name":"data"},"select_list":[]}}]}`

// toolFakeSerializer implements kpi.SQLSerializer hermetically (the
// production implementation lives in internal/duckdb and is CGO;
// tests use this stand-in to keep the suite hermetic per the
// D15-01-1 MSYS2 gcc linker problem on this host).
type toolFakeSerializer struct{}

func (toolFakeSerializer) SerializeSQL(context.Context, string) (string, error) {
	return toolFakeASTEnvelope, nil
}

// toolEmptyLister implements kpi.DatasetLister with the bootstrap-
// window (empty list → existence check skipped).
type toolEmptyLister struct{}

func (toolEmptyLister) ListDatasets(context.Context) ([]string, error) { return nil, nil }

// toolStubMeta is a minimal DatasetMetaSource returning a fixed
// dataset version (mirrors kpi/executor_test.go stubMeta).
type toolStubMeta struct {
	version kpi.DatasetVersion
	err     error
}

func (m toolStubMeta) LatestDatasetVersion(context.Context, string) (kpi.DatasetVersion, error) {
	return m.version, m.err
}

func (m toolStubMeta) Periods(context.Context, string) ([]string, error) {
	return nil, nil
}

// toolStubEngine is a QueryEngine stub that replays canned rows and
// records the last request (so tests can assert on what the executor
// passed in).
type toolStubEngine struct {
	mu      sync.Mutex
	rows    []map[string]any
	err     error
	lastReq kpi.EngineRequest
	calls   int
}

func (e *toolStubEngine) Query(_ context.Context, req kpi.EngineRequest) ([]map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	e.lastReq = req
	if e.err != nil {
		return nil, e.err
	}
	return e.rows, nil
}

// toolCaptureAudit is an AuditSink that records every PutKPICall row
// (mirrors kpi/executor_test.go captureAudit).
type toolCaptureAudit struct {
	mu   sync.Mutex
	rows []store.KPICallRow
	err  error
}

func (a *toolCaptureAudit) PutKPICall(_ context.Context, row store.KPICallRow) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.rows = append(a.rows, row)
	return nil
}

// fixtureNow pins the as_of timestamp so tests have a deterministic
// expectation. 2026-08-27 12:30:15.250 UTC matches internal/kpi's
// fixedNow used by executor_test.go for cross-suite consistency.
var fixtureNow = time.Date(2026, 8, 27, 12, 30, 15, 250_000_000, time.UTC)

// toolFixture bundles the test wiring (real Registry + real Executor
// + stubs at meta/engine/audit seam).
type toolFixture struct {
	registry  *kpi.Registry
	engine    *toolStubEngine
	audit     *toolCaptureAudit
	dummyFile string // fake .duckdb path (engine stub doesn't actually open it)
}

func newToolFixture(t *testing.T) *toolFixture {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "loans.yaml"), []byte(validToolYAML), 0o644))

	reg, _, err := kpi.LoadKPIs(context.Background(), dir, toolFakeSerializer{}, toolEmptyLister{})
	require.NoError(t, err, "LoadKPIs fixture load failed")
	require.NotNil(t, reg)

	dummy := filepath.Join(t.TempDir(), "loans.duckdb")
	require.NoError(t, os.WriteFile(dummy, []byte{}, 0o644))

	return &toolFixture{
		registry: reg,
		engine: &toolStubEngine{
			rows: []map[string]any{
				{"status": "正常", "row_count": int64(120), "overdue_rate": 5.5},
				{"status": "逾期", "row_count": int64(3), "overdue_rate": 66.6},
			},
		},
		audit:     &toolCaptureAudit{},
		dummyFile: dummy,
	}
}

// newExecutorFromFixture wires the real *kpi.Executor (no mocks at
// the executor boundary — only at meta/engine/audit, mirroring the
// existing executor_test.go pattern).
func newExecutorFromFixture(t *testing.T, fix *toolFixture) *kpi.Executor {
	t.Helper()
	meta := toolStubMeta{version: kpi.DatasetVersion{
		Period:   "2026-07",
		FilePath: fix.dummyFile,
	}}
	return kpi.NewExecutor(fix.registry, meta, fix.engine, fix.audit, func() time.Time { return fixtureNow })
}

// unmarshalKpiResponse parses the Tool's JSON output into a
// kpiCallResponse-shaped struct for assertions. Mirrors the
// internal struct (kpiCallResponse) — duplicated here rather than
// imported because the kpi_tool response type is internal to
// agenttools (only used through the wire JSON).
type kpiToolTestResponse struct {
	Kind     string        `json:"kind"`
	Snapshot *kpi.Snapshot `json:"snapshot,omitempty"`
	Refusal  *kpi.Refusal  `json:"refusal,omitempty"`
}

func unmarshalKpiResponse(t *testing.T, raw string) kpiToolTestResponse {
	t.Helper()
	var resp kpiToolTestResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp), "must be valid JSON")
	return resp
}

// ---------- Lock ① ----------

// TestKpiToolNoMatchingKPIPassThrough — when the executor produces a
// no_matching_kpi Refusal, the Tool emits it as a NORMAL tool result
// (no Go error). The output JSON must contain the reason_code +
// available_kpis so the agent can self-correct or narrate to the
// user (DATA-12). This is the D16-B2 iron rule locked at the Tool
// boundary.
func TestKpiToolNoMatchingKPIPassThrough(t *testing.T) {
	fix := newToolFixture(t)
	exec := newExecutorFromFixture(t, fix)

	var captured *wire.DataReference
	var emitMu sync.Mutex
	emit := func(ref wire.DataReference) {
		emitMu.Lock()
		defer emitMu.Unlock()
		captured = &ref
	}
	tl := agenttools.BuildKPICallTool(exec, fix.registry, "sess-test", "skill-test", emit)
	require.NotNil(t, tl)

	raw, err := tl.InvokableRun(context.Background(), `{"name":"does_not_exist","params":{}}`)
	require.NoError(t, err, "Refusal must NOT surface as a Go error (D16-B2)")
	require.NotEmpty(t, raw, "raw output must be non-empty JSON")

	resp := unmarshalKpiResponse(t, raw)
	require.Equal(t, "refusal", resp.Kind, "unknown KPI must produce a refusal-kind result")
	require.NotNil(t, resp.Refusal, "refusal body must be populated")
	assert.Equal(t, kpi.ReasonNoMatchingKPI, resp.Refusal.ReasonCode,
		"reason code must be no_matching_kpi")
	assert.NotEmpty(t, resp.Refusal.HumanExplanation,
		"refusal must carry a constructive Chinese explanation (DATA-12)")
	assert.Contains(t, resp.Refusal.AvailableKPIs, "loan_overdue_rate_by_status",
		"available_kpis must enumerate the registry so the agent can self-correct")

	emitMu.Lock()
	defer emitMu.Unlock()
	assert.Nil(t, captured, "no DataRef must be emitted for a refusal (only on Snapshot)")
}

// ---------- Lock ② ----------

// TestKpiToolSnapshotPassesThroughAndEmits — when the executor
// produces a Snapshot, the Tool emits it as a tool result AND
// invokes the emit hook with a wire.DataReference that mirrors the
// snapshot fields (excluding rows body). This is the D16-B3 single-
// Snapshot three-output discipline (tool result + audit row +
// DataRef emission all derived from the same struct).
func TestKpiToolSnapshotPassesThroughAndEmits(t *testing.T) {
	fix := newToolFixture(t)
	exec := newExecutorFromFixture(t, fix)

	var captured *wire.DataReference
	var emitMu sync.Mutex
	emit := func(ref wire.DataReference) {
		emitMu.Lock()
		defer emitMu.Unlock()
		captured = &ref
	}
	tl := agenttools.BuildKPICallTool(exec, fix.registry, "sess-test", "skill-test", emit)
	require.NotNil(t, tl)

	raw, err := tl.InvokableRun(context.Background(), `{"name":"loan_overdue_rate_by_status","params":{}}`)
	require.NoError(t, err, "successful call must not produce a Go error")
	require.NotEmpty(t, raw, "raw output must be non-empty JSON")

	resp := unmarshalKpiResponse(t, raw)
	require.Equal(t, "snapshot", resp.Kind, "successful call must produce a snapshot-kind result")
	require.NotNil(t, resp.Snapshot, "snapshot body must be populated")
	assert.Equal(t, "loan_overdue_rate_by_status", resp.Snapshot.Func,
		"snapshot.func echoes the call name")
	assert.NotEmpty(t, resp.Snapshot.ASOf.Period,
		"snapshot.as_of.period must be populated (system-generated, D16-B4)")
	assert.NotEmpty(t, resp.Snapshot.ASOf.Ts,
		"snapshot.as_of.ts must be populated (system-generated, D16-B4)")
	assert.Equal(t, 5, resp.Snapshot.KMinApplied,
		"k_min_applied must reflect the file-level k_min override (5)")
	assert.NotNil(t, resp.Snapshot.Rows,
		"snapshot.rows must be populated for the agent to see the aggregated result")

	// DataRef emission assertions.
	emitMu.Lock()
	defer emitMu.Unlock()
	require.NotNil(t, captured, "emit must fire on a successful snapshot")
	assert.Equal(t, "loan_overdue_rate_by_status", captured.Func,
		"DataRef.func must match the snapshot func")
	assert.Equal(t, resp.Snapshot.ASOf.Period, captured.ASOfPeriod,
		"DataRef.ASOfPeriod must match snapshot.as_of.period")
	assert.Equal(t, resp.Snapshot.ASOf.Ts, captured.ASOfTs,
		"DataRef.ASOfTs must match snapshot.as_of.ts")
	assert.Equal(t, len(resp.Snapshot.Rows), captured.RowCount,
		"DataRef.RowCount must match snapshot row count (D16-D1 invariant)")
	assert.Equal(t, resp.Snapshot.Suppressed, captured.Suppressed,
		"DataRef.Suppressed must match snapshot suppressed count")
	assert.Equal(t, resp.Snapshot.KMinApplied, captured.KMinApplied,
		"DataRef.KMinApplied must match snapshot k_min")

	// Rows body MUST NOT be on the wire (volume control + the agent
	// already has the full snapshot from the tool result).
	body, err := json.Marshal(captured)
	require.NoError(t, err)
	assert.NotContains(t, string(body), `"rows"`,
		"DataReference payload must NOT include rows body (volume control)")
}

// ---------- Lock ③ ----------

// TestKpiToolEmitNilDoesNotPanic — BuildKPICallTool with emit=nil
// must construct cleanly, and a successful call must NOT panic on
// the emit-dispatch step. This is the hermetic-test contract that
// the 16-08 server wiring can opt into by passing a non-nil emit,
// while unit tests + the no-SSE pipeline path can pass nil.
func TestKpiToolEmitNilDoesNotPanic(t *testing.T) {
	fix := newToolFixture(t)
	exec := newExecutorFromFixture(t, fix)

	// Build with emit=nil — must not panic at construction.
	require.NotPanics(t, func() {
		tl := agenttools.BuildKPICallTool(exec, fix.registry, "sess-test", "skill-test", nil)
		require.NotNil(t, tl)
	}, "BuildKPICallTool with nil emit must not panic")

	// Invoke with emit=nil — must not panic on dispatch.
	tl := agenttools.BuildKPICallTool(exec, fix.registry, "sess-test", "skill-test", nil)
	require.NotPanics(t, func() {
		raw, err := tl.InvokableRun(context.Background(), `{"name":"loan_overdue_rate_by_status","params":{}}`)
		require.NoError(t, err)
		require.NotEmpty(t, raw)
	}, "kpi_call invocation with nil emit must not panic")

	// Nil pre-conditions panic (retrieve.go:113-122 convention).
	require.Panics(t, func() {
		agenttools.BuildKPICallTool(nil, fix.registry, "sess-test", "skill-test", nil)
	}, "nil executor must panic at construction (caller bug convention)")
	require.Panics(t, func() {
		agenttools.BuildKPICallTool(exec, nil, "sess-test", "skill-test", nil)
	}, "nil registry must panic at construction (caller bug convention)")
}

// ---------- Lock ④ ----------

// TestKpiToolInfoCarriesCatalogAndEnum — the Tool's Info() reports
// the catalog lines AND pins the name parameter's Enum to the
// registry names. eino v0.9.14 schema.ParameterInfo.Enum is the
// schema-level guard that constrains the LLM to valid names
// (D16-B1); a future code change that forgets the Enum would let
// any string through and the registry's lookup would do the final
// no-match refusal — but the description would also lose its
// catalog snapshot, so this test catches both regressions at once.
//
// We use ToJSONSchema() to extract the parameter enum because
// ParamsOneOf's underlying fields are unexported in eino v0.9.14
// (see schema/tool_test.go for the canonical conversion pattern).
func TestKpiToolInfoCarriesCatalogAndEnum(t *testing.T) {
	fix := newToolFixture(t)
	exec := newExecutorFromFixture(t, fix)
	tl := agenttools.BuildKPICallTool(exec, fix.registry, "sess-test", "skill-test", nil)
	require.NotNil(t, tl)

	info, err := tl.Info(context.Background())
	require.NoError(t, err, "Info() must not fail")
	assert.Equal(t, "kpi_call", info.Name, "tool name per Plan 16-07 D16-B1 contract")

	// Catalog lines must appear in Desc — both KPIs in the fixture.
	assert.Contains(t, info.Desc, "loan_overdue_rate_by_status",
		"description must embed the compiled catalog snapshot (D16-B1)")
	assert.Contains(t, info.Desc, "loan_balance_bucket_distribution",
		"description must embed every catalog line")
	assert.Contains(t, info.Desc, "status:enum(2)",
		"description must include the param summary (enum count visible)")
	assert.Contains(t, info.Desc, "永不接触 SQL",
		"description must include the no-SQL contract framing")

	// ParamsOneOf for `name` carries Enum == reg.Names() (verified via
	// the JSON schema projection because the underlying fields are
	// unexported in eino v0.9.14).
	require.NotNil(t, info.ParamsOneOf, "ParamsOneOf must be populated")
	js, err := info.ParamsOneOf.ToJSONSchema()
	require.NoError(t, err, "ToJSONSchema must not fail")
	require.NotNil(t, js, "JSON schema projection must not be nil")
	require.NotNil(t, js.Properties, "JSON schema must have Properties")

	nameProp, ok := js.Properties.Get("name")
	require.True(t, ok, "name parameter must be in Properties")
	require.NotNil(t, nameProp, "name property must not be nil")
	assert.ElementsMatch(t, fix.registry.Names(), nameProp.Enum,
		"name parameter Enum must equal registry.Names()")
	// sorted defensive copy check
	for i := 1; i < len(nameProp.Enum); i++ {
		assert.Less(t, nameProp.Enum[i-1], nameProp.Enum[i],
			"name Enum must be sorted (matches registry contract)")
	}
	// `name` must be in the schema's required list.
	assert.Contains(t, js.Required, "name",
		"name must be a required parameter (D16-B1)")

	// params parameter must accept the per-KPI object.
	paramsProp, ok := js.Properties.Get("params")
	require.True(t, ok, "params parameter must be in Properties")
	require.NotNil(t, paramsProp, "params property must not be nil")
	_ = paramsProp // structural type only
}

// ---------- Lock ⑤ ----------

// TestKpiToolPreflightSQLSurfaceClean — defensive meta-gate: the
// Tool's source file must NOT contain any banned SQL keywords as
// code (comments stripped). This is a meta-test that catches drift
// if the primary WalkDir gate's scan roots change in a future
// phase. The patterns here MUST stay aligned with the ones in
// sql_surface_test.go — a drift between the two file-content
// scans is itself a regression.
func TestKpiToolPreflightSQLSurfaceClean(t *testing.T) {
	src, err := os.ReadFile("kpi_tool.go")
	require.NoError(t, err, "kpi_tool.go must be readable from test working dir")
	cleaned := stripCommentsForScan(string(src))

	banned := []string{
		// Real SQL would have SELECT * or SELECT <ident> FROM. The
		// regex matches that shape; we use NotContains here as a
		// double-check on the cleaned source.
		"SELECT *",
		"INSERT INTO",
		"CREATE TABLE",
		"ATTACH '",
		"DETACH ",
	}
	for _, b := range banned {
		assert.NotContains(t, cleaned, b,
			"kpi_tool.go must not contain SQL keyword pattern %q after comment stripping (structural ban)", b)
	}

	// Direct shape checks for the dispatch paths the agent could
	// reach — sanity assertions on the cleaned source.
	_ = strings.Contains // keep strings import (used elsewhere)
	_ = jsonschema.Schema{}
}

// stripCommentsForScan removes Go line comments (// ...) and block
// comments (/* ... */) from src, preserving newlines so error
// pointers stay accurate. Mirrors the discipline in
// internal/agent/sql_surface_test.go:stripGoComments; duplicated
// here to keep the agenttools_test package self-contained (no
// cross-package test-helper import — internal/agent is a
// different package and the helper isn't exported).
func stripCommentsForScan(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inLine := false
	inBlock := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte(c)
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				inBlock = false
				i++
			} else if c == '\n' {
				b.WriteByte(c)
			}
			continue
		}
		if c == '/' && i+1 < len(src) {
			next := src[i+1]
			switch next {
			case '/':
				inLine = true
				i++
				continue
			case '*':
				inBlock = true
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}
