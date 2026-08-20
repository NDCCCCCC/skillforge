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

// plan_api_test.go is Plan 12-04 Task 1's server-side verification
// gate. All tests are hermetic: in-memory SQLite + scripted
// ChatModel stubs; no live GLM / Milvus / embedding calls.
//
// Covered contracts:
//
//   - TestPlanAPIReplayEndpoint — PLAN-05 BE replay: 5 todos via
//     PutPlanTodo (upserts with status changes); GET /api/run/:id/plan
//     returns the ordered sequence with latest status per todo.
//   - TestPlanAPIReplayEndpointEmpty — unknown runID → 200 + empty
//     todos array (existence-leak posture).
//   - TestPlanAPIApproveValidation — POST with empty Decision → 422
//     (HITL-02 gate).
//   - TestPlanAPIApproveExpiredCheckpoint — D11-C2: pending row +
//     checkpoint with mismatched skill_version → 410 Gone +
//     state="expired" + audit chain preserved.
//   - TestPlanAPIApproveUnknownRun — non-existent runID → 404 via the
//     pending-row cross-session moat (mirrors approvalPostHandler).
//   - TestPlanAPIApproveUnknownApproval — valid runID but approvalId
//     matching no pending row → 404; NO resolution audit row written
//     (T-12-04-2 forged-approvalId gate).
//   - TestPlanAPISessionIsolation — runA's plan_todos do NOT leak to
//     runB's GET (T-3-SQLI / SC-9 isolation).
package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/wire"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// planTestHertz spins up a Hertz server with the two 12-04 routes.
// Mirrors approvalTestHertz's listener + warm-up pattern.
func planTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/run/:id/plan", deps.planGetHandler)
	h.POST("/api/run/:id/plan/:approvalId/approve", deps.planApproveHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	cleanup := func() {
		_ = h.Close()
		<-errCh
	}

	addr := "127.0.0.1:" + portStr(port)
	if err := waitForListener(addr, 3*time.Second); err != nil {
		cleanup()
		t.Fatalf("server warmup: %v", err)
	}
	return addr, cleanup
}

// getPlan issues GET /api/run/:id/plan against addr and returns
// (status, parsed body, raw bytes).
func getPlan(t *testing.T, addr, runID string) (int, planGetResponse, []byte) {
	t.Helper()
	url := "http://" + addr + "/api/run/" + runID + "/plan"
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := planGetResponse{}
	if len(raw) > 0 && resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &out), "plan get body must decode")
	}
	return resp.StatusCode, out, raw
}

// postPlanApprove issues POST /api/run/:id/plan/:approvalId/approve
// and returns (status, raw body).
func postPlanApprove(t *testing.T, addr, runID, approvalID string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	url := "http://" + addr + "/api/run/" + runID + "/plan/" + approvalID + "/approve"
	resp, err := http.Post(url, "application/json", strings.NewReader(string(raw)))
	require.NoError(t, err, "POST %s", url)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

// planTestManifest returns a write-capable manifest for the plan approve
// path. Declares plan_approve (12-03 registered it on
// AllowedWriteTargets) AND memory:facts — the interruptSession fixture
// drives a scripted memory_facts_write call to produce the durable
// checkpoint; the handler paths under test are target-agnostic below
// the pending-row gate (which the tests seed explicitly with
// target=plan_approve).
func planTestManifest() *skill.Manifest {
	m := &skill.Manifest{
		Name:        "compliance",
		Description: "plan api test skill",
		Agent:       "ChatModelAgent",
		Writes:      []string{"plan_approve", "memory:facts"},
	}
	m.SetResolvedTiersForTest([]string{"权威"})
	return m
}

// seedPlanTodos writes N todos via PutPlanTodo (the write path 12-02's
// writeTodosAuditedTool uses).
func seedPlanTodos(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, runID string, rows []sqlite.PlanTodoRow) {
	t.Helper()
	for _, r := range rows {
		require.NoError(t, s.PutPlanTodo(ctx, r), "seed PutPlanTodo %s", r.TodoID)
	}
}

// ---------------------------------------------------------------------------
// GET /api/run/:id/plan (PLAN-05 BE replay) tests
// ---------------------------------------------------------------------------

// TestPlanAPIReplayEndpoint — PLAN-05: 5 todo writes (1 initial upsert
// + 4 status-change upserts on the SAME (run_id, todo_id) keys) must
// return the LATEST status per todo (INSERT OR REPLACE last-write-wins
// per TestPlanTodosDuplicatePKUpsert), ordered by ts ASC.
func TestPlanAPIReplayEndpoint(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	runID := "run-replay-01"

	base := time.Now().UTC()
	seedPlanTodos(t, ctx, s, runID, []sqlite.PlanTodoRow{
		// Initial 5-todo plan (upserts).
		{RunID: runID, TodoID: "t1", Content: "retrieve clauses", ActiveForm: "retrieving", Status: "pending", Ts: base, Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t2", Content: "analyze", Status: "pending", Ts: base.Add(time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t3", Content: "draft verdict", Status: "pending", Ts: base.Add(2 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t4", Content: "ground check", Status: "pending", Ts: base.Add(3 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t5", Content: "emit", Status: "pending", Ts: base.Add(4 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		// Status-change upserts (same keys, new status + ts) — the
		// write_todos Tool emits a FRESH snapshot per call; the PK
		// collapses same-key writes to the latest.
		{RunID: runID, TodoID: "t1", Content: "retrieve clauses", ActiveForm: "retrieving", Status: "completed", Ts: base.Add(5 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t2", Content: "analyze", Status: "in_progress", Ts: base.Add(6 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t3", Content: "draft verdict", Status: "completed", Ts: base.Add(7 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: runID, TodoID: "t4", Content: "ground check", Status: "in_progress", Ts: base.Add(8 * time.Second), Op: sqlite.PlanTodoOpUpsert},
	})

	deps := &Deps{Store: s}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, resp, raw := getPlan(t, addr, runID)
	require.Equal(t, http.StatusOK, status, "body: %s", string(raw))
	require.Equal(t, runID, resp.RunID)
	require.Len(t, resp.Todos, 5, "5 distinct todo keys must return 5 rows (PK collapses same-key upserts)")
	assert.Equal(t, 5, resp.Count)

	// Ordered by ts ASC (the store's canonical order): t1..t5 by their
	// LATEST write ts.
	wantStatus := map[string]string{
		"t1": "completed",
		"t2": "in_progress",
		"t3": "completed",
		"t4": "in_progress",
		"t5": "pending",
	}
	for _, todo := range resp.Todos {
		assert.Equal(t, wantStatus[todo.ID], todo.Status,
			"todo %s must carry its LATEST status (last-write-wins replay)", todo.ID)
	}

	// Latest ts across all rows = base+8s (t4's last upsert).
	assert.Equal(t, base.Add(8*time.Second).Unix(), resp.GeneratedAt,
		"generatedAt is the max ts across the returned rows")

	// ActiveForm round-trips: t1 (whose re-upsert carries the
	// "retrieving" form) sits at index 1 under ts-ASC of latest writes
	// (t5@+4s < t1@+5s < t2@+6s < t3@+7s < t4@+8s) — locate it by ID
	// rather than position so the assertion pins the FIELD contract,
	// not the ordering quirk.
	var t1 *wire.PlanTodo
	for i := range resp.Todos {
		if resp.Todos[i].ID == "t1" {
			t1 = &resp.Todos[i]
			break
		}
	}
	require.NotNil(t, t1, "t1 must be present in the replay response")
	assert.Equal(t, "retrieving", t1.ActiveForm, "activeForm must round-trip through the replay endpoint")

	// The canonical store order (ts ASC of the LATEST write per key)
	// is the response order — lock it so the FE PlanReplayView can
	// rely on a stable sequence.
	wantOrder := []string{"t5", "t1", "t2", "t3", "t4"}
	gotOrder := make([]string, 0, len(resp.Todos))
	for _, todo := range resp.Todos {
		gotOrder = append(gotOrder, todo.ID)
	}
	assert.Equal(t, wantOrder, gotOrder, "replay rows are ts-ASC by latest write")
}

// TestPlanAPIReplayEndpointEmpty — PLAN-05: unknown runID returns 200
// with empty todos array (NOT 404 — existence-leak posture mirrors
// sessionHistoryHandler).
func TestPlanAPIReplayEndpointEmpty(t *testing.T) {
	s := historyTestStore(t)

	deps := &Deps{Store: s}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, resp, raw := getPlan(t, addr, "run-unknown-99")
	require.Equal(t, http.StatusOK, status, "body: %s", string(raw))
	assert.Empty(t, resp.Todos, "unknown run must return empty todos")
	assert.Equal(t, 0, resp.Count)
	assert.Equal(t, "run-unknown-99", resp.RunID)
	assert.Equal(t, int64(0), resp.GeneratedAt)
}

// TestPlanAPISessionIsolation — T-3-SQLI / SC-9: runA's plan_todos do
// NOT leak into runB's GET. The WHERE run_id=? parameter binding is
// the isolation enforcement.
func TestPlanAPISessionIsolation(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)

	base := time.Now().UTC()
	seedPlanTodos(t, ctx, s, "run-iso-a", []sqlite.PlanTodoRow{
		{RunID: "run-iso-a", TodoID: "t1", Content: "runA private step", Status: "pending", Ts: base, Op: sqlite.PlanTodoOpUpsert},
	})

	deps := &Deps{Store: s}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, resp, raw := getPlan(t, addr, "run-iso-b-000000001")
	require.Equal(t, http.StatusOK, status, "body: %s", string(raw))
	assert.Empty(t, resp.Todos, "runB must see ZERO of runA's todos (SC-9 isolation)")
	assert.Equal(t, 0, resp.Count)
}

// ---------------------------------------------------------------------------
// POST /api/run/:id/plan/:approvalId/approve (PLAN-03 BE confirm) tests
// ---------------------------------------------------------------------------

// TestPlanAPIApproveValidation — POST with empty Decision → 422
// (ApprovalDecision.Validate fires FIRST; no audit row, no resume).
func TestPlanAPIApproveValidation(t *testing.T) {
	s := historyTestStore(t)
	manifest := planTestManifest()
	runID := "run-validate-1"

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
	}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, body := postPlanApprove(t, addr, runID, "approval-0001", map[string]any{
		"decision":    "",
		"target":      "plan_approve",
		"approval_id": "approval-0001",
	})
	require.Equal(t, http.StatusUnprocessableEntity, status, "body: %s", string(body))
	assert.Contains(t, string(body), "decision")
}

// TestPlanAPIApproveExpiredCheckpoint — D11-C2: a pending plan_approve
// row + a checkpoint whose (skill_version, prompt_hash) snapshot does
// NOT match the active Skill's snapshot → 410 Gone + state="expired" +
// audit_chain_preserved=true. The checkpoint row is deleted; the audit
// chain (pending row) is preserved.
func TestPlanAPIApproveExpiredCheckpoint(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := planTestManifest()
	runID := "run-expired-01"
	cpID := agent.RunnerCheckPointID(runID)

	// Phase 1: real interrupt + durable checkpoint (mirrors the
	// approval_api_test pattern).
	interruptSession(t, ctx, s, runID, manifest)

	// Stamp the checkpoint with a STALE snapshot (mismatches the
	// active Skill's derived snapshot by construction).
	stale := hitl.SkillSnapshot{SkillID: "compliance", Version: "v1", PromptHash: "ph-v1"}
	stampCheckpointSnapshot(t, ctx, s, cpID, stale)

	// Seed the pending plan_approve row so the pending-row lookup
	// (which runs BEFORE the expire-check, mirroring
	// approvalPostHandler) passes and the 410 path is the one under
	// test.
	seedPendingApproval(t, ctx, s, runID, stale, "approval-0001", hitl.PlanApproveTarget,
		[]byte(`{"planId":"plan-1","steps":[{"id":"step-1","content":"x","status":"pending"}]}`))

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
	}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, body := postPlanApprove(t, addr, runID, "approval-0001", map[string]any{
		"decision":    "approve",
		"target":      "plan_approve",
		"approval_id": "approval-0001",
	})
	require.Equal(t, http.StatusGone, status, "body: %s", string(body))
	assert.Contains(t, string(body), `"expired"`)
	assert.Contains(t, string(body), `"audit_chain_preserved":true`)

	// The checkpoint row is deleted (the 410 path's Delete).
	_, existed, err := s.Get(ctx, cpID)
	require.NoError(t, err)
	assert.False(t, existed, "expired checkpoint must be deleted on the 410 path")

	// NO resolution audit row was written (the 410 path runs BEFORE
	// MarkApproved / MarkRejected) — and the pending row survives
	// (T-3-AUDIT INSERT-only).
	for _, action := range []string{"action_approved", "action_rejected"} {
		rows, qErr := s.QueryAudit(ctx, store.AuditFilter{
			SessionID: runID,
			Action:    action,
			Limit:     1,
		})
		require.NoError(t, qErr)
		assert.Empty(t, rows, "no %s row may be written on the expire path", action)
	}
	pendingRows, qErr := s.QueryAudit(ctx, store.AuditFilter{
		SessionID: runID,
		Action:    "action_pending_approval",
		Limit:     5,
	})
	require.NoError(t, qErr)
	assert.NotEmpty(t, pendingRows, "the pending audit row must survive the 410 (audit chain preserved)")
}

// TestPlanAPIApproveUnknownRun — POST for a runID with NO pending
// plan_approve row → 404 (the cross-session moat fires on the
// pending-row lookup, mirroring approvalPostHandler; a wrong run_id
// simply finds no row and learns nothing about which runs exist).
func TestPlanAPIApproveUnknownRun(t *testing.T) {
	s := historyTestStore(t)
	manifest := planTestManifest()

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
	}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, body := postPlanApprove(t, addr, "run-nocp-0001", "approval-0001", map[string]any{
		"decision":    "approve",
		"target":      "plan_approve",
		"approval_id": "approval-0001",
	})
	require.Equal(t, http.StatusNotFound, status,
		"unknown run (no pending row) must 404 via the pending-row moat; body: %s", string(body))
	assert.Contains(t, string(body), "no pending plan approval")
}

// TestPlanAPIApproveUnknownApproval — POST with a runID that HAS a
// pending plan_approve row but an approvalId matching no row → 404,
// and NO resolution audit row is written (the 404 fires before the
// D11-C3 write — a forged approvalId can never mint an
// action_approved row, T-12-04-2).
func TestPlanAPIApproveUnknownApproval(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := planTestManifest()
	runID := "run-unk-appr-1"

	// Seed a REAL pending plan_approve row for approval-0001 (a
	// different ID than the one under test) so the run itself is
	// legitimate — only the approvalId is unknown.
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, runID, snap, "approval-0001", hitl.PlanApproveTarget,
		[]byte(`{"planId":"plan-1","steps":[{"id":"step-1","content":"x","status":"pending"}]}`))

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
	}
	addr, cleanup := planTestHertz(t, deps)
	defer cleanup()

	status, body := postPlanApprove(t, addr, runID, "approval-unknown", map[string]any{
		"decision":    "approve",
		"target":      "plan_approve",
		"approval_id": "approval-unknown",
	})
	require.Equal(t, http.StatusNotFound, status,
		"unknown approvalId must 404 (no pending row for it); body: %s", string(body))
	assert.Contains(t, string(body), "no pending plan approval")

	// NO resolution row may exist for the forged approvalId — the
	// pending-row moat precedes the audit write (D11-C3 ordering).
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID: runID,
		Action:    "action_approved",
		Limit:     5,
	})
	require.NoError(t, err)
	assert.Empty(t, rows, "a forged approvalId must never mint an action_approved row")
}
