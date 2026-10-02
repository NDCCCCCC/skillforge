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
	"regexp"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// emptyBatchesStore is a minimal store that returns zero rows.
type emptyBatchesStore struct{}

func (emptyBatchesStore) ListAllImportBatches(context.Context) ([]store.ImportBatchRow, error) {
	return nil, nil
}

func (emptyBatchesStore) Close() error { return nil }

// newBatchesStore returns an in-memory sqlite store with two import batch rows.
func newBatchesStore(t *testing.T) store.Store {
	t.Helper()
	st, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	rows := []store.ImportBatchRow{
		{
			BatchID:              "batch-001",
			Dataset:              "loans",
			Period:               "2026-07",
			RawSizeBytes:         1024000,
			DesensitizedRowCount: 5000,
			DiscardTS:            "2026-07-01T12:00:00Z",
			CreatedAt:            base.Add(time.Hour), // newer = comes first in DESC order
		},
		{
			BatchID:              "batch-002",
			Dataset:              "claims",
			Period:               "2026-07",
			RawSizeBytes:         512000,
			DesensitizedRowCount: 2500,
			DiscardTS:            "",
			CreatedAt:            base,
		},
	}
	for _, row := range rows {
		require.NoError(t, st.PutImportBatch(context.Background(), row))
	}
	return st
}

// invokeAdminBatches issues GET /api/admin/batches against a Hertz engine that
// has ONLY the retrieval route bound.
func invokeAdminBatches(t *testing.T, deps *Deps, adminToken string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/batches", deps.batchesAdminHandler)
	var headers []ut.Header
	if adminToken != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: adminToken})
	}
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/batches", nil, headers...)
}

// TestAdminBatchesHandler tests the three terminal response paths of
// batchesAdminHandler: 403 (no token), 503 (store nil), and 200 (ok).
func TestAdminBatchesHandler(t *testing.T) {
	// ① no admin token → 403 (and the env-unset variant, same posture).
	t.Run("no admin token", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{Store: newBatchesStore(t)}
		assert.Equal(t, http.StatusForbidden, invokeAdminBatches(t, deps, "").Code)
		assert.Equal(t, http.StatusForbidden, invokeAdminBatches(t, deps, "wrong-token").Code)

		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
		assert.Equal(t, http.StatusForbidden, invokeAdminBatches(t, deps, "the-real-token").Code,
			"unset SKILLFORGE_ADMIN_TOKEN denies every caller")
	})

	// ② token + store with rows → 200 with correct JSON shape.
	t.Run("batch list response", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{Store: newBatchesStore(t)}

		resp := invokeAdminBatches(t, deps, "the-real-token")
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body importBatchesAdminResponse
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

		require.Len(t, body.Batches, 2, "two batches inserted")

		// First batch is completed (has discard_ts).
		assert.Equal(t, "batch-001", body.Batches[0].BatchID)
		assert.Equal(t, "loans", body.Batches[0].Dataset)
		assert.Equal(t, "completed", body.Batches[0].Status)
		assert.Equal(t, int64(5000), body.Batches[0].DesensitizedRowCount)
		assert.Equal(t, int64(1024000), body.Batches[0].RawSizeBytes)
		assert.Equal(t, "2026-07-01T12:00:00Z", body.Batches[0].DiscardTS)

		// Second batch is pending (no discard_ts).
		assert.Equal(t, "batch-002", body.Batches[1].BatchID)
		assert.Equal(t, "claims", body.Batches[1].Dataset)
		assert.Equal(t, "pending", body.Batches[1].Status)
		assert.Equal(t, "", body.Batches[1].DiscardTS)
	})

	// ③ nil store → 503 fail-closed.
	t.Run("store not wired", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{}
		resp := invokeAdminBatches(t, deps, "the-real-token")
		require.Equal(t, http.StatusServiceUnavailable, resp.Code)
		assert.Contains(t, resp.Body.String(), "store not wired")
	})

	// ④ empty store → 200 with empty batches array.
	t.Run("empty batch list", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		emptySt, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
		require.NoError(t, err)
		t.Cleanup(func() { _ = emptySt.Close() })
		deps := &Deps{Store: emptySt}
		resp := invokeAdminBatches(t, deps, "the-real-token")
		require.Equal(t, http.StatusOK, resp.Code)
		var body importBatchesAdminResponse
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		require.Len(t, body.Batches, 0, "empty store returns zero rows")
	})
}

// TestAdminBatchesRetrievalOnly structural retrieval-only lock (ADM-02 / T-20-02-MUTATE):
// admin_batches.go must contain no mutation verb, and the route table must bind
// /api/admin/batches for retrieval only. A future "convenience" editing endpoint
// would be a registration backdoor around the INSERT-only audit table, so it has
// to break this test first.
func TestAdminBatchesRetrievalOnly(t *testing.T) {
	src, err := os.ReadFile("admin_batches.go")
	require.NoError(t, err)
	mutationVerb := regexp.MustCompile(`POST|PUT|DELETE|PATCH`)
	assert.Empty(t, mutationVerb.FindAllString(string(src), -1),
		"admin_batches.go must contain zero mutation verbs (retrieval-only by construction)")

	routes, err := os.ReadFile("server.go")
	require.NoError(t, err)
	routeLine := regexp.MustCompile(`(?m)^\s*h\.(GET|POST|PUT|DELETE|PATCH)\("/api/admin/batches".*$`)
	found := routeLine.FindAllStringSubmatch(string(routes), -1)
	require.Len(t, found, 1, "/api/admin/batches must be registered exactly once")
	assert.Equal(t, "GET", found[0][1], "/api/admin/batches must be a retrieval route only")
}
