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

// Package agentcallbacks (continued) — classifier.go is the
// D11-A2 trust-staircase layer 2: the refusal-classifier BeforeModel
// seam.
//
// The classifier wrapper satisfies
// internal/server/classifier.RefusalClassifier.Classify(ctx,
// RefusalClassifierInput{Query, Chunks}) (no History field — D10-A1).
// 11-01 ships a seam-only stub: NewClassifierHandler returns a
// callbacks.Handler whose OnStart logs "classifier.BeforeModel fired"
// + delegates to the wrapper via the supplied accessor funcs. The
// real call shape lives in 11-04 when the Runner path is fully wired.
package agentcallbacks

import (
	"context"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	template "github.com/cloudwego/eino/utils/callbacks"

	"github.com/skillforge/skillforge/internal/server/classifier"
)

// ClassifierRefusalFunc is the seam Deps.RefusalClassifier satisfies
// (the wrapper struct lives in internal/server/classifier; same shape).
// Defined here so callers can build a ClassifierRefusalFunc from any
// implementation without importing internal/server.
type ClassifierRefusalFunc func(ctx context.Context, in classifier.RefusalClassifierInput) (classifier.RefusalClassifierOutput, error)

// QueryAccessor returns the current turn's rewritten query for the
// classifier wrapper. The Callback input shape on v0.9.14 does NOT
// carry the query text (it flows through schema.Message in the
// prompt, not through the CallbackInput struct — see eino v0.9.14
// utils/callbacks/template.go:530-535). The accessor is the
// per-request closure that bridges from the chatHandler boundary.
//
// The accessor pattern mirrors accCtxKey / emitCtxKey in
// internal/agent/agent.go (Phase 3 used the same ctx-injection
// pattern for the chunk accumulator and the per-request DecisionEmitFunc).
type QueryAccessor func() string

// ChunksAccessor returns the current turn's rendered [ref-N] DATA
// block slice for the classifier wrapper. Mirrors QueryAccessor (the
// Callback input shape does not carry the chunks).
type ChunksAccessor func() []string

// classifierCtxKey is the unexported context.Value key the OnStart
// handler uses to stash the classifier result for the AfterModel
// handler to read. Populated by OnStart, read by the chatHandler
// boundary post-drainRunnerEvents.
type classifierCtxKey struct{}

// NewClassifierHandler returns a callbacks.Handler that fires on the
// ChatModel BeforeModel seam (D10-A1 / D11-A2 — refusal classifier
// BEFORE the verdict LLM call).
//
// 11-01 ships a seam-only stub: OnStart logs "classifier.BeforeModel
// fired" + delegates to the supplied ClassifierRefusalFunc. The
// actual Decision=="refuse" cancellation logic lands in 11-04 (the
// chatHandler boundary will inspect the stashed result and short-
// circuit to a refusal emit).
//
// CurrentQuery + currentChunks are accessor functions because the
// Callback input shape on v0.9.14 does NOT carry the query text or
// retrieved chunks (those flow through schema.Message in the prompt,
// not through the CallbackInput struct — see eino v0.9.14
// utils/callbacks/template.go:530-535). The accessor functions close
// over the chatHandler's per-request state.
func NewClassifierHandler(c ClassifierRefusalFunc, currentQuery QueryAccessor, currentChunks ChunksAccessor) callbacks.Handler {
	modelHandler := &template.ModelCallbackHandler{
		OnStart: func(ctx context.Context, _ *callbacks.RunInfo, _ *model.CallbackInput) context.Context {
			if c == nil {
				// No wrapper wired → leave the seam inert (the
				// chatHandler boundary will still run the wrapper
				// inline at its existing Step 7 position). 11-04
				// replaces this branch with the cancellation hook.
				log.Printf("[INFO] agentcallbacks: classifier.BeforeModel fired (no wrapper wired; 11-01 no-op)")
				return ctx
			}
			q := ""
			if currentQuery != nil {
				q = currentQuery()
			}
			var chunks []string
			if currentChunks != nil {
				chunks = currentChunks()
			}
			out, err := c(ctx, classifier.RefusalClassifierInput{Query: q, Chunks: chunks})
			if err != nil {
				log.Printf("[WARN] agentcallbacks: classifier.BeforeModel: Classify error (non-fatal, 11-01 seam-only): %v", err)
				return ctx
			}
			if out.Decision == "refuse" {
				log.Printf("[INFO] agentcallbacks: classifier.BeforeModel refused (reason=%s, confidence=%.2f) — 11-04 wires ctx-cancel",
					out.Reason, out.Confidence)
			}
			// Stash the result for the chatHandler boundary to read
			// post-drainRunnerEvents. 11-01 stores via context; 11-04
			// will read it back when wiring the cancellation hook.
			return context.WithValue(ctx, classifierCtxKey{}, out)
		},
	}
	return template.NewHandlerHelper().
		ChatModel(modelHandler).
		Handler()
}