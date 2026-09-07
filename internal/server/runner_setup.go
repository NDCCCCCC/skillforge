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

// Package server — runner_setup.go is Phase 11 / Plan 11-01's
// Runner-path chatHandler implementation. It is the D11-A1
// one-line-dispatch substrate that the production chatHandler will
// migrate to in Plan 11-04 (when the trust staircase + ChunkAccumulator
// + HITL Tool integration land).
//
// 11-01 lands the seam only: the new ChatHandler (this file) drives
// adk.NewRunner().Run(ctx, msgs) one-line dispatch, drains the
// AgentEvent iterator via agent.DrainRunnerEvents, and emits the
// same SSE chunk sequence the legacy chatHandler emits (D-04
// byte-identity). The route binding h.POST(\"/api/chat\", deps.chatHandler)
// stays on the legacy VerdictPipeline path (internal/server/chat.go)
// for 11-01 so existing Phase 8/9/10 acceptance tests keep passing;
// 11-04 flips the binding to the new ChatHandler.
//
// D11-A4 (<200ms cancel propagation): the same ctx flows from
// chatHandler → adk.Runner.Run AND through the agent.DrainRunnerEvents
// select-before-Next loop AND the SSE writer. Cancellation tears
// down all three sides within the same iteration (the Eino iterator
// does NOT poll ctx cooperatively inside Next(), so the select is
// the load-bearing anchor for the SC-8 / AGNT-04 200ms lock).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/report"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// ChatHandler is the Runner-path /api/chat handler (Plan 11-01 /
// D11-A1 + D11-A4). It is the new chatHandler body — the legacy one
// in internal/server/chat.go stays in place as the active binding
// for 11-01 (so existing tests keep passing). Plan 11-04 flips
// server.New's h.POST(\"/api/chat\", ...) to deps.ChatHandler.
//
// Behavior:
//
//  1. Decode body (chatRequest struct from internal/server/chat.go).
//  2. Resolve session_id (X-Session-Id header OR newSessionID).
//  3. Find the manifest via Loader (existing findSkill).
//  4. Build Runner via agent.BuildRunner(ctx, RunnerBuildConfig{...}).
//  5. Single-line D11-A1 dispatch:
//     iter, err := runner.Run(ctx, []ms)
//  6. Drain AgentEvents via agent.DrainRunnerEvents(ctx, iter, emit).
//
// The ctx is shared between Runner.Run, DrainRunnerEvents, and the SSE
// writer — cancellation tears all three down within the same iteration
// (D11-A4 <200ms lock).
//
// D-04 / D15 invariants preserved: the SSE chunk sequence is byte-
// identical to the legacy path (eventToUIMessageChunk emits the same
// text-start / text-delta / text-end / finish-step / finish types the
// legacy emitVerdictStream / emitRefusal / emitDecision emit). No new
// SSE chunk types are introduced (D15).
//
// D11-A2 trust-staircase (D11-A2): the BeforeAgent / BeforeModel /
// AfterModel callbacks are registered via cfg.Callbacks (11-04 wires
// the trust floor + classifier + grounding). 11-01 ships the empty
// default (the chatHandler boundary still runs the trust floor /
// classifier / grounding checks inline at their existing positions).
func (s *Deps) ChatHandler(ctx context.Context, c *app.RequestContext) {
	// UTF-8 sanity check (mirrors internal/server/chat.go:150).
	if raw := c.Request.Body(); len(raw) > 0 && !utf8.Valid(raw) {
		c.SetStatusCode(400)
		_ = wire.EmitChunk(c, errorChunk("request body must be valid UTF-8 (client likely sent a legacy codepage such as GBK/Shift-JIS)"))
		return
	}

	var chatReq chatRequest
	if err := c.BindAndValidate(&chatReq); err != nil {
		_ = emitRefusal(c, verdict.Refusal{Reason: verdict.RefusalReasonOutOfScope, CanRephrase: false})
		return
	}
	if chatReq.Query == "" {
		chatReq.Query = lastUserQuery(chatReq.Messages)
	}
	if chatReq.Query == "" {
		c.SetStatusCode(400)
		_ = wire.EmitChunk(c, errorChunk("query is required"))
		return
	}
	if chatReq.Skill == "" {
		chatReq.Skill = defaultSkill
	}

	sessionID := string(c.Request.Header.Peek("X-Session-Id"))
	if sessionID == "" {
		sessionID = newSessionID()
	}
	c.Response.Header.SetBytesV("X-Session-Id", []byte(sessionID))

	manifest := s.findSkill(chatReq.Skill)
	if manifest == nil {
		_ = emitRefusal(c, verdict.Refusal{Reason: verdict.RefusalReasonOutOfScope, CanRephrase: false})
		return
	}

	// Build the Runner (D11-A1 + D11-B1 + D11-B4 substrate).
	// 11-04 binds the SQLiteStore via adk.Runner.WithCheckPointStore
	// (the seam exists in internal/agent/runner_setup.go but the
	// chatHandler-bound store attachment lands in 11-04 to keep this
	// plan's blast radius small).
	//
	// Phase 16 / Plan 16-08: the data-channel dependencies ride along.
	// KPIExecutor + KPIRegistry are nil unless cmd/server wired the KPI
	// pipeline; BuildRunner is fail-closed for data-channel Skills when
	// they are nil (D16-C3), and tier-channel Skills ignore them.
	//
	// WR-01 (18-REVIEW): a per-request, session-bound AuditWriter is
	// injected so the registry HITL handlers persist REAL
	// action_pending_approval rows (the shared Deps.AuditWriter is NOT
	// mutated — WithIdentity is stateful and concurrent chat requests
	// would race it; a fresh writer per request is a cheap struct).
	// The store also rides as the CR-04 reports audit sink and — FAN-05
	// (18-VERIFICATION GAP-1) — as the data-channel fan-out
	// classifier-decision audit sink.
	var auditWriter *hitl.AuditWriter
	var reportAudit report.ReportAuditSink
	var fanOutAudit agent.FanOutAuditSink
	if s.Store != nil {
		auditWriter = hitl.NewAuditWriter(s.Store, agent.PlatformFloorVersion).
			WithIdentity(sessionID, "", activeSkillSnapshot(manifest))
		reportAudit = s.Store
		fanOutAudit = &fanOutAuditAdapter{
			store:     s.Store,
			sessionID: sessionID,
			skillID:   chatReq.Skill,
			floorVer:  agent.PlatformFloorVersion,
		}
	}
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:    manifest,
		Embedder:    s.Embedder,
		Retriever:   s.Retriever,
		ChatModel:   s.ChatModel,
		SessionID:   sessionID,
		KPIExecutor: s.KPIExecutor,
		KPIRegistry: s.KPIRegistry,
		// CR-03 (26-REVIEW): thread the same registry approval_api and
		// plan_api already pass. Without it a tier-channel Skill that
		// declares memory:facts fails hard here ("kb registry required")
		// and the accumulator's citable gate stays inert. Locked by
		// TestChatHandlerBuildRunnerWiresKBRegistry.
		KBRegistry: s.KBRegistry,
		// v1.1.1 (Phase 21 wiring): nil unless EXTERNAL_AUTH_ENABLED=1
		// (6c-5); only an external:authorized Skill consumes it and
		// BuildRunner fail-fasts if the Skill declares the channel while
		// the server did not wire it.
		ExternalAuthManager: s.ExternalAuthManager,
		DataRefEmit:         s.dataRefEmitter(c),
		AuditWriter:         auditWriter,
		ReportAudit:         reportAudit,
		ReportPublishEmit:   s.reportPublishEmitter(c),
		FanOutAudit:         fanOutAudit,
	})
	if err != nil {
		log.Printf("[ERROR] server: ChatHandler BuildRunner failed: %v", err)
		_ = wire.EmitChunk(c, errorChunk(err.Error()))
		return
	}

	// D11-A1 one-line dispatch.
	msgs := []*schema.Message{schema.UserMessage(chatReq.Query)}
	iter := runner.Run(ctx, msgs)
	// Note: runner.Run returns a *AsyncIterator synchronously; errors
	// surface as the first event's Err field (verified at eino v0.9.14
	// adk/runner.go:102-105). The legacy Runner.Query helper also
	// returns just the iterator; the chatHandler boundary drains the
	// first error event in the loop below.
	_ = adk.AgentInput{}

	// D11-A4 drain — ctx-shared with the SSE writer.
	emit := agent.EmitFunc(func(chunk wire.UIMessageChunk) error {
		return wire.EmitChunk(c, chunk)
	})
	if err := agent.DrainRunnerEvents(ctx, iter, emit); err != nil {
		log.Printf("[ERROR] server: ChatHandler DrainRunnerEvents failed: %v", err)
		// The SSE stream is already in flight — we cannot change status
		// here. Emit an error chunk + finish-step so the client sees a
		// terminal state, then persist the audit row.
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
		s.writeAuditAndTurn(ctx, sessionID, chatReq.Skill, chatReq.Query, verdict.Verdict{}, "", "internal_error", nil)
		return
	}

	// 11-01 ship: emit a terminal finish-step + finish (the legacy
	// chatHandler emits these after the verdict stream drains). 11-04
	// moves the verdict parse + audit write into this handler; for 11-01
	// we log + persist a minimal audit row (best-effort).
	log.Printf("[INFO] server: ChatHandler drain complete (sess=%s skill=%s)", sessionID, chatReq.Skill)
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})

	// 11-01 stub: verdict parse + trust floor + grounding land in
	// 11-04. For now write a minimal audit row with refusal_reason=\"\"
	// (the verdict path is captured in the audit row's verdict_json in
	// the legacy chatHandler — 11-04 re-uses writeAuditAndTurn with the
	// parsed verdict).
	s.writeAuditAndTurn(ctx, sessionID, chatReq.Skill, chatReq.Query, verdict.Verdict{}, chatReq.Skill, "", nil)
}

// dataRefEmitter builds the per-request SSE data-reference emitter for
// the data channel (Phase 16 / Plan 16-08 / D16-B3). The returned closure
// is handed to agent.RunnerBuildConfig.DataRefEmit; the kpi_call Tool
// invokes it exactly once per successful Snapshot, and the payload rides
// the SAME data-decision envelope every other decision uses (D16 overload
// pattern — no new SSE chunk type, so the D15 wire-type alarm and the
// D-04 transport-swap contract stay untouched).
//
// Step is 0 by design: the hop counter lives inside internal/agent's
// per-Run stepCounter (callbacks.go) and is not observable from the
// server boundary, and the FE consumes data-reference parts by decision
// KIND (kpi_reference → data-citation chip), not by hop ordinal. Emitting
// a fabricated step number would be worse than an honest zero.
//
// Emission failures are logged, never propagated: the SSE stream is
// already in flight and a dropped citation chip must not abort the answer
// (the authoritative copy of the same data lives in the kpi_calls audit
// row — KPI-02).
func (s *Deps) dataRefEmitter(c *app.RequestContext) agenttools.DataRefEmitFunc {
	return func(ref wire.DataReference) {
		payload := ref
		if err := runnerEmitDecision(c, wire.DecisionPayload{
			Step:     0,
			Decision: wire.DecisionKPIReference,
			DataRef:  &payload,
		}); err != nil {
			log.Printf("[WARN] server: emit kpi_reference data-decision failed (func=%s): %v", ref.Func, err)
		}
	}
}

// reportPublishEmitter builds the per-request SSE emission hook for the
// WR-02 (18-REVIEW) report_published card: after a successful publish
// the report_publish HITL tool invokes it with a data-decision payload
// carrying Decision="report_published" + the ReportCard, and it rides
// the SAME data-decision envelope as every other decision (D16 overload
// pattern — no new SSE chunk type, D-04/D15 untouched). Emission
// failures are logged, never propagated: the published file + the
// reports DB row are the source of truth, a dropped card must not fail
// the publish.
func (s *Deps) reportPublishEmitter(c *app.RequestContext) hitl.ReportPublishEmitFunc {
	return func(p wire.DecisionPayload) {
		if err := runnerEmitDecision(c, p); err != nil {
			log.Printf("[WARN] server: emit %s data-decision failed (toolCallId=%s): %v", p.Decision, p.ToolCallID, err)
		}
	}
}

// runnerEmitDecision mirrors the legacy chatHandler's emitDecision
// wrapper — kept here so future plans can wire the per-request
// decision-emit closure into the Runner path without touching
// chat.go (D11-A2 trust-staircase will register a BeforeAgent
// callback that emits decision chunks; this helper is the seam).
//
// 11-01 ships the seam only — the BeforeAgent callback registration
// lands in 11-04.
func runnerEmitDecision(c *app.RequestContext, p wire.DecisionPayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal decision: %w", err)
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})
}

// compile-time guard: keep the skill + agent imports live so a future
// refactor that drops the BuildRunner call cannot silently remove the
// runner-path seam without breaking this file's build first.
var (
	_ = agent.RunnerCheckPointID
	_ = (*skill.Manifest)(nil)

	// FAN-05 (18-VERIFICATION GAP-1): the adapter MUST keep satisfying
	// the agent-declared audit seam (structural — internal/agent cannot
	// import internal/server, so the lock lives on this side).
	_ agent.FanOutAuditSink = (*fanOutAuditAdapter)(nil)
)

// fanOutAuditAdapter adapts the session store to agent.FanOutAuditSink
// (FAN-05, 18-VERIFICATION GAP-1: the data-channel fan_out_dispatch
// classifier decision lands in the audit table). It carries the same
// session/skill identity the HITL audit rows use, plus the
// PlatformFloorVersion every INSERT path is contract-bound to populate
// (store.AuditRow drift-window semantics). The Deps field type is the
// store.Store INTERFACE — no sqlite driver dependency at this layer.
type fanOutAuditAdapter struct {
	store     store.Store
	sessionID string
	skillID   string
	floorVer  string
}

func (a *fanOutAuditAdapter) PutAudit(ctx context.Context, action, runPath, transferFrom, transferTo, reason string) error {
	_, err := a.store.PutAudit(ctx, store.AuditRow{
		Timestamp:            time.Now().UTC(),
		SessionID:            a.sessionID,
		SkillID:              a.skillID,
		Action:               action,
		RefusalReason:        reason,
		RunPath:              runPath,
		TransferFrom:         transferFrom,
		TransferTo:           transferTo,
		PlatformFloorVersion: a.floorVer,
	})
	return err
}
