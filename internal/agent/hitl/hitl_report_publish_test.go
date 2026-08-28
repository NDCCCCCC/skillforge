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

// hitl_report_publish_test.go is Phase 18's hermetic test suite for
// the report_publish HITL Tool (18-REVIEW CR-01 regression). Coverage
// mirrors the hitl_plan_approve_test.go convention:
//
//   - Pass 1 (interrupt) is driven end-to-end through InvokableRun
//     with the REAL production args shape — template as a JSON object
//     (which json.Unmarshal materializes as map[string]any, the exact
//     shape that broke the approve path).
//   - Pass 2 (publish) is covered by driving publishReport directly
//     with a map[string]any template — the post-JSON-round-trip
//     representation. eino@v0.9.14 exposes no public seam to inject
//     interrupt state (wasInterrupted=true + ApprovalDecision payload)
//     without a real Runner cycle (see hitl_plan_approve_test.go's
//     NOTE); the resume-context wiring is covered by the server-side
//     approval integration tests.
package hitl

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// chdirTempRepo moves the test process into a fresh temp dir so the
// publish path's reports/<period>/vN.html writes never touch the repo
// working tree. Restores the original CWD on cleanup.
func chdirTempRepo(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// stubReportSink is the CR-04 reports-table audit seam stub: it
// records every PutReport row (or fails with its err).
type stubReportSink struct {
	rows []store.ReportRow
	err  error
}

func (s *stubReportSink) PutReport(_ context.Context, row store.ReportRow) error {
	if s.err != nil {
		return s.err
	}
	s.rows = append(s.rows, row)
	return nil
}

// validTemplateMap is the JSON-round-trip shape of a minimal valid
// monthly-report template — exactly what json.Unmarshal produces from
// the LLM's tool-call args (map[string]any, NOT *report.ReportTemplate).
func validTemplateMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	raw := `{"title":"2026-08 业务月报","sections":[{"title":"业务概览","kpis":[],"slots":[{"type":"narrative"}]}]}`
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	return m
}

// TestReportPublishFirstPassInterrupts — Pass 1 with the production
// JSON shape (template as an object) must insert the pending-approval
// audit row and return an interrupt, never a validation error.
func TestReportPublishFirstPassInterrupts(t *testing.T) {
	called := 0
	var capturedInfo *ApprovalInfo
	h := &reportPublishTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, info *ApprovalInfo) error {
				called++
				capturedInfo = info
				return nil
			},
		},
	}

	args := `{"period":"2026-08","template":{"title":"2026-08 业务月报","sections":[{"title":"业务概览","kpis":[],"slots":[{"type":"narrative"}]}]},"slots":{},"skillId":"monthly-report","sessionId":"sess-1","kpiCallIds":[1,2]}`
	out, err := h.InvokableRun(context.Background(), args)

	assert.Empty(t, out, "Pass 1 must not produce string output")
	require.Error(t, err, "Pass 1 must return an interrupt error")
	assert.Contains(t, strings.ToLower(err.Error()), "interrupt",
		"Pass 1 error must surface the interrupt signal")
	assert.Equal(t, 1, called, "audit hook must fire exactly once on Pass 1 (D11-C3)")
	require.NotNil(t, capturedInfo)
	assert.Equal(t, ReportPublishToolName, capturedInfo.ToolName)
	assert.Equal(t, ReportPublishTarget, capturedInfo.Target)

	// WR-03: Pass 1 re-marshals the args to freeze the server-derived
	// templateVersion — the audit bytes are the REWRITTEN args (the
	// same bytes stored for StatefulInterrupt, so the D11 byte-equality
	// invariant holds). Assert semantic equality + the injected hash.
	var echoed ReportPublishArgs
	require.NoError(t, json.Unmarshal([]byte(capturedInfo.ArgumentsInJSON), &echoed))
	assert.Equal(t, "2026-08", echoed.Period)
	assert.Equal(t, "monthly-report", echoed.SkillID)
	assert.NotEmpty(t, echoed.TemplateVersion,
		"the rewritten args must carry the server-derived template content hash")
}

// TestReportPublishApprovePublishesFromJSONTemplate — CR-01
// regression. The approve path receives the template as the
// post-JSON-round-trip map[string]any (never a *report.ReportTemplate);
// publishReport must convert it and actually publish instead of
// failing with "template is not a *report.Template".
func TestReportPublishApprovePublishesFromJSONTemplate(t *testing.T) {
	chdirTempRepo(t)

	sink := &stubReportSink{}
	h := &reportPublishTool{reports: sink}
	args := &ReportPublishArgs{
		Period:     "2026-08",
		Template:   validTemplateMap(t), // JSON-round-trip shape — the CR-01 bug shape
		Slots:      map[int]map[int]string{0: {0: "八月业务平稳运行。"}},
		SkillID:    "monthly-report",
		KPICallIDs: []int64{11, 22},
	}

	res, err := h.publishReport(context.Background(), args, "tc-approve-1")
	require.NoError(t, err, "the approve path must publish from the JSON-round-trip template shape")
	require.NotNil(t, res)
	assert.Equal(t, 1, res.Version, "first publish of a period is v1")
	assert.Equal(t, filepath.Join("reports", "2026-08", "v1.html"), res.FilePath)
	assert.True(t, strings.HasPrefix(res.URL, "/reports/2026-08/"), "URL must stay inside /reports/")
	_, statErr := os.Stat(res.FilePath)
	assert.NoError(t, statErr, "the rendered HTML file must exist on disk")

	// CR-04: the publish must leave exactly one reports audit row
	// linking the artifact to its approval + KPI calls.
	require.Len(t, sink.rows, 1, "publish must append the reports audit row (CR-04)")
	row := sink.rows[0]
	assert.Equal(t, "2026-08", row.Period)
	assert.Equal(t, "monthly-report", row.SkillID)
	assert.Equal(t, 1, row.Version)
	assert.Equal(t, "tc-approve-1", row.ApprovalAuditID, "the approval linkage must ride the row (D11-C4)")
	assert.Equal(t, "[11,22]", row.KPICallIDs)
	assert.Equal(t, res.FilePath, row.FilePath)
	assert.NotEmpty(t, row.ResultDigest)
	assert.NotEmpty(t, row.TemplateVersion)
	assert.Equal(t, res.Digest, row.ResultDigest)
	assert.Equal(t, res.TemplateVersion, row.TemplateVersion)
}

// TestReportPublishRequiresAuditSink — CR-04 fail-closed: without the
// reports sink the publish must refuse (no file-without-audit-row
// state), even though the render itself would succeed.
func TestReportPublishRequiresAuditSink(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{} // reports sink NOT wired
	args := &ReportPublishArgs{Period: "2026-08", Template: validTemplateMap(t)}
	res, err := h.publishReport(context.Background(), args, "tc-1")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "audit sink not wired")
	_, statErr := os.Stat(filepath.Join("reports", "2026-08"))
	assert.True(t, os.IsNotExist(statErr), "no file may be written when the audit sink is missing")
}

// kpiTemplateMap is a template whose section declares one KPI — the
// monthly-report shape (every section has KPIs).
func kpiTemplateMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	raw := `{"title":"2026-08 业务月报","sections":[{"title":"贷款质量","kpis":["KPI_NPL_RATIO"],"slots":[{"type":"risk_note"}]}]}`
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	return m
}

// TestReportPublishBindsKPISnapshot — CR-06 regression: the approve
// path binds the frozen-args KPI snapshot into the render (previously
// a literal nil Snapshot was passed and buildMarkdown's snap.Get
// panicked mid-resume).
func TestReportPublishBindsKPISnapshot(t *testing.T) {
	chdirTempRepo(t)

	sink := &stubReportSink{}
	h := &reportPublishTool{reports: sink}
	args := &ReportPublishArgs{
		Period:   "2026-08",
		Template: kpiTemplateMap(t),
		Slots:    map[int]map[int]string{0: {0: "NPL 率 1.23%，风险可控。"}},
		SkillID:  "monthly-report",
		Snapshot: map[string]any{"KPI_NPL_RATIO": 1.23},
	}
	res, err := h.publishReport(context.Background(), args, "tc-kpi-1")
	require.NoError(t, err, "publish with a populated snapshot must succeed, not panic")
	require.NotNil(t, res)
	html, readErr := os.ReadFile(res.FilePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(html), "1.23", "the KPI value must be bound into the rendered HTML")
}

// TestReportPublishRefusesKPITemplateWithoutSnapshot — CR-06
// fail-closed: a KPI-declaring template with no snapshot in the frozen
// args is "insufficient_kpi_data" — an error, never a nil-interface
// panic.
func TestReportPublishRefusesKPITemplateWithoutSnapshot(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{reports: &stubReportSink{}}
	args := &ReportPublishArgs{
		Period:   "2026-08",
		Template: kpiTemplateMap(t),
		// Snapshot deliberately absent.
	}
	res, err := h.publishReport(context.Background(), args, "tc-kpi-2")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "insufficient_kpi_data")
	_, statErr := os.Stat(filepath.Join("reports", "2026-08"))
	assert.True(t, os.IsNotExist(statErr), "nothing may be written without KPI data")
}

// TestInjectDepsPass1PersistsPendingApprovalRow — WR-01 (18-REVIEW)
// regression. With the production wiring (real AuditWriter over a real
// sqlite store, injected via InjectDeps — the exact call BuildRunner
// makes), a Pass-1 interrupt must produce a VISIBLE
// action_pending_approval audit row carrying the target + frozen args.
// The previous build silently degraded to noopAuditWriter, so the
// operator approval surface (fed exclusively from those rows) never
// saw the request.
func TestInjectDepsPass1PersistsPendingApprovalRow(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = st.Close() }()

	writer := NewAuditWriter(st, "test-floor-1.0.0").
		WithIdentity("sess-wr01", "", SkillSnapshot{
			SkillID:    "monthly-report",
			Version:    "sha256:abcdef0123456789",
			PromptHash: "00112233445566778899aabbccddeeff",
		})
	// The production wiring path (agent.BuildRunner → hitl.InjectDeps)
	// mutates the REGISTRY singleton; the tool the LLM reaches is the
	// same instance LookupHandler returns.
	InjectDeps(writer, st, nil)

	args := `{"period":"2026-08","template":{"title":"2026-08 业务月报","sections":[{"title":"业务概览","kpis":[],"slots":[{"type":"narrative"}]}]},"slots":{},"skillId":"monthly-report","sessionId":"sess-wr01","kpiCallIds":[]}`
	out, err := LookupHandler(ReportPublishTarget).InvokableRun(ctx, args)
	assert.Empty(t, out)
	require.Error(t, err, "Pass 1 must interrupt")

	rows, err := st.QueryAudit(ctx, store.AuditFilter{
		SessionID: "sess-wr01",
		Action:    "action_pending_approval",
		Limit:     10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the pending-approval row must be visible to the operator surface")
	row := rows[0]
	assert.Equal(t, ReportPublishTarget, row.Query, "the target column carries the write target")
	assert.NotEmpty(t, row.ToolCallID, "D11-C4 identity must be stamped")
	// WR-03: the frozen bytes are the REWRITTEN args (templateVersion
	// injected, same bytes that StatefulInterrupt stored).
	var frozen ReportPublishArgs
	require.NoError(t, json.Unmarshal(row.FrozenArgs, &frozen))
	assert.Equal(t, "2026-08", frozen.Period)
	assert.NotEmpty(t, frozen.TemplateVersion)
	assert.Equal(t, "sha256:abcdef0123456789", row.SkillVersion, "D11-C2 snapshot must ride the row")
}

// TestReportPublishAuditWriteFailureIsRecoverable — CR-04 ordering
// contract: the FILE is written first, then the DB row. A failed row
// write must surface an error that carries the file path (the file is
// already published — recoverable, never silently dropped).
func TestReportPublishAuditWriteFailureIsRecoverable(t *testing.T) {
	chdirTempRepo(t)

	sink := &stubReportSink{err: errors.New("disk full")}
	h := &reportPublishTool{reports: sink}
	args := &ReportPublishArgs{Period: "2026-08", Template: validTemplateMap(t)}
	res, err := h.publishReport(context.Background(), args, "tc-1")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), filepath.Join("reports", "2026-08", "v1.html"),
		"the error must carry the already-published file path")
	assert.Contains(t, err.Error(), "disk full")
}

// TestReportPublishRejectsInvalidTemplate — fail-closed (WR-06
// publish-side half): a template whose slot type is outside the
// enum must be rejected before any file write, not rendered.
func TestReportPublishRejectsInvalidTemplate(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{reports: &stubReportSink{}}
	var bad map[string]any
	raw := `{"title":"t","sections":[{"title":"s","kpis":[],"slots":[{"type":"freeform"}]}]}`
	require.NoError(t, json.Unmarshal([]byte(raw), &bad))

	args := &ReportPublishArgs{Period: "2026-08", Template: bad}
	res, err := h.publishReport(context.Background(), args, "tc-1")
	require.Error(t, err, "invalid slot type must fail closed")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "template invalid")
	_, statErr := os.Stat(filepath.Join("reports", "2026-08"))
	assert.True(t, os.IsNotExist(statErr), "no report directory may be created on validation failure")
}

// TestReportPublishRejectsUnconvertibleTemplate — a template payload
// that cannot re-encode into the typed struct (e.g. a bare string)
// must fail closed with a decode error.
func TestReportPublishRejectsUnconvertibleTemplate(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{reports: &stubReportSink{}}
	args := &ReportPublishArgs{Period: "2026-08", Template: "not-a-template"}
	res, err := h.publishReport(context.Background(), args, "tc-1")
	require.Error(t, err)
	assert.Nil(t, res)
}
