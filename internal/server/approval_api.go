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
	"fmt"
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

	// auditActionChangeRequested is the D18-D4 third-state resolution
	// (要求修改) — the operator sends the draft back to the LLM with a
	// change reason. Resolves the pending row for idempotency exactly
	// like approved/rejected.
	auditActionChangeRequested = "action_change_requested"

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

// manifestForSkill resolves the *skill.Manifest for a skill_id by name
// from the loaded Skill registry. Used by approvalPostHandler (and
// planApproveHandler) to derive a per-row D11-C2 snapshot from the
// pending audit row's skill_id — rather than from the global
// Deps.ActiveManifest (which main.go only wires from the first loaded
// Skill, so pending rows for any other skill would always fail the
// expire-check). Returns nil when the loader is absent or the skill_id
// is not loaded — caller treats nil the same way as activeSkillSnapshot
// (zero-value snapshot → checkpoint filter returns "not found" →
// approve path falls through to fresh retrieval).
func manifestForSkill(loader *skill.Loader, skillID string) *skill.Manifest {
	if loader == nil || skillID == "" {
		return nil
	}
	for _, m := range loader.Skills() {
		if m != nil && m.Name == skillID {
			return m
		}
	}
	return nil
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
	return hitl.NewAuditWriter(s.Store, agent.PlatformFloorVersion)
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

	// Resolved set: tool_call_ids with an action_approved,
	// action_rejected, or (D18-D4) action_change_requested row
	// (operator decisions AND D11-D1 rule auto-passes — the auto-pass
	// row is an action_approved row with
	// refusal_reason="auto_passed_rule").
	resolved := map[string]bool{}
	for _, action := range []string{auditActionApproved, auditActionRejected, auditActionChangeRequested} {
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
	// Phase 52: ChatModel 是委托式包装（热重载），stub 语义改由
	// ChatModelAvailable 判定（upstream 槽实时状态），nil 判断保留兜底。
	if s.Store == nil || !ChatModelAvailable(s.ChatModel) || s.ActiveManifest == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "approval endpoint not initialized",
		})
		return
	}
	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid session id"})
		return
	}
	approvalID := c.Param("approvalId")
	if !sessionIDRegex.MatchString(approvalID) {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid approval id"})
		return
	}

	// decodeApprovalRequest: JSON decode + approvalID reconciliation + Validate
	decision, err := s.decodeApprovalRequest(ctx, c, sessionID, approvalID)
	if err != nil {
		return // error response already written by decodeApprovalRequest
	}

	// validateAndResolveApproval: idempotency + pending-row lookup + spoof check + expire-check
	row, err := s.validateAndResolveApproval(ctx, c, sessionID, approvalID, decision)
	if err != nil {
		return // error response already written by validateAndResolveApproval
	}

	// executeApprovalResume: audit resolution + Runner rebuild + ResumeWithParams
	iter, err := s.executeApprovalResume(ctx, c, sessionID, approvalID, decision, row)
	if err != nil {
		return // error response already written by executeApprovalResume
	}

	// drainResumeEvents: drain SSE + re-interrupt self-heal + decision chunk emit
	s.drainResumeEvents(ctx, c, sessionID, approvalID, decision, row, iter)
}

// decodeApprovalRequest decodes the POST body, reconciles the approvalID
// field, and validates the decision. Returns the validated decision or an
// error with a response already written to c.
func (s *Deps) decodeApprovalRequest(ctx context.Context, c *app.RequestContext, sessionID, approvalID string) (*hitl.ApprovalDecision, error) {
	var req approvalPostRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return nil, fmt.Errorf("invalid JSON")
	}
	if req.ApprovalID == "" {
		req.ApprovalID = approvalID
	} else if req.ApprovalID != approvalID {
		c.JSON(http.StatusBadRequest, map[string]string{
			"field": "approval_id",
			"msg":   "body approval_id does not match the URL segment",
		})
		return nil, fmt.Errorf("approval_id mismatch")
	}

	// HITL-02 / D11-B2: Validate FIRST (422 on failure).
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
		return nil, fmt.Errorf("validation failed")
	}
	return decision, nil
}

// validateAndResolveApproval checks idempotency, looks up the pending row,
// performs the spoof check, and runs the D11-C2 expire-check. Returns
// the pending row or an error with a response already written to c.
func (s *Deps) validateAndResolveApproval(ctx context.Context, c *app.RequestContext, sessionID, approvalID string, decision *hitl.ApprovalDecision) (*store.AuditRow, error) {
	// Idempotency (T-11-04-06): INSERT-only audit chain means a resolution
	// row is the only signal that this approval is already handled.
	if s.approvalAlreadyResolved(ctx, sessionID, approvalID) {
		c.JSON(http.StatusOK, map[string]string{"state": "already_resolved"})
		return nil, fmt.Errorf("already resolved")
	}

	// Pending-row lookup (T-3-SQLI ?-parameterized). Wrong session → no rows → 404.
	pending, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     auditActionPending,
		ToolCallID: approvalID,
		Limit:      1,
	})
	if err != nil {
		log.Printf("[WARN] server: approval pending lookup (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "approval lookup failed"})
		return nil, fmt.Errorf("lookup failed")
	}
	if len(pending) == 0 {
		c.JSON(http.StatusNotFound, map[string]string{"error": "no pending approval for this session"})
		return nil, fmt.Errorf("no pending row")
	}
	row := &pending[0]

	// Spoof check: non-empty FE target must match the audit row's target.
	if !decision.TargetMatches(row.Query) {
		c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"field": "target",
			"msg":   "does not match the pending approval's target",
		})
		return nil, fmt.Errorf("spoof check failed")
	}

	// D11-C2 EXPIRE-CHECK: skill_version / prompt_hash mismatch → 410 Gone.
	cpID := agent.RunnerCheckPointID(sessionID)
	rowManifest := manifestForSkill(s.Loader, row.SkillID)
	if rowManifest == nil {
		rowManifest = s.ActiveManifest
	}
	snap := activeSkillSnapshot(rowManifest)
	_, checkpointValid, err := s.Store.GetCheckpointByVersionFilter(ctx, cpID, snap.Version, snap.PromptHash)
	if err != nil {
		log.Printf("[WARN] server: approval expire-check (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "approval expire-check failed"})
		return nil, fmt.Errorf("expire-check failed")
	}
	if !checkpointValid && snap.PromptHash != "" && row.PromptHash != snap.PromptHash {
		log.Printf("[INFO] server: approval expire-check (sess=%s cp=%s) audit.prompt_hash=%s != snap.prompt_hash=%s → expired",
			sessionID, cpID, row.PromptHash, snap.PromptHash)
		c.JSON(http.StatusGone, map[string]any{
			"state":                 approvalStateExpired,
			"audit_chain_preserved": true,
		})
		return nil, fmt.Errorf("expired")
	}
	log.Printf("[INFO] server: approval expire-check (sess=%s cp=%s) fresh state, proceeding", sessionID, cpID)
	return row, nil
}

// executeApprovalResume writes the audit resolution row, rebuilds the
// Runner, and calls ResumeWithParams. Returns the iterator or an error
// with a response already written to c.
func (s *Deps) executeApprovalResume(ctx context.Context, c *app.RequestContext, sessionID, approvalID string, decision *hitl.ApprovalDecision, row *store.AuditRow) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	// Audit resolution row (D11-C3 fail-closed writer).
	frozenArgs := decision.FrozenArgs
	if len(frozenArgs) == 0 {
		frozenArgs = row.FrozenArgs
	}
	writer := s.auditWriterOrDefault()
	// Per-row manifest + D11-C2 snapshot resolved ONCE with the SAME
	// fallback chain the expire-check above used (per-row manifest →
	// global ActiveManifest — tests construct Deps without a Loader;
	// production wires both). Without the fallback a Loader-less Deps
	// (or an unregistered row.SkillID) yields a ZERO snapshot and the
	// fail-closed writer refuses the resolution row with "skill
	// snapshot required (D11-C2)" — a 500 masked as "audit write
	// failed" right after this same request deemed the row fresh.
	// Regression from 27db382 (refactor 33-04); mirrors plan_api.go.
	resumeManifest := manifestForSkill(s.Loader, row.SkillID)
	if resumeManifest == nil {
		resumeManifest = s.ActiveManifest
	}
	snap := activeSkillSnapshot(resumeManifest)

	switch decision.Decision {
	case "deny":
		if _, err := writer.MarkRejected(ctx, sessionID, "", snap, approvalID, approvalID, row.Query, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: approval MarkRejected (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "audit write failed"})
			return nil, fmt.Errorf("audit write failed")
		}
	case "request_change":
		if _, err := writer.MarkChangeRequested(ctx, sessionID, "", snap, approvalID, approvalID, row.Query, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: approval MarkChangeRequested (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "audit write failed"})
			return nil, fmt.Errorf("audit write failed")
		}
	default:
		if _, err := writer.MarkApproved(ctx, sessionID, "", snap, approvalID, approvalID, row.Query, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: approval MarkApproved (sess=%s id=%s) failed: %v", sessionID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "audit write failed"})
			return nil, fmt.Errorf("audit write failed")
		}
	}

	// Rebuild Runner (D11-B4): resume is a NEW Runner pointing at the SAME
	// checkPointID. Must use the pending row's skill manifest for correctness.
	cpStore, ok := s.Store.(adk.CheckPointStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "store does not support checkpoints"})
		return nil, fmt.Errorf("checkpoint store unavailable")
	}
	cpID := agent.RunnerCheckPointID(sessionID)

	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        resumeManifest,
		Embedder:        s.Embedder,
		Retriever:       s.Retriever,
		ChatModel:       s.ChatModel,
		SessionID:       sessionID,
		SkillID:         row.SkillID,
		CheckPointStore: cpStore,
		KPIExecutor:     s.KPIExecutor,
		KPIRegistry:     s.KPIRegistry,
		PlanTodoStore:   planTodoStoreOf(s.Store),
		PlanTraceEmit:   s.planTraceEmitter(c),
		KBRegistry:      s.KBRegistry,
		AuditWriter: hitl.NewAuditWriter(s.Store, agent.PlatformFloorVersion).
			WithIdentity(sessionID, "", snap),
		ReportAudit:       s.Store,
		ReportPublishEmit: s.reportPublishEmitter(c),
	})
	if err != nil {
		log.Printf("[ERROR] server: approval BuildRunner (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "runner build failed"})
		return nil, fmt.Errorf("runner build failed")
	}

	// ResumeWithParams: Targets key is interrupt UUID (FE-echoed) or approvalID fallback.
	resumeTarget := decision.ApprovalID
	iter, err := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
		Targets: map[string]any{resumeTarget: decision},
	})
	if err != nil {
		log.Printf("[ERROR] server: approval ResumeWithParams (sess=%s cp=%s) failed: %v", sessionID, cpID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "resume failed"})
		return nil, fmt.Errorf("resume failed")
	}
	return iter, nil
}

// drainResumeEvents drains the resumed run into the SSE stream, handles
// the re-interrupt self-heal, and emits the final decision chunk.
func (s *Deps) drainResumeEvents(ctx context.Context, c *app.RequestContext, sessionID, approvalID string, decision *hitl.ApprovalDecision, row *store.AuditRow, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	cpID := agent.RunnerCheckPointID(sessionID)

	// Re-interrupt self-heal: captures new interrupt UUID from a target-miss
	// re-interrupt, then retries once with that UUID.
	var reInterruptID string
	emit := func(chunk wire.UIMessageChunk) error {
		if reInterruptID == "" && chunk.Type == "data-decision" && len(chunk.Data) > 0 {
			var p struct {
				Decision    string `json:"decision"`
				InterruptID string `json:"interruptId"`
			}
			if json.Unmarshal(chunk.Data, &p) == nil && p.Decision == "interrupted" && p.InterruptID != "" {
				reInterruptID = p.InterruptID
				log.Printf("[INFO] server: approval resume (sess=%s) target miss → Tool re-interrupted (new interruptId=%s); will self-heal with one retry",
					sessionID, p.InterruptID)
			}
		}
		return emitChunkClosure(c)(chunk)
	}

	// DrainRunnerEvents: D11-A4 <200ms cancel lock applies here too.
	if err := agent.DrainRunnerEvents(ctx, iter, emit); err != nil {
		log.Printf("[ERROR] server: approval drain (sess=%s) failed: %v", sessionID, err)
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
		return
	}

	// Self-heal retry: bounded to ONE retry — a second miss means a non-target
	// interrupt shape and the operator sees the new card.
	if reInterruptID != "" && reInterruptID != decision.ApprovalID {
		// Re-build runner to get a fresh iterator (original iter is exhausted).
		cpStore, _ := s.Store.(adk.CheckPointStore)
		resumeManifest := manifestForSkill(s.Loader, row.SkillID)
		if resumeManifest == nil {
			resumeManifest = s.ActiveManifest
		}
		snap := activeSkillSnapshot(resumeManifest)
		runner, _ := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
			Manifest:        resumeManifest,
			Embedder:        s.Embedder,
			Retriever:       s.Retriever,
			ChatModel:       s.ChatModel,
			SessionID:       sessionID,
			SkillID:         row.SkillID,
			CheckPointStore: cpStore,
			KPIExecutor:     s.KPIExecutor,
			KPIRegistry:     s.KPIRegistry,
			PlanTodoStore:   planTodoStoreOf(s.Store),
			PlanTraceEmit:   s.planTraceEmitter(c),
			KBRegistry:      s.KBRegistry,
			AuditWriter: hitl.NewAuditWriter(s.Store, agent.PlatformFloorVersion).
				WithIdentity(sessionID, "", snap),
			ReportAudit:       s.Store,
			ReportPublishEmit: s.reportPublishEmitter(c),
		})
		iter2, err2 := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
			Targets: map[string]any{reInterruptID: decision},
		})
		if err2 != nil {
			log.Printf("[WARN] server: approval self-heal resume (sess=%s id=%s) failed: %v", sessionID, reInterruptID, err2)
		} else if err := agent.DrainRunnerEvents(ctx, iter2, emit); err != nil {
			log.Printf("[WARN] server: approval self-heal drain (sess=%s) failed: %v", sessionID, err)
		}
	}

	// Emit the resolution via the EXISTING data-decision chunk type (D-04 + D15).
	decisionEvent := decisionEventResumed
	switch decision.Decision {
	case "deny":
		decisionEvent = decisionEventDenied
	case "request_change":
		decisionEvent = wire.DecisionPlanChangeRequested
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
// (action_approved | action_rejected | action_change_requested)
// exists for the session + toolCallID. Store errors are treated as
// "not resolved" — the caller's 404 path is the safe direction for a
// lookup failure on this idempotency probe.
func (s *Deps) approvalAlreadyResolved(ctx context.Context, sessionID, toolCallID string) bool {
	for _, action := range []string{auditActionApproved, auditActionRejected, auditActionChangeRequested} {
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
