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

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// TestAdminSchedulesRetrievalOnly asserts that admin_schedules.go contains no
// mutation HTTP verbs (POST / PUT / DELETE / PATCH). This is the structural
// source-grep lock for the T-20-05-MUTATE INSERT-only invariant: the admin
// schedules sub-view is read-only by construction.
func TestAdminSchedulesRetrievalOnly(t *testing.T) {
	src, err := os.ReadFile("admin_schedules.go")
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

// fakeAdminSchedulesStore is a minimal store.Store stub used to test
// adminSchedulesHandler response shapes. Only ListReportRuns is wired; all
// other methods panic if called.
type fakeAdminSchedulesStore struct {
	rows []store.ReportRunRow
	err  error
}

func (f *fakeAdminSchedulesStore) ListReportRuns(_ context.Context, _, _ string, _ int) ([]store.ReportRunRow, error) {
	return f.rows, f.err
}

// Unused store.Store methods — panic if reached.
func (f *fakeAdminSchedulesStore) GetSession(_ context.Context, _ string) ([]store.SessionMessage, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) AppendTurn(_ context.Context, _ string, _, _ store.SessionMessage) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) UpdateAuditMetrics(_ context.Context, _ int64, _, _, _, _ *float64) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutAudit(_ context.Context, _ store.AuditRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutAuditMeta(_ context.Context, _ store.AuditMeta) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListSessions(_ context.Context) ([]store.SessionSummary, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) RenameSession(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) DeleteSessionTurns(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) InsertPendingApproval(_ context.Context, _ store.PendingApprovalRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) MarkApproved(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) MarkRejected(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) MarkChangeRequested(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) MarkRuleRevoked(_ context.Context, _ store.RuleRevokeRow) (int64, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) SetCheckpointWithSnapshot(_ context.Context, _ string, _ []byte, _, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) GetCheckpointByVersionFilter(_ context.Context, _, _, _ string) ([]byte, bool, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) Delete(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) QueryFanoutByRunPath(_ context.Context, _ string) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListFanoutEvents(_ context.Context, _ string, _ int) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutDataset(_ context.Context, _ store.DatasetRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListDatasets(_ context.Context) ([]store.DatasetRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) DeleteDataset(_ context.Context, _ string) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutImportBatch(_ context.Context, _ store.ImportBatchRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) LatestImportBatch(_ context.Context, _, _ string) (*store.ImportBatchRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListImportBatches(_ context.Context, _ string) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutDatasetDelete(_ context.Context, _ store.DatasetDeleteRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutKPICall(_ context.Context, _ store.KPICallRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListKPICalls(_ context.Context, _ store.KPICallsFilter) ([]store.KPICallRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) LatestKPICallByFunc(_ context.Context) (map[string]time.Time, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutReport(_ context.Context, _ store.ReportRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListReports(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) GetReport(_ context.Context, _ string, _ int) (*store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) GetLatestReport(_ context.Context, _ string) (*store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) ListReportsBySkill(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) PutReportRun(_ context.Context, _ store.ReportRunRow) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) GetReportRun(_ context.Context, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) GetLatestReportRunBySkill(_ context.Context, _, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) Health(_ context.Context) error {
	panic("not implemented")
}
func (f *fakeAdminSchedulesStore) Close() error {
	panic("not implemented")
}

var _ store.Store = (*fakeAdminSchedulesStore)(nil)

// spyAdminSchedulesStore captures the filter args passed to ListReportRuns.
type spyAdminSchedulesStore struct {
	calls []struct{ skillID, period string; limit int }
	rows  []store.ReportRunRow
}

func (s *spyAdminSchedulesStore) ListReportRuns(_ context.Context, skillID, period string, limit int) ([]store.ReportRunRow, error) {
	s.calls = append(s.calls, struct{ skillID, period string; limit int }{skillID, period, limit})
	return s.rows, nil
}

// Unused store.Store methods — panic if reached.
func (s *spyAdminSchedulesStore) GetSession(_ context.Context, _ string) ([]store.SessionMessage, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) AppendTurn(_ context.Context, _ string, _, _ store.SessionMessage) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) UpdateAuditMetrics(_ context.Context, _ int64, _, _, _, _ *float64) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutAudit(_ context.Context, _ store.AuditRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutAuditMeta(_ context.Context, _ store.AuditMeta) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListSessions(_ context.Context) ([]store.SessionSummary, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) RenameSession(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) DeleteSessionTurns(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) InsertPendingApproval(_ context.Context, _ store.PendingApprovalRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) MarkApproved(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) MarkRejected(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) MarkChangeRequested(_ context.Context, _ store.ApprovalResolutionRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) MarkRuleRevoked(_ context.Context, _ store.RuleRevokeRow) (int64, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) SetCheckpointWithSnapshot(_ context.Context, _ string, _ []byte, _, _ string) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) GetCheckpointByVersionFilter(_ context.Context, _, _, _ string) ([]byte, bool, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) Delete(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) QueryFanoutByRunPath(_ context.Context, _ string) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListFanoutEvents(_ context.Context, _ string, _ int) ([]store.FanoutEvent, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutDataset(_ context.Context, _ store.DatasetRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListDatasets(_ context.Context) ([]store.DatasetRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) DeleteDataset(_ context.Context, _ string) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutImportBatch(_ context.Context, _ store.ImportBatchRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) LatestImportBatch(_ context.Context, _, _ string) (*store.ImportBatchRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListImportBatches(_ context.Context, _ string) ([]store.ImportBatchRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutDatasetDelete(_ context.Context, _ store.DatasetDeleteRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutKPICall(_ context.Context, _ store.KPICallRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListKPICalls(_ context.Context, _ store.KPICallsFilter) ([]store.KPICallRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) LatestKPICallByFunc(_ context.Context) (map[string]time.Time, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutReport(_ context.Context, _ store.ReportRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListReports(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) GetReport(_ context.Context, _ string, _ int) (*store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) GetLatestReport(_ context.Context, _ string) (*store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) ListReportsBySkill(_ context.Context, _ string, _ int) ([]store.ReportRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) PutReportRun(_ context.Context, _ store.ReportRunRow) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) GetReportRun(_ context.Context, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) GetLatestReportRunBySkill(_ context.Context, _, _ string) (*store.ReportRunRow, error) {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) Health(_ context.Context) error {
	panic("not implemented")
}
func (s *spyAdminSchedulesStore) Close() error {
	panic("not implemented")
}

var _ store.Store = (*spyAdminSchedulesStore)(nil)

// TestAdminSchedulesHandler tests the three terminal response paths of
// adminSchedulesHandler: 403 (no token), 503 (store nil), and 200 (ok).
func TestAdminSchedulesHandler(t *testing.T) {
	os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
	defer os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")

	t.Run("403_without_token", func(t *testing.T) {
		os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		h := app.NewApp(app.EngineOpts{})
		h.GET("/api/admin/schedules", func(ctx context.Context, c *app.RequestContext) {
			deps := &Deps{Store: &fakeAdminSchedulesStore{}}
			deps.adminSchedulesHandler(ctx, c)
		})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/schedules", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("503_store_nil", func(t *testing.T) {
		os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
		defer os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		h := app.NewApp(app.EngineOpts{})
		h.GET("/api/admin/schedules", func(ctx context.Context, c *app.RequestContext) {
			deps := &Deps{Store: nil}
			deps.adminSchedulesHandler(ctx, c)
		})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/schedules", nil)
		req.Header.Set("X-Admin-Token", "test-admin-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("200_ok_empty", func(t *testing.T) {
		os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
		defer os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		h := app.NewApp(app.EngineOpts{})
		h.GET("/api/admin/schedules", func(ctx context.Context, c *app.RequestContext) {
			deps := &Deps{Store: &fakeAdminSchedulesStore{rows: nil}}
			deps.adminSchedulesHandler(ctx, c)
		})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/schedules", nil)
		req.Header.Set("X-Admin-Token", "test-admin-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("200_ok_with_rows", func(t *testing.T) {
		os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
		defer os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		h := app.NewApp(app.EngineOpts{})
		h.GET("/api/admin/schedules", func(ctx context.Context, c *app.RequestContext) {
			deps := &Deps{
				Store: &fakeAdminSchedulesStore{
					rows: []store.ReportRunRow{
						{
							ID:            1,
							RunID:         "run-001",
							SkillID:       "compliance",
							Period:        "2026-08",
							CronExpr:      "0 0 * * *",
							FiredAt:       time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
							Status:        "completed",
							DriftDetected: false,
						},
						{
							ID:            2,
							RunID:         "run-002",
							SkillID:       "compliance",
							Period:        "2026-08",
							CronExpr:      "0 0 * * *",
							FiredAt:       time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
							Status:        "failed",
							DriftDetected: true,
						},
					},
				},
			}
			deps.adminSchedulesHandler(ctx, c)
		})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/schedules", nil)
		req.Header.Set("X-Admin-Token", "test-admin-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		// Verify response envelope contains "schedules" key.
		body := rec.Body.String()
		if !strings.Contains(body, `"schedules"`) {
			t.Errorf("expected response to contain 'schedules' key, got: %s", body)
		}
		// Verify drift_status mapping.
		if !strings.Contains(body, `"drift_status":"drifted"`) {
			t.Errorf("expected drift_status 'drifted' in body, got: %s", body)
		}
		if !strings.Contains(body, `"drift_status":"ok"`) {
			t.Errorf("expected drift_status 'ok' in body, got: %s", body)
		}
	})
}

// TestAdminSchedulesListReportRunsArgs verifies that ListReportRuns is called
// with empty strings for skillID and period (no filter) and limit 100.
func TestAdminSchedulesListReportRunsArgs(t *testing.T) {
	os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
	defer os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")

	spy := &spyAdminSchedulesStore{}
	deps := &Deps{Store: spy}

	h := app.NewApp(app.EngineOpts{})
	h.GET("/api/admin/schedules", deps.adminSchedulesHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/schedules", nil)
	req.Header.Set("X-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 ListReportRuns call, got %d", len(spy.calls))
	}
	if spy.calls[0].skillID != "" {
		t.Errorf("expected empty skillID, got %q", spy.calls[0].skillID)
	}
	if spy.calls[0].period != "" {
		t.Errorf("expected empty period, got %q", spy.calls[0].period)
	}
	if spy.calls[0].limit != 100 {
		t.Errorf("expected limit=100, got %d", spy.calls[0].limit)
	}
}
