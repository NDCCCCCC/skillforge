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
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/wire"
)

// emitCounterEvidence is the SC-6 / CMPL-06 post-verdict annotation pass
// (Plan 05-06, D5-13). It runs an INDEPENDENT retrieval pass against the
// 历史 tier and emits the contradicting chunks as ANNOTATION — they
// NEVER enter the verdict's chunksByID grounding map.
//
// THE D5-13 TRUST MOAT LIVES IN THE FUNCTION SIGNATURE.
//
// This method receives ONLY (ctx, c, qvec, now). It does NOT receive the
// verdict's chunksByID map. There is no parameter through which 历史
// chunks could leak into grounding. A future "simplification" that adds
// a chunksByID parameter here is a TRUST-BOUNDARY BREACH — the
// regression TestCounterEvidence_NotGrounded catches it at the runtime
// layer, but the compiler is the primary defense (the signature refuses
// the leak).
//
// Why a separate pass instead of extending `reads` to [权威, 历史]?
//
//   - Mixing 历史 into `reads` would put 历史 chunks into the SAME
//     chunksByID map that GroundingChecker (D2-01) consumes; the
//     verdict's trust floor (D-09) would no longer be "权威 only".
//   - Per D5-13: counter-evidence is annotation, not grounding. The
//     verdict continues to read ONLY 权威. The 历史 tier surfaces
//     AFTER the verdict is finalized, with a visible disclaimer ("not
//     part of the verdict") so the operator sees the agent's known
//     contradictions without confusing them with the verdict's basis.
//
// Wire shape: ONE data-decision chunk with decision="counter_evidence"
// carrying conflictChunks []wire.ConflictChunk in the payload. Reuses
// the existing wire type (D5-04 extended data-decision for exactly this
// kind of structured annotation — no new wire type per D5-14 budget).
//
// Error handling: best-effort. If Search fails (历史 tier outage,
// network blip), the error is returned to the caller, who logs and
// continues — the verdict still stands. The annotation is non-authoritative
// (UI-SPEC §SC-6 error state). Audit row records the absence of
// counter-evidence.
//
// Threat model: T-5-25 (moat erosion) mitigated by signature + the
// regression test. T-5-30 (repudiation / silent skip) accepted —
// retriever errors log; the verdict stands; audit row records the
// missing counter-evidence.
func (s *Deps) emitCounterEvidence(ctx context.Context, c *app.RequestContext, qvec []float32, now int64) error {
	// Retrieve against the 历史 tier ONLY. A leak to 权威 / 个人 here
	// would route grounded material into the annotation channel and
	// break the moat — TestCounterEvidence_PassesHistoryTierOnly pins
	// the literal tier slice via stubRetriever.LastTiers().
	hits, err := s.Retriever.Search(ctx, qvec, []string{"历史"}, 5, now)
	if err != nil {
		// Non-fatal — the verdict stands. The caller (VerdictPipeline)
		// logs the error and continues. NO counter_evidence chunk is
		// emitted on this path (TestCounterEvidence_RetrieverErrorIsNonFatal).
		return fmt.Errorf("counter-evidence: search 历史 tier: %w", err)
	}

	// Build the conflict-chunk payload. Snippets are bounded so a 4KB
	// chunk does not bloat the SSE frame — 200 runes is enough for the
	// FE row to render a meaningful one-line preview (the operator can
	// click through to ChunkSourceView for the full text).
	conflicts := make([]wire.ConflictChunk, 0, len(hits))
	for _, h := range hits {
		conflicts = append(conflicts, wire.ConflictChunk{
			ChunkID:       strconv.FormatInt(h.ID, 10),
			Tier:          h.Tier,
			Source:        h.Source,
			Snippet:       snippetOf(h.Content, 200),
			EffectiveFrom: h.EffectiveFrom,
			EffectiveTo:   h.EffectiveTo,
		})
	}

	// Emit ONE data-decision chunk carrying the conflict rows. The FE
	// MessageBubble dispatcher routes decision='counter_evidence' to
	// CounterEvidenceCard (Plan 05-06 Task 2). The disclaimer sentence
	// is hardcoded in the FE — the BE does not pass it on the wire.
	payload := wire.DecisionPayload{
		Step:           0, // step=0 signals "post-verdict annotation" (not a hop)
		Decision:       "counter_evidence",
		ConflictChunks: conflicts,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("counter-evidence: marshal DecisionPayload: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-decision",
		Data: data,
	}); err != nil {
		return fmt.Errorf("counter-evidence: emit data-decision: %w", err)
	}
	return nil
}

// snippetOf returns the first maxRunes runes of s, with a trailing
// ellipsis when truncation happens. Used to keep the ConflictChunk.Snippet
// field a one-line preview rather than the full chunk text — the FE row
// shows the snippet; clicking the row opens ChunkSourceView for the
// verbatim source.
//
// Mirrors the truncate helper in pipeline.go (kept local so annotate.go
// has no internal dependency beyond wire).
func snippetOf(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
