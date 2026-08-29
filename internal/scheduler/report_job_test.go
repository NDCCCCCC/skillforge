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

package scheduler

import (
	"context"
	"os"
	"testing"

	"github.com/skillforge/skillforge/internal/report"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAudit mirrors internal/report's publish_test.go stub: a minimal
// ReportAuditSink recording the rows it receives.
type stubAudit struct{ rows []store.ReportRow }

func (s *stubAudit) PutReport(_ context.Context, row store.ReportRow) error {
	s.rows = append(s.rows, row)
	return nil
}

// Compile-time lock for the 19-05 threading: the storeDB value RunReportJob
// already holds satisfies the narrow CR-04 audit seam, so the automated path
// can pass it straight into report.Publish without a new adapter.
var _ report.ReportAuditSink = (store.Store)(nil)

// chdirTemp moves the process into a fresh temp dir so publish writes
// never touch the repo working tree (same pattern as report/publish_test.go).
func chdirTemp(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestExecuteReportRunWiresAuditSink — 19-05 gap closure (PORT-04 leg).
// The automated cron path must pass a live audit sink into report.Publish:
// the CR-04 fail-closed contract rejects a nil sink, so 19-04's nil
// threading made every automated run end status="failed" with an empty
// output_url and an orphaned vN.html file.
func TestExecuteReportRunWiresAuditSink(t *testing.T) {
	chdirTemp(t)

	audit := &stubAudit{}
	snapshots, url, err := executeReportRun(context.Background(), "monthly-report", "2026-08", nil, nil, audit)

	require.NoError(t, err)
	assert.Equal(t, "/reports/2026-08/v1.html", url)
	assert.Empty(t, snapshots) // nil kpiExec → empty snapshot map, no error
	require.Len(t, audit.rows, 1)
	row := audit.rows[0]
	assert.Equal(t, "monthly-report", row.SkillID)
	assert.Equal(t, "2026-08", row.Period)
	assert.Equal(t, 1, row.Version)
	assert.Empty(t, row.ApprovalAuditID) // automated run: no HITL approval row
	assert.NotEmpty(t, row.FilePath)
	assert.NotEmpty(t, row.ResultDigest)
	assert.NotEmpty(t, row.TemplateVersion)
}

// TestExecuteReportRunNilAuditFailsClosed — the CR-04 contract stays
// visible at the scheduler layer: a nil sink is a run failure, never a
// silent file-only publish.
func TestExecuteReportRunNilAuditFailsClosed(t *testing.T) {
	chdirTemp(t)

	_, url, err := executeReportRun(context.Background(), "monthly-report", "2026-08", nil, nil, nil)

	require.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "audit sink not wired")
}
