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

package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/llm"
)

// Judge is the LLM-as-judge abstraction over the 5 atomic operations the
// 4 metrics need. Each method is ONE structured-output LLM call at
// temperature=0 (determinism established in spikes 001-003).
//
// The interface is consumed by:
//
//   - Faithfulness (faithfulness.go)        — ExtractClaims + Entails
//   - AnswerRelevancy (metrics.go)          — GenerateQuestions
//   - ContextPrecision (metrics.go)         — IsChunkRelevant
//   - ContextRecall (metrics.go)            — IsAttributable
//
// Implementations:
//
//   - glmJudge — production: drives NewGLMChatModel (D3-07 slot).
//   - StubJudge — tests: deterministic canned responses so
//     TestRunnerReproducibility can assert identical scores within 1e-9
//     (SC-6) without burning real GLM quota.
//
// Pitfall 8 counter-measures (judge self-eval optimism) are documented in
// eval/README.md: (a) the deterministic Go grounding floor catches
// worst-case hallucinations REGARDLESS of judge; (b) human spot-check on
// ~10% sample is a documented non-blocking Phase 3 exit criterion; (c)
// the JUDGE_MODEL config slot allows Phase 6 cross-evaluation with
// DeepSeek or another model.
type Judge interface {
	// ExtractClaims decomposes a natural-language answer into a list of
	// atomic, pronoun-free statements. Used by Faithfulness as stage 1.
	// Empty/whitespace answer returns an empty slice (Faithfulness then
	// returns 1.0 — refusal answer, nothing to refute).
	ExtractClaims(ctx context.Context, answer string) ([]string, error)

	// Entails judges whether a single atomic claim is directly inferable
	// from the given context (the concatenated retrieved chunks). Used by
	// Faithfulness as stage 2 (NLI verdict per claim).
	Entails(ctx context.Context, claim, context string) (bool, error)

	// GenerateQuestions produces N plausible questions to which the given
	// answer would be a reasonable response. Used by AnswerRelevancy:
	// each generated question + the original query are embedded via
	// BGE-M3 and the score is the mean pairwise cosine similarity.
	GenerateQuestions(ctx context.Context, answer string) ([]string, error)

	// IsChunkRelevant judges whether a single retrieved chunk is relevant
	// to the user's query. Used by ContextPrecision (rank-weighted
	// precision@k over the retriever's hits).
	IsChunkRelevant(ctx context.Context, query, chunk string) (bool, error)

	// IsAttributable judges whether a single statement from the ground
	// truth can be attributed to (directly inferred from) the given
	// context. Used by ContextRecall: attributable/total.
	IsAttributable(ctx context.Context, statement, context string) (bool, error)
}

// ---------------------------------------------------------------------------
// glmJudge — production Judge implementation over NewGLMChatModel.
// ---------------------------------------------------------------------------

// glmJudge implements Judge against a GLM ChatModel. Each method renders
// a structured-output prompt and parses a JSON response; temp=0 is
// enforced by the caller-supplied chat model (openai.NewChatModel
// defaults to the provider default — production wraps the model in the
// eval package with temperature=0 explicitly via WithTemperature on the
// Generate call site, NOT here, to keep this type stateless).
//
// All prompting is in English to match the ported Ragas prompts verbatim
// (the prompt text is calibration-sensitive; translating to Chinese would
// drift the "Ragas 口径"). The agent's answer / contexts may be Chinese —
// the GLM judge handles cross-lingual entailment natively.
type glmJudge struct {
	cm model.ToolCallingChatModel
}

// NewJudge constructs a production glmJudge over the given ToolCallingChatModel.
// The model MUST be non-nil; callers obtain it via NewGLMChatModel(ctx).
// Returned type is the Judge interface so callers depend on the
// abstraction, not the concrete impl (StubJudge substitutes in tests).
func NewJudge(cm model.ToolCallingChatModel) Judge {
	if cm == nil {
		panic("eval.NewJudge: cm is nil (caller bug — obtain a model via NewGLMChatModel first)")
	}
	return &glmJudge{cm: cm}
}

// generateJSON is the shared structured-output call. It renders the
// system+user prompt pair, calls the ChatModel with temperature=0, and
// parses the response as JSON into out. out MUST be a pointer. The
// caller owns the schema definition; this helper only handles transport.
//
// Errors:
//   - ChatModel call failure → wrapped, returned.
//   - Empty response → ErrEmptyJudgeResponse.
//   - JSON parse failure → wrapped with a truncated raw preview for debug.
func (j *glmJudge) generateJSON(ctx context.Context, sys, user string, out any) error {
	msgs := []*schema.Message{
		schema.SystemMessage(sys),
		schema.UserMessage(user),
	}
	// temperature=0 — the determinism contract (spikes 001-003).
	resp, err := j.cm.Generate(ctx, msgs, model.WithTemperature(0))
	if err != nil {
		return fmt.Errorf("eval.judge: Generate: %w", err)
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return ErrEmptyJudgeResponse
	}
	raw := resp.Content
	if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), out); err != nil {
		return fmt.Errorf("eval.judge: parse JSON: %w (raw: %s)", err, truncate(raw, 200))
	}
	return nil
}

// ExtractClaims implements the ported Ragas LONG_FORM_ANSWER_PROMPT.
// Returns the flattened list of atomic claims across all parsed
// statement groups. The prompt text is the VERBATIM Ragas source — see
// faithfulness.go for the constant and eval/NOTICE for attribution.
func (j *glmJudge) ExtractClaims(ctx context.Context, answer string) ([]string, error) {
	if strings.TrimSpace(answer) == "" {
		return nil, nil
	}
	user := fmt.Sprintf(longFormAnswerUser, answer)
	var resp struct {
		Analysis []struct {
			SentenceIndex int      `json:"sentence_index"`
			SimplerClaims []string `json:"simpler_statements"`
		} `json:"analysis"`
	}
	if err := j.generateJSON(ctx, longFormAnswerSystem, user, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Analysis))
	for _, g := range resp.Analysis {
		out = append(out, g.SimplerClaims...)
	}
	return out, nil
}

// Entails implements the ported Ragas NLI_STATEMENTS_MESSAGE — single-claim
// variant. Returns true if the claim is directly inferable from ctx.
func (j *glmJudge) Entails(ctx context.Context, claim, contextText string) (bool, error) {
	user := fmt.Sprintf(nliStatementsUser, contextText, claim)
	var resp struct {
		Answer []struct {
			Statement string `json:"statement"`
			Reason    string `json:"reason"`
			Verdict   int    `json:"verdict"`
		} `json:"answer"`
	}
	if err := j.generateJSON(ctx, nliStatementsSystem, user, &resp); err != nil {
		return false, err
	}
	for _, a := range resp.Answer {
		if a.Verdict == 1 {
			return true, nil
		}
	}
	return false, nil
}

// GenerateQuestions is the Go-authored AnswerRelevancy prompt. Asks for
// exactly 3 plausible questions the given answer would address (3 is the
// Ragas default; a smaller N keeps the embedder round-trips cheap while
// still averaging out single-query noise).
func (j *glmJudge) GenerateQuestions(ctx context.Context, answer string) ([]string, error) {
	if strings.TrimSpace(answer) == "" {
		return nil, nil
	}
	user := fmt.Sprintf(generateQuestionsUser, answer)
	var resp struct {
		Questions []string `json:"questions"`
	}
	if err := j.generateJSON(ctx, generateQuestionsSystem, user, &resp); err != nil {
		return nil, err
	}
	return resp.Questions, nil
}

// IsChunkRelevant is the Go-authored ContextPrecision prompt (yes/no
// per chunk in rank order).
func (j *glmJudge) IsChunkRelevant(ctx context.Context, query, chunk string) (bool, error) {
	user := fmt.Sprintf(isChunkRelevantUser, query, chunk)
	var resp struct {
		Relevant bool `json:"relevant"`
	}
	if err := j.generateJSON(ctx, isChunkRelevantSystem, user, &resp); err != nil {
		return false, err
	}
	return resp.Relevant, nil
}

// IsAttributable is the Go-authored ContextRecall prompt (yes/no per
// ground-truth statement).
func (j *glmJudge) IsAttributable(ctx context.Context, statement, contextText string) (bool, error) {
	user := fmt.Sprintf(isAttributableUser, statement, contextText)
	var resp struct {
		Attributable bool `json:"attributable"`
	}
	if err := j.generateJSON(ctx, isAttributableSystem, user, &resp); err != nil {
		return false, err
	}
	return resp.Attributable, nil
}

// ---------------------------------------------------------------------------
// StubJudge — deterministic Judge for hermetic tests.
// ---------------------------------------------------------------------------

// StubJudge is a Judge implementation that returns canned deterministic
// responses. It is consumed by TestRunnerReproducibility (SC-6) and the
// Faithfulness unit test — both need identical scores across two runs
// without burning real GLM quota.
//
// Behavior is fully controlled by the exported fields, all of which
// default to "succeed with a one-element response" so a zero-value
// StubJudge is usable. Tests override the specific fields they care
// about (e.g. ClaimsForAnswer returns a fixed list of atomic claims).
//
// The stub does NOT validate inputs — passing empty strings to a stub
// method returns the canned value as-is. This mirrors the production
// contract: the metric functions handle empty-input edge cases BEFORE
// calling the judge (e.g. Faithfulness short-circuits to 1.0 on empty
// answer without calling ExtractClaims).
type StubJudge struct {
	// ClaimsForAnswer maps an answer string to a pre-baked claim list.
	// A missing entry falls back to DefaultClaims (so tests only need
	// to override the specific cases they assert on). Tests that want
	// the stub to ALWAYS return the same list regardless of input can
	// leave the map nil and rely on DefaultClaims.
	ClaimsForAnswer map[string][]string
	DefaultClaims   []string

	// EntailsClaim maps a claim string to its entailment verdict.
	// Missing entry → DefaultEntails.
	EntailsClaim   map[string]bool
	DefaultEntails bool

	// QuestionsForAnswer maps an answer to its pre-baked question list.
	// Missing entry → DefaultQuestions.
	QuestionsForAnswer map[string][]string
	DefaultQuestions   []string

	// RelevantChunk marks a (query, chunk) pair as relevant. Missing →
	// DefaultRelevant.
	RelevantChunk   map[string]bool
	DefaultRelevant bool

	// AttributableStatement marks a statement as attributable. Missing →
	// DefaultAttributable.
	AttributableStatement map[string]bool
	DefaultAttributable   bool

	// Err, if non-nil, is returned by every method instead of a response.
	// Useful for testing error-propagation paths.
	Err error
}

// Compile-time check that *StubJudge satisfies Judge.
var _ Judge = (*StubJudge)(nil)

// ExtractClaims returns the canned claim list for the given answer.
func (s *StubJudge) ExtractClaims(_ context.Context, answer string) ([]string, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if claims, ok := s.ClaimsForAnswer[answer]; ok {
		return claims, nil
	}
	return s.DefaultClaims, nil
}

// Entails returns the canned verdict for the given claim.
func (s *StubJudge) Entails(_ context.Context, claim, _ string) (bool, error) {
	if s.Err != nil {
		return false, s.Err
	}
	if v, ok := s.EntailsClaim[claim]; ok {
		return v, nil
	}
	return s.DefaultEntails, nil
}

// GenerateQuestions returns the canned question list for the answer.
func (s *StubJudge) GenerateQuestions(_ context.Context, answer string) ([]string, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if qs, ok := s.QuestionsForAnswer[answer]; ok {
		return qs, nil
	}
	return s.DefaultQuestions, nil
}

// IsChunkRelevant returns the canned verdict for the (query, chunk) pair.
// The map key is query+"\x00"+chunk so a test can distinguish relevance
// across multiple chunks for the same query.
func (s *StubJudge) IsChunkRelevant(_ context.Context, query, chunk string) (bool, error) {
	if s.Err != nil {
		return false, s.Err
	}
	key := query + "\x00" + chunk
	if v, ok := s.RelevantChunk[key]; ok {
		return v, nil
	}
	return s.DefaultRelevant, nil
}

// IsAttributable returns the canned verdict for the statement.
func (s *StubJudge) IsAttributable(_ context.Context, statement, _ string) (bool, error) {
	if s.Err != nil {
		return false, s.Err
	}
	if v, ok := s.AttributableStatement[statement]; ok {
		return v, nil
	}
	return s.DefaultAttributable, nil
}

// ---------------------------------------------------------------------------
// Shared helpers + sentinel errors.
// ---------------------------------------------------------------------------

// ErrEmptyJudgeResponse is returned by generateJSON when the model
// returned an empty content string. Distinct from a JSON parse error so
// callers can treat "model gave up" differently from "model returned
// malformed JSON".
var ErrEmptyJudgeResponse = errors.New("eval.judge: model returned empty response")

// truncate keeps error messages bounded. Mirrors internal/server/truncate.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---------------------------------------------------------------------------
// Go-authored prompts (the 3 metrics that are NOT ported from Ragas).
// Kept here (not in metrics.go) so all judge prompts live in one place
// for review. Faithfulness's ported Ragas prompts live in faithfulness.go
// next to the metric they drive (so the eval/NOTICE attribution stays
// co-located with the ported text).
// ---------------------------------------------------------------------------

const generateQuestionsSystem = `You are an evaluator for an answer-relevance metric. Given an answer, generate plausible questions that the answer would naturally address.

Output ONLY a JSON object with this exact shape:
{"questions": ["question1", "question2", "question3"]}

Rules:
- Generate exactly 3 questions.
- Each question should be one a user might have asked to receive this answer.
- Keep each question focused on the answer's main topic.
- Output the JSON object only — no prose, no fences.`

const generateQuestionsUser = `Generate 3 plausible questions for the following answer.

Answer:
%s`

const isChunkRelevantSystem = `You are a retrieval-relevance evaluator. Given a user's query and a single retrieved text chunk, judge whether the chunk is relevant to answering the query.

Output ONLY a JSON object with this exact shape:
{"relevant": true | false}

Rule: return "relevant": true if the chunk's content addresses the query's topic (even partially); false if the chunk is unrelated. Output JSON only.`

const isChunkRelevantUser = `Query:
%s

Chunk:
%s`

const isAttributableSystem = `You are a context-attribution evaluator. Given a statement and a context, judge whether the statement can be directly attributed to (inferred from) the context.

Output ONLY a JSON object with this exact shape:
{"attributable": true | false}

Rule: return "attributable": true if the statement is directly supported by the context text; false if the statement requires information not present in the context, or contradicts it. Output JSON only.`

const isAttributableUser = `Statement:
%s

Context:
%s`

// longFormAnswerSystem / longFormAnswerUser / nliStatementsSystem /
// nliStatementsUser live in faithfulness.go — they are the VERBATIM
// ported Ragas prompts and stay co-located with the Faithfulness metric
// so the Apache-2.0 attribution is unambiguous.
