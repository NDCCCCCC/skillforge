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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, args, capturedInfo.ArgumentsInJSON,
		"the ApprovalInfo must echo the verbatim args bytes")
}

// TestReportPublishApprovePublishesFromJSONTemplate — CR-01
// regression. The approve path receives the template as the
// post-JSON-round-trip map[string]any (never a *report.ReportTemplate);
// publishReport must convert it and actually publish instead of
// failing with "template is not a *report.Template".
func TestReportPublishApprovePublishesFromJSONTemplate(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{}
	args := &ReportPublishArgs{
		Period:   "2026-08",
		Template: validTemplateMap(t), // JSON-round-trip shape — the CR-01 bug shape
		Slots:    map[int]map[int]string{0: {0: "八月业务平稳运行。"}},
		SkillID:  "monthly-report",
	}

	res, err := h.publishReport(context.Background(), args)
	require.NoError(t, err, "the approve path must publish from the JSON-round-trip template shape")
	require.NotNil(t, res)
	assert.Equal(t, 1, res.Version, "first publish of a period is v1")
	assert.Equal(t, filepath.Join("reports", "2026-08", "v1.html"), res.FilePath)
	assert.True(t, strings.HasPrefix(res.URL, "/reports/2026-08/"), "URL must stay inside /reports/")
	_, statErr := os.Stat(res.FilePath)
	assert.NoError(t, statErr, "the rendered HTML file must exist on disk")
}

// TestReportPublishRejectsInvalidTemplate — fail-closed (WR-06
// publish-side half): a template whose slot type is outside the
// enum must be rejected before any file write, not rendered.
func TestReportPublishRejectsInvalidTemplate(t *testing.T) {
	chdirTempRepo(t)

	h := &reportPublishTool{}
	var bad map[string]any
	raw := `{"title":"t","sections":[{"title":"s","kpis":[],"slots":[{"type":"freeform"}]}]}`
	require.NoError(t, json.Unmarshal([]byte(raw), &bad))

	args := &ReportPublishArgs{Period: "2026-08", Template: bad}
	res, err := h.publishReport(context.Background(), args)
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

	h := &reportPublishTool{}
	args := &ReportPublishArgs{Period: "2026-08", Template: "not-a-template"}
	res, err := h.publishReport(context.Background(), args)
	require.Error(t, err)
	assert.Nil(t, res)
}
