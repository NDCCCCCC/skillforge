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

// Package wire — types.go is Phase 12 / Plan 12-05 Task 4: the home of
// the decision VALUE constants for the DecisionPayload.Decision string
// enum (D16 overload pattern, extended from Phase 3 / Phase 11).
//
// P12-L3 lock: forced_stop rides on the EXISTING `data-decision`
// discriminator. It is a NEW VALUE in the Decision string enum, NOT a
// new wire chunk type — the ValidTypes count stays unchanged (23;
// the chunk_test.go counter asserts this — the discriminator-count
// grep lock lives there), the wire union is untouched, and the TS
// mirror (web/src/types/wire-types.ts) needs only a new case in its
// decision switch. 12-07 ships the FE half of the paired commit under
// the D15 lockstep invariant.
//
// Why a wire-level constant when internal/agent/callbacks.go already
// declares agent.DecisionForcedStop? The wire package is the
// wire-contract single source of truth; internal/agent stays
// decoupled from Hertz-aware code (see callbacks.go's DecisionRecord
// rationale). The two constants carry the same string value — the
// agent-side emitter copies the wire constant at its 12-07 wiring
// site (an in-package wire test cannot import internal/agent to lock
// the equality directly: agent imports wire, which would be a
// test-only import cycle).
package wire

// Decision enum values — the `decision` string field on
// DecisionPayload. The field is string-typed on purpose: extending
// the catalog is additive (old consumers ignore unknown values; new
// consumers switch on the ones they know). NEVER repurpose an
// existing value; add a new constant + a FE mirror case instead.
//
// Current catalog (emit sites in parentheses):
//
//	retrieve                — agent retrieve hop (Phase 3)
//	answer                  — agent final answer hop (Phase 3)
//	refuse                  — refusal classifier (Phase 3)
//	forced_stop             — MaxStep / budget gate (Phase 3 + 12-05)
//	route                   — skill router (D5-04)
//	counter_evidence        — 历史-tier conflicts (D5-13)
//	history_projected       — Phase 10 D10-B4
//	history_budget_truncated— Phase 10 D10-B4
//	history_query_rewritten — Phase 10 D10-C3
//	history_classifier_refused — Phase 10 D10-C3
//	tool-resumed            — HITL-04 approval resolution (Phase 11)
//	tool-denied             — HITL-04 approval resolution (Phase 11)
//
// 12-05 consumers: the budget middleware's emitter seam
// (agentcallbacks.BudgetEmitter) writes
// DecisionPayload{Step: 0, Decision: DecisionForcedStop, Query: <the
// "PLAN-04 budget exhausted: <layer>" reason>} — the reason rides the
// Query field per the Phase 10 transparency convention (Query carries
// human-readable context for non-retrieve decisions), NOT a new
// Reason field (adding one would churn the D15 TS mirror for no
// parsing gain).
const (
	// DecisionForcedStop is the PLAN-04 budget-gate terminal decision.
	// Emitted when ANY of the 3 layers trips (budget_layer: max_iter |
	// token | loop, mirrored on the action_budget_triggered audit
	// row). Conservative by contract: the gate fires BEFORE the run
	// reaches exhaustion, never after.
	DecisionForcedStop = "forced_stop"
)
