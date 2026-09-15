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
	"log"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// The chitchat gate (greeting-refusal-gate debug session, 2026-09-14 —
// .planning/debug/greeting-refusal-gate.md).
//
// Problem: VerdictPipeline treated EVERY inbound query as a knowledge
// query. A pure greeting ("你好") paid embed + retrieve + (on the live
// build) a full LLM refusal-classifier round-trip (~17s measured) and
// terminated in one of the five refusal gates — the user saw the 拒答
// card for a friendly hello. Which gate fired varied with environment
// (degraded RETRIEVER_BACKEND=none → no_authorized_source; live Milvus
// low scores → low_retrieval_score; classifier refuse → out_of_scope),
// but the outcome was always the refusal card.
//
// Fix shape (root cause is the MISSING branch, so the fix adds the
// branch): a deterministic WHOLE-QUERY match against a closed greeting
// set, evaluated at the pipeline entry BEFORE embed/retrieve. On a hit
// the pipeline emits a plain text reply (no data-policy frame — no
// refusal card, no verdict card, no feedback row) and persists the turn
// with the reply TEXT as the assistant content so the session-restore
// plain-prose branch (handlers_session_history.go renderRow) renders it
// verbatim instead of RenderRefusal's refusal sentence.
//
// Why rules, not an LLM intent classifier: the project's documented
// lesson (agent-llm-query-rewrite.md) is that LLM-driven gating drifts
// and costs a round-trip per message — the exact failure mode this gate
// removes. A closed set + whole-query match is zero-cost, zero-latency,
// and cannot drift; the cost is recall (compound greetings like
// "你好在吗" fall through to the knowledge gates), which is the safe
// direction to fail: those users get the pre-existing refusal flow, not
// a wrong answer.

// refusalReasonChitchat is the audit-row refusal_reason code recorded
// when the chitchat gate fires. It follows the "internal_error"
// precedent: a server-owned reason code that never enters the
// refusalReasonText / history.refusalReasonText copy maps (those maps
// render refusal CARDS, and the chitchat path emits no card). Its only
// consumer is the audit trail, where it distinguishes greeting turns
// from knowledge turns.
const refusalReasonChitchat = "chitchat"

// chitchatReplyText is the deterministic greeting reply. A constant by
// the semantic-determinism convention (chat.go conclusionText): the
// readable surface is a pure constant, so it cannot hallucinate, cannot
// drift between live stream and session restore, and costs no LLM call.
const chitchatReplyText = "你好！有什么可以帮您？我会基于当前 Skill 已授权的知识源回答问题，并给出引用依据。"

// chitchatGreetings is the closed greeting set, compared against the
// NORMALIZED whole query (whitespace + punctuation trimmed, Latin
// case-folded). Membership is whole-string: "history" does not match
// "hi", and "你好，年假有几天" does not match "你好" — both are
// knowledge queries that must reach retrieval (locked by
// TestIsChitchatQuery + TestVerdictPipelineDefault_
// GreetingPrefixedKnowledgeQueryStillRetrieves).
var chitchatGreetings = map[string]struct{}{}

func init() {
	for _, g := range []string{
		// 中文问候
		"你好", "您好", "你好呀", "您好呀", "你好啊", "您好啊",
		"哈喽", "哈罗", "嗨", "嗨嗨", "嘿",
		"早上好", "上午好", "中午好", "下午好", "晚上好",
		"早安", "午安", "晚安",
		"在吗", "在么", "在不在",
		"谢谢", "谢啦", "多谢", "感谢",
		"再见", "拜拜", "回见",
		// Latin greetings (matched case-insensitively after normalization)
		"hi", "hello", "hey",
		"good morning", "good afternoon", "good evening", "good night",
		"thanks", "thank you", "thx",
		"bye", "bye bye", "goodbye",
	} {
		chitchatGreetings[g] = struct{}{}
	}
}

// chitchatTrimCutset is the whitespace + CJK/ASCII punctuation run the
// detector strips from both ends before the whole-query membership
// check. Full-width space (　) included — the usual CJK IME residue.
const chitchatTrimCutset = " \t\n\r　，。！？；：、·~～!?,.;:'\"“”‘’"

// isChitchatQuery reports whether the query is a PURE greeting /
// smalltalk turn rather than a knowledge query. The empty string is
// NOT chitchat (decodeChatRequest already 400s it; a direct pipeline
// call must not greet on an empty query either).
func isChitchatQuery(query string) bool {
	q := strings.TrimSpace(query)
	q = strings.Trim(q, chitchatTrimCutset)
	if q == "" {
		return false
	}
	_, ok := chitchatGreetings[strings.ToLower(q)]
	return ok
}

// emitChitchatResponse is the greeting short-circuit response shape.
// It mirrors emitRefusalWithChunks' persistence contract (exactly one
// audit row + one (user, assistant) session turn, best-effort) and
// emitRefusal's frame ordering (data-turn first when a turn id exists),
// but emits a PLAIN text message: no data-policy frame, so the FE
// renders a normal text bubble — no refusal card, no feedback row.
func (s *Deps) emitChitchatResponse(ctx context.Context, c *app.RequestContext, sessionID, skillID, query string) error {
	// assistantContent = the reply TEXT (not the refusal code) so the
	// restore handler's non-verdict non-refusal branch renders it
	// verbatim; the audit row still carries refusalReasonChitchat for
	// the audit trail (writeAuditAndTurnWithContent override).
	turnID, _ := s.writeAuditAndTurnWithContent(ctx, sessionID, skillID, query, verdict.Verdict{}, "", refusalReasonChitchat, chitchatReplyText, nil)
	if turnID > 0 {
		if err := emitTurnID(c, turnID); err != nil {
			log.Printf("[WARN] server: data-turn emit failed (chitchat; audit row already written; non-fatal): %v", err)
		}
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-start", ID: "msg-1"}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: chitchatReplyText}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"}); err != nil {
		return err
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"}); err != nil {
		return err
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"})
}
