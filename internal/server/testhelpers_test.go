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
	"encoding/json"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/verdict"
)

// stubEmbedder is the test double for the Embedder seam. Returns a
// 1024-dim constant vector regardless of input text — the indexer
// tests prove the embed call is plumbed correctly; the pipeline tests
// only need a fixed vector so the search step has something to score
// against.
type stubEmbedder struct {
	vec   []float32
	calls []string
	mu    sync.Mutex
}

func newStubEmbedder() *stubEmbedder {
	v := make([]float32, 1024)
	v[0] = 1.0
	return &stubEmbedder{vec: v}
}

// Compile-time check that stubEmbedder implements embedder.Embedder.
var _ embedder.Embedder = (*stubEmbedder)(nil)

func (e *stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, text)
	return e.vec, nil
}

// Dimensions returns the embedder output dimensionality (matches
// embedder.Embedder interface).
func (e *stubEmbedder) Dimensions() int { return 1024 }

// CallCount returns the number of embed invocations (for assertion
// symmetry with the runner counter).
func (e *stubEmbedder) CallCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

// LastQuery returns the most recent embed query (or empty if none).
func (e *stubEmbedder) LastQuery() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.calls) == 0 {
		return ""
	}
	return e.calls[len(e.calls)-1]
}

// stubRetriever is the test double for the Retriever seam. Returns
// whatever chunks were pre-loaded; tests assert the tier list the
// pipeline passed.
type stubRetriever struct {
	chunks []retriever.Hit
	err    error
	lastTop int
	// capturedTiers records every tier slice the pipeline passed.
	// TestPersonalTierRefusal asserts on the latest tier list.
	capturedTiers [][]string
	mu            sync.Mutex
}

// Compile-time check that stubRetriever implements retriever.Retriever.
var _ retriever.Retriever = (*stubRetriever)(nil)

func (s *stubRetriever) Health(_ context.Context) error { return nil }

func (s *stubRetriever) Search(_ context.Context, _ []float32, tiers []string, topK int, _ int64) ([]retriever.Hit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Copy the tier slice so later assertions don't see downstream mutation.
	tCopy := make([]string, len(tiers))
	copy(tCopy, tiers)
	s.capturedTiers = append(s.capturedTiers, tCopy)
	s.lastTop = topK
	if s.err != nil {
		return nil, s.err
	}
	return s.chunks, nil
}

func (s *stubRetriever) Ensure(_ context.Context) error { return nil }
func (s *stubRetriever) Insert(_ context.Context, _ []retriever.Document) error {
	return nil
}
func (s *stubRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil, nil
}
func (s *stubRetriever) Disconnect() error { return nil }

// LastTiers returns the most recent tier list (or nil if none).
func (s *stubRetriever) LastTiers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.capturedTiers) == 0 {
		return nil
	}
	return s.capturedTiers[len(s.capturedTiers)-1]
}

// CallCount returns the number of search invocations (for refusal-path
// assertions: the refusal tests expect CallCount==1 even though the
// runner is NEVER called).
func (s *stubRetriever) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.capturedTiers)
}

// stubRunner is the test double for the Runner seam. Each call records
// the options slice (for TestTemperaturePassthrough) + emits a canned
// AgentEvent iterator carrying the JSON verdict string supplied at
// construction time.
//
// CallCount supports refusal-by-construction assertions: when the
// pipeline short-circuits (0 chunks, low score), the runner's
// CallCount must remain 0.
type stubRunner struct {
	// cannedJSON is the verdict string the iterator yields. Set once at
	// construction; the runner does not parse it — the pipeline does.
	cannedJSON string

	// capturedOpts records the options slice from each Query call.
	// TestTemperaturePassthrough asserts WithTemperature(skill.Temperature)
	// appeared in one of these.
	capturedOpts [][]adk.AgentRunOption

	// capturedQueries records the user-message strings.
	capturedQueries []string

	count int
	mu    sync.Mutex
}

func newStubRunner(cannedJSON string) *stubRunner {
	return &stubRunner{cannedJSON: cannedJSON}
}

func (r *stubRunner) Query(_ context.Context, query string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	// Copy opts to detach from the caller's stack frame.
	optsCopy := make([]adk.AgentRunOption, len(opts))
	copy(optsCopy, opts)
	r.capturedOpts = append(r.capturedOpts, optsCopy)
	r.capturedQueries = append(r.capturedQueries, query)

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	// Build one AgentEvent with the canned verdict as the assistant
	// message content. Phase 2 single-shot verdict → exactly one event.
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: r.cannedJSON,
	}
	ev := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming: false,
				Role:        schema.Assistant,
				Message:     msg,
			},
		},
	}
	gen.Send(ev)
	gen.Close()
	return iter
}

func (r *stubRunner) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// LastOpts returns the most recent options slice (for option-shape
// assertions). nil if the runner was never called.
func (r *stubRunner) LastOpts() []adk.AgentRunOption {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.capturedOpts) == 0 {
		return nil
	}
	return r.capturedOpts[len(r.capturedOpts)-1]
}

// CapturedQueries returns a copy of the recorded user-message strings
// the runner received. Used by TestTemperaturePassthrough as a sanity
// check that the user query reached the LLM call.
func (r *stubRunner) CapturedQueries() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.capturedQueries))
	copy(out, r.capturedQueries)
	return out
}

// cannedVerdictJSON returns a JSON-encoded Verdict string for use as
// stubRunner input. Tests use it to build the mock LLM response.
func cannedVerdictJSON(status string, confidence float64, chunkIDs []string, span string, tier string) string {
	cits := make([]verdict.Citation, 0, len(chunkIDs))
	for _, id := range chunkIDs {
		cits = append(cits, verdict.Citation{ChunkID: id, Span: span, Tier: tier})
	}
	v := verdict.Verdict{
		Status:         status,
		Confidence:     confidence,
		Citations:      cits,
		ReviewRequired: confidence < 0.9,
	}
	b, _ := json.Marshal(v)
	return string(b)
}