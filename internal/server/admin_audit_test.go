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
// See License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/skillforge/skillforge/internal/store"
)

// TestAdminAuditRetrievalOnly asserts that admin_audit.go contains no
// mutation HTTP verbs (POST / PUT / DELETE / PATCH). This is the
// structural source-grep lock for the T-20-06-AUDIT INSERT-only
// invariant: the admin audit sub-view is read-only by construction.
func TestAdminAuditRetrievalOnly(t *testing.T) {
	src, err := os.ReadFile("admin_audit.go")
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		switch {
		case strings.Contains(line, "POST"),
			strings.Contains(line, "PUT"),
			strings.Contains(line, "DELETE"),
			strings.Contains(line, "PATCH"):
			t.Errorf("line %d: contains mutation verb: %s", i+1, line)
		}
	}
}

// fakeAdminAuditStore is a minimal store.Store stub used to test
// adminAuditHandler response shapes. Only QueryAudit is wired; all
// other methods panic if called (the handler under test only calls
// QueryAudit).
type fakeAdminAuditStore struct {
	rows []store.AuditRow
	err  error
}

func (f *fakeAdminAuditStore) QueryAudit(_ context.Context, _ store.AuditFilter) ([]store.AuditRow, error) {
	return f.rows, f.err
}
func (f *fakeAdminAuditStore) AuditBySessionAt(_ context.Context, _ string, _ int64) ([]store.AuditRow, error) { panic("not implemented") }

// Unused store.Store methods — panic if reached.
func (f *fakeAdminAuditStore) GetSession(_ context.Context, _ string) ([]store.SessionMessage, error) {
	panic("not implemented")
}
// AppendTurn satisfies the interface; 0 = no real row id (no sqlite
// backend behind this fake) — downstream data-turn emitters treat 0 as
// the degrade signal (Phase 29 / Plan 29-01 contract).
func (f *fakeAdminAuditStore) AppendTurn(_ context.Context, _ string, _, _ store.SessionMessage) (int64, error) {
	return 0, nil
}
func (f *fakeAdminAuditStore) UpdateAuditMetrics(_ context.Context, _ int64, _, _, _, _ *float64) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutAudit(_ context.Context, _ store.AuditRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutAuditMeta(_ context.Context, _ store.AuditMeta) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListSessions(_ context.Context) ([]store.SessionSummary, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) RenameSession(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) DeleteSessionTurns(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) InsertPendingApproval(_ context.Context, _ store.PendingApprovalRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) MarkApproved(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) MarkRejected(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) MarkChangeRequested(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) MarkRuleRevoked(_ context.Context, _ store.RuleRevokeRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) SetCheckpointWithSnapshot(_ context.Context, _ string, _ []byte, _, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) GetCheckpointByVersionFilter(_ context.Context, _, _, _ string) ([]byte, bool, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) Delete(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) QueryFanoutByRunPath(_ context.Context, _ string) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListFanoutEvents(_ context.Context, _ string, _ int) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutDataset(_ context.Context, _ store.DatasetRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListDatasets(_ context.Context) ([]store.DatasetRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) DeleteDataset(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutImportBatch(_ context.Context, _ store.ImportBatchRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) LatestImportBatch(_ context.Context, _, _ string) (*store.ImportBatchRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListImportBatches(_ context.Context, _ string) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListAllImportBatches(_ context.Context) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutDatasetDelete(_ context.Context, _ store.DatasetDeleteRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutKPICall(_ context.Context, _ store.KPICallRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListKPICalls(_ context.Context, _ store.KPICallsFilter) ([]store.KPICallRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) LatestKPICallByFunc(_ context.Context) (map[string]time.Time, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutReport(_ context.Context, _ store.ReportRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListReports(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) GetReport(_ context.Context, _ string, _ int) (*store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) GetLatestReport(_ context.Context, _ string) (*store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListReportsBySkill(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) PutReportRun(_ context.Context, _ store.ReportRunRow) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) ListReportRuns(_ context.Context, _, _ string, _ int) ([]store.ReportRunRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) GetReportRun(_ context.Context, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) GetLatestReportRunBySkill(_ context.Context, _, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) Health(_ context.Context) error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) CountRows(_ context.Context, _ string) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) Close() error {
	panic("not implemented")
}
func (f *fakeAdminAuditStore) HasUploadBySHA256(_ context.Context, _, _ string) (bool, error) {
	panic("not implemented")
}

var _ store.Store = (*fakeAdminAuditStore)(nil)

// spyAdminAuditStore captures the AuditFilter passed to QueryAudit.
type spyAdminAuditStore struct {
	calls []store.AuditFilter
}

func (s *spyAdminAuditStore) QueryAudit(_ context.Context, filter store.AuditFilter) ([]store.AuditRow, error) {
	s.calls = append(s.calls, filter)
	return nil, nil
}

func (s *spyAdminAuditStore) AuditBySessionAt(_ context.Context, _ string, _ int64) ([]store.AuditRow, error) {
	return nil, nil
}

// Unused store.Store methods — panic if reached.
func (s *spyAdminAuditStore) GetSession(_ context.Context, _ string) ([]store.SessionMessage, error) {
	panic("not implemented")
}
// AppendTurn satisfies the interface; 0 = no real row id (no sqlite
// backend behind this spy) — downstream data-turn emitters treat 0 as
// the degrade signal (Phase 29 / Plan 29-01 contract).
func (s *spyAdminAuditStore) AppendTurn(_ context.Context, _ string, _, _ store.SessionMessage) (int64, error) {
	return 0, nil
}
func (s *spyAdminAuditStore) UpdateAuditMetrics(_ context.Context, _ int64, _, _, _, _ *float64) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutAudit(_ context.Context, _ store.AuditRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutAuditMeta(_ context.Context, _ store.AuditMeta) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListSessions(_ context.Context) ([]store.SessionSummary, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) RenameSession(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) DeleteSessionTurns(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) InsertPendingApproval(_ context.Context, _ store.PendingApprovalRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) MarkApproved(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) MarkRejected(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) MarkChangeRequested(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) MarkRuleRevoked(_ context.Context, _ store.RuleRevokeRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) SetCheckpointWithSnapshot(_ context.Context, _ string, _ []byte, _, _ string) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) GetCheckpointByVersionFilter(_ context.Context, _, _, _ string) ([]byte, bool, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) Delete(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) QueryFanoutByRunPath(_ context.Context, _ string) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListFanoutEvents(_ context.Context, _ string, _ int) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutDataset(_ context.Context, _ store.DatasetRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListDatasets(_ context.Context) ([]store.DatasetRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) DeleteDataset(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutImportBatch(_ context.Context, _ store.ImportBatchRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) LatestImportBatch(_ context.Context, _, _ string) (*store.ImportBatchRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListImportBatches(_ context.Context, _ string) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListAllImportBatches(_ context.Context) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutDatasetDelete(_ context.Context, _ store.DatasetDeleteRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutKPICall(_ context.Context, _ store.KPICallRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListKPICalls(_ context.Context, _ store.KPICallsFilter) ([]store.KPICallRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) LatestKPICallByFunc(_ context.Context) (map[string]time.Time, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutReport(_ context.Context, _ store.ReportRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListReports(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) GetReport(_ context.Context, _ string, _ int) (*store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) GetLatestReport(_ context.Context, _ string) (*store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListReportsBySkill(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) PutReportRun(_ context.Context, _ store.ReportRunRow) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) ListReportRuns(_ context.Context, _, _ string, _ int) ([]store.ReportRunRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) GetReportRun(_ context.Context, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) GetLatestReportRunBySkill(_ context.Context, _, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) Health(_ context.Context) error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) CountRows(_ context.Context, _ string) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminAuditStore) Close() error {
	panic("not implemented")
}
func (s *spyAdminAuditStore) HasUploadBySHA256(_ context.Context, _, _ string) (bool, error) {
	panic("not implemented")
}

var _ store.Store = (*spyAdminAuditStore)(nil)

// invokeAdminAudit issues GET /api/admin/audit against a Hertz engine that
// has ONLY the admin audit route bound. Mirrors invokeAdminBatches pattern.
func invokeAdminAudit(t *testing.T, deps *Deps, adminToken string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/audit", deps.adminAuditHandler)
	var headers []ut.Header
	if adminToken != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: adminToken})
	}
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/audit", nil, headers...)
}

// TestAdminAuditHandler tests the three terminal response paths of
// adminAuditHandler: 403 (no token), 503 (store nil), and 200 (ok).
func TestAdminAuditHandler(t *testing.T) {
	t.Run("403_without_token", func(t *testing.T) {
		os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		deps := &Deps{Store: &fakeAdminAuditStore{}}
		if code := invokeAdminAudit(t, deps, "").Code; code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", code)
		}
	})

	t.Run("503_store_nil", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
		deps := &Deps{Store: nil}
		if code := invokeAdminAudit(t, deps, "test-admin-token").Code; code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", code)
		}
	})

	t.Run("200_ok", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
		deps := &Deps{
			Store: &fakeAdminAuditStore{
				rows: []store.AuditRow{
					{
						ID:                    1,
						Timestamp:             time.Now(),
						SessionID:             "sess-admin-1",
						UserID:                "admin",
						SkillID:               "compliance",
						Query:                 "show me all pending approvals",
						PromptHash:            "abc123",
						ModelVersion:          "v1",
						VerdictJSON:           "{}",
						RefusalReason:         "",
						ChunkIDs:              "",
						Action:                "action_pending_approval",
						SkillVersion:          "1.0",
						FrozenArgs:            nil,
						ToolCallID:            "tc-1",
						ApprovalID:            "tc-1",
						ScopeSessionID:        "sess-admin-1",
						BudgetLayer:           "",
						RunPath:               "",
						TransferFrom:          "",
						TransferTo:            "",
						PlatformFloorVersion:  "1.0.0",
						ActorType:             "admin",
					},
				},
			},
		}
		if code := invokeAdminAudit(t, deps, "test-admin-token").Code; code != http.StatusOK {
			t.Errorf("expected 200, got %d", code)
		}
	})
}

// TestAdminAuditActorType verifies that QueryAudit is called with
// ActorType="admin" when the admin audit handler is invoked.
func TestAdminAuditActorType(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
	spy := &spyAdminAuditStore{}
	deps := &Deps{Store: spy}
	invokeAdminAudit(t, deps, "test-admin-token")
	if len(spy.calls) == 0 {
		t.Fatalf("expected at least one QueryAudit call, got none")
	}
	if got := spy.calls[0].ActorType; got != "admin" {
		t.Errorf("expected ActorType=admin, got %q", got)
	}
}