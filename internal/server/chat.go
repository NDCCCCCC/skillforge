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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/reprohash"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// chatRequest is the POST /api/chat body shape. Skill is optional —
// empty string defaults to "compliance" (the only Skill Phase 2
// supports; Phase 5 adds semantic routing).
type chatRequest struct {
	Query string `json:"query"`
	Skill string `json:"skill"`
	// Messages is the AI SDK v5 DefaultChatTransport body shape. The
	// frontend useChat hook sends {messages:[UIMessage]} (not {query});
	// chatHandler extracts the last user turn's text as the query so the
	// React UI works against this custom {query,skill} backend without a
	// bespoke transport. See lastUserQuery.
	Messages []chatMessage `json:"messages"`
}

// chatMessage is the minimal AI SDK v5 UIMessage shape the backend reads:
// role + parts (only text parts are extracted). Other UIMessage fields
// (id, metadata) are ignored by json decode.
type chatMessage struct {
	Role  string    `json:"role"`
	Parts []msgPart `json:"parts"`
}

// msgPart is the minimal UIMessagePart shape: a text part carries the
// user's input in Text. Non-text parts (file/tool/reasoning) are ignored.
type msgPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// lastUserQuery extracts the text of the last user message from an AI SDK
// v5 UIMessage list (scanned newest-first). The frontend's useChat +
// DefaultChatTransport sends the full conversation as {messages}; the
// backend is single-turn (VerdictPipeline), so only the latest user turn
// matters. Returns "" when there is no user text part — the caller's
// empty-query check then 400s with "query is required".
func lastUserQuery(msgs []chatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "user" {
			continue
		}
		for _, p := range msgs[i].Parts {
			if p.Type == "text" && p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

// defaultSkill is the Phase 2-only Skill name. The pipeline refuses on
// any unknown skill with RefusalReasonOutOfScope — no implicit fallback.
const defaultSkill = "compliance"

// chatHandler is the SSE-emitting POST /api/chat handler. It is the
// single HTTP entry point in this package; pipeline.go owns the Phase 2
// orchestration, this file owns the wire + the request decode + the
// skill-scope lookup + (Phase 3) the session_id plumbing + agent.Run
// dispatch + audit/session writes.
//
// Behavior:
//  1. Decode JSON body — empty query → 400.
//  2. Resolve skill name → *skill.Manifest via deps.Loader.Skills()
//     (Phase 2 fallback), OR via the Plan 05-02 Router if Registry is
//     wired (D5-05 route-then-run). Empty skill → defaultSkill. Unknown
//     skill → emit out_of_scope refusal + finish-step.
//  3. Resolve session_id (X-Session-Id header OR newSessionID) and
//     echo it back on the response (AGNT-05 / SC-9).
//  4. Branch on Agent availability:
//     - Agent path (s.Agent != nil, OPT-IN via USE_AGENT_PATH=true at
//     cmd/server/main.go): runAgent (ReAct loop + audit write +
//     session turn append + data-decision SSE emission).
//     - VerdictPipeline (s.Agent == nil, the PRODUCTION DEFAULT):
//     code-forced original-query retrieval → LLM emits verdict JSON
//     only. Audit + session writes happen inside VerdictPipeline at
//     each emit site (refusal / verdict) and in this handler on the
//     error branch — same writeAuditAndTurn helper as runAgentPath.
//     Existing acceptance tests that construct a Deps with only
//     Runner set (no Agent) stay green via this path; Plan 03-06's
//     eval golden-set runner also uses it.
//
// Hertz's WithHostPorts(addr) + h.Spin() call shape (cmd/server/main.go)
// blocks here until SIGINT/SIGTERM — this handler runs on every
// request concurrently.
func (s *Deps) chatHandler(ctx context.Context, c *app.RequestContext) {
	// DEBUG (260815-gcw): /api/chat diagnostic logging seam.
	// Trigger: `curl -X POST :8888/api/chat -H 'Content-Type: application/json' -d '{"query":"...","skill":"compliance"}'`
	// Expected log sequence (happy path fires all 14 sites):
	//   1. chatHandler enter  2. routing to VerdictPipeline (Loader)  3. dispatching to VerdictPipeline
	//   4. VerdictPipeline enter  5. streamAndAssemble entering  6. calling PipelineChatModel.Stream
	//   7. Stream returned OK  8. about to call first Recv  9. recv token_n=N  10. completed
	// Hang-location decision: no log 1 = hertz; log 1 only = decode/lookup;
	// logs 1-3 only = embed/retrieve; logs 1-5 only = Stream() (GLM/endpoint);
	// logs 1-7 only = first Recv; slow recv N = mid-stream; logs OK but no FE draft = SSE wire.
	// UTF-8 validity guard (BEFORE BindAndValidate). Sonic — Hertz's JSON
	// decoder — silently replaces invalid UTF-8 bytes in string values with
	// U+FFFD, so by the time req.Query is populated the original encoding
	// error is unrecoverable. A Windows shell running a legacy GBK codepage
	// POSTs "年假有几天" as mojibake bytes; after sonic's sanitization it
	// becomes a U+FFFD string that embeds to a garbage vector and silently
	// recalls the wrong chunk (live failure: anti_corruption_clause.md 0.31
	// instead of annual_leave.md 0.65) — a confidently-wrong verdict that
	// looks like a RAG bug. Check the RAW request body so the operator gets
	// an explicit 400 instead. The React frontend sends UTF-8 JSON via
	// fetch, so legitimate clients never trip this — it catches
	// misconfigured CLI clients (curl from a non-UTF-8 shell).
	if raw := c.Request.Body(); len(raw) > 0 && !utf8.Valid(raw) {
		c.SetStatusCode(http.StatusBadRequest)
		_ = wire.EmitChunk(c, errorChunk("request body must be valid UTF-8 (client likely sent a legacy codepage such as GBK/Shift-JIS)"))
		return
	}

	var req chatRequest
	if err := c.BindAndValidate(&req); err != nil {
		// BindAndValidate decodes the JSON body and validates basic types.
		// A malformed body is a 400 — Hertz handles status before we get here.
		emitRefusal(c, verdict.Refusal{
			Reason:      verdict.RefusalReasonOutOfScope,
			CanRephrase: false,
		})
		return
	}
	// AI SDK v5 compatibility: DefaultChatTransport sends {messages:[UIMessage]}
	// not {query}. Extract the last user turn's text part so the React useChat
	// hook works against this custom backend. Live-discovered when
	// VITE_REAL_BACKEND first connected the UI (Phase 7): every request 400'd
	// "query is required" because the body had no `query` field.
	if req.Query == "" {
		req.Query = lastUserQuery(req.Messages)
	}
	if req.Query == "" {
		c.SetStatusCode(http.StatusBadRequest)
		_ = wire.EmitChunk(c, errorChunk("query is required"))
		return
	}
	if req.Skill == "" {
		req.Skill = defaultSkill
	}

	// Phase 17 (DASH-01 live-UAT gap closure): data-channel Skills (reads
	// declares "data:") ride the Runner path — the VerdictPipeline below is
	// a tier-channel retrieval pipeline with no kpi_call surface, so a
	// data-channel request would silently degrade into an out_of_scope
	// refusal. Delegating to (*Deps).ChatHandler (runner_setup.go) hands
	// the request to BuildRunner's channel branch (D16-C3: data →
	// {kpi_call}), which also carries the dataRefEmitter wiring (16-08).
	// The body is buffered by hertz (Request.Body() is a peek, and the
	// runner handler re-binds), so delegation after BindAndValidate is
	// safe. Tier-channel requests are unaffected.
	if m := s.findSkill(req.Skill); m != nil && m.HasDataChannel() {
		log.Printf("[INFO] server: chatHandler delegating data-channel skill=%s to Runner path", req.Skill)
		s.runnerChatHandler(ctx, c)
		return
	}

	// Phase 3: session_id resolution (AGNT-05 / SC-9). The header is the
	// client's claim; if absent the server generates a fresh crypto/rand
	// ID (T-3-SESSIONID — 32 hex chars = 128 bits of entropy, ASVS V6).
	// The ID is echoed back on the response so a client that did not send
	// one can capture and reuse it for the next turn. Resolved unconditionally
	// so both the agent path and the VerdictPipeline default benefit from
	// session correlation in the audit row.
	sessionID := string(c.Request.Header.Peek("X-Session-Id"))
	if sessionID == "" {
		sessionID = newSessionID()
	}
	c.Response.Header.SetBytesV("X-Session-Id", []byte(sessionID))

	// DEBUG (260815-gcw): entry log — fires before any routing decision so
	// operators can confirm /api/chat was reached even if the request hangs
	// inside VerdictPipeline or LLM. query is truncated to 80 chars to bound
	// log volume; full query is in the audit row regardless.
	log.Printf("[INFO] server: chatHandler enter query=%q skill=%s sess=%s", truncate(req.Query, 80), req.Skill, sessionID)

	// Agent path (runAgentPath) is OPT-IN via USE_AGENT_PATH=true at
	// cmd/server/main.go (which gates agent construction). The PRODUCTION
	// DEFAULT is the VerdictPipeline branch below (code-forced original-
	// query retrieval → LLM emits verdict JSON only), which is robust
	// against LLM query-rewrite drift. The agent path delegates query
	// rewrite / chunk selection / refusal judgement to the LLM; on
	// GLM-5.2 every layer drifts (年假 was rewritten to 出差住宿费 → wrong
	// doc → unanswered on a query the retriever answers correctly with
	// the original phrasing), so it is retained in-tree as an experimental
	// opt-in. See .planning/debug/agent-llm-query-rewrite.md for the
	// live-verified failure evidence. Tests that set s.Agent directly
	// still exercise runAgentPath (chat_session_test.go); production
	// leaves Agent nil unless USE_AGENT_PATH=true. The agent closes over
	// its own manifest (set at NewAgent construction), so findSkill is
	// NOT consulted on this branch — findSkill's manifest is used only by
	// the VerdictPipeline default. req.Skill flows into the audit row's
	// SkillID column on both paths.
	// DEBUG (260815-gcw): Agent path branch (OPT-IN via USE_AGENT_PATH=true).
	// Fires only when cmd/server wired s.Agent — the production default leaves
	// Agent nil and falls through to the VerdictPipeline branch below.
	log.Printf("[INFO] server: chatHandler routing to runAgentPath (USE_AGENT_PATH=true) skill=%s sess=%s", req.Skill, sessionID)
	if s.Agent != nil {
		s.runAgentPath(ctx, c, req, sessionID)
		return
	}

	// Plan 05-02 / D5-05: route-then-run. When the Registry + Router are
	// wired, chatHandler invokes the Router FIRST and emits the routing
	// decision chunk (data-decision with decision="route" + SkillScores
	// + Selected + Ambiguous) BEFORE VerdictPipeline. On Ambiguous /
	// empty-snapshot / embedder-error paths the verdict is NOT invoked —
	// the FE renders the SkillPicker so the user picks (D5-03).
	// DEBUG (260815-gcw): Registry+Router branch (D5-05 route-then-run).
	// Fires when cmd/server wired both Registry + Router; the Router picks a
	// Skill BEFORE VerdictPipeline runs, emitting data-decision + SkillScores.
	log.Printf("[INFO] server: chatHandler routing to Registry+Router skill=%s sess=%s", req.Skill, sessionID)
	var manifest *skill.Manifest
	if s.Registry != nil && s.Router != nil {
		routed, ok := s.routeThenRun(ctx, c, req)
		if !ok {
			// Router short-circuited (empty snapshot, ambiguous, embedder
			// error). The SSE stream is already finalized by routeThenRun.
			return
		}
		manifest = routed
	} else {
		// Phase 2 fallback: resolve manifest via Loader.
		manifest = s.findSkill(req.Skill)
		// DEBUG (260815-gcw): Loader fallback (Phase 2 default). Fires when
		// the Registry+Router branch above did not take (production cmd/server
		// leaves Registry nil). findSkill returned a non-nil manifest, so
		// VerdictPipeline will run below.
		log.Printf("[INFO] server: chatHandler routing to VerdictPipeline (Loader) skill=%s sess=%s", req.Skill, sessionID)
		if manifest == nil {
			// Unknown skill → out_of_scope refusal. The finish-step after the
			// data-policy is the canonical terminator — clients expect it.
			_ = emitRefusal(c, verdict.Refusal{
				Reason:      verdict.RefusalReasonOutOfScope,
				CanRephrase: false,
			})
			return
		}
	}

	// DEBUG (260815-gcw): VerdictPipeline dispatch — the moment we hand off
	// to the orchestration that drives embed → retrieve → LLM → grounding.
	// If this log appears but VerdictPipeline never logs (D-1), the hang is
	// in the function-call boundary (nil manifest, ctx cancel, etc.).
	log.Printf("[INFO] server: chatHandler dispatching to VerdictPipeline skill=%s sess=%s", req.Skill, sessionID)
	if err := s.VerdictPipeline(ctx, c, req.Query, manifest, sessionID, req.Skill); err != nil {
		// Pipeline returned an error AFTER the first SSE frame was written
		// (we cannot change status now). Emit an error chunk so the client
		// sees something meaningful, then finish-step. Persist the audit
		// row with refusal_reason="internal_error" (GEVL-03: every query
		// writes one audit row; mirrors runAgentPath's error call site at
		// the bottom of runAgentPath). Refusal + verdict shapes are already
		// persisted inside VerdictPipeline at their emit sites — this
		// branch covers only the true-error returns (embed / retrieve /
		// LLM drain / verdict parse / validate).
		//
		// 260817 verdict-stream-hang: pre-fix this branch did NOT log the
		// error and emitted only errorChunk + finish-step — the message
		// never completed its text lifecycle (the Phase-7 unregistered-
		// message failure mode) and App.tsx does not read useChat's error,
		// so the FE froze at the 生成结论 node with zero backend logs.
		// Now: log the full error server-side, then emit a complete text
		// lifecycle carrying a deterministic readable message so the FE
		// always reaches a visible terminal state (ai v7 keeps processing
		// chunks after an error chunk — dist/index.js case "error" only
		// calls onError). The readable text is a fixed template (no
		// internal error detail leaks to the client).
		log.Printf("[ERROR] server: chatHandler verdict pipeline failed sess=%s: %v", sessionID, err)
		_ = wire.EmitChunk(c, errorChunk(err.Error()))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-start", ID: "msg-1"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: pipelineErrorText})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
		// 29-01: the returned turn id is discarded here — Plan 29-04
		// emits the data-turn frame in this window.
		_, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", "internal_error", nil)
		return
	}
}

// routeThenRun is the Plan 05-02 / D5-05 helper that drives the Router
// BEFORE VerdictPipeline. Emits the routing decision chunk via
// EmitChunk (no new wire type — D5-04 extends the existing data-decision
// payload). On ambiguous / empty-snapshot / embedder-error paths,
// returns (nil, false) so chatHandler can short-circuit WITHOUT
// invoking VerdictPipeline — the FE renders the SkillPicker instead.
//
// Return values:
//
//	manifest, true  — router picked a Skill; chatHandler must invoke
//	                   VerdictPipeline with manifest.
//	nil, false      — router short-circuited; SSE stream is already
//	                   finalized (data-decision emitted; chatHandler
//	                   must NOT call VerdictPipeline).
func (s *Deps) routeThenRun(ctx context.Context, c *app.RequestContext, req chatRequest) (*skill.Manifest, bool) {
	snap := s.Registry.Snapshot()
	result, err := s.Router.Route(ctx, req.Query, snap)
	if err != nil {
		// Embedder outage (BGE-M3 unreachable). Per spike-findings, a
		// hash-based fallback would silently mask the outage — we emit
		// an error envelope so the operator notices. chatHandler maps
		// the error envelope to a structured 500 body.
		c.SetStatusCode(http.StatusInternalServerError)
		_ = wire.EmitChunk(c, errorChunk(fmt.Sprintf("router: %v", err)))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		return nil, false
	}

	// Emit the routing decision chunk (D5-04: extends the existing
	// data-decision wire type, no new wire type). Step: 0 because the
	// routing decision precedes the first agent hop. Decision: "route".
	payload := wire.DecisionPayload{
		Step:        0,
		Decision:    "route",
		SkillScores: result.Scores,
		Selected:    result.Selected,
		Ambiguous:   result.Ambiguous,
	}
	if err := emitDecision(c, payload); err != nil {
		c.SetStatusCode(http.StatusInternalServerError)
		_ = wire.EmitChunk(c, errorChunk(fmt.Sprintf("emit routing decision: %v", err)))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		return nil, false
	}

	if result.Ambiguous {
		// Ambiguous: emit a finish-step so the client sees the stream
		// close cleanly. The FE renders the SkillPicker with the
		// SkillScores so the user can pick (D5-03).
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		return nil, false
	}

	// Resolve the routed Skill's manifest from the registry snapshot.
	entry, ok := snap[result.Selected]
	if !ok || entry == nil {
		// Defensive: the Selected Skill disappeared between Route() and
		// here (race with a hot-reload). Surface as out_of_scope — the
		// user can retry against the new snapshot.
		_ = emitRefusal(c, verdict.Refusal{
			Reason:      verdict.RefusalReasonOutOfScope,
			CanRephrase: false,
		})
		return nil, false
	}
	return entry.Manifest, true
}

// runAgentPath is the Phase 3 /api/chat handler branch (OPT-IN via
// USE_AGENT_PATH=true — the production default is VerdictPipeline; see
// chatHandler). It wires the per-request emitDecision closure into the
// agent via ctx-value (Plan 03-05 Rule 3 deviation — the agent's
// DecisionEmitFunc cannot be re-bound post-construction;
// internal/agent.WithRunEmit injects a per-Run override carrying the
// current *app.RequestContext). It then calls Agent.Run, emits the
// verdict / refusal / error SSE frames, and persists the audit row +
// session turn BEFORE returning (GEVL-03 + AGNT-05). Audit / session
// write failures are logged and swallowed — the response has already
// streamed; failing the request over a persistence error is worse than
// degraded durability (T-3-AUDIT-WRITE disposition: accept; Phase 6
// adds fail-closed audit policy).
//
// AGNT-04 (SC-8): ctx cancellation propagates Hertz → Agent.Run →
// reactAgent.Generate → compose.Graph. The SC-8 end-to-end test
// (chat_session_test.go:TestCancelPropagationMidTool) asserts < 200ms
// cancel latency.
func (s *Deps) runAgentPath(ctx context.Context, c *app.RequestContext, req chatRequest, sessionID string) {
	// DEBUG (260815-gcw): Agent-path entry log. Only fires when
	// USE_AGENT_PATH=true at cmd/server/main.go wired s.Agent; the production
	// default (VerdictPipeline) does not call this function. Mirrors the
	// chatHandler enter log so operators can correlate the branch decision.
	log.Printf("[INFO] server: runAgentPath enter query=%q skill=%s sess=%s", truncate(req.Query, 80), req.Skill, sessionID)
	// Session history load (best-effort — AGNT-05 correlation). Errors
	// are logged and swallowed; the agent still runs with no history.
	// The history is NOT passed into Agent.Run in Phase 3 (the agent is
	// single-turn per query; multi-turn conversation memory feeding is
	// Phase 5+ — the load is exercised here so SC-9 end-to-end can
	// assert cross-session isolation at the HTTP→Store boundary).
	if s.Store != nil {
		if _, err := s.Store.GetSession(ctx, sessionID); err != nil {
			log.Printf("[WARN] server: session history load (sess=%s) failed: %v (continuing with empty history; non-fatal)", sessionID, err)
		}
	}

	// Bind the per-request emitDecision closure into the agent via ctx.
	// The closure converts the agent's 5-positional-arg DecisionEmitFunc
	// into a wire.DecisionPayload and writes a data-decision SSE chunk
	// (ARAG-01 / SC-1 end-to-end wiring).
	emitClosure := agent.DecisionEmitFunc(func(step int, decision, toolCallID, query string, confidence float64) {
		_ = emitDecision(c, wire.DecisionPayload{
			Step:       step,
			Decision:   decision,
			ToolCallID: toolCallID,
			Query:      query,
			Confidence: confidence,
		})
	})
	agentCtx := agent.WithRunEmit(ctx, emitClosure)

	v, refusal, err := s.Agent.Run(agentCtx, sessionID, req.Query)
	if err != nil {
		// Agent error (cancel, ChatModel failure, parse failure). Emit
		// the error frame + finish-step, then persist the audit row
		// with RefusalReason="internal_error" (GEVL-03: every query
		// writes one audit row before the handler returns).
		_ = wire.EmitChunk(c, errorChunk(err.Error()))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		_, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", "internal_error", nil)
		if ctx.Err() != nil {
			log.Printf("[INFO] server: agent run cancelled (sess=%s): %v", sessionID, err)
		} else {
			log.Printf("[ERROR] server: agent run (sess=%s): %v", sessionID, err)
		}
		return
	}

	if refusal != nil {
		// Trust floor or out-of-scope refusal (verdict is zero-value).
		_ = emitRefusal(c, *refusal)
		_, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", refusal.Reason, nil)
		return
	}

	// Verdict path. chunks=nil is safe: sourcePathFor returns the chunk
	// ID itself when no chunk matches, so the citation badge in the UI
	// still renders with the chunk ID as the title. The agent's
	// chunkAccumulator is not exposed across the package boundary; a
	// future Plan can lift it via an accessor if richer source-document
	// metadata is needed.
	_ = emitVerdictStream(c, v, nil, nil)
	// Close the AI SDK v5 message lifecycle. emitVerdictStream opens the
	// text part (text-start + conclusion text-delta) but deliberately
	// does NOT close it (the VerdictPipeline closes it after its
	// post-verdict annotation passes). This path has no annotation
	// passes, so close here: text-end closes the text part, finish-step
	// ends the step, finish ends the message. All three are required for
	// useChat to register the assistant message (live Phase 7 finding —
	// see the Step-11 comment in pipeline.go).
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"})
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
	_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
	// 29-01: the returned turn id is discarded here — Plan 29-04
	// emits the data-turn frame in this window (right after finish).
	_, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, v, req.Skill, "", nil)
}

// writeAuditAndTurn persists the audit row + (user, assistant) session
// turn pair + (Phase 10) the audit_meta sibling row. Called from
// runAgentPath on every response shape (verdict, refusal, error) AND
// from VerdictPipeline (the production default path) at its refusal /
// verdict emit sites, plus chatHandler's error branch for the
// pipeline's true-error returns. All three writes are best-effort:
// errors are logged and swallowed because the response has already
// streamed (T-3-AUDIT-WRITE accept disposition). The audit row carries
// the SHA-256 prompt_hash (T-3-HASH) so cross-run reproducibility is
// structurally guaranteed.
//
// Phase 10 (Plan 10-02) — the new meta parameter carries the
// Phase 10 sibling-table audit row. Pass nil on paths where no
// Phase 10 signals exist (e.g. chatHandler's error branch — the
// pipeline's true-error returns happen before Step 0 history-load).
// The helper internally swallows the meta PutAuditMeta error to keep
// the call-site symmetric with the audit + session-turn swallows.
//
// modelVersion is hard-coded "glm-5.2" for Phase 3 (the only supported
// model). Phase 5 lifts this to a Deps field when multi-model routing
// arrives. refusalReason is "" for the verdict path, or the refusal
// code ("no_authorized_source" / "internal_error" / ...) otherwise.
// skillNameForHash is the skillID on the verdict path (so the hash
// pins the skill that produced the verdict) and "" on refusal / error
// paths (mirrors runAgentPath's historical args exactly — refusal rows
// hash only model_version since no skill-specific prompt was rendered).
//
// Phase 29 / Plan 29-01 (D-29-01): returns the assistant row's
// session_turn.id so the chat path can identify the message server-side
// (the feedback vote key). The posture stays best-effort: a swallowed
// error or an idempotent replay yields (0, nil), and the caller must
// not emit a data-turn frame for id 0. Plan 29-04 owns the actual
// data-turn emission window; this plan only opens the return channel.
func (s *Deps) writeAuditAndTurn(ctx context.Context, sessionID, skillID, query string, v verdict.Verdict, skillNameForHash, refusalReason string, meta *store.AuditMeta) (int64, error) {
	if s.Store == nil {
		return 0, nil
	}
	const modelVersion = "glm-5.2"
	verdictJSON := ""
	chunkIDs := ""
	if refusalReason == "" {
		if b, err := json.Marshal(v); err == nil {
			verdictJSON = string(b)
		}
		ids := make([]string, 0, len(v.Citations))
		for _, cit := range v.Citations {
			ids = append(ids, cit.ChunkID)
		}
		chunkIDs = strings.Join(ids, ",")
	}
	row := store.AuditRow{
		Timestamp:            time.Now().UTC(),
		SessionID:            sessionID,
		SkillID:              skillID,
		Query:                query,
		PromptHash:           computePromptHash(skillNameForHash, modelVersion, s.renderedPromptBody(skillNameForHash)),
		ModelVersion:         modelVersion,
		VerdictJSON:          verdictJSON,
		RefusalReason:        refusalReason,
		ChunkIDs:             chunkIDs,
		PlatformFloorVersion: agent.PlatformFloorVersion,
	}
	id, err := s.Store.PutAudit(ctx, row)
	if err != nil {
		log.Printf("[WARN] server: audit write (sess=%s): %v (response already streamed; non-fatal)", sessionID, err)
	}
	// Phase 10: write the audit_meta sibling row when the audit row
	// landed. The FK ties the meta row to the audit row's ID. nil meta
	// → no-op (a first-turn / empty-history path can legitimately
	// choose not to write a meta row).
	if id > 0 && meta != nil {
		meta.QueryID = id
		writeAuditMeta(ctx, s.Store, meta)
	}
	assistantContent := verdictJSON
	if refusalReason != "" {
		assistantContent = refusalReason
	}
	turnID, err := s.Store.AppendTurn(ctx, sessionID,
		store.SessionMessage{Role: "user", Content: query, Ts: time.Now().UTC()},
		store.SessionMessage{Role: "assistant", Content: assistantContent, Ts: time.Now().UTC()},
	)
	if err != nil {
		log.Printf("[WARN] server: session turn append (sess=%s): %v (response already streamed; non-fatal)", sessionID, err)
		return 0, nil
	}
	return turnID, nil
}

// newSessionID generates a fresh 32-char hex session ID (128 bits of
// entropy from crypto/rand — ASVS V6 / T-3-SESSIONID). On the
// exceedingly-rare crypto/rand.Read failure we fall back to a
// timestamp-suffixed counter ID — the request MUST NOT abort over a
// session-id generation failure (degraded mode is preferable to a 500).
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: 16 hex chars of timestamp + 16 hex chars of counter.
		// NOT cryptographically strong, but uniqish under a single
		// process restart. The branch is exercised only on a broken
		// /dev/urandom — production deployment should fail loud
		// elsewhere when that is the case.
		var fallback [16]byte
		binaryTime := uint64(time.Now().UnixNano())
		for i := 0; i < 8; i++ {
			fallback[i] = byte(binaryTime >> (i * 8))
		}
		cnt := sessionIDFallbackCounter.Add(1)
		for i := 0; i < 8; i++ {
			fallback[8+i] = byte(cnt >> (i * 8))
		}
		return hex.EncodeToString(fallback[:])
	}
	return hex.EncodeToString(b[:])
}

// sessionIDFallbackCounter backs the crypto/rand-failure branch of
// newSessionID. Kept as an atomic so concurrent fallback calls still
// produce distinct IDs.
var sessionIDFallbackCounter atomic.Uint64

// emitDecision writes one data-decision SSE chunk for the supplied
// DecisionPayload. The chatHandler wraps this in a closure that
// captures the per-request *app.RequestContext and binds it into the
// agent via internal/agent.WithRunEmit (see runAgentPath).
//
// Marshal failure returns a non-nil error (the caller is the agent's
// callback goroutine; the agent logs and continues — a malformed
// payload should never block the agent loop).
func emitDecision(c *app.RequestContext, p wire.DecisionPayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal decision: %w", err)
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})
}

// computePromptHash returns the SHA-256 hex digest that pins one
// reproducible agent configuration: (skillName + renderedPromptBody +
// modelVersion). GEVL-03 reproducibility — given the same Skill, the
// same rendered prompt and the same model, the hash is byte-identical
// across runs, so cross-run comparison of audit rows carrying the same
// prompt_hash guarantees the same configuration produced the verdict.
//
// Phase 6 / Q1 — THE LIFT IS DONE. Phase 3 hashed ONLY
// (skillName + modelVersion) and deferred the prompt body to Phase 5;
// Phase 5 shipped without doing it. That weak hash made the frozen
// eval set's prompt_hash pin (SC-6 / D6-13) meaningless: editing a
// Skill's steps or agent declaration WITHOUT renaming it left the hash
// unchanged, so the frozen gate would pass green while the system had
// actually drifted. The rendered body is now part of the digest, which
// is exactly what makes internal/eval.AssertPin able to detect drift.
//
// The digest itself lives in internal/reprohash so internal/eval can
// re-compute it byte-identically without importing this package (which
// would be an import cycle). This function stays as the named,
// package-local seam the audit write path calls.
//
// MIGRATION NOTE (accepted, T-06-02): audit rows written BEFORE this
// change carry prompt_hash values produced by the old algorithm and are
// intentionally left stale. prompt_hash is a REPRODUCIBILITY key, not a
// lookup key — nothing joins on it, no external consumer reads it, and
// the store is dev-scale. Rows are immutable (T-3-AUDIT INSERT-only), so
// back-filling them is not an option we want anyway.
func computePromptHash(skillName, modelVersion, renderedPromptBody string) string {
	return reprohash.PromptHash(skillName, modelVersion, renderedPromptBody)
}

// renderedPromptBody returns the system-prompt body that this Deps'
// configuration would send to the model — the third input to
// computePromptHash (Phase 6 / Q1).
//
// Resolution order:
//  1. The wired Agent's own rendering. This is the authoritative source
//     on the Phase 3+ agent path: it is literally the prompt the model
//     saw for the request being audited.
//  2. The routed Skill's manifest via the Loader, when no Agent is
//     wired (VerdictPipeline default / unit-test Deps).
//  3. "" when neither is available — the hash still pins
//     (skillName, modelVersion), i.e. it degrades to exactly the old
//     Phase 3 semantics rather than failing the request.
func (s *Deps) renderedPromptBody(skillName string) string {
	if s.Agent != nil {
		return s.Agent.RenderedSystemPrompt()
	}
	if m := s.findSkill(skillName); m != nil {
		return agent.RenderSystemPrompt(m)
	}
	return ""
}

// findSkill returns the manifest for name in deps.Loader.Skills(), or
// nil if the Skill is not loaded (unknown name, or Loader was not
// initialized for tests).
//
// Phase 17 widening (live-UAT gap closure, D14-E2 first step): exact
// Manifest.Name lookup runs FIRST (Manifest.Name is the directory
// basename — loader.go:143), so data-channel Skills like kpi-dashboard
// are addressable by their real name. The Phase-2 literal gate
// (name == "compliance" → first loaded Skill) is preserved as the
// fallback because the deployment Skill dir ships simple-qa under the
// historical "compliance" chat contract (D14-E2); full SYSK-05 map
// routing stays deferred as recorded.
func (s *Deps) findSkill(name string) *skill.Manifest {
	if s.Loader == nil {
		return nil
	}
	for _, m := range s.Loader.Skills() {
		if m.Name == name {
			return m
		}
	}
	if name != defaultSkill {
		// Not found by exact name and not the legacy alias — out of scope.
		return nil
	}
	for _, m := range s.Loader.Skills() {
		// Legacy Phase-2 fallback: "compliance" resolves to the first
		// TIER-channel Skill (the deployment dir's simple-qa). Data-channel
		// Skills must be skipped here — the legacy alias feeds the
		// VerdictPipeline, which has no kpi_call surface; a data-channel
		// manifest would route the request down the wrong path (and flip
		// the D-04 transport-swap frame contract on the compliance tests).
		if m.HasDataChannel() {
			continue
		}
		return m
	}
	return nil
}

// emitVerdictStream is the SSE-emission tail of the verdict pipeline.
// Per Pattern 2 (02-RESEARCH.md §lines 299-323):
//   - one source-document chunk per citation (SC-6; chunk_id rides in
//     SourceID, tier rides in the data-policy Verdict payload's
//     Citations[].Tier field — keeps source-document AI-SDK-v5-clean).
//   - one data-policy chunk carrying the entire Verdict JSON
//     (CMPL-02 / CMPL-05).
//
// finish-step is NOT emitted here — the caller owns it so it lands after
// the post-verdict annotation passes (see the function-body comment).
//
// The tier info stays in the data-policy payload rather than the
// source-document metadata because the AI SDK v5 UIMessageChunk
// discriminated union has no "tier" field on the source-document
// variant; stuffing it in Data would not parse through the v5
// client parser. The Verdict payload is the canonical place.
func emitVerdictStream(c *app.RequestContext, v verdict.Verdict, chunks []retriever.Hit, reg kb.Registry) error {
	// text-start anchors the AI SDK v5 assistant message so useChat
	// registers it in chat.messages. A data-only stream (source-document +
	// data-policy) creates no message without a content chunk.
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-start", ID: "msg-1"}); err != nil {
		return fmt.Errorf("emit text-start: %w", err)
	}
	// text-delta carrying the human-readable conclusion. The Verdict
	// schema is structured-only (status/confidence/citations — the LLM is
	// prompted to emit JSON and nothing else), so the conclusion SENTENCE
	// is derived deterministically here from the already-grounded fields
	// (conclusionText). This is what makes the final answer visible in the
	// chat conversation stream itself — the data-policy chunk below feeds
	// the structured 结论 tab UI, but a text part is what a user reading
	// the chat expects as "the answer" (the Phase 4 mock fixtures, e.g.
	// __fixtures__/multi-part.json, always carried conclusion text-deltas;
	// the backend originally shipped a single-space placeholder, which
	// left the real backend's chat stream conclusion-less — see
	// .planning/debug/chat-conclusion-not-in-chat.md).
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: conclusionText(v, reg)}); err != nil {
		return fmt.Errorf("emit text-delta: %w", err)
	}
	// 1. source-document per citation (SC-6). mediaType is REQUIRED by the
	// AI SDK v5 source-document schema (index.js:6672-6679): sourceId +
	// mediaType + title are all non-optional. Omitting mediaType fails zod
	// validation, which THROWS and kills the whole SSE stream — every
	// subsequent chunk (data-policy verdict, data-redline) is then dropped
	// (the live Phase 7 failure: verdict rendered nothing). text/markdown
	// is the accurate type for policy-clause source docs.
	for _, cit := range v.Citations {
		if err := wire.EmitChunk(c, wire.UIMessageChunk{
			Type:      "source-document",
			ID:        "src-" + cit.ChunkID,
			SourceID:  cit.ChunkID,
			MediaType: "text/markdown",
			Title:     titleFor(chunks, cit.ChunkID),
		}); err != nil {
			return fmt.Errorf("emit source-document for %s: %w", cit.ChunkID, err)
		}
	}
	// 2. data-policy carrying the entire Verdict.
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal verdict: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-policy",
		Data: payload,
	}); err != nil {
		return fmt.Errorf("emit data-policy: %w", err)
	}
	// finish-step is intentionally NOT emitted here. The caller owns it so
	// it lands AFTER every post-verdict annotation pass (counter-evidence,
	// redline). Emitting it here — right after the verdict — made the AI
	// SDK v5 frontend stop reading the stream before the data-redline chunk
	// arrived, so the redline write hit a half-closed socket (live failure:
	// wsasend connection-aborted, Phase 7). VerdictPipeline and runAgentPath
	// each emit finish-step as their final chunk.
	return nil
}

// emitRefusal emits the refusal-path SSE stream: a text part carrying the
// human-readable refusal sentence (refusalText), then a data-policy chunk
// with the structured Refusal, then finish-step + finish. This is the
// canonical "refusal path" stream — the SC-2 (no_authorized_source),
// GEVL-01 (low_retrieval_score), and D2-13 (out_of_scope) reasons all use
// the same shape (ARAG-05).
//
// The text part matters for the same reason it does in
// emitVerdictStream: a data-only stream leaves the assistant message
// unregistered in useChat (live Phase 7 finding — see the Step-11 comment
// in pipeline.go), so the refusal would be invisible in the chat stream.
// The Phase 4 mock fixtures (e.g. __fixtures__/refusal-out-of-scope.json)
// always carried a refusal text-delta; this closes the same backend/FE
// contract gap fixed for verdicts.
//
// Returns the last wire.EmitChunk error (always nil in practice —
// writer.go:157 flushes immediately so a writer error is catastrophic).
func emitRefusal(c *app.RequestContext, r verdict.Refusal) error {
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-start", ID: "msg-1"}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: refusalText(r)}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"}); err != nil {
		return err
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal refusal: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-policy",
		Data: payload,
	}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"}); err != nil {
		return err
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
}

// conclusionText derives the human-readable conclusion sentence for a
// Verdict, deterministically, from the structured fields the pipeline
// already verified (status validated by verdict.Validate; every
// citation span grounded by verdict.GroundingChecker). No LLM call, no
// new schema field — the sentence is a template over grounded data, so
// it cannot hallucinate (semantic-determinism philosophy: the readable
// surface is a pure function of the trust-checked structure).
//
// 260817-vsm (verdict-subject-mismatch): the renderer now leads with
// Answer when populated (factual queries) and falls back to
// ComplianceStatus (Compliance Skill) or Status (other Skills) when not.
// This fixes the "结论主体是合规/不合规而非回答" root cause: factual queries
// ("采购合同 50 万元以下由谁审批？") now render the direct answer as the
// main body, with the compliance-style verdict badge trailing only when
// relevant.
func conclusionText(v verdict.Verdict, reg kb.Registry) string {
	pct := int(v.Confidence*100 + 0.5)
	var b strings.Builder

	// Branch 1: factual query — Answer is the main body. ComplianceStatus
	// (if set) is dropped from the sentence (it'd be "not_applicable",
	// which adds no signal). The review-required / confidence framing
	// is preserved.
	if v.Answer != "" {
		// 260817-i36 (④): the LLM's Answer often carries its own terminal
		// punctuation ("。", ".", "！", "!", "？", "?"). The template adds
		// another "。", producing 「。。」 in the chat conclusion. Strip the
		// trailing sentence-punctuation run first so the sentence always
		// ends with exactly one "。".
		answer := strings.TrimRight(strings.TrimSpace(v.Answer), "。.！!？?")
		fmt.Fprintf(&b, "结论：%s。", answer)
		if v.ReviewRequired {
			b.WriteString("置信度未达自动采信阈值，需人工复核。")
		}
		if len(v.Citations) > 0 {
			fmt.Fprintf(&b, "依据 %s tier 条款：「%s」。", tierDisplayName(reg, v.Citations[0].Tier), v.Citations[0].Span)
		}
		return normalizeConclusionPunctuation(b.String())
	}

	// Branch 2: compliance-review or other Skill query — the main body
	// is the verdict. Prefer ComplianceStatus (Compliance Skill-specific
	// verdict code) when populated; otherwise fall back to the
	// platform-neutral Status.
	mainStatus := v.ComplianceStatus
	if mainStatus == "" {
		mainStatus = v.Status
	}
	fmt.Fprintf(&b, "结论：%s（置信度 %d%%）。", mainStatus, pct)
	if len(v.Citations) > 0 {
		fmt.Fprintf(&b, "依据 %s tier 条款：「%s」。", tierDisplayName(reg, v.Citations[0].Tier), v.Citations[0].Span)
	}
	if v.ReviewRequired {
		b.WriteString("置信度未达自动采信阈值，需人工复核。")
	}
	return normalizeConclusionPunctuation(b.String())
}

// normalizeConclusionPunctuation is the defensive final pass shared by
// both conclusionText branches (260817-i36 ④): no matter how the
// sentence was assembled, any "。。" run collapses to a single "。".
// Branch 1 already strips the Answer's terminal punctuation; this pass
// covers any future template / status-string path that might reintroduce
// a doubled period.
func normalizeConclusionPunctuation(s string) string {
	return strings.ReplaceAll(s, "。。", "。")
}

// refusalReasonText maps the wire refusal-reason codes to the
// operator-facing Chinese sentences rendered in the chat stream. Unknown
// codes (a future reason constant the FE/BE pair has not taught this
// table) fall through to a generic sentence rather than leaking the raw
// code into the user-facing text.
var refusalReasonText = map[string]string{
	verdict.RefusalReasonNoAuthorizedSource: "未检索到可回答该问题的授权知识源",
	verdict.RefusalReasonLowRetrievalScore:  "检索到的条款与该问题相关度不足",
	verdict.RefusalReasonOutOfScope:         "该问题超出当前 Skill 的处理范围",
}

// refusalText derives the human-readable refusal sentence for a Refusal
// (same deterministic-template contract as conclusionText).
func refusalText(r verdict.Refusal) string {
	msg, ok := refusalReasonText[r.Reason]
	if !ok {
		msg = "无法基于已授权的知识源回答该问题"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "无法给出结论：%s。", msg)
	if r.CanRephrase && r.SuggestedAlternativeQuery != "" {
		fmt.Fprintf(&b, "可尝试调整问题，例如：「%s」。", r.SuggestedAlternativeQuery)
	}
	return b.String()
}

// sourcePathFor returns the original Source path for the chunk whose
// numeric ID matches chunkID (rendered as a decimal string). If no
// chunk matches, the chunk ID itself is returned as the title (still
// useful for the citation badge, even if the source filename is
// unknown to the handler).
func sourcePathFor(chunks []retriever.Hit, chunkID string) string {
	for _, ch := range chunks {
		if fmt.Sprintf("%d", ch.ID) == chunkID {
			return ch.Source
		}
	}
	return chunkID
}

// titleFor derives the source-document chip title for a citation
// (260817-i36 ③): "<source path> · chunk <id>", so two citations into
// different chunks of the SAME document render distinguishable chips in
// the chat stream. When sourcePathFor misses (the chunk ID is not in the
// retriever result set) its fallback IS the chunk ID — appending
// " · chunk <id>" would self-repeat ("9 · chunk 9"), so the miss path
// keeps the bare ID.
func titleFor(chunks []retriever.Hit, chunkID string) string {
	p := sourcePathFor(chunks, chunkID)
	if p == chunkID {
		return p
	}
	return fmt.Sprintf("%s · chunk %s", p, chunkID)
}

// drainFinalContent consumes a Runner.Query iterator until close,
// returning the concatenated assistant message content. For a no-tools
// ChatModelAgent the iterator emits exactly one AgentEvent whose
// Output.MessageOutput carries the final assistant message.
//
// This function graduates from scripts/spiked/e2e.go:341-374 — the proven
// shape from the Phase 1 spike. Any future streaming work (Phase 3+)
// will iterate over multiple events; for the Phase 2 single-shot
// verdict path the final message is all we need.
//
// If any event has a non-nil Err, drain returns that error immediately
// after concatenating whatever content was buffered (so the caller can
// surface a meaningful error without losing partial output).
func drainFinalContent(iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	var content []string
	var firstErr error
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil && firstErr == nil {
			firstErr = ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			msg, mErr := ev.Output.MessageOutput.GetMessage()
			if mErr != nil && firstErr == nil {
				firstErr = mErr
			}
			if msg != nil && msg.Content != "" {
				content = append(content, msg.Content)
			}
		}
	}
	return joinStrings(content), firstErr
}

// joinStrings is the local string-join helper. It avoids importing
// "strings" just for one call; we already use strings in pipeline.go
// for the prompt builder. Kept local for clarity.
func joinStrings(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p
	}
	return out
}

// errorChunk renders an error UIMessageChunk for the post-pipeline
// failure path. ErrorText is the only field set; the AI SDK v5 parser
// surfaces it as a "something went wrong" message in the chat UI.
//
// The error message MUST NOT contain the API key or any other
// T-1-13-sensitive value. The pipeline already enforces this — it
// passes our own wrapped errors whose messages never include env
// values.
func errorChunk(msg string) wire.UIMessageChunk {
	return wire.UIMessageChunk{Type: "error", ErrorText: msg}
}

// pipelineErrorText is the user-facing terminal message emitted on the
// chatHandler error branch (260817 verdict-stream-hang). Deterministic
// and detail-free by design: the internal error stays in server logs
// (T-1-13 — no env values / upstream internals leak to the client),
// the FE gets a readable terminal state instead of a frozen 生成结论
// node.
const pipelineErrorText = "结论生成失败：模型服务暂时不可用，请稍后重试。"

// Compile-time guarantee: schema.UserMessage is imported for future
// streaming hooks (Phase 3+); this package will not silently lose the
// import if a future refactor removes the explicit use.
var _ = schema.UserMessage
