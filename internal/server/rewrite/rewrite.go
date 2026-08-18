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
// WITHOUT WARRANTIES OR CONDITIONS OF CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package rewrite implements the Phase 10 query-rewriter — a lightweight
// LLM call that resolves anaphoric references ("那第二条呢", "它呢", "上面那个")
// in user follow-up queries into self-contained sentences that the retriever
// + classifier can act on (D10-C1).
//
// Why a separate package:
//
//   - D10-C1 — best-effort, fail-soft: a rewriter timeout / parse failure
//     falls back to the ORIGINAL query and the pipeline continues. The
//     rewriter is NEVER a hard gate; if it crashes the user still gets
//     an answer.
//   - D10-C2 — rewrite output NEVER enters the evidence chain. Only the
//     retriever (embedding) and the classifier see the rewritten query;
//     the verdict LLM still sees the original query + chunks (the trust
//     floor is preserved).
//   - D10-C3 — the rewritten query is emitted as a data-decision event
//     so the FE can show "已理解为: XX" transparency.
//
// The rewriter is structurally separated from the classifier package:
// the classifier's input struct has no History field (D10-A1 — see
// internal/server/classifier/types.go); the rewriter DOES see history
// (its job is to resolve anaphora that requires history). The asymmetry
// is intentional and locked by the D10-A1 type-level grep gate.
package rewrite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/llm"
)

// rewriterSystemTemplate instructs the LLM to rewrite the user's query
// into a self-contained sentence, returning JSON. The "return unchanged"
// instruction is the pass-through path for queries with no anaphoric
// reference (RESOLQ 5 — covers both rewrites and no-op rewrites in one
// prompt so the LLM never invents edits on a clean query).
const rewriterSystemTemplate = `You are a query rewriter for a Chinese compliance review chatbot.

Your task: given the conversation history (one user/assistant pair per line) and the user's NEW follow-up query, return a single JSON object of the form:

{"rewritten_query": "<self-contained Chinese sentence that captures the user's intent>"}

Rules:
1. If the new query references an earlier turn ("那第二条呢", "它呢", "上面那个", "第一条的内容"), expand it into a self-contained sentence using the relevant history ("第二条的内容").
2. If the new query is already self-contained (no anaphora), return it UNCHANGED.
3. NEVER add information that is NOT in the history.
4. NEVER answer the query — only rewrite it.
5. Output ONLY the JSON object. No prose, no markdown.`

// rewriterUserTemplate takes two %s slots: (1) the rendered history
// text (one turn per line, trimmed to 4000 chars), (2) the new query.
const rewriterUserTemplate = `Conversation history (chronological, oldest first):
%s

New user query: %s

Return the JSON object.`

// historyCharBudget caps the rendered history text at 4000 chars. The
// rewriter LLM call latency must stay under 2s; bounding the prompt
// keeps GLM-5.2 inside that window. The first 4000 chars are kept
// (chronological order — oldest first); the truncation marker tells the
// LLM that some history was dropped so it does not infer a missing
// earlier turn.
const historyCharBudget = 4000

// truncationMarker is appended when the rendered history exceeds
// historyCharBudget. The LLM is instructed (via rewriterSystemTemplate)
// to never invent edits, so the marker is informational only — it does
// not bias the rewrite.
const truncationMarker = "\n... [earlier history truncated]"

// DefaultRewriteTimeout is the default Timeout on QueryRewriter. The
// rewriter is best-effort; a slow / failing LLM call must NEVER block
// the pipeline beyond this budget (D10-T-10-08 — DoS mitigation).
const DefaultRewriteTimeout = 2 * time.Second

// QueryRewriter is the Phase 10 query-rewriter wrapper. It is wired into
// Deps.Rewriter in cmd/server/main.go (Plan 02 Task 4 — B2 production
// assembly). Nil-gate semantics: a nil Rewriter disables only the rewrite
// step — the pipeline continues with the original query + history prefix.
type QueryRewriter struct {
	// cm is the LLM used to rewrite queries. Production wires the same
	// GLM-5.2 ChatModel that the verdict pipeline uses (cmd/server
	// shares `cm` across PipelineChatModel + Rewriter + RefusalClassifier
	// — one model, three call shapes).
	cm model.ToolCallingChatModel

	// Timeout caps the LLM call duration. Exposed as a struct field so
	// tests can shorten it (TestRewrite_TimeoutFallback uses a 50ms
	// timeout to assert fallback within Timeout + 100ms).
	Timeout time.Duration
}

// NewQueryRewriter validates the constructor inputs and returns a
// ready-to-call QueryRewriter. Surfaces construction errors so
// cmd/server can fail-fast at startup (mirrors the existing
// `cm, err := openai.NewChatModel(...)` posture in main.go).
//
// The constructor does NOT validate the underlying ChatModel — model
// construction failures surface from openai.NewChatModel in main.go
// (Phase 10 keeps that gate single-source).
func NewQueryRewriter(cm model.ToolCallingChatModel) (*QueryRewriter, error) {
	if cm == nil {
		return nil, fmt.Errorf("rewrite.NewQueryRewriter: cm must be non-nil")
	}
	return &QueryRewriter{cm: cm, Timeout: DefaultRewriteTimeout}, nil
}

// Rewrite resolves anaphoric references in the user's new query using
// the sanitized history. The returned string is the rewritten query
// (== original query on no-anaphora / failure paths are the caller's
// concern; this function returns the rewritten string verbatim from
// the LLM, or ("", err) on any failure).
//
// Failure modes (the caller MUST handle these by falling back to the
// original query — D10-C1 best-effort):
//
//   - LLM call exceeds Timeout: context.DeadlineExceeded
//   - LLM call returns error: wrapped error
//   - LLM output is not valid JSON: wrapped parse error
//   - LLM output is missing the rewritten_query key: wrapped validation error
//
// The history slice is the sanitized output from history.Project
// (D9-B1/B2/B3 + D10-A2 neutral placeholder + D10-B3 budget window) —
// the rewriter sees only role text, never refusal-semantic content
// (D10-A2 — history sanitization happens in internal/history BEFORE
// the rewriter is called).
func (r *QueryRewriter) Rewrite(ctx context.Context, history []schema.Message, query string) (string, error) {
	if r == nil || r.cm == nil {
		return "", fmt.Errorf("rewrite.Rewrite: rewriter not initialized")
	}
	if query == "" {
		return "", fmt.Errorf("rewrite.Rewrite: query must be non-empty")
	}

	// Step A: render history as plain text + apply the 4000-char trim.
	historyText := renderHistoryText(history)
	if len(historyText) > historyCharBudget {
		historyText = historyText[:historyCharBudget] + truncationMarker
	}

	// Step B: build the prompt.
	userPrompt := fmt.Sprintf(rewriterUserTemplate, historyText, query)
	extraFields := map[string]any{
		"response_format": map[string]string{"type": "json_object"},
		"thinking":        map[string]string{"type": "disabled"},
	}
	opts := []model.Option{
		model.WithTemperature(0.0),
		model.WithMaxTokens(80),
		openai.WithExtraFields(extraFields),
	}

	// Step C: run the LLM call under a context.WithTimeout derived
	// from the caller's ctx. A timeout surfaces as ctx.DeadlineExceeded
	// which the caller treats as a fallback trigger.
	timeoutCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	msgs := []*schema.Message{
		schema.SystemMessage(rewriterSystemTemplate),
		schema.UserMessage(userPrompt),
	}
	resp, err := r.cm.Generate(timeoutCtx, msgs, opts...)
	if err != nil {
		return "", fmt.Errorf("rewrite.Rewrite: LLM call: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("rewrite.Rewrite: LLM returned nil message")
	}

	// Parse the {"rewritten_query": "..."} envelope. llm.ExtractJSON
	// strips fences + prose + isolated braces (Phase 3 spike hardening)
	// before json.Unmarshal so the call is robust to a slightly
	// over-talkative LLM.
	raw := llm.ExtractJSON(resp.Content)
	if raw == "" {
		return "", fmt.Errorf("rewrite.Rewrite: empty JSON envelope")
	}
	var payload struct {
		RewrittenQuery string `json:"rewritten_query"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("rewrite.Rewrite: parse JSON envelope: %w", err)
	}
	if payload.RewrittenQuery == "" {
		return "", fmt.Errorf("rewrite.Rewrite: rewritten_query is empty")
	}
	return payload.RewrittenQuery, nil
}

// renderHistoryText renders a []schema.Message as plain text, one
// turn per line ("user: …" / "assistant: …"). Used as the history
// slot in the rewriter's user prompt. The output is intentionally
// text-only — the rewriter's job is anaphora resolution, not verdict
// synthesis, so structured metadata would be noise.
//
// Empty / role-without-content turns are skipped; this matches the
// history.Project invariant that only user/assistant roles with
// non-empty content reach this seam.
func renderHistoryText(history []schema.Message) string {
	var b strings.Builder
	for _, m := range history {
		role := "user"
		if m.Role == schema.Assistant {
			role = "assistant"
		} else if m.Role != schema.User {
			continue
		}
		if m.Content == "" {
			continue
		}
		// Newline inside content would break the one-turn-per-line
		// invariant the prompt depends on. Collapse to a single space.
		content := strings.ReplaceAll(m.Content, "\n", " ")
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteString("\n")
	}
	return b.String()
}