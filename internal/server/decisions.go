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

package server

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/wire"
)

// emitHistoryDecision writes one data-decision SSE chunk carrying a
// Phase 10 history-pipeline event. Mirrors emitDecision (chat.go) but
// is dedicated to the 4 history decision-string constants in wire.go:
//
//   - wire.DecisionHistoryProjected        (history load + sanitize)
//   - wire.DecisionHistoryBudgetTruncated  (char-budget window dropped ≥1 msg)
//   - wire.DecisionHistoryQueryRewritten   (rewriter LLM produced output)
//   - wire.DecisionHistoryClassifierRefused (refusal classifier short-circuits)
//
// Best-effort by design (D10-B4 + D10-C3): these are observability +
// FE transparency events, NOT trust-bearing decisions. A marshal / emit
// failure is logged and swallowed — the verdict / refusal path
// propagates errors through its own emit surface, so a dead socket is
// already detected at the trust-bearing emit sites.
//
// Phase 10 FE mirror (Plan 10-03):
//
//   - HistoryRewriteHint   renders DecisionHistoryQueryRewritten as a
//     small chip above the message ("已理解为: <rewritten_query>")
//   - HistoryBudgetChip    renders DecisionHistoryBudgetTruncated as a
//     chip showing the dropped message count
//   - HistoryAuditMeta     silently ignores history_projected and
//     history_classifier_refused (they exist for the BE audit_meta row,
//     not for the FE)
//
// The function is intentionally non-exported (lowercase) — only the
// VerdictPipeline inside this package emits history decisions, and the
// godoc + the package wiring make the call surface obvious.
func (s *Deps) emitHistoryDecision(c *app.RequestContext, p wire.DecisionPayload) {
	if s == nil || c == nil {
		return
	}
	payload, err := json.Marshal(p)
	if err != nil {
		log.Printf("[WARN] server: marshal history decision %+v: %v (non-fatal)", p, err)
		return
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload}); err != nil {
		log.Printf("[WARN] server: emit history decision %s: %v (non-fatal)", p.Decision, err)
	}
}

// _ keeps fmt referenced when the helper signature evolves.
var _ = fmt.Sprintf