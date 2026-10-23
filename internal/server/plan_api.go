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

// plan_api.go is Phase 12 / Plan 12-04's BE half of the planning-layer
// HTTP surface:
//
//	GET  /api/run/:id/plan               — PLAN-05 BE replay substrate.
//	    Returns the plan_todos audit rows for the run in ts-ASC order,
//	    the latest snapshot per (run_id, todo_id). The FE PlanReplayView
//	    (12-07) consumes this to render the run's plan evolution.
//
//	POST /api/run/:id/plan/:approvalId/approve — PLAN-03 BE confirm endpoint.
//	    Mirrors approval_api.go (Phase 11 11-04): validates
//	    ApprovalDecision (Target="plan_approve") → D11-C2 expire-check
//	    (skill_version + prompt_hash) → AuditWriter.MarkApproved /
//	    MarkRejected → Runner.ResumeWithParams → data-decision SSE
//	    chunk {decision: "plan-approved" | "plan-denied"}.
//
// D-04 + D15 invariant: NO new SSE chunk types are introduced. Only
// the data-decision payload's `decision` STRING enum is extended
// ({plan-approved, plan-denied}); the wire union (wire.ValidTypes)
// is untouched — mirrored from approval_api.go's posture.
//
// D8-04 invariant: this file does NOT call tool.StatefulInterrupt.
// The interrupt lives in internal/agent/hitl/hitl_plan_approve.go
// (12-03). planApproveHandler only validates the approval_id, writes
// the audit row, and triggers the Runner resume; it does not itself
// initiate the HITL Tool's interrupt path.
//
// Path note: the path uses /api/run/:id/plan (NOT /api/session/:id/plan)
// because a run is the unit of plan evolution, distinct from a session
// which can hold many runs. Mirrors Phase 11's separation between
// session-level approval endpoints (keyed by session_id) and the
// future run-level plan endpoints.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/wire"
)

// planTodoQuerier is the narrow store capability planGetHandler needs
// (the 12-02 QueryPlanTodos seam). Declared as a local interface (not
// promoted onto store.Store) because the row type
// sqlite.PlanTodoRow is owned by the sqlite package — promoting the
// method to the interface would require a store-owned row mirror and
// a signature change in sqlite/plan_todos.go (12-02's file). The
// runtime assertion mirrors the adk.CheckPointStore pattern used by
// planApproveHandler below.
type planTodoQuerier interface {
	QueryPlanTodos(ctx context.Context, runID string) ([]sqlite.PlanTodoRow, error)
}

// planDecisionEventApproved / planDecisionEventDenied are the two
// data-decision `decision` STRING values (D-04 string-enum extension;
// NO new SSE chunk types — D15). Mirrors approval_api.go's
// decisionEventResumed / decisionEventDenied pattern.
const (
	planDecisionEventApproved = "plan-approved"
	planDecisionEventDenied   = "plan-denied"
)

// planGetResponse is the GET /api/run/:id/plan wire shape. Mirrors
// wire.PlanTracePayload (12-02 PLAN-02 live trace) for FE consumption:
// the FE PlanReplayView (12-07) treats the replay response as a
// post-hoc PlanTracePayload. The Count field is added for FE-side
// affordance (e.g. "5 todos" badges).
type planGetResponse struct {
	RunID       string          `json:"runId"`
	Todos       []wire.PlanTodo `json:"todos"`
	GeneratedAt int64           `json:"generatedAt"`
	Count       int             `json:"count"`
}

// planApproveRequest is the POST body for /api/run/:id/plan/:approvalId/approve.
// Mirrors approval_api.go::approvalPostRequest but trimmed to the fields the
// plan_approve Tool emits (the FE passes the same ApprovalDecision shape it
// would send to /api/session/:id/approval/:approvalId). InterruptID is the
// optional eino resume-target UUID the FE echoes from the data-decision
// interrupted payload (see approval_api.go's file-level resume-target note:
// probe-tested on eino v0.9.14, the raw approvalID alone re-interrupts).
type planApproveRequest struct {
	Decision    string          `json:"decision"`
	Reason      string          `json:"reason,omitempty"`
	Target      string          `json:"target"`
	FrozenArgs  json.RawMessage `json:"frozen_args,omitempty"`
	Scope       string          `json:"scope,omitempty"`
	ApprovalID  string          `json:"approval_id"`
	InterruptID string          `json:"interrupt_id,omitempty"`
}

// planGetHandler implements GET /api/run/:id/plan (PLAN-05 BE replay
// substrate). Returns the run's plan_todos audit rows in ts-ASC order
// (the canonical store order — see QueryPlanTodos). Empty result → 200
// + empty list (existence-leak posture: a 404 would tell an attacker
// which run IDs are valid).
//
// Status codes:
//
//	200 — valid run (empty or populated).
//	400 — malformed runID regex.
//	404 — Deps.Store nil (defensive misconfiguration).
//	500 — store error.
func (s *Deps) planGetHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "plan store not initialized",
		})
		return
	}
	runID := c.Param("id")
	if !sessionIDRegex.MatchString(runID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid run id",
		})
		return
	}

	// T-3-SQLI: QueryPlanTodos is ?-parameterized (sqlite.PlanTodoRow
	// flows through db.QueryContext(ctx, queryPlanTodosSQL, runID)).
	q, ok := s.Store.(planTodoQuerier)
	if !ok {
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "plan replay requires a sqlite-backed store",
		})
		return
	}
	rows, err := q.QueryPlanTodos(ctx, runID)
	if err != nil {
		log.Printf("[WARN] server: plan get (run=%s) failed: %v", runID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "plan query failed",
		})
		return
	}

	// Translate the store rows into the wire.PlanTodo shape (12-02
	// lockstep invariant). The replay semantics are "latest snapshot
	// per (run_id, todo_id)" — INSERT OR REPLACE in PutPlanTodo
	// collapses same-key writes to a single row, so the loop here is
	// a 1:1 translation (no in-memory merge needed).
	todos := make([]wire.PlanTodo, 0, len(rows))
	var latestTs int64
	for _, r := range rows {
		todos = append(todos, wire.PlanTodo{
			ID:         r.TodoID,
			Content:    r.Content,
			ActiveForm: r.ActiveForm,
			Status:     r.Status,
		})
		if r.Ts.Unix() > latestTs {
			latestTs = r.Ts.Unix()
		}
	}

	c.JSON(http.StatusOK, planGetResponse{
		RunID:       runID,
		Todos:       todos,
		GeneratedAt: latestTs,
		Count:       len(todos),
	})
}

// planApproveHandler implements POST /api/run/:id/plan/:approvalId/approve
// (PLAN-03 BE confirm endpoint). Mirrors approval_api.go's
// approvalPostHandler with target="plan_approve" as the only
// functional difference. Step order:
//
//  1. Deps gate (Store + ChatModel + ActiveManifest).
//  2. URL segment regex validation (runID + approvalID).
//  3. JSON decode + body/URL ApprovalID cross-check.
//  4. ApprovalDecision.Validate (422, HITL-02 deny-reason gate) +
//     canonical-target spoof check.
//  5. Idempotency probe — a resolved approval returns 200
//     already_resolved (T-11-04-06 posture).
//  6. Pending-row lookup (404 cross-session moat) + row-target gate
//     (T-12-04-2: ONLY plan_approve rows are resolvable here).
//  7. D11-C2 expire-check (skill_version + prompt_hash); expired → 410 Gone.
//
// Per-row manifest lookup (mirrors approval_api.go:manifestForSkill):
// resolve the manifest by the pending row's skill_id rather than the
// global Deps.ActiveManifest (which main.go only wires from the first
// loaded Skill). A pending plan_approve row written against, e.g.,
// monthly-report must use monthly-report's manifest here or the
// expire-check will mark it stale.
//  8. AuditWriter.MarkApproved / MarkRejected (D11-C3 fail-closed;
//     HITL-07 edited FrozenArgs win over the pending row's bytes).
//  9. Runner.ResumeWithParams (interrupt-UUID target, approvalID
//     fallback) + SSE drain.
//  10. data-decision SSE chunk {decision: "plan-approved" | "plan-denied"}.
//
// Status codes:
//
//	200 — approval resolved (SSE stream carries the drained resume
//	      events + the data-decision chunk).
//	400 — malformed runID / approvalID / JSON body / approvalID mismatch.
//	404 — Deps misconfiguration OR no active Skill (cross-session moat).
//	410 — D11-C2 expire-check mismatch; checkpoint deleted, audit
//	      chain preserved.
//	422 — ApprovalDecision.Validate failure.
//	500 — store / audit-writer / runner error.
//
// D8-04 invariant: this handler does NOT call tool.StatefulInterrupt.
// The interrupt is owned by hitl/hitl_plan_approve.go (12-03); this
// handler validates the resume target and triggers the Runner, which
// in turn resumes the Tool's Pass 2 path.
func (s *Deps) planApproveHandler(ctx context.Context, c *app.RequestContext) {
	// Phase 52: 委托式 ChatModel 的 stub 语义由 ChatModelAvailable 判定。
	if s.Store == nil || !ChatModelAvailable(s.ChatModel) || s.ActiveManifest == nil || s.Embedder == nil || s.Retriever == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "plan approval endpoint not initialized",
		})
		return
	}
	runID := c.Param("id")
	if !sessionIDRegex.MatchString(runID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid run id",
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

	var req planApproveRequest
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
	// the audit writer or the Runner (mirrors approvalPostHandler).
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

	// Target spoof check: the FE-claimed target MUST match the plan
	// approval's canonical target ("plan_approve"). This is a
	// defense-in-depth over ApprovalDecision.TargetMatches (which
	// allows an empty claim); here the plan_approve target is a known
	// constant and a mismatch is always a spoof signal.
	if decision.Target != "" && decision.Target != hitl.PlanApproveTarget {
		c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"field": "target",
			"msg":   "must be plan_approve for plan approval endpoint",
		})
		return
	}
	// Force the canonical target for downstream audit rows.
	decision.Target = hitl.PlanApproveTarget

	// Idempotency (mirrors approvalPostHandler's T-11-04-06 posture):
	// the audit chain is INSERT-only, so the pending row never
	// disappears — "already resolved" is detectable ONLY by the
	// presence of a resolution row. A second POST for a resolved plan
	// approval returns 200 already_resolved rather than re-executing
	// the write + resume.
	if s.approvalAlreadyResolved(ctx, runID, approvalID) {
		c.JSON(http.StatusOK, map[string]string{
			"state": "already_resolved",
		})
		return
	}

	// Pending-row lookup (T-3-SQLI ?-parameterized). Wrong run → no
	// rows → 404 (the cross-session authorization moat, mirrors
	// approvalPostHandler's T-11-04-02 posture).
	//
	// The row-target gate below is the T-12-04-2
	// elevation-of-privilege mitigation: the plan endpoint may ONLY
	// resolve plan_approve rows. Without it, a POST carrying a
	// memory:facts / review_report pending approval's tool_call_id
	// could resolve THAT approval through /plan/ — the audit row's
	// target column is the single source of truth for which endpoint
	// owns the resolution.
	pending, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID:  runID,
		Action:     auditActionPending,
		ToolCallID: approvalID,
		Limit:      1,
	})
	if err != nil {
		log.Printf("[WARN] server: plan pending lookup (run=%s id=%s) failed: %v", runID, approvalID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "plan approval lookup failed",
		})
		return
	}
	if len(pending) == 0 {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "no pending plan approval for this run",
		})
		return
	}
	row := pending[0]
	if row.Query != hitl.PlanApproveTarget {
		c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"field": "target",
			"msg":   "pending approval is not a plan_approve target",
		})
		return
	}

	// D11-C2 EXPIRE-CHECK — before any audit resolution row is
	// written. A skill_version / prompt_hash mismatch (Skill
	// hot-reload since the interrupt) expires the checkpoint: Delete
	// the CHECKPOINT row (never the audit row — T-3-AUDIT INSERT-only)
	// and answer 410 Gone so the FE collapses the PlanApprovalCard
	// into the expired state.
	//
	// Per-row manifest lookup (manifestForSkill in approval_api.go):
	// the pending row carries the skill_id that produced the
	// interrupt. Resolve the manifest from the loader so the
	// expire-check matches the row's actual skill, not the boot-time
	// global Deps.ActiveManifest. Snapshot fallback chain: per-row
	// manifest → global ActiveManifest (mirrors approval_api.go —
	// tests construct Deps without a Loader; production wires both).
	cpID := agent.RunnerCheckPointID(runID)
	rowManifest := manifestForSkill(s.Loader, row.SkillID)
	if rowManifest == nil {
		rowManifest = s.ActiveManifest
	}
	snap := activeSkillSnapshot(rowManifest)
	_, checkpointValid, err := s.Store.GetCheckpointByVersionFilter(ctx, cpID, snap.Version, snap.PromptHash)
	if err != nil {
		log.Printf("[WARN] server: plan expire-check (run=%s) failed: %v", runID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "plan expire-check failed",
		})
		return
	}
	if !checkpointValid {
		// No checkpoint row at all (existed=false) → treat as fresh
		// state, NOT expired. The D11-C2 expire-check is meant to
		// detect hot-reload drift; an absent checkpoint is the normal
		// case for runs whose Runner didn't write one (eino's
		// adk.CheckPointStore behavior under certain interrupt shapes
		// — plan_approve in particular). Mark expired ONLY on a real
		// hash mismatch between the audit row's stored snapshot and
		// the live recomputed snapshot. This mirrors
		// approvalPostHandler (D11-C2 fix landed 2026-09-08) — prior
		// to that fix plan_api treated every missing checkpoint as
		// expired and PlanApprovalCard always 410'd on resume.
		if snap.PromptHash != "" && row.PromptHash != snap.PromptHash {
			log.Printf("[INFO] server: plan expire-check (run=%s cp=%s) audit.prompt_hash=%s != snap.prompt_hash=%s → expired",
				runID, cpID, row.PromptHash, snap.PromptHash)
			if dErr := s.Store.Delete(ctx, cpID); dErr != nil {
				log.Printf("[WARN] server: plan expire-check delete (run=%s cp=%s) failed: %v", runID, cpID, dErr)
			}
			c.JSON(http.StatusGone, map[string]any{
				"state":                 approvalStateExpired,
				"audit_chain_preserved": true,
			})
			return
		}
		log.Printf("[INFO] server: plan expire-check (run=%s cp=%s) no checkpoint row + audit snapshot matches → fresh state, proceeding",
			runID, cpID)
	}

	// Audit resolution row (D11-C3 fail-closed writer). Mirrors
	// approvalPostHandler exactly with target=plan_approve. HITL-07
	// edit-before-approve: the operator's edited FrozenArgs win;
	// otherwise the pending row's byte snapshot is carried verbatim.
	frozenArgs := decision.FrozenArgs
	if len(frozenArgs) == 0 {
		frozenArgs = row.FrozenArgs
	}
	writer := s.auditWriterOrDefault()
	// CR-03 (18-REVIEW): three-way audit resolution (mirrors
	// approvalPostHandler) — deny → rejected, request_change → the
	// D18-D4 change_requested row, everything else → approved.
	switch decision.Decision {
	case "deny":
		if _, err := writer.MarkRejected(ctx, runID, "", snap, approvalID, approvalID, hitl.PlanApproveTarget, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: plan MarkRejected (run=%s id=%s) failed: %v", runID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "audit write failed",
			})
			return
		}
	case "request_change":
		if _, err := writer.MarkChangeRequested(ctx, runID, "", snap, approvalID, approvalID, hitl.PlanApproveTarget, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: plan MarkChangeRequested (run=%s id=%s) failed: %v", runID, approvalID, err)
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "audit write failed",
			})
			return
		}
	default:
		if _, err := writer.MarkApproved(ctx, runID, "", snap, approvalID, approvalID, hitl.PlanApproveTarget, frozenArgs, decision.Reason); err != nil {
			log.Printf("[ERROR] server: plan MarkApproved (run=%s id=%s) failed: %v", runID, approvalID, err)
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
	// Resolve the runner manifest: prefer the row's actual skill, fall
	// back to Deps.ActiveManifest when loader lookup misses (defensive
	// — BuildRunner requires a non-nil Manifest).
	resumeManifest := rowManifest
	if resumeManifest == nil {
		resumeManifest = s.ActiveManifest
	}

	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        resumeManifest,
		Embedder:        s.Embedder,
		Retriever:       s.Retriever,
		ChatModel:       s.ChatModel,
		SessionID:       runID,
		SkillID:         row.SkillID,
		CheckPointStore: cpStore,
		// D16-C3: a data-channel Skill's resume path needs the SAME
		// executor + registry the chat path passes (runner_setup.go
		// ChatHandler) — BuildRunner fail-closes without them, which
		// would break every monthly-report / kpi-dashboard resume.
		// Mirrors approvalPostHandler (D16-C3 fix landed 2026-09-08);
		// prior to the mirror plan_api.go returned 500 "runner build
		// failed" on every data-channel resume.
		KPIExecutor: s.KPIExecutor,
		KPIRegistry: s.KPIRegistry,
		// P12-L2 deep-archetype write_todos substrate (same seams the
		// chat path passes — the resumed run continues the SAME agent
		// tree and its write_todos Tool must keep INSERTing).
		PlanTodoStore: planTodoStoreOf(s.Store),
		PlanTraceEmit: s.planTraceEmitter(c),
		// Plan 26-06: the hitl tool construction fails closed without a
		// registry when the manifest declares memory:facts — thread the
		// same registry the annotate/KB admin surfaces already use.
		KBRegistry: s.KBRegistry,
		// WR-01: same session-bound audit wiring as the approval
		// endpoint (fresh writer — the shared Deps.AuditWriter is never
		// identity-mutated).
		AuditWriter: hitl.NewAuditWriter(s.Store, agent.PlatformFloorVersion).
			WithIdentity(runID, "", snap),
		ReportAudit:       s.Store,
		ReportPublishEmit: s.reportPublishEmitter(c),
	})
	if err != nil {
		log.Printf("[ERROR] server: plan BuildRunner (run=%s) failed: %v", runID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "runner build failed",
		})
		return
	}

	// Resume. The Targets key is the interrupt UUID echoed by the FE
	// (see approval_api.go's file-level resume-target note — eino
	// v0.9.14 probe-tested that the raw tool_call_id re-interrupts);
	// the approvalID is the D11-C4 plan-literal fallback.
	resumeTarget := req.InterruptID
	if resumeTarget == "" {
		resumeTarget = approvalID
	}
	iter, err := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
		Targets: map[string]any{resumeTarget: decision},
	})
	if err != nil {
		log.Printf("[ERROR] server: plan ResumeWithParams (run=%s cp=%s) failed: %v", runID, cpID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("resume failed: %v", err),
		})
		return
	}

	// Drain the resumed run into the SSE stream.
	//
	// Re-interrupt self-heal (mirrors approvalPostHandler's pattern,
	// landed 2026-09-08 via commit 1135c99): when the resume target
	// missed (the restored-pending-row path has no interrupt_id — the
	// FE POST fell back to the approvalID, which eino does not match),
	// the HITL Tool re-interrupts (leaf-contract preservation) and the
	// drain sees a data-decision {decision: "interrupted"} chunk
	// carrying the NEW interrupt UUID (runner_sse.go emitInterruptChunks).
	// The wrapper captures it; after the first drain closes we retry
	// the resume ONCE with that UUID, which deterministically hits the
	// Tool's GetResumeContext and unblocks the approved execution
	// (P12-L2 deep-agent's 4-step plan continues: KPI calls → render →
	// report_publish → audit). Without this self-heal the resumed
	// Runner emits plan-approved and exits without ever executing the
	// plan's 4 steps — the operator sees an empty completed state.
	var reInterruptID string
	emit := func(chunk wire.UIMessageChunk) error {
		if reInterruptID == "" && chunk.Type == "data-decision" && len(chunk.Data) > 0 {
			var p struct {
				Decision    string `json:"decision"`
				InterruptID string `json:"interruptId"`
			}
			if json.Unmarshal(chunk.Data, &p) == nil && p.Decision == "interrupted" && p.InterruptID != "" {
				reInterruptID = p.InterruptID
				log.Printf("[INFO] server: plan resume (run=%s) target miss → Tool re-interrupted (new interruptId=%s); will self-heal with one retry",
					runID, p.InterruptID)
			}
		}
		return emitChunkClosure(c)(chunk)
	}
	if err := agent.DrainRunnerEvents(ctx, iter, emit); err != nil {
		log.Printf("[ERROR] server: plan drain (run=%s) failed: %v", runID, err)
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
		return
	}
	if reInterruptID != "" && reInterruptID != resumeTarget {
		// The re-interrupt persisted a NEW checkpoint (adk/runner.go:330);
		// resuming against the same cpID with the captured UUID resolves
		// the Tool's Pass-2. Bounded to ONE retry — a second miss means a
		// non-target interrupt shape and the operator sees the new card.
		iter2, err2 := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
			Targets: map[string]any{reInterruptID: decision},
		})
		if err2 != nil {
			log.Printf("[WARN] server: plan self-heal resume (run=%s id=%s) failed: %v", runID, reInterruptID, err2)
		} else if err := agent.DrainRunnerEvents(ctx, iter2, emit); err != nil {
			log.Printf("[WARN] server: plan self-heal drain (run=%s) failed: %v", runID, err)
		}
	}

	// D-04 + D15: emit the resolution via the EXISTING data-decision
	// chunk type — only the decision string is new (plan-approved /
	// plan-denied / D18-D4 plan_change_requested).
	decisionEvent := planDecisionEventApproved
	switch decision.Decision {
	case "deny":
		decisionEvent = planDecisionEventDenied
	case "request_change":
		decisionEvent = wire.DecisionPlanChangeRequested
	}
	if err := runnerEmitDecision(c, wire.DecisionPayload{
		Step:       0,
		Decision:   decisionEvent,
		ToolCallID: approvalID,
		Query:      hitl.PlanApproveTarget,
	}); err != nil {
		log.Printf("[WARN] server: plan decision chunk (run=%s) failed: %v", runID, err)
	}
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
}
