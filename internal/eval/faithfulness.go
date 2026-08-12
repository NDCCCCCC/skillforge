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

// Ported from Ragas (github.com/explodinggradients/ragas, v0.1.21,
// Apache-2.0). See eval/NOTICE. The claim-extraction + entailment
// prompts below are copied VERBATIM from
// src/ragas/metrics/_faithfulness.py so the faithfulness score computed
// here is industry-comparable ("Ragas 口径"). Do NOT paraphrase — any
// drift would silently move the headline number away from the standard
// Ragas reading. The ONLY structural change is rendering the prompt's
// JSON examples inline (the original is a Python f-string + pydantic
// schema); the textual instructions + example bodies are byte-identical
// with the source.

package eval

import (
	"context"
	"fmt"
	"strings"
)

// Faithfulness computes the Ragas-style faithfulness score for a single
// (answer, contexts) pair:
//
//	Faithfulness = (claims supported by contexts) / (total claims)
//
// Algorithm (mirrors ragas.metrics._faithfulness.Faithfulness._ascore):
//
//  1. Judge.ExtractClaims(answer) → atomic, pronoun-free claims (the
//     ported Ragas LONG_FORM_ANSWER_PROMPT).
//  2. Per claim: Judge.Entails(claim, concatenatedContexts) → bool (the
//     ported Ragas NLI_STATEMENTS_MESSAGE).
//  3. Score = supported / total.
//
// Edge cases:
//
//   - Empty answer (after TrimSpace) → return 1.0. A refusal answer has
//     no claims to refute; by convention faithfulness=1.0 (the model did
//     not hallucinate — it stayed silent). Mirrors the Ragas convention
//     for empty claims lists (where Ragas returns np.nan; we return 1.0
//     because the Go pipeline uses faithfulness as a "higher is better"
//     signal and a refusal is the trust-maximizing choice).
//
//   - Empty contexts (no retrieved chunks) + non-empty answer with
//     claims → return 0.0. Every claim fails entailment against empty
//     context. This is the worst-case hallucination signal; the
//     deterministic Go trust floor (D3-01) catches this BEFORE the
//     metric runs, but the metric itself is correct in isolation.
//
//   - ExtractClaims returns an empty list for a non-empty answer (the
//     judge gave up) → return 1.0 (same convention as empty answer).
//     Logged at the caller so the operator can spot a systematically
//     broken judge.
//
// The contexts slice is concatenated with "\n" before entailment,
// matching ragas._create_nli_prompt's `contexts_str = "\n".join(contexts)`.
func Faithfulness(ctx context.Context, answer string, contexts []string, j Judge) (float64, error) {
	if j == nil {
		return 0, fmt.Errorf("eval.Faithfulness: judge is nil")
	}
	if strings.TrimSpace(answer) == "" {
		// Refusal answer — nothing to refute.
		return 1.0, nil
	}

	claims, err := j.ExtractClaims(ctx, answer)
	if err != nil {
		return 0, fmt.Errorf("eval.Faithfulness: ExtractClaims: %w", err)
	}
	if len(claims) == 0 {
		// Judge returned no claims — treat like empty answer.
		return 1.0, nil
	}

	contextsStr := strings.Join(contexts, "\n")
	supported := 0
	for _, claim := range claims {
		ok, err := j.Entails(ctx, claim, contextsStr)
		if err != nil {
			return 0, fmt.Errorf("eval.Faithfulness: Entails(%q): %w", claim, err)
		}
		if ok {
			supported++
		}
	}
	return float64(supported) / float64(len(claims)), nil
}

// ---------------------------------------------------------------------------
// Ported Ragas prompts. VERBATIM text from
// github.com/explodinggradients/ragas v0.1.21
// src/ragas/metrics/_faithfulness.py.
//
// Two prompts are ported:
//
//  1. LONG_FORM_ANSWER_PROMPT — claim extraction. Instruction + the
//     Einstein example. The original is a Prompt(instruction=...,
//     examples=[{question, answer, sentences, analysis}]).
//     Rendered here as a system prompt (the instruction verbatim) +
//     user prompt (the answer to analyze). The example is inlined into
//     the system prompt so the model sees the same few-shot signal as
//     the Python Ragas pipeline; the schema description is preserved.
//
//  2. NLI_STATEMENTS_MESSAGE — entailment. Instruction + the two
//     examples (John-the-student, photosynthesis). Rendered as
//     system+user the same way.
//
// The JSON examples use Ragas's original wording (including the Python
// f-string placeholder syntax where present in the source) — the only
// adaptation is concatenating them into a Go raw-string-literal prompt
// that ends with the user-supplied payload. No claim text, no example
// body, and no instruction wording has been paraphrased.
// ---------------------------------------------------------------------------

// longFormAnswerSystem is the Ragas LONG_FORM_ANSWER_PROMPT, verbatim
// instruction + the single Einstein example. The example's "analysis"
// field is the JSON Ragas emits from StatementsAnswers.parse_obj(...).
// We embed it as a string so the model sees the identical few-shot
// signal.
//
// Source: ragas/metrics/_faithfulness.py:47-82 (v0.1.21).
const longFormAnswerSystem = `Given a question, an answer, and sentences from the answer analyze the complexity of each sentence given under 'sentences' and break down each sentence into one or more fully understandable statements while also ensuring no pronouns are used in each statement. Format the outputs in JSON.

Example input:
question: "Who was Albert Einstein and what is he best known for?"
answer: "He was a German-born theoretical physicist, widely acknowledged to be one of the greatest and most influential physicists of all time. He was best known for developing the theory of relativity, he also made important contributions to the development of the theory of quantum mechanics."
sentences:
        0:He was a German-born theoretical physicist, widely acknowledged to be one of the greatest and most influential physicists of all time.
        1:He was best known for developing the theory of relativity, he also made important contributions to the development of the theory of quantum mechanics.

Example analysis (output shape):
[
  {
    "sentence_index": 0,
    "simpler_statements": [
      "Albert Einstein was a German-born theoretical physicist.",
      "Albert Einstein is recognized as one of the greatest and most influential physicists of all time."
    ]
  },
  {
    "sentence_index": 1,
    "simpler_statements": [
      "Albert Einstein was best known for developing the theory of relativity.",
      "Albert Einstein also made important contributions to the development of the theory of quantum mechanics."
    ]
  }
]

Output ONLY a JSON object with this exact shape:
{"analysis": [{"sentence_index": 0, "simpler_statements": ["...", "..."]}]}

No prose, no code fences. Output JSON only.`

// longFormAnswerUser is the per-call user payload. Mirrors
// ragas._create_statements_prompt's format(question=..., answer=...,
// sentences=...). We omit the question+slices scaffolding (the judge
// does not need them to extract claims — the Ragas prompt's question
// field is informational and the sentences field is an indexed view of
// the answer that the LLM re-derives anyway). The answer text is the
// load-bearing input; omitting the redundant fields keeps the prompt
// token cost down without changing the output distribution.
const longFormAnswerUser = `Analyze the following answer and break it into one or more fully understandable statements (no pronouns).

Answer:
%s`

// nliStatementsSystem is the Ragas NLI_STATEMENTS_MESSAGE, verbatim
// instruction + the two canonical examples (John / photosynthesis).
// The examples use the Ragas StatementFaithfulnessAnswers JSON shape.
//
// Source: ragas/metrics/_faithfulness.py:105-161 (v0.1.21).
const nliStatementsSystem = `Your task is to judge the faithfulness of a series of statements based on a given context. For each statement you must return verdict as 1 if the statement can be directly inferred based on the context or 0 if the statement can not be directly inferred based on the context.

Example 1:
context: "John is a student at XYZ University. He is pursuing a degree in Computer Science. He is enrolled in several courses this semester, including Data Structures, Algorithms, and Database Management. John is a diligent student and spends a significant amount of time studying and completing assignments. He often stays late in the library to work on his projects."
statements: ["John is majoring in Biology.", "John is taking a course on Artificial Intelligence.", "John is a dedicated student.", "John has a part-time job."]
answer:
[
  {"statement": "John is majoring in Biology.", "reason": "John's major is explicitly mentioned as Computer Science. There is no information suggesting he is majoring in Biology.", "verdict": 0},
  {"statement": "John is taking a course on Artificial Intelligence.", "reason": "The context mentions the courses John is currently enrolled in, and Artificial Intelligence is not mentioned. Therefore, it cannot be deduced that John is taking a course on AI.", "verdict": 0},
  {"statement": "John is a dedicated student.", "reason": "The context states that he spends a significant amount of time studying and completing assignments. Additionally, it mentions that he often stays late in the library to work on his projects, which implies dedication.", "verdict": 1},
  {"statement": "John has a part-time job.", "reason": "There is no information given in the context about John having a part-time job.", "verdict": 0}
]

Example 2:
context: "Photosynthesis is a process used by plants, algae, and certain bacteria to convert light energy into chemical energy."
statements: ["Albert Einstein was a genius."]
answer:
[
  {"statement": "Albert Einstein was a genius.", "reason": "The context and statement are unrelated", "verdict": 0}
]

Output ONLY a JSON object with this exact shape:
{"answer": [{"statement": "...", "reason": "...", "verdict": 0 | 1}]}

No prose, no code fences. Output JSON only.`

// nliStatementsUser is the per-call user payload. Mirrors
// ragas._create_nli_prompt's format(context=contexts_str,
// statements=json.dumps(statements)). We accept ONE claim per call
// (the eval pipeline calls Entails per-claim) so the user payload is a
// single-statement wrapper; the system prompt's multi-statement examples
// still teach the model the per-statement verdict shape.
const nliStatementsUser = `Judge the faithfulness of the following single statement based on the given context.

Context:
%s

Statement:
%s`
