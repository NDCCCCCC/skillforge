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

// approval_api.go is Phase 11 / Plan 11-04's BE half of the HITL
// approval gate (HITL-01 / HITL-02 / HITL-03 + D11-C1 dual endpoint +
// D11-C2 expire-check).
//
// Endpoints:
//
//	GET  /api/session/:id/approval               — list pending
//	    action_pending_approval audit rows with a per-row state
//	    (active | expired | auto_passed) derived from the D11-C2
//	    skill_version + prompt_hash comparison against the active
//	    Skill snapshot.
//	POST /api/session/:id/approval/:approvalId   — operator decision.
//	    Order: regex gates → deps gate → JSON decode →
//	    ApprovalDecision.Validate (422, HITL-02 deny-reason) →
//	    pending-row lookup (404, cross-session moat) → already-resolved
//	    idempotency (200) → target spoof check (422) → D11-C2
//	    expire-check (410 Gone + checkpoint Delete; audit row
//	    preserved per T-3-AUDIT) → AuditWriter.MarkApproved /
//	    MarkRejected → Runner.ResumeWithParams + drain →
//	    data-decision{decision: tool-resumed | tool-denied} SSE chunk.
//
// D-04 + D15 invariant: NO new SSE chunk types are introduced. Only
// the data-decision payload's `decision` STRING enum is extended
// ({interrupted, tool-resumed, tool-denied, rule-matched}); the wire
// union (wire.ValidTypes) is untouched.
//
// Resume-target note (verified empirically against eino v0.9.14):
// ResumeWithParams Targets keys must be the interrupt UUID captured
// from AgentEvent.Action.Interrupted.InterruptContexts[i].ID at
// interrupt time — neither the raw tool_call_id nor the joined
// address string resumes the Tool (both probe-tested: the Tool
// re-interrupts). The POST body therefore accepts an optional
// `interrupt_id` (the FE echoes it from the data-decision
// interrupted payload); when absent the handler falls back to the
// approvalID (D11-C4 plan-literal contract).
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/wire"
)

// Audit action enum literals (the store layer persists them; the
// handlers filter on them). Kept local so the HTTP surface does not
// depend on unexported store constants.
const (
	auditActionPending  = "action_pending_approval"
	auditActionApproved = "action_approved"
	auditActionRejected = "action_rejected"

	// approvalStateActive — the pending row's D11-C2 snapshot matches
	// the active Skill; the operator may approve/deny it.
	approvalStateActive = "active"
	// approvalStateExpired — skill_version or prompt_hash drifted
	// (Skill hot-reload since the interrupt). The FE renders the row
	// as non-actionable (HITL-03).
	approvalStateExpired = "expired"
	// approvalStateAutoPassed — a resolution row (action_approved |
	// action_rejected, including rule auto-pass rows flagged with
	// refusal_reason="auto_passed_rule") already exists for the same
	// tool_call_id; the pending row is historical.
	approvalStateAutoPassed = "auto_passed"

	// decisionEventResumed / decisionEventDenied are the two new
	// data-decision `decision` string values (D-04 string-enum
	// extension; NO new SSE chunk types — D15).
	decisionEventResumed = "tool-resumed"
	decisionEventDenied  = "tool-denied"
)

// pendingApproval is one row of the GET /api/session/:id/approval
// response — the FE ApprovalCard / approval-drawer wire contract.
type pendingApproval struct {
	ApprovalID   string          `json:"approval_id"`
	Target       string          `json:"target"`
	ToolName     string          `json:"tool_name"`
	FrozenArgs   json.RawMessage `json:"frozen_args"`
	Reason       string          `json:"reason,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	State        string          `json:"state"`
	SkillVersion string          `json:"skill_version"`
	PromptHash   string          `json:"prompt_hash"`
}

// approvalListResponse is the GET response envelope.
type approvalListResponse struct {
	Approvals []pendingApproval `json:"approvals"`
}

// approvalPostRequest is the POST body. It mirrors the
// hitl.ApprovalDecision JSON shape plus the optional interrupt_id
// (the eino resume-target UUID — see the file-level note).
type approvalPostRequest struct {
	Decision    string          `json:"decision"`
	Reason      string          `json:"reason,omitempty"`
	Target      string          `json:"target"`
	FrozenArgs  json.RawMessage `json:"frozen_args,omitempty"`
	Scope       string          `json:"scope,omitempty"`
	ApprovalID  string          `json:"approval_id"`
	InterruptID string          `json:"interrupt_id,omitempty"`
}

// activeSkillSnapshot derives the D11-C2 snapshot of the active Skill:
// identity + content version + prompt hash. The version is derived
// from the rendered prompt body ("sha256:<16 hex>") so it changes
// exactly when the Skill content changes — stable across process
// restarts (see hitl.NewSkillSnapshot's documented deviation:
// skill.Manifest has no Version field).
func activeSkillSnapshot(m *skill.Manifest) hitl.SkillSnapshot {
	if m == nil {
		return hitl.SkillSnapshot{}
	}
	return hitl.NewSkillSnapshot(m.Name, "", agent.RenderSystemPrompt(m))
}

// toolNameForTarget resolves the LLM-facing Tool name for an audit
// row's target via the hitl registry (no hardcoded target→name map —
// the registry is the D11-B3 source of truth). Empty when the target
// has no registered handler.
func toolNameForTarget(ctx context.Context, target string) string {
	h := hitl.LookupHandler(target)
	if h == nil {
		return ""
	}
	info, err := h.Info(ctx)
	if err != nil || info == nil {
		return ""
	}
	return info.Name
}

// auditWriterOrDefault returns Deps.AuditWriter, constructing an
// ephemeral one from Deps.Store when the field is unset (unit tests
// that build Deps literally). The returned writer is a local — Deps
// is never mutated on the request path (concurrency-safe).
func (s *Deps) auditWriterOrDefault() *hitl.AuditWriter {
	if s.AuditWriter != nil {
		return s.AuditWriter
	}
	return hitl.NewAuditWriter(s.Store)
}

// approvalGetHandler implements GET /api/session/:id/approval
// (D11-C1). Returns the session's pending approval rows with the
// derived state. Empty result → 200 + empty list (existence-leak
// posture, mirrors sessionHistoryHandler — a 404 would tell an
// attacker which session IDs are valid).
//
// Status codes: 200 (valid session, empty or populated) | 400
// (malformed session id) | 404 (Deps.Store nil — misconfiguration) |
// 500 (store error).
func (s *Deps) approvalGetHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "approval store not initialized",
		})
		return
	}
	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}

	// T-3-SQLI: every filter value is a ?-placeholder at the store
	// layer (QueryAudit builds the WHERE clause from hardcoded
	// parameterized literals only).
	pending, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID: sessionID,
		Action:    auditActionPending,
		Limit:     50,
	})
	if err != nil {
		log.Printf("[WARN] server: approval list (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "approval list failed",
		})
		return
	}

	// Resolved set: tool_call_ids with an action_approved or
	// action_rejected row (operator decisions AND D11-D1 rule
	// auto-passes — the auto-pass row is an action_approved row with
	// refusal_reason="auto_passed_rule").
	resolved := map[string]bool{}
	for _, action := range []string{auditActionApproved, auditActionRejected} {
		rows, qErr := s.Store.QueryAudit(ctx, store.AuditFilter{
			SessionID: sessionID,
			Action:    action,
			Limit:     200,
		})
		if qErr != nil {
			log.Printf("[WARN] server: approval list resolutions (sess=%s action=%s) failed: %v", sessionID, action, qErr)
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "approval list failed",
			})
			return
		}
		for _, r := range rows {
			resolved[r.ToolCallID] = true
		}
	}

	snap := activeSkillSnapshot(s.ActiveManifest)
	out := make([]pendingApproval, 0, len(pending))
	for _, row := range pending {
		state := approvalStateActive
		switch {
		case resolved[row.ToolCallID]:
			state = approvalStateAutoPassed
		case s.ActiveManifest == nil:
			// Fail-closed: without the active Skill the freshness of
			// the pending row cannot be proven — render it expired so
			// the FE never offers an actionable approve against an
			// unverifiable Skill context.
			state = approvalStateExpired
		case row.SkillVersion != snap.Version || row.PromptHash != snap.PromptHash:
			state = approvalStateExpired
		}
		approvalID := row.ApprovalID
		if approvalID == "" {
			approvalID = row.ToolCallID // D11-C4 single identity
		}
		out = append(out, pendingApproval{
			ApprovalID:   approvalID,
			Target:       row.Query, // the audit writer stores the write target in the query column
			ToolName:     toolNameForTarget(ctx, row.Query),
			FrozenArgs:   json.RawMessage(row.FrozenArgs),
			Reason:       row.RefusalReason,
			CreatedAt:    row.Timestamp,
			State:        state,
			SkillVersion: row.SkillVersion,
			PromptHash:   row.PromptHash,
		})
	}

	c.JSON(http.StatusOK, approvalListResponse{Approvals: out})
}

// approvalPostHandler implements POST /api/session/:id/approval/:approvalId
// (D11-C1). See the file-level comment for the mandatory step order.
//
// Status codes (D11-C1 + D11-C2 + HITL-02):
//
//	200 — approval resolved (SSE stream carries the drained resume
//	      events + the data-decision chunk), or idempotent
//	      already_resolved (JSON).
//	400 — malformed sessionID / approvalID / JSON body / approvalID
//	      mismatch between URL and body.
//	404 — Deps misconfiguration OR no pending row for this
//	      session+approvalID (cross-session moat: a wrong session_id
//	      simply finds no row).
//	410 — D11-C2 expire-check mismatch; checkpoint deleted, audit
//	      chain preserved.
//	422 — ApprovalDecision.Validate failure (field named so the FE
//	      highlights the failing input) or target spoof.
//	500 — store / audit-writer / runner error.
func (s *Deps) approvalPostHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil || s.ChatModel == nil || s.ActiveManifest == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "approval endpoint not initialized",
		})
		return
	}
	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}
	approvalID := c.Param("approvalId")
	if !sessionIDRegex.MatchString(approvalID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid approval id",
		})
		return
	}

	var req approvalPostRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
		return
	}
	if req.ApprovalID == "" {
		req.ApprovalID = approvalID
	} else if req.ApprovalID != approvalID {
		c.JSON(http.StatusBadRequest, map[string]string{
			"field": "approval_id",
			"msg":   "body approval_id does not match the URL segment",
		})
		return
	}

	// HITL-02 / D11-B2: Validate FIRST. A bad decision never reaches
	// the audit writer or the Runner (the 422 and 200 paths are
	// mutually exclusive at the HTTP boundary).
	decision := &hitl.ApprovalDecision{
		Decision:   req.Decision,
		Reason:     req.Reason,
		Target:     req.Target,
		FrozenArgs: req.FrozenArgs,
		Scope:      req.Scope,
		ApprovalID: req.ApprovalID,
	}
	if err := decision.Validate(); err != nil {
		resp := map[string]string{"error": "invalid approval decision"}
		if verr, ok := err.(*hitl.ValidationError); ok {
			resp["field"] = verr.Field
			resp["msg"] = verr.Msg
		}
		c.JSON(http.StatusUnprocessableEntity, resp)
		return
	}

	// Pending-row lookup (T-3-SQLI ?-parameterized). Wrong session →
	// no rows → 404 (the cross-session authorization moat,
	// T-11-04-02).
	pending, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     auditActionPending,
		ToolCallID: approvalID,
		Limit:      1,
	})
	if err != nil {
		log.Printf("[WARN] server: approval pending lookup (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "approval lookup failed",
		})
		return
	}
	if len(pending) == 0 {
		// Idempotency (T-11-04-06): a second POST for an already
		// resolved approval returns 200 already_resolved rather than
		// 404 — the operator's UI retries must not look like attacks.
		if s.approvalAlreadyResolved(ctx, sessionID, approvalID) {
			c.JSON(http.StatusOK, map[string]string{
				"state": "already_resolved",
			})
			return
		}
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "no pending approval for this session",
		})
		return
	}
	row := pending[0]

	// Spoof check: a non-empty FE-claimed target must match the audit
	// row's target (empty claims are recoverable — see
	// ApprovalDecision.TargetMatches).
	if !decision.TargetMatches(row.Query) {
		c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"field": "target",
			"msg":   "does not match the pending approval's target",
		})
		return
	}

	// D11-C2 EXPIRE-CHECK — before any audit resolution row is
	// written. A skill_version / prompt_hash mismatch (Skill
	// hot-reload since the interrupt) expires the checkpoint: Delete
	// the CHECKPOINT row (never the audit row — T-3-AUDIT INSERT-only)
	// and answer 410 Gone so the FE collapses the ApprovalCard into
	// the expired state (HITL-03).
	cpID := agent.RunnerCheckPointID(sessionID)
	snap := activeSkillSnapshot(s.ActiveManifest)
	_, checkpointValid, err := s.Store.GetCheckpointByVersionFilter(ctx, cpID, snap.Version, snap.PromptHash)
	if err != nil {
		log.Printf("[WARN] server: approval expire-check (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "approval expire-check failed",
		})
		return
	}
	if !checkpointValid {
		if dErr := s.Store.Delete(ctx, cpID); dErr != nil {
			// Delete is idempotent; a failure here is logged but does
			// not change the response — the checkpoint is unusable
			// either way (the version filter already rejected it).
			log.Printf("[WARN] server: approval expire-check delete (sess=%s cp=%s) failed: %v", sessionID, cpID, dErr)
		}
		c.JSON(http.StatusGone, map[string]any{
			"state":                 approvalStateExpired,
			"audit_chain_preserved": true,
		})
		return
	}

	// Audit resolution row (D11-C3 fail-closed writer — the audit
	// chain is the legal-evidence anchor; a write failure is a 500,
	// never a swallow). HITL-07 edit-before-approve: the operator's
	// edited FrozenArgs win; otherwise the pending row's byte snapshot
	// is carried verbatim.
	frozenArgs := decision.FrozenArgs
	if len(frozenArgs) == 0 {
		frozenArgs = row.FrozenArgs
	}
	writer := s.auditWriterOrDefault()
	if decision.Decision == "deny" {
		if _, err := writer.MarkRejected(ctx, sessionID, "", snap, approvalID, approvalID, row.Query, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: approval MarkRejected (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "audit write failed",
			})
			return
		}
	} else {
		if _, err := writer.MarkApproved(ctx, sessionID, "", snap, approvalID, approvalID, row.Query, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: approval MarkApproved (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "audit write failed",
			})
			return
		}
	}

	// Re-hydrate the per-session Runner (D11-B4: resume is a NEW
	// Runner instance pointing at the SAME store + same
	// checkPointID). eino v0.9.14 attaches the CheckPointStore only
	// via RunnerConfig — surfaced through RunnerBuildConfig (11-04).
	cpStore, ok := s.Store.(adk.CheckPointStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "store does not support checkpoints",
		})
		return
	}
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        s.ActiveManifest,
		Embedder:        s.Embedder,
		Retriever:       s.Retriever,
		ChatModel:       s.ChatModel,
		SessionID:       sessionID,
		SkillID:         s.ActiveManifest.Name,
		CheckPointStore: cpStore,
	})
	if err != nil {
		log.Printf("[ERROR] server: approval BuildRunner (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "runner build failed",
		})
		return
	}

	// Resume. The Targets key is the interrupt UUID echoed by the FE
	// (see the file-level resume-target note); the approvalID is the
	// D11-C4 plan-literal fallback.
	resumeTarget := req.InterruptID
	if resumeTarget == "" {
		resumeTarget = approvalID
	}
	iter, err := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
		Targets: map[string]any{resumeTarget: decision},
	})
	if err != nil {
		log.Printf("[ERROR] server: approval ResumeWithParams (sess=%s cp=%s) failed: %v", sessionID, cpID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "resume failed",
		})
		return
	}

	// Drain the resumed run into the SSE stream. The D11-A4 <200ms
	// cancel lock applies on the resume path too (anchored by
	// DrainRunnerEvents' select-before-Next; locked by
	// TestCancelPropagationMidToolRunner's resume case).
	emit := emitChunkClosure(c)
	if err := agent.DrainRunnerEvents(ctx, iter, emit); err != nil {
		// The SSE stream is already in flight — mirror ChatHandler:
		// emit terminal chunks so the client sees a bounded state.
		log.Printf("[ERROR] server: approval drain (sess=%s) failed: %v", sessionID, err)
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
		return
	}

	// D-04 + D15: emit the resolution via the EXISTING data-decision
	// chunk type — only the decision string is new.
	decisionEvent := decisionEventResumed
	if decision.Decision == "deny" {
		decisionEvent = decisionEventDenied
	}
	if err := runnerEmitDecision(c, wire.DecisionPayload{
		Step:       0,
		Decision:   decisionEvent,
		ToolCallID: approvalID,
		Query:      row.Query,
	}); err != nil {
		log.Printf("[WARN] server: approval decision chunk (sess=%s) failed: %v", sessionID, err)
	}
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
}

// approvalAlreadyResolved reports whether a resolution row
// (action_approved | action_rejected) exists for the session +
// toolCallID. Store errors are treated as "not resolved" — the
// caller's 404 path is the safe direction for a lookup failure on
// this idempotency probe.
func (s *Deps) approvalAlreadyResolved(ctx context.Context, sessionID, toolCallID string) bool {
	for _, action := range []string{auditActionApproved, auditActionRejected} {
		rows, err := s.Store.QueryAudit(ctx, store.AuditFilter{
			SessionID:  sessionID,
			Action:     action,
			ToolCallID: toolCallID,
			Limit:      1,
		})
		if err == nil && len(rows) > 0 {
			return true
		}
	}
	return false
}

// emitChunkClosure wraps wire.EmitChunk for agent.DrainRunnerEvents.
// SSE write failures propagate as the drain error (the drain tears
// down the Runner early rather than pumping into a closed socket).
func emitChunkClosure(c *app.RequestContext) agent.EmitFunc {
	return agent.EmitFunc(func(chunk wire.UIMessageChunk) error {
		return wire.EmitChunk(c, chunk)
	})
}
