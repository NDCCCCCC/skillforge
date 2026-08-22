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

// approval_api_test.go is Plan 11-04's server-side verification gate.
// All tests are hermetic: in-memory SQLite + scripted ChatModel stubs;
// no live GLM / Milvus / embedding calls.
//
// Covered contracts:
//
//   - TestExpiredCheckpointUIState — D11-C2: a pending row whose
//     (skill_version, prompt_hash) snapshot no longer matches the
//     active Skill renders state="expired" on GET and 410 Gone on
//     POST; the audit row survives (T-3-AUDIT INSERT-only).
//   - TestApprovalEndpointBoundToSession — cross-session moat: a POST
//     for an approvalID that belongs to a DIFFERENT session finds no
//     pending row → 404.
//   - TestDataDecisionExtensionNoNewChunkTypes — D-04 + D15: a valid
//     approve drives the full resume round trip and emits
//     data-decision{decision: tool-resumed}; NO new SSE chunk types
//     appear on the wire.
//   - TestCancelPropagationMidToolRunner — D11-A4 <200ms cancel lock
//     extended to the Runner RESUME path (11-01 locked the Run path).
package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// scriptedApprovalModel is a ToolCallingChatModel scripted on the
// conversation history (not a turn counter — the resume path gets a
// FRESH model instance with a restored history, so a counter-keyed
// script would loop):
//
//   - no tool message in history → emit a memory_facts_write tool call
//     (drives the HITL Tool's Pass-1 interrupt);
//   - a tool message present     → emit the final answer.
type scriptedApprovalModel struct {
	mu    sync.Mutex
	calls int
}

var _ model.ToolCallingChatModel = (*scriptedApprovalModel)(nil)

const (
	approvalToolCallID = "tc-000001"
	approvalToolArgs   = `{"key":"employee_count","value":"150"}`
)

func hasToolResult(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if m != nil && m.Role == schema.Tool {
			return true
		}
	}
	return false
}

func (m *scriptedApprovalModel) scripted(msgs []*schema.Message) *schema.Message {
	if !hasToolResult(msgs) {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   approvalToolCallID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      hitl.MemoryFactsToolName,
					Arguments: approvalToolArgs,
				},
			}},
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: "final answer after approval"}
}

func (m *scriptedApprovalModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.scripted(msgs), nil
}

func (m *scriptedApprovalModel) Stream(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *scriptedApprovalModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// blockingAfterToolModel behaves like scriptedApprovalModel until the
// post-tool turn, where Stream blocks until ctx is cancelled —
// mirrors the production GLM stream behavior that the D11-A4 200ms
// cancel lock is anchored against.
type blockingAfterToolModel struct {
	inner scriptedApprovalModel
}

var _ model.ToolCallingChatModel = (*blockingAfterToolModel)(nil)

func (m *blockingAfterToolModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if hasToolResult(msgs) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return m.inner.Generate(ctx, msgs, opts...)
}

func (m *blockingAfterToolModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if hasToolResult(msgs) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	msg, err := m.inner.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *blockingAfterToolModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// approvalManifest returns the write-capable manifest the approval
// tests drive (memory:facts write target + a resolved tier for the
// retrieve tool).
func approvalManifest() *skill.Manifest {
	m := &skill.Manifest{
		Name:        "compliance",
		Description: "approval test skill",
		Agent:       "ChatModelAgent",
		Writes:      []string{"memory:facts"},
	}
	m.SetResolvedTiersForTest([]string{"权威"})
	return m
}

// approvalSnapshot derives the D11-C2 snapshot the ACTIVE Skill would
// produce — the SAME derivation the handler uses
// (hitl.NewSkillSnapshot over agent.RenderSystemPrompt).
func approvalSnapshot(m *skill.Manifest) hitl.SkillSnapshot {
	return hitl.NewSkillSnapshot(m.Name, "", agent.RenderSystemPrompt(m))
}

// approvalTestHertz spins up a Hertz server with the three 11-04
// routes registered. Mirrors historyTestHertz.
func approvalTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/session/:id/approval", deps.approvalGetHandler)
	h.POST("/api/session/:id/approval/:approvalId", deps.approvalPostHandler)
	h.GET("/api/session/:id/audit", deps.auditListHandler)

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

// interruptSession drives Phase 1 of the approval round trip: a real
// BuildRunner-constructed Runner executes one Run whose scripted
// memory_facts_write tool call interrupts; the checkpoint persists to
// s. Returns the captured interrupt UUID (the eino resume-target key —
// verified by probe: ONLY this UUID resumes the Tool).
func interruptSession(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, sessionID string, m *skill.Manifest) string {
	t.Helper()
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        m,
		Embedder:        newStubEmbedder(),
		Retriever:       &stubRetriever{},
		ChatModel:       &scriptedApprovalModel{},
		SessionID:       sessionID,
		SkillID:         m.Name,
		CheckPointStore: s,
	})
	require.NoError(t, err, "phase 1 BuildRunner")

	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("persist the employee count")},
		adk.WithCheckPointID(agent.RunnerCheckPointID(sessionID)))
	var interruptID string
	var sawInterrupt bool
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		require.NoError(t, ev.Err, "phase 1 events must be error-free")
		if ev.Action != nil && ev.Action.Interrupted != nil {
			sawInterrupt = true
			require.NotEmpty(t, ev.Action.Interrupted.InterruptContexts)
			interruptID = ev.Action.Interrupted.InterruptContexts[0].ID
		}
	}
	require.True(t, sawInterrupt, "the scripted tool call MUST surface Action.Interrupted")
	require.NotEmpty(t, interruptID, "interrupt UUID must be captured")

	// The checkpoint must be durably persisted BEFORE the resume.
	_, existed, err := s.Get(ctx, agent.RunnerCheckPointID(sessionID))
	require.NoError(t, err)
	require.True(t, existed, "checkpoint must persist after the interrupt")
	return interruptID
}

// seedPendingApproval writes the action_pending_approval audit row
// through the fail-closed writer (the SAME code path the Tool's seam
// uses in production).
func seedPendingApproval(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, sessionID string, snap hitl.SkillSnapshot, toolCallID, target string, frozenArgs []byte) {
	t.Helper()
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	_, err := w.WritePendingApproval(ctx, sessionID, "", snap, toolCallID, target, frozenArgs, "")
	require.NoError(t, err, "seed pending approval row")
}

// stampCheckpointSnapshot rewrites the checkpoint row with the D11-C2
// snapshot columns (the payload bytes are carried over verbatim).
func stampCheckpointSnapshot(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, cpID string, snap hitl.SkillSnapshot) {
	t.Helper()
	payload, existed, err := s.Get(ctx, cpID)
	require.NoError(t, err)
	require.True(t, existed, "checkpoint must exist before stamping")
	require.NoError(t, s.SetCheckpointWithSnapshot(ctx, cpID, payload, snap.Version, snap.PromptHash))
}

// postApproval issues POST /api/session/:id/approval/:approvalId and
// returns (status, rawBody).
func postApproval(t *testing.T, addr, sessionID, approvalID string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	url := "http://" + addr + "/api/session/" + sessionID + "/approval/" + approvalID
	resp, err := http.Post(url, "application/json", strings.NewReader(string(raw)))
	require.NoError(t, err, "POST %s", url)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

// getApprovals issues GET /api/session/:id/approval and returns
// (status, parsed, rawBody).
func getApprovals(t *testing.T, addr, sessionID string) (int, approvalListResponse, []byte) {
	t.Helper()
	url := "http://" + addr + "/api/session/" + sessionID + "/approval"
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := approvalListResponse{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &out), "approval list body must decode")
	}
	return resp.StatusCode, out, raw
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestExpiredCheckpointUIState — D11-C2 / HITL-03. A pending row
// written against Skill snapshot (v1, ph-v1) renders state="expired"
// once the ACTIVE Skill no longer matches; the POST for the same
// approval answers 410 Gone with audit_chain_preserved=true. The
// audit row itself is NEVER deleted (T-3-AUDIT INSERT-only).
func TestExpiredCheckpointUIState(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-expired-01"

	// Pending row stamped with a STALE snapshot (the active Skill's
	// snapshot differs by construction — see approvalSnapshot).
	stale := hitl.SkillSnapshot{SkillID: "compliance", Version: "v1", PromptHash: "ph-v1"}
	seedPendingApproval(t, ctx, s, sessionID, stale, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))

	deps := &Deps{Store: s, ChatModel: &scriptedApprovalModel{}, ActiveManifest: manifest}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	// GET: the stale row renders expired; a row stamped with the
	// ACTIVE snapshot renders active (control case).
	activeSnap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, activeSnap, "tc-000002", "memory:facts", []byte(approvalToolArgs))

	status, list, _ := getApprovals(t, addr, sessionID)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, list.Approvals, 2)
	states := map[string]string{}
	for _, a := range list.Approvals {
		states[a.ApprovalID] = a.State
	}
	assert.Equal(t, "expired", states[approvalToolCallID],
		"stale snapshot must render state=expired (D11-C2)")
	assert.Equal(t, "active", states["tc-000002"],
		"matching snapshot must render state=active (control)")

	// POST: expire-check fires → 410 Gone; audit chain preserved.
	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":    "approve",
		"target":      "memory:facts",
		"approval_id": approvalToolCallID,
	})
	require.Equal(t, http.StatusGone, status, "POST body: %s", string(body))
	assert.Contains(t, string(body), `"expired"`)
	assert.Contains(t, string(body), `"audit_chain_preserved":true`)

	// T-3-AUDIT: the pending row survives the 410 path.
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_pending_approval",
		ToolCallID: approvalToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the audit row is INSERT-only and survives the expire path")

	// And NO resolution row was written (the 410 path runs BEFORE
	// MarkApproved / MarkRejected).
	for _, action := range []string{"action_approved", "action_rejected"} {
		res, qErr := s.QueryAudit(ctx, store.AuditFilter{
			SessionID:  sessionID,
			Action:     action,
			ToolCallID: approvalToolCallID,
			Limit:      1,
		})
		require.NoError(t, qErr)
		assert.Empty(t, res, "no %s row may be written on the expire path", action)
	}
}

// TestApprovalEndpointBoundToSession — the cross-session
// authorization moat (T-11-04-02): a POST for an approvalID that
// belongs to a different session finds no pending row → 404. The
// audit row is untouched.
func TestApprovalEndpointBoundToSession(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()

	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, "sess-correct-01", snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))

	deps := &Deps{Store: s, ChatModel: &scriptedApprovalModel{}, ActiveManifest: manifest}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	status, body := postApproval(t, addr, "sess-wrong-0002", approvalToolCallID, map[string]any{
		"decision":    "approve",
		"target":      "memory:facts",
		"approval_id": approvalToolCallID,
	})
	require.Equal(t, http.StatusNotFound, status,
		"wrong session must find no pending row (cross-session moat); body: %s", string(body))

	// The pending row for the CORRECT session is unchanged.
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  "sess-correct-01",
		Action:     "action_pending_approval",
		ToolCallID: approvalToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// TestDataDecisionExtensionNoNewChunkTypes — D-04 + D15 wire-type
// preservation on the approve path. A valid approve drives the FULL
// resume round trip (checkpoint → ResumeWithParams → Tool Pass 2 →
// final answer) and emits data-decision{decision:"tool-resumed"};
// NO new SSE chunk types appear.
func TestDataDecisionExtensionNoNewChunkTypes(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-datadec-01"
	cpID := agent.RunnerCheckPointID(sessionID)

	// Phase 1: real interrupt + durable checkpoint.
	interruptID := interruptSession(t, ctx, s, sessionID, manifest)

	// Seed the pending row + stamp the checkpoint with the ACTIVE
	// snapshot so the expire-check passes.
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))
	stampCheckpointSnapshot(t, ctx, s, cpID, snap)

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
	}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
	})
	require.Equal(t, http.StatusOK, status, "approve must succeed; body: %s", string(body))

	text := string(body)
	// The resume round trip actually executed: the Tool's Pass-2
	// success string reached the SSE stream.
	assert.Contains(t, text, "memory_facts_write OK",
		"the resumed Tool must execute and its output must stream")
	// D-04 string-enum extension is observable.
	assert.Contains(t, text, `"type":"data-decision"`,
		"the resolution must surface via the EXISTING data-decision chunk type")
	assert.Contains(t, text, `"decision":"tool-resumed"`,
		"approve must emit data-decision{decision: tool-resumed}")
	// D15: NO new SSE chunk types — the wire union is unchanged.
	for _, banned := range []string{
		"data-approval-resolved",
		"data-approval-denied",
		"data-approval",
		"approval-resolved",
		"approval-denied",
	} {
		assert.NotContains(t, text, banned,
			"no new SSE chunk discriminator %q may appear (D-04 + D15)", banned)
	}

	// The audit chain recorded the approval (action_approved row).
	res, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_approved",
		ToolCallID: approvalToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, res, 1, "MarkApproved must persist the resolution row")

	// Idempotency (T-11-04-06): a second POST for the same approval
	// answers already_resolved instead of re-executing.
	status2, body2 := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
	})
	require.Equal(t, http.StatusOK, status2)
	assert.Contains(t, string(body2), "already_resolved")
}

// TestCancelPropagationMidToolRunner — D11-A4 <200ms cancel lock on
// the RESUME path (extends 11-01's Run-path lock). A resumed Runner
// whose post-tool model call blocks on ctx must tear down within
// 200ms of cancellation.
func TestCancelPropagationMidToolRunner(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-cancel-01"
	cpID := agent.RunnerCheckPointID(sessionID)

	// Phase 1: interrupt + checkpoint (non-blocking scripted model).
	interruptID := interruptSession(t, ctx, s, sessionID, manifest)

	// Phase 2: resume with a model that BLOCKS on the post-tool turn.
	resumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, err := agent.BuildRunner(resumeCtx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        newStubEmbedder(),
		Retriever:       &stubRetriever{},
		ChatModel:       &blockingAfterToolModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
	})
	require.NoError(t, err)

	iter, err := runner.ResumeWithParams(resumeCtx, cpID, &adk.ResumeParams{
		Targets: map[string]any{interruptID: &hitl.ApprovalDecision{
			Decision:   "approve",
			Target:     "memory:facts",
			ApprovalID: approvalToolCallID,
		}},
	})
	require.NoError(t, err, "ResumeWithParams must load the checkpoint")

	type result struct{ err error }
	resCh := make(chan result, 1)
	go func() {
		resCh <- result{err: agent.DrainRunnerEvents(resumeCtx, iter, nil)}
	}()

	// Warm-up: let the resume reach the blocking model call.
	time.Sleep(50 * time.Millisecond)
	cancelTime := time.Now()
	cancel()

	select {
	case res := <-resCh:
		elapsed := time.Since(cancelTime)
		if res.err != nil {
			require.ErrorIs(t, res.err, context.Canceled,
				"drain should surface ctx cancellation, got %v", res.err)
		}
		require.Less(t, elapsed, 200*time.Millisecond,
			"D11-A4: resume-path cancel did not propagate within 200ms (elapsed=%v)", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("D11-A4 FAIL: resume drain did not return within 2s of cancel")
	}
}

// ---------------------------------------------------------------------------
// audit_api.go tests (GET /api/session/:id/audit)
// ---------------------------------------------------------------------------

// TestAuditListHandler — the AuditTimelineTab back-end contract:
// action-filter validation, limit bounds, ?-parameterized store
// query, ts-ordered wire shape.
func TestAuditListHandler(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	sessionID := "sess-audit-001"

	snap := hitl.SkillSnapshot{SkillID: "compliance", Version: "v1", PromptHash: "ph1"}
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	_, err := w.WritePendingApproval(ctx, sessionID, "", snap, "tc-000001", "memory:facts", []byte(`{"key":"k"}`), "")
	require.NoError(t, err)
	_, err = w.MarkApproved(ctx, sessionID, "", snap, "tc-000001", "tc-000001", "memory:facts", []byte(`{"key":"k"}`), "ok")
	require.NoError(t, err)

	deps := &Deps{Store: s}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	get := func(url string) (int, auditListResponse, string) {
		t.Helper()
		resp, err := http.Get("http://" + addr + url)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		out := auditListResponse{}
		if resp.StatusCode == http.StatusOK {
			require.NoError(t, json.Unmarshal(raw, &out))
		}
		return resp.StatusCode, out, string(raw)
	}

	// All actions for the session.
	status, all, _ := get("/api/session/" + sessionID + "/audit")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 2, all.Total)
	require.Len(t, all.Rows, 2)

	// Filtered by action type.
	status, pending, _ := get("/api/session/" + sessionID + "/audit?type=action_pending_approval")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 1, pending.Total)
	assert.Equal(t, "action_pending_approval", pending.Rows[0].Action)
	assert.Equal(t, "tc-000001", pending.Rows[0].ToolCallID)
	assert.Equal(t, "memory:facts", pending.Rows[0].Target)

	// Invalid type → 400.
	status, _, raw := get("/api/session/" + sessionID + "/audit?type=DROP%20TABLE")
	require.Equal(t, http.StatusBadRequest, status, "body: %s", raw)

	// limit=0 → 400; limit>200 caps silently.
	status, _, _ = get("/api/session/" + sessionID + "/audit?limit=0")
	require.Equal(t, http.StatusBadRequest, status)
	status, capped, _ := get("/api/session/" + sessionID + "/audit?limit=500")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 2, capped.Total)

	// Unknown session → 200 + empty (existence-leak posture).
	status, empty, _ := get("/api/session/sess-unknown-99/audit")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 0, empty.Total)
	assert.Empty(t, empty.Rows)
}
