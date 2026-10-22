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

// Package stubchat is the Phase 49 stub LLM (D-07 / SC3): when no LLM API
// key is configured it stands in for the real chat model on
// Deps.PipelineChatModel, so the production verdict pipeline runs
// UNCHANGED — parse → verdict.Validate → verdict.GroundingCheck →
// GroundingChecker all execute exactly as with a real model. This package
// exempts NOTHING from those checks (D-10): its output is a deterministic
// verdict JSON whose Answer and citation Span are verbatim substrings of
// the retrieved chunk bodies, which is precisely why the real grounding
// moat still verifies it.
//
// Shape precedent: internal/knowledge/ecl/stub.go — deterministic rule
// extraction kept strictly separate from model calls. This package is NOT
// a vector fallback (D-01 forbids those); it only replaces the LLM step.
//
// Stub is deliberately zero-field and immutable: concurrent Stream calls
// cannot race (TestStub_ConcurrentStreamByteIdentical), and it implements
// model.BaseChatModel ONLY — no WithTools — so the type system itself
// prevents wiring it into tool-calling consumers (rewriter / classifier /
// wizard), turning a mis-wiring into a compile error instead of a runtime
// surprise.
package stubchat

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Stub is the deterministic LLM stand-in. Zero fields on purpose: no
// mutable state ⇒ no locks ⇒ trivially concurrency-safe.
type Stub struct{}

// New returns the singleton-shaped stub (still cheap to construct).
func New() *Stub { return &Stub{} }

// userOf returns the Content of the last User message in msgs — pipeline
// builds [System, ...history, User] and renders the evidence blocks into
// the user message (buildPrompt).
func userOf(msgs []*schema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == schema.User {
			return msgs[i].Content
		}
	}
	return ""
}

// stubJSON deterministically renders the verdict for the user prompt and
// marshals it. Marshal failure is a real programming error (the shape is
// static) — propagate, never swallow.
func stubJSON(user string) (string, error) {
	raw, err := json.Marshal(renderVerdict(extractRefBlocks(user)))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// chunkSize is the rune-slice size per streamed token.
const chunkSize = 64

// Stream returns the stub verdict as a token stream. NEVER empty: a
// zero-token stream is judged an upstream failure by pipeline.streamOnce
// and triggers 4 backoff retries. Tokens are rune-sliced (NOT byte-sliced):
// the JSON contains Chinese text, and cutting a multi-byte rune would make
// the concatenation invalid UTF-8 and fail json.Unmarshal downstream.
// StreamReaderFromArray (not Pipe+goroutine): the stub output is fully
// deterministic, so an array-backed reader gives the same token-by-token
// Recv semantics with zero concurrency — no scheduler interleaving can
// truncate the stream.
func (s *Stub) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	raw, err := stubJSON(userOf(msgs))
	if err != nil {
		return nil, err
	}
	runes := []rune(raw)
	tokens := make([]*schema.Message, 0, (len(runes)+chunkSize-1)/chunkSize)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		tokens = append(tokens, schema.AssistantMessage(string(runes[i:end]), nil))
	}
	return schema.StreamReaderFromArray(tokens), nil
}

// Generate returns the same verdict JSON as a single Assistant message;
// byte-identical to the Stream concatenation (TestStub_GenerateMatchesStream).
func (s *Stub) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	raw, err := stubJSON(userOf(msgs))
	if err != nil {
		return nil, err
	}
	return schema.AssistantMessage(raw, nil), nil
}

// Compile-time interface assertion: BaseChatModel only — see the package
// comment for why WithTools is intentionally absent.
var _ model.BaseChatModel = (*Stub)(nil)
