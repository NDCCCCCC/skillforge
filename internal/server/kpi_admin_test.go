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
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// adminKPIFixtureYAML is a two-KPI hermetic definition file. The fake AST
// serializer below returns the canonical single-BASE_TABLE envelope so the
// loader's Layer 2 validation passes without CGO.
const adminKPIFixtureYAML = `
dataset: loans
description: kpi_admin_test fixture
k_min: 5
kpis:
  - name: loan_overdue_rate_by_status
    description: 各状态贷款逾期率
    sql: |
      SELECT status AS status,
             count(*) AS row_count
      FROM ds_loans.data
      WHERE (:status IS NULL OR status = :status)
      GROUP BY status
    count_column: row_count
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
    count_column: row_count
    params: {}
`

// adminKPIASTEnvelope mirrors internal/agent/tools/kpi_tool_test.go's
// hermetic json_serialize_sql response (single BASE_TABLE SELECT shape).
const adminKPIASTEnvelope = `{"error":false,"error_type":"","error_message":"","statements":[{"node":{"type":"SELECT_NODE","from_table":{"type":"BASE_TABLE","schema_name":"ds_loans","table_name":"data"},"select_list":[]}}]}`

type adminKPIFakeSerializer struct{}

func (adminKPIFakeSerializer) SerializeSQL(context.Context, string) (string, error) {
	return adminKPIASTEnvelope, nil
}

type adminKPIEmptyLister struct{}

func (adminKPIEmptyLister) ListDatasets(context.Context) ([]string, error) { return nil, nil }

// newAdminKPIRegistry loads the fixture through the REAL loader (only the
// CGO serializer seam is faked) so the catalog / signature the endpoint
// reports are the production shapes.
func newAdminKPIRegistry(t *testing.T) *kpi.Registry {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "loans.yaml"), []byte(adminKPIFixtureYAML), 0o644))
	reg, _, err := kpi.LoadKPIs(context.Background(), dir, adminKPIFakeSerializer{}, adminKPIEmptyLister{})
	require.NoError(t, err)
	require.NotNil(t, reg)
	return reg
}

// newAdminKPIStore returns an in-memory sqlite store carrying two kpi_calls
// audit rows (one per fixture KPI).
func newAdminKPIStore(t *testing.T) store.Store {
	t.Helper()
	st, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	base := time.Date(2026, 8, 27, 3, 11, 0, 0, time.UTC)
	for i, name := range []string{"loan_overdue_rate_by_status", "loan_balance_bucket_distribution"} {
		require.NoError(t, st.PutKPICall(context.Background(), store.KPICallRow{
			TS:             base.Add(time.Duration(i) * time.Minute),
			SessionID:      "sess-admin-kpi",
			SkillID:        "data-qa",
			Func:           name,
			ArgsJSON:       `{}`,
			ASOfPeriod:     "2026-07",
			KMinApplied:    5,
			ResultSnapshot: `{"rows":[]}`,
			Signature:      "sig-fixture",
		}))
	}
	return st
}

// invokeAdminKPIs issues GET /api/admin/kpis against a Hertz engine that has
// ONLY the retrieval route bound (mirrors platform_prompt_test.go).
func invokeAdminKPIs(t *testing.T, deps *Deps, adminToken string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/kpis", deps.kpiAdminHandler)
	var headers []ut.Header
	if adminToken != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: adminToken})
	}
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kpis", nil, headers...)
}

// TestAdminKPIs — Phase 16 / Plan 16-08 Task 3 (KPI-01): the read-only
// admin KPI view. Four faces: token gate, catalog+signature+latest_calls
// composition, fail-closed 503 when the pipeline is not wired, and the
// structural retrieval-only lock (T-16-08-02).
func TestAdminKPIs(t *testing.T) {
	// ① no admin token → 403 (and the env-unset variant, same posture).
	t.Run("no admin token", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{KPIRegistry: newAdminKPIRegistry(t)}
		assert.Equal(t, http.StatusForbidden, invokeAdminKPIs(t, deps, "").Code)
		assert.Equal(t, http.StatusForbidden, invokeAdminKPIs(t, deps, "wrong-token").Code)

		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
		assert.Equal(t, http.StatusForbidden, invokeAdminKPIs(t, deps, "the-real-token").Code,
			"unset SKILLFORGE_ADMIN_TOKEN denies every caller")
	})

	// ② token + registry + audit rows → 200 with signature, both KPI names,
	// and both latest_calls keys.
	t.Run("catalog and latest calls", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		reg := newAdminKPIRegistry(t)
		deps := &Deps{KPIRegistry: reg, Store: newAdminKPIStore(t)}

		resp := invokeAdminKPIs(t, deps, "the-real-token")
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body kpiAdminResponse
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

		assert.Equal(t, reg.Signature(), body.Signature, "signature must be the registry content hash (D16-A4)")
		assert.NotEmpty(t, body.Signature)

		require.Len(t, body.KPIs, 2)
		names := []string{body.KPIs[0].Name, body.KPIs[1].Name}
		assert.Contains(t, names, "loan_overdue_rate_by_status")
		assert.Contains(t, names, "loan_balance_bucket_distribution")
		for _, e := range body.KPIs {
			assert.Equal(t, "loans", e.Dataset, "each entry reports its logical dataset")
			assert.NotEmpty(t, e.Description)
			assert.NotEmpty(t, e.ParamsSummary)
		}

		require.Len(t, body.LatestCalls, 2)
		assert.Contains(t, body.LatestCalls, "loan_overdue_rate_by_status")
		assert.Contains(t, body.LatestCalls, "loan_balance_bucket_distribution")
		for _, ts := range body.LatestCalls {
			_, err := time.Parse(time.RFC3339, ts)
			assert.NoError(t, err, "latest_calls values are RFC3339 timestamps")
		}
	})

	// ③ nil registry → 503 fail-closed (never an empty-looking 200).
	t.Run("registry not wired", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{}
		resp := invokeAdminKPIs(t, deps, "the-real-token")
		require.Equal(t, http.StatusServiceUnavailable, resp.Code)
		assert.Contains(t, resp.Body.String(), "KPI pipeline not wired")
	})

	// ④ Structural retrieval-only lock (KPI-01 / Pitfall 14 / T-16-08-02):
	// kpi_admin.go must contain no mutation verb, and the route table must
	// bind /api/admin/kpis for retrieval only. A future "convenience"
	// editing endpoint would be a registration backdoor around the
	// controlled-file design, so it has to break this test first.
	t.Run("retrieval only", func(t *testing.T) {
		src, err := os.ReadFile("kpi_admin.go")
		require.NoError(t, err)
		mutationVerb := regexp.MustCompile(`POST|PUT|DELETE|PATCH`)
		assert.Empty(t, mutationVerb.FindAllString(string(src), -1),
			"kpi_admin.go must contain zero mutation verbs (retrieval-only by construction)")

		routes, err := os.ReadFile("server.go")
		require.NoError(t, err)
		routeLine := regexp.MustCompile(`(?m)^\s*h\.(GET|POST|PUT|DELETE|PATCH)\("/api/admin/kpis".*$`)
		found := routeLine.FindAllStringSubmatch(string(routes), -1)
		require.Len(t, found, 1, "/api/admin/kpis must be registered exactly once")
		assert.Equal(t, "GET", found[0][1], "/api/admin/kpis must be a retrieval route only")
	})
}
