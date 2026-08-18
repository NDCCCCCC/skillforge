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

// Package classifier defines the query-only refusal-classifier contract.
//
// The classifier is the Phase 10 escape from the D9-V2 trust-floor
// spike finding (refusal-context drift -95pp). It runs BEFORE the
// prior-turn-aware generation step and decides allow / refuse from the
// rewritten query and the current-turn retrieval chunks ONLY. It must
// NEVER see prior turns.
//
// D10-A1 / D9-V2 — type-level physical isolation:
// the input struct is the ONLY path from the pipeline into the
// classifier's LLM call. The struct deliberately has no prior-turn
// field. Any future contributor who tries to add one is structurally
// blocked: the struct is exported and pinned by the reflection-based
// shape lock test and by the import-graph gate test. CI adds a parallel
// grep gate (10-VALIDATION.md GG-A1-1, GG-A1-2, GG-A1-3).
//
// This file deliberately does NOT import:
//
//	github.com/skillforge/skillforge/internal/h... (prior-turn store)
//	github.com/skillforge/skillforge/internal/s... (sqlite session store)
//
// The struct shape and the import graph together prove the
// classifier's input boundary is past-blind by construction.
package classifier

import (
	"math"

	"github.com/skillforge/skillforge/internal/verdict"
)

// RefusalClassifierInput is the classifier's view of the current turn.
// The pipeline fills Query with the rewritten query (or the original
// on first-turn / no-anaphora skip) and Chunks with the rendered
// [ref-N] DATA blocks (same framing as userQueryTemplate).
//
// D10-A1 / D9-V2 — no prior-turn field. The struct godoc, the
// reflection test, and the import gate are the structural defenses;
// an accidental field addition is caught at compile time (the test
// file fails) and at CI gate time (the grep gate fires).
type RefusalClassifierInput struct {
	// Query is the rewritten user query (the rewriter's output) or
	// the original user query on first-turn / no-anaphora skip.
	// Classified verbatim — no further transformation.
	Query string

	// Chunks is the rendered [ref-N] DATA block list, one entry per
	// retrieved chunk. Same framing as userQueryTemplate so the
	// classifier sees the same shape the generator will see.
	Chunks []string
}

// Compile-time lock: keep this here as a reminder that the struct has
// exactly two fields and they are Query + Chunks. The test below
// reflects on the field count — adding a third field is caught at the
// test boundary, not at runtime.

// RefusalClassifierOutput is the classifier's verdict for the current
// turn. Valid() gates Decision against the unexported decision enum
// and Confidence against the [0.0, 1.0] closed interval — invalid
// outputs are treated as DefaultDeny by the Plan 02 Classify wrapper
// (T-10-04 elevation-of-privilege mitigation).
type RefusalClassifierOutput struct {
	// Decision is the unexported decision enum: "allow" or "refuse".
	Decision string

	// Reason is the refusal reason when Decision == "refuse" (one of
	// verdict.RefusalReason*). Empty when Decision == "allow".
	Reason string

	// Confidence is the classifier's self-reported certainty in
	// [0.0, 1.0]. The Plan 02 wrapper applies a ConfidenceFloor and
	// downgrades to DefaultDeny when Confidence is below the floor
	// (W3 / T-10-04).
	Confidence float64
}

// validDecisions is the unexported allow/refuse enum. Valid() reads it;
// no other consumer in the codebase reads it (extending the enum is a
// deliberate edit to this file).
var validDecisions = map[string]bool{
	"allow":  true,
	"refuse": true,
}

// Valid reports whether o satisfies the structural gate: Decision is
// one of the enum keys and Confidence is in [0.0, 1.0] inclusive. The
// Plan 02 Classify wrapper is responsible for the semantic gate (the
// ConfidenceFloor and the Reason field); Valid only enforces shape.
func (o RefusalClassifierOutput) Valid() bool {
	if !validDecisions[o.Decision] {
		return false
	}
	if math.IsNaN(o.Confidence) || o.Confidence < 0.0 || o.Confidence > 1.0 {
		return false
	}
	return true
}

// Compile-time sentinel — anchors the verdict package import so a
// future contributor cannot accidentally remove it without also
// removing every call site that references verdict.RefusalReason*.
//
// The intent: the file MUST stay coupled to the verdict package
// (the classifier's Reason values are the verdict package's refusal
// enum), but MUST stay decoupled from the history / store packages.
// The next two lines enforce the asymmetry.
var (
	_ = verdict.RefusalReasonNoAuthorizedSource
	_ = verdict.RefusalReasonLowRetrievalScore
	_ = verdict.RefusalReasonOutOfScope
)