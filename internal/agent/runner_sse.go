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

// Package agent — runner_sse.go is Phase 11 / Plan 11-01's AgentEvent
// → wire.UIMessageChunk translator. It is the Adapter that bridges the
// new adk.Runner AsyncIterator[*adk.AgentEvent] stream and the existing
// internal/wire.UIMessageChunk wire envelope that the React UI + AI SDK
// v5 already consume.
//
// Why this file exists:
//
//   - Plan 11-01 rewires chatHandler to drive a single adk.NewRunner().Run(ctx, msgs)
//     one-line dispatch (D11-A1). The Runner emits AgentEvents; the
//     existing chatHandler boundary has to emit UIMessageChunks. This
//     adapter is the seam between the two.
//   - D-04 / D15 invariants: the wire contract is byte-identical to
//     the pre-Runner emitVerdictStream / emitRefusal / emitDecision
//     sequences. No new SSE chunk types are introduced (D15). The
//     transport-swap byte-identity test (D-04) stays green because the
//     adapter emits the same chunk types in the same order for the
//     same logical event sequence.
//   - D11-A4 (<200ms cancel propagation): the drain loop polls ctx.Done()
//     BEFORE each iter.Next() call so cancellation tears down both the
//     Runner iterator and the SSE writer within the same iteration. The
//     Eino iterator does NOT poll ctx cooperatively inside Next(), so
//     the select is the load-bearing anchor for the SC-8 / AGNT-04
//     200ms responsiveness lock.
package agent

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/wire"
)

// EmitFunc is the closure shape that drains Runner events into SSE.
// It mirrors wire.EmitChunk's error semantics so an SSE write failure
// propagates back up the drain loop and tears down the Runner early
// (rather than continuing to pump events into a closed socket — a
// netpoll write-after-close is a hung-process risk on Windows).
//
// Implemented by internal/server/runner_setup.go as a thin wrapper
// over wire.EmitChunk; declared here so unit tests can substitute a
// captured-emit slice without standing up Hertz.
type EmitFunc func(chunk wire.UIMessageChunk) error

// eventToUIMessageChunk translates one Runner AgentEvent into the SSE
// chunk sequence the pre-Runner chatHandler emitted (D-04 byte-identity).
//
// Event-kind handling (eino v0.9.14 internal/core/address.go +
// adk/runner.go:310-337):
//
//   - Output != nil && Output.MessageOutput != nil → emit text-start
//     (id="msg-1") + text-delta per message content + text-end. Mirrors
//     the existing emitVerdictStream / emitRefusal text part lifecycle.
//   - Output.MessageOutput.Message has ToolCalls → 11-01 emits no SSE
//     chunk (the HITL Tool ships in 11-02). The adapter just logs the
//     tool-call id so 11-02 can wire the approval-flow chunk sequence.
//   - Action != nil && Action.Interrupted != nil → log only (11-02
//     emits the approval-request data-* chunk).
//   - All other kinds → empty slice (no-op).
//
// Returns []wire.UIMessageChunk (zero-length when the event is a
// no-op) so the drain loop can iterate uniformly.
func eventToUIMessageChunk(e *adk.AgentEvent) []wire.UIMessageChunk {
	if e == nil {
		return nil
	}
	if e.Err != nil {
		// Surface the error as an error chunk so the client sees a
		// meaningful terminal state. The drain loop also returns the
		// error separately (the chunk is the user-visible mirror).
		payload, _ := json.Marshal(map[string]string{"error": e.Err.Error()})
		return []wire.UIMessageChunk{{Type: "error", ErrorText: e.Err.Error(), Data: payload}}
	}

	// Action.Interrupted — 11-02 emits the approval-request
	// data-decision chunk. Each HITL Tool (memory:facts / review_report
	// / plan_approve) parks the Runner via tool.StatefulInterrupt with
	// a *hitl.ApprovalInfo as the user-facing Info. We surface the
	// ApprovalInfo fields as the chunk payload so the FE ApprovalCard
	// (and PlanApprovalCard for plan_approve) can render the pending
	// gate.
	//
	// Wire shape (consumed by FE RealChatTransport.dispatchApprovalDecision
	// line 89 `case 'interrupted'`):
	//   decision, approvalId, toolCallId, toolName, target,
	//   frozenArgs (object), reason, createdAt, skillVersion,
	//   promptHash, sessionId, skillId.
	//
	// No new SSE chunk type is introduced — this rides the existing
	// data-decision discriminator (D-04 + D15).
	if e.Action != nil && e.Action.Interrupted != nil {
		chunks := emitInterruptChunks(e.Action.Interrupted.InterruptContexts)
		if len(chunks) == 0 {
			log.Printf("[INFO] agent: Runner interrupted (InterruptContexts=%d) — no root-cause ApprovalInfo to surface",
				len(e.Action.Interrupted.InterruptContexts))
			return nil
		}
		log.Printf("[INFO] agent: Runner interrupted (InterruptContexts=%d) — emitted %d approval-request chunk(s)",
			len(e.Action.Interrupted.InterruptContexts), len(chunks))
		return chunks
	}

	// Output event with a message → text-start + text-delta + text-end
	// lifecycle on the "msg-1" id (the canonical AI SDK v5 text part
	// namespace). The AgentEvent.MessageOutput returns the final
	// assistant message; the message may carry tool calls (the
	// post-Run retrieval / answer branches), which we do NOT translate
	// to SSE chunks in 11-01.
	if e.Output == nil || e.Output.MessageOutput == nil {
		return nil
	}
	msg, mErr := e.Output.MessageOutput.GetMessage()
	if mErr != nil {
		return []wire.UIMessageChunk{{Type: "error", ErrorText: mErr.Error()}}
	}
	if msg == nil {
		return nil
	}

	// 11-01: do NOT emit any tool-call SSE chunk. The HITL Tool ships
	// in 11-02; the chunk sequence is frozen until then.
	if len(msg.ToolCalls) > 0 {
		log.Printf("[INFO] agent: Runner emitted %d tool call(s) (first id=%s) — 11-02 wires tool-input-* chunks",
			len(msg.ToolCalls), msg.ToolCalls[0].ID)
	}

	out := []wire.UIMessageChunk{
		{Type: "text-start", ID: "msg-1"},
	}
	if msg.Content != "" {
		out = append(out, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: msg.Content})
	}
	out = append(out, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"})
	return out
}

// emitInterruptChunks turns the Runner's InterruptContexts into the SSE
// chunk sequence the FE dispatchApprovalDecision expects when it sees
// decision="interrupted". The wire payload is keyed by ApprovalInfo
// (the value every HITL Tool passes to tool.StatefulInterrupt) — see
// hitl_plan_approve.go:250 and hitl_memory_facts.go. The session id
// and skill id are not on the interrupt ctx so, so the FE fallbacks
// (`asStr(data.sessionId)` → "") handle the missing fields.
//
// Returns an empty slice when no InterruptCtx carries a root-cause
// ApprovalInfo (caller falls back to the legacy log-only path so the
// D-04 byte-identity contract is preserved for non-HITL interrupts).
func emitInterruptChunks(contexts []*adk.InterruptCtx) []wire.UIMessageChunk {
	if len(contexts) == 0 {
		return nil
	}
	var chunks []wire.UIMessageChunk
	for _, ctx := range contexts {
		if ctx == nil || !ctx.IsRootCause {
			continue
		}
		info, ok := ctx.Info.(*hitl.ApprovalInfo)
		if !ok || info == nil {
			continue
		}
		// FrozenArgs is the verbatim JSON the LLM passed to the tool;
		// surface as a JSON object so FE's `asObj(data.frozenArgs)`
		// .length check sees populated keys (otherwise the dispatch
		// in RealChatTransport line 121 falls back to the empty obj
		// which loses operator-review context).
		var frozenArgs any
		if info.ArgumentsInJSON != "" {
			if err := json.Unmarshal([]byte(info.ArgumentsInJSON), &frozenArgs); err != nil {
				log.Printf("[WARN] agent: interrupt %s frozen_args not valid JSON, surfacing as string: %v",
					info.ToolCallID, err)
				frozenArgs = info.ArgumentsInJSON
			}
		}
		payload := map[string]any{
			"step":         0,
			"decision":     "interrupted",
			"approvalId":   info.ToolCallID,
			"toolCallId":   info.ToolCallID,
			"toolName":     info.ToolName,
			"target":       info.Target,
			"frozenArgs":   frozenArgs,
			"reason":       info.Reason,
			"createdAt":    time.Now().UnixMilli(),
			"skillVersion": info.SkillVersion,
			"promptHash":   info.PromptHash,
			// interruptId is the eino resume address (InterruptCtx.ID).
			// ResumeWithParams' Targets key MUST be this UUID — the
			// approvalID fallback re-interrupts the Tool instead of
			// resuming it (empirically verified against eino v0.9.14;
			// see approval_api.go's file-level resume-target note). The
			// FE echoes it back as the POST body's interrupt_id.
			"interruptId": ctx.ID,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[WARN] agent: marshal interrupt chunk (tool=%s target=%s) failed: %v",
				info.ToolName, info.Target, err)
			continue
		}
		chunks = append(chunks, wire.UIMessageChunk{Type: "data-decision", Data: data})
	}
	return chunks
}

// DrainRunnerEvents pulls every event from iter and emits the
// corresponding UIMessageChunk sequence via emit. The loop returns on
// iterator EOF, an emit error, or ctx cancellation.
//
// D11-A4 <200ms cancel propagation anchor: select { case <-ctx.Done():
// return ctx.Err(); default: } BEFORE each iter.Next() call. Eino's
// AsyncIterator.Next() does not poll ctx cooperatively, so the select
// is the load-bearing piece — without it the drain could sit on Next()
// for the full Run lifetime even after the client closed the SSE
// socket (or the HTTP request was cancelled).
//
// Returns the FIRST non-nil error encountered (ctx.Err() on cancel,
// iter error on Eino-side failure, emit error on SSE write failure).
func DrainRunnerEvents(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent], emit EmitFunc) error {
	if iter == nil {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ev, ok := iter.Next()
		if !ok {
			return nil
		}
		chunks := eventToUIMessageChunk(ev)
		for _, ch := range chunks {
			if emit == nil {
				continue
			}
			if err := emit(ch); err != nil {
				return err
			}
		}
	}
}

// Compile-time anchor: keep the adk import path live so a future
// refactor that removes the AgentEvent read above cannot silently
// drop the dep without breaking this file's build first.
var _ = (*adk.AgentEvent)(nil)
