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

// audit_prov_test.go — Phase 24 Task 24-01: /api/session/:id/audit/prov.
// Mirrors audit_test.go's handler-test posture (newAuditHandlerHertz /
// ut.PerformRequest). The load-bearing assertions:
//
//   - SC-9 isolation — the document contains ONLY the requested session's
//     rows (the session_id filter is the security boundary).
//   - Empty session → 200 with an empty (but well-formed) document, not 404
//     — "nothing happened yet" has an empty provenance graph.
//   - Nil store → 503, the standard degraded posture.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/auditprov"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

func newProvHandlerHertz(t *testing.T, deps *Deps) *server.Hertz {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/session/:id/audit/prov", deps.sessionProvHandler)
	return h
}

func invokeProvHandler(t *testing.T, deps *Deps, sessionID string) *ut.ResponseRecorder {
	t.Helper()
	h := newProvHandlerHertz(t, deps)
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/session/"+sessionID+"/audit/prov", nil)
}

func seedProvRows(t *testing.T, s *sqlite.SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	rows := []store.AuditRow{
		{ID: 1, Timestamp: base, SessionID: "sess-A", UserID: "u1", SkillID: "compliance", Query: "qA", ModelVersion: "glm-5.2", VerdictJSON: `{"status":"ok"}`, ChunkIDs: "c1", PlatformFloorVersion: "1.1.0"},
		{ID: 2, Timestamp: base.Add(time.Minute), SessionID: "sess-B", UserID: "u2", SkillID: "simple-qa", Query: "qB", ModelVersion: "glm-5.2", VerdictJSON: `{"status":"ok"}`, ChunkIDs: "c9", PlatformFloorVersion: "1.1.0"},
		{ID: 3, Timestamp: base.Add(2 * time.Minute), SessionID: "sess-A", UserID: "u1", SkillID: "compliance", Query: "qA2", RefusalReason: "no_authorized_source"},
	}
	for i := range rows {
		_, err := s.PutAudit(ctx, rows[i])
		require.NoError(t, err, "seed[%d]", i)
	}
}

// TestSessionProv_Isolation — the PROV document for sess-A contains
// exactly sess-A's turns; sess-B's row never leaks in (SC-9).
func TestSessionProv_Isolation(t *testing.T) {
	s := newTestStore(t)
	seedProvRows(t, s)
	deps := &Deps{Store: s}

	got := invokeProvHandler(t, deps, "sess-A")
	require.Equal(t, http.StatusOK, got.Code)

	var doc auditprov.Document
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &doc))

	require.Len(t, doc.Activity, 2, "sess-A has exactly 2 rows")
	assert.Contains(t, doc.Activity, "sf:turn-1")
	assert.Contains(t, doc.Activity, "sf:turn-3")
	assert.NotContains(t, doc.Activity, "sf:turn-2", "sess-B's row must not leak")

	for id := range doc.Activity {
		assert.NotContains(t, id, "query-qB", "no cross-session content")
	}
}

// TestSessionProv_EmptySessionIsEmptyDocument — an unknown / empty session
// yields 200 + an empty document (no entities/activities), never 404.
func TestSessionProv_EmptySessionIsEmptyDocument(t *testing.T) {
	s := newTestStore(t)
	seedProvRows(t, s)
	deps := &Deps{Store: s}

	got := invokeProvHandler(t, deps, "sess-never-existed")
	require.Equal(t, http.StatusOK, got.Code)

	var doc auditprov.Document
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &doc))
	assert.Empty(t, doc.Activity)
	assert.Empty(t, doc.Entity)
}

// TestSessionProv_NilStoreReturns503 — the standard degraded posture.
func TestSessionProv_NilStoreReturns503(t *testing.T) {
	deps := &Deps{Store: nil}
	got := invokeProvHandler(t, deps, "sess-A")
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
}
