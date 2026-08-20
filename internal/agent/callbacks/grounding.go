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

// Package agentcallbacks is the Phase 11 / Plan 11-01 namespace for the
// Runner-path callbacks.Handler implementations: trust floor
// (BeforeAgent), refusal classifier (BeforeModel), and grounding
// checker (AfterModel). It is the D11-A2 trust-staircase seam — each
// Handler is registered in a known order and the Runner invokes them
// at the corresponding AgentEvent boundary.
//
// Why a new package (rather than internal/agent/callbacks.go):
//
//   - D11-A2 (trust-staircase): the three callbacks form a single
//     ordered concern (BeforeAgent → BeforeModel → AfterModel). A
//     dedicated package keeps the ordering and the per-Handler
//     registration grouped.
//   - D11-A3 (split pipeline.go into 5 files): the new file split
//     parallels the legacy agent/callbacks.go decision-emit pattern —
//     that file stays untouched (Agent.Run still uses it via the
//     react.Agent path); this new package is the Runner-path
//     equivalent.
//   - 11-01 ships seam-only: each Handler is a no-op stub that logs
//     "fired" so the trust-staircase ordering can be verified in
//     integration tests. The full behavior lands in 11-04 (when the
//     ChunkAccumulator is re-threaded through the Runner path).
package agentcallbacks

import (
	"context"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	template "github.com/cloudwego/eino/utils/callbacks"
)

// groundingCtxKey is the context.Value key that carries the
// per-Run chunk map from the BeforeModel accumulator hook to the
// AfterModel grounding checker. Populated by the chatHandler boundary
// (internal/server/runner_setup.go) for the 11-04 implementation.
//
// 11-01 ships seam-only: the key exists so the AfterModel stub can be
// parameterized by the chatHandler in a follow-up plan without breaking
// the Handler signature.
type groundingCtxKey struct{}

// NewGroundingHandler returns a callbacks.Handler that fires on the
// ChatModel AfterModel seam (D11-A2 layer 3 — D2-01 grounding checker
// reusing internal/verdict/grounding.go::Normalize).
//
// 11-01 ships a no-op stub: the handler logs "grounding.AfterModel
// fired". The stub exists so the trust-staircase ordering is
// verifiable in a future integration test; the full behavior lands
// in 11-04 when the ChunkAccumulator is re-threaded through the
// Runner path (the verdict.GroundingChecker needs chunksByID, which
// the chatHandler boundary computes from the accumulator's
// TierMatched map).
//
// The OnEnd signature follows eino@v0.9.14
// utils/callbacks/template.go:525-528 (ModelCallbackHandler.OnEnd).
//
// D-04 / D15 invariant preservation — the Handler does NOT emit any
// SSE chunk in 11-01. Grounding happens at the chatHandler boundary
// post-drainRunnerEvents (the verdict is parsed from the model's final
// message, then GroundingChecker mutates it, then writeAuditAndTurn
// persists). The AfterModel callback is the seam where 11-04 will
// re-thread the ChunkAccumulator so grounding can happen inline.
func NewGroundingHandler() callbacks.Handler {
	modelHandler := &template.ModelCallbackHandler{
		OnEnd: func(ctx context.Context, _ *callbacks.RunInfo, _ *model.CallbackOutput) context.Context {
			log.Printf("[INFO] agentcallbacks: grounding.AfterModel fired (11-01 no-op stub; 11-04 wires ChunkAccumulator)")
			return ctx
		},
	}
	return template.NewHandlerHelper().
		ChatModel(modelHandler).
		Handler()
}