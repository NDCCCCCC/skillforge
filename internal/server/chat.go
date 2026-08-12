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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
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
//  2. Resolve skill name → *skill.Manifest via deps.Loader.Skills().
//     Empty skill → defaultSkill. Unknown skill → emit out_of_scope
//     refusal + finish-step.
//  3. Resolve session_id (X-Session-Id header OR newSessionID) and
//     echo it back on the response (AGNT-05 / SC-9).
//  4. Branch on Agent availability:
//     - Phase 3 path (s.Agent != nil): runAgent (ReAct loop + audit
//       write + session turn append + data-decision SSE emission).
//     - Phase 2 fallback (s.Agent == nil): VerdictPipeline (retained
//       so existing acceptance tests that construct a Deps with only
//       Runner set stay green during the Phase 3 migration; Plan 03-06
//       also uses this path for the eval golden-set runner).
//
// Hertz's WithHostPorts(addr) + h.Spin() call shape (cmd/server/main.go)
// blocks here until SIGINT/SIGTERM — this handler runs on every
// request concurrently.
func (s *Deps) chatHandler(ctx context.Context, c *app.RequestContext) {
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
	if req.Query == "" {
		c.SetStatusCode(http.StatusBadRequest)
		_ = wire.EmitChunk(c, errorChunk("query is required"))
		return
	}
	if req.Skill == "" {
		req.Skill = defaultSkill
	}

	manifest := s.findSkill(req.Skill)
	if manifest == nil {
		// Unknown skill → out_of_scope refusal. The finish-step after the
		// data-policy is the canonical terminator — clients expect it.
		_ = emitRefusal(c, verdict.Refusal{
			Reason:      verdict.RefusalReasonOutOfScope,
			CanRephrase: false,
		})
		return
	}

	// Phase 3: session_id resolution (AGNT-05 / SC-9). The header is the
	// client's claim; if absent the server generates a fresh crypto/rand
	// ID (T-3-SESSIONID — 32 hex chars = 128 bits of entropy, ASVS V6).
	// The ID is echoed back on the response so a client that did not send
	// one can capture and reuse it for the next turn.
	sessionID := string(c.Request.Header.Peek("X-Session-Id"))
	if sessionID == "" {
		sessionID = newSessionID()
	}
	c.Response.Header.SetBytesV("X-Session-Id", []byte(sessionID))

	// Phase 3 path: agent wired → run the ReAct loop + audit + session
	// persistence + decision SSE emission. The Phase 2 fallback
	// (VerdictPipeline) preserves the existing acceptance tests.
	if s.Agent != nil {
		s.runAgentPath(ctx, c, req, manifest, sessionID)
		return
	}

	if err := s.VerdictPipeline(ctx, c, req.Query, manifest); err != nil {
		// Pipeline returned an error AFTER the first SSE frame was written
		// (we cannot change status now). Emit an error chunk so the client
		// sees something meaningful, then finish-step.
		_ = wire.EmitChunk(c, errorChunk(err.Error()))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		return
	}
}

// runAgentPath is the Phase 3 /api/chat handler branch. It wires the
// per-request emitDecision closure into the agent via ctx-value (Plan
// 03-05 Rule 3 deviation — the agent's DecisionEmitFunc cannot be
// re-bound post-construction; internal/agent.WithRunEmit injects a
// per-Run override carrying the current *app.RequestContext). It then
// calls Agent.Run, emits the verdict / refusal / error SSE frames, and
// persists the audit row + session turn BEFORE returning (GEVL-03 +
// AGNT-05). Audit / session write failures are logged and swallowed —
// the response has already streamed; failing the request over a
// persistence error is worse than degraded durability (T-3-AUDIT-WRITE
// disposition: accept; Phase 6 adds fail-closed audit policy).
//
// AGNT-04 (SC-8): ctx cancellation propagates Hertz → Agent.Run →
// reactAgent.Generate → compose.Graph. The SC-8 end-to-end test
// (chat_session_test.go:TestCancelPropagationMidTool) asserts < 200ms
// cancel latency.
func (s *Deps) runAgentPath(ctx context.Context, c *app.RequestContext, req chatRequest, manifest *skill.Manifest, sessionID string) {
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
		s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", "internal_error")
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
		s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", refusal.Reason)
		return
	}

	// Verdict path. chunks=nil is safe: sourcePathFor returns the chunk
	// ID itself when no chunk matches, so the citation badge in the UI
	// still renders with the chunk ID as the title. The agent's
	// chunkAccumulator is not exposed across the package boundary; a
	// future Plan can lift it via an accessor if richer source-document
	// metadata is needed.
	_ = emitVerdictStream(c, v, nil)
	s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, v, req.Skill, "")
}

// writeAuditAndTurn persists the audit row + (user, assistant) session
// turn pair. Called from runAgentPath on every response shape (verdict,
// refusal, error). Both writes are best-effort: errors are logged and
// swallowed because the response has already streamed (T-3-AUDIT-WRITE
// accept disposition). The audit row carries the SHA-256 prompt_hash
// (T-3-HASH) so cross-run reproducibility is structurally guaranteed.
//
// modelVersion is hard-coded "glm-5.2" for Phase 3 (the only supported
// model). Phase 5 lifts this to a Deps field when multi-model routing
// arrives. refusalReason is "" for the verdict path, or the refusal
// code ("no_authorized_source" / "internal_error" / ...) otherwise.
func (s *Deps) writeAuditAndTurn(ctx context.Context, sessionID, skillID, query string, v verdict.Verdict, skillNameForHash, refusalReason string) {
	if s.Store == nil {
		return
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
		Timestamp:     time.Now().UTC(),
		SessionID:     sessionID,
		SkillID:       skillID,
		Query:         query,
		PromptHash:    computePromptHash(skillNameForHash, modelVersion),
		ModelVersion:  modelVersion,
		VerdictJSON:   verdictJSON,
		RefusalReason: refusalReason,
		ChunkIDs:      chunkIDs,
	}
	if _, err := s.Store.PutAudit(ctx, row); err != nil {
		log.Printf("[WARN] server: audit write (sess=%s): %v (response already streamed; non-fatal)", sessionID, err)
	}
	assistantContent := verdictJSON
	if refusalReason != "" {
		assistantContent = refusalReason
	}
	if err := s.Store.AppendTurn(ctx, sessionID,
		store.SessionMessage{Role: "user", Content: query, Ts: time.Now().UTC()},
		store.SessionMessage{Role: "assistant", Content: assistantContent, Ts: time.Now().UTC()},
	); err != nil {
		log.Printf("[WARN] server: session turn append (sess=%s): %v (response already streamed; non-fatal)", sessionID, err)
	}
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

// computePromptHash returns the SHA-256 hex digest of (skillName +
// modelVersion). GEVL-03 reproducibility — given the same Skill +
// model, the hash is byte-identical across runs, so cross-run
// comparison of audit rows with the same prompt_hash guarantees the
// same Skill+model produced the verdict. The system-prompt body is
// NOT hashed in Phase 3 because the agent's prompt (agent.go:systemPrompt)
// is a compile-time constant — Phase 5 (hot-reloadable Skill-driven
// prompts) lifts the rendered prompt body into the hash input.
func computePromptHash(skillName, modelVersion string) string {
	h := sha256.New()
	h.Write([]byte(skillName))
	h.Write([]byte(modelVersion))
	return hex.EncodeToString(h.Sum(nil))
}

// findSkill returns the manifest for name in deps.Loader.Skills(), or
// nil if the Skill is not loaded (unknown name, or Loader was not
// initialized for tests).
//
// Phase 2 simplification: there is exactly ONE Skill (compliance).
// Phase 5 introduces a Name field on Manifest and a real map lookup
// (SYSK-05 routing). For now, the function matches name == "compliance"
// against the first loaded Skill, returning nil for any other name.
// Tests that bypass the Loader pass manifests directly to
// VerdictPipeline so this lookup is never exercised in the unit suite.
func (s *Deps) findSkill(name string) *skill.Manifest {
	if s.Loader == nil {
		return nil
	}
	if name != defaultSkill {
		// Phase 2 supports only "compliance"; unknown names are out of scope.
		return nil
	}
	for _, m := range s.Loader.Skills() {
		// The first loaded Skill is the compliance Skill (the only one
		// that ships in Phase 2).
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
//   - one finish-step terminator.
//
// The tier info stays in the data-policy payload rather than the
// source-document metadata because the AI SDK v5 UIMessageChunk
// discriminated union has no "tier" field on the source-document
// variant; stuffing it in Data would not parse through the v5
// client parser. The Verdict payload is the canonical place.
func emitVerdictStream(c *app.RequestContext, v verdict.Verdict, chunks []retriever.Hit) error {
	// 1. source-document per citation (SC-6).
	for _, cit := range v.Citations {
		if err := wire.EmitChunk(c, wire.UIMessageChunk{
			Type:     "source-document",
			ID:       "src-" + cit.ChunkID,
			SourceID: cit.ChunkID,
			Title:    sourcePathFor(chunks, cit.ChunkID),
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
	// 3. finish-step terminator (AGNT-03).
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
}

// emitRefusal emits a single data-policy chunk carrying the Refusal,
// then a finish-step. This is the canonical "refusal path" stream —
// both the SC-2 (no_authorized_source), GEVL-01 (low_retrieval_score),
// and D2-13 (out_of_scope) reasons use the same shape (ARAG-05).
//
// Returns the last wire.EmitChunk error (always nil in practice —
// writer.go:157 flushes immediately so a writer error is catastrophic).
func emitRefusal(c *app.RequestContext, r verdict.Refusal) error {
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
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
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

// drainFinalContent consumes a Runner.Query iterator until close,
// returning the concatenated assistant message content. For a no-tools
// ChatModelAgent the iterator emits exactly one AgentEvent whose
// Output.MessageOutput carries the final assistant message.
//
// This function graduates from cmd/spiked/e2e.go:341-374 — the proven
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

// Compile-time guarantee: schema.UserMessage is imported for future
// streaming hooks (Phase 3+); this package will not silently lose the
// import if a future refactor removes the explicit use.
var _ = schema.UserMessage