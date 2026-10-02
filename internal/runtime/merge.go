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

// Package runtime — merge.go is Phase 13 / Plan 13-02's FAN-03
// deterministic merge pure function. It is the load-bearing moat
// for:
//
//   - (a) merge determinism — 10× byte-identity contract test
//     (TestMergeSubAgentResultsByteIdentity). Any non-deterministic
//     behavior (map iteration, time.Now, randomness, goroutines) would
//     fail this test and block audit replay (Pitfall 3).
//   - (b) conservative partial-failure merge — any refused/faulted
//     sub-agent → merged verdict is needs_input + confidence 0
//     (D2-14 posture per P13-L5; P-COMPL-1 / Pitfall 4 mitigation).
//
// The function is PURE: no audit write, no SSE emit, no map
// iteration, no goroutines. Deterministic input → byte-identical JSON
// output across N calls.
//
// JSON tag order matters (FAN-03 contract test):
//
//   - SubAgentResult tags: id → status → sub_query → verdict →
//     refusal → confidence → citations → faulted_reason. The first
//     field (id) is the sort key 1; subsequent fields mirror the
//     input order without disrupting byte-identity.
//   - MergedVerdict tags: status → confidence → citations →
//     review_required → sub_results → run_path. The status field is
//     the merge-output discriminator; subsequent fields follow the
//     canonical render order.
//
// Future maintainers adding a field to either struct MUST append it
// AFTER the existing sort-key fields — inserting a field BEFORE the
// existing first field would change the canonical byte sequence and
// break TestMergeSubAgentResultsByteIdentity.
package runtime

import (
	"sort"

	"github.com/skillforge/skillforge/internal/verdict"
)

// SubAgent status enum constants (FAN-03 / P13-L4 + P13-L5).
// 13-03's per-sub-agent grounding call sets one of these three values
// based on the ApplyTrustStage output:
//
//   - SubAgentStatusOK — grounded verdict returned; merge treats it
//     as a contributing sub-agent (citations unioned, confidence
//     min-aggregated).
//   - SubAgentStatusRefused — ApplyTrustStage returned a Refusal
//     (trust-floor refusal — D-09 / D2-14 posture); merge
//     conservatively downgrades the merged verdict to needs_input +
//     confidence 0.
//   - SubAgentStatusFaulted — sub-agent execution failed (panic / LLM
//     timeout / retriever error / context deadline); merge treats
//     this as refusal-equivalent per P13-L5 (Pitfall 4 mitigation).
//
// Unknown status values are silently ignored by mergeSubAgentResults
// (defensive — 13-03 sets one of the 3 known values; future extensions
// adding new statuses MUST add a case here or be ignored).
const (
	SubAgentStatusOK      = "ok"
	SubAgentStatusRefused = "refused"
	SubAgentStatusFaulted = "faulted"
)

// SubAgentResult is the per-sub-agent envelope produced by 13-03's
// per-sub-agent grounding call (ApplyTrustStage wraps the LLM
// response) and consumed by mergeSubAgentResults.
//
// JSON tag order matches the FAN-03 SC-3 sort key order (id is the
// primary sort key; the byte-identity contract test depends on
// stable JSON marshaling). Adding a new field MUST come AFTER the
// existing fields to preserve the test contract.
type SubAgentResult struct {
	ID            string             `json:"id"`
	Status        string             `json:"status"` // SubAgentStatusOK | SubAgentStatusRefused | SubAgentStatusFaulted
	SubQuery      string             `json:"sub_query"`
	Verdict       *verdict.Verdict   `json:"verdict,omitempty"`
	Refusal       *verdict.Refusal   `json:"refusal,omitempty"`
	Confidence    float64            `json:"confidence"`
	Citations     []verdict.Citation `json:"citations"`
	FaultedReason string             `json:"faulted_reason,omitempty"`
}

// MergedVerdict is the post-merge envelope the orchestrator emits
// after the 5-step pipeline. Status carries one of:
//
//   - verdict.CompliantStatus ("compliant") — multi-source agreement
//     (every sub-agent Status=ok + min-confidence aggregation). The
//     FE renderer (RightChat verdict card) does NOT distinguish
//     fan-out vs single-vertex (FAN-04 cross-archetype consistency).
//   - verdict.DowngradeStatus ("needs_input") — conservative partial-
//     failure merge (any sub-agent refused/faulted → merged verdict
//     is needs_input + confidence 0).
//
// The Status field is bound to verdict.ComplianceStatus-shaped
// strings (the plan's `verdict.ComplianceStatus` type does not exist
// as a distinct type — Verdict.ComplianceStatus is a string field);
// the merge uses verdict.CompliantStatus + verdict.DowngradeStatus
// constants directly.
//
// RunPath is the slash-separated agent-path string from eino's
// runctx.RunPath (runctx.go:347); 13-03 fills it from the
// orchestrator context. Empty during 13-02 (the Run skeleton doesn't
// carry RunPath yet).
type MergedVerdict struct {
	Status         string             `json:"status"` // verdict.CompliantStatus | verdict.DowngradeStatus
	Confidence     float64            `json:"confidence"`
	Citations      []verdict.Citation `json:"citations"`
	ReviewRequired bool               `json:"review_required"`
	SubResults     []SubAgentResult   `json:"sub_results"`
	RunPath        string             `json:"run_path,omitempty"`
}

// MergeStatusConfirmed is the all-OK merge output. Aliased to
// verdict.CompliantStatus for the platform verdict card renderer.
const MergeStatusConfirmed = verdict.CompliantStatus

// MergeStatusNeedsInput is the conservative partial-failure merge
// output. Aliased to verdict.DowngradeStatus for the platform
// refusal card renderer.
const MergeStatusNeedsInput = verdict.DowngradeStatus

// MergeConfidenceFloor is the per-sub-agent confidence floor for the
// all-OK merge path. A min-aggregated confidence below this value
// forces ReviewRequired=true (D2-13 calibration — Phase 5 ApplyCritique
// threshold).
//
// 0.89 is chosen to match the verdict.ApplyDowngrade ConfidenceFloor
// (internal/verdict/grounding.go:204); using the same constant keeps
// single-vertex and fan-out review gates consistent.
const MergeConfidenceFloor = 0.89

// mergeSubAgentResults is FAN-03 SC-3's load-bearing pure function.
// Combines N sub-agent results into a single MergedVerdict for the
// parent Run.
//
// Contract:
//
//   - DETERMINISTIC: same input slice → byte-identical JSON output
//     across N calls. The 10× contract test
//     (TestMergeSubAgentResultsByteIdentity) is the regression lock.
//   - NO SIDE EFFECTS: no audit write, no SSE emit, no map iteration,
//     no time.Now, no goroutines, no panic on edge inputs.
//   - KEY SORT: sub-results sorted by ID ASC (stable sort); citations
//     sorted by (ChunkID ASC, Span ASC). Both sorts use
//     sort.SliceStable (deterministic for equal keys).
//   - CONSERVATIVE PARTIAL-FAILURE (P13-L5): any sub-agent
//     Status=refused or Status=faulted → merged verdict is
//     {Status: needs_input, Confidence: 0, ReviewRequired: true}
//     even if other sub-agents are OK. This is D2-14 posture —
//     default-deny over partial information.
//   - ALL-OK: every sub-agent Status=ok → merged verdict is
//     {Status: compliant, Confidence: min(per-sub-agent),
//     ReviewRequired: min < 0.89}. Citations are deduped (first-wins)
//     and unioned.
//
// Caller (13-03's FanOutOrchestrator.Run Step 4) invokes this AFTER
// all sub-agents complete (or timeout). The merge is the last
// deterministic step before the merged verdict is emitted via
// ApplyTrustStage (Step 5).
func mergeSubAgentResults(results []SubAgentResult) MergedVerdict {
	// Defensive copy + key sort by ID ASC (FAN-03 determinism —
	// absorb input shuffle / Pitfall 3 mitigation).
	sorted := make([]SubAgentResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	// Bucket by status (P13-L5 conservative posture). Slice iteration
	// ONLY — no map iteration.
	var refused, faulted, ok []SubAgentResult
	for _, r := range sorted {
		switch r.Status {
		case SubAgentStatusRefused:
			refused = append(refused, r)
		case SubAgentStatusFaulted:
			faulted = append(faulted, r)
		case SubAgentStatusOK:
			ok = append(ok, r)
			// unknown status → silently ignored (defensive; 13-03
			// sets one of the 3 known values).
		}
	}

	// Conservative partial-failure merge (D2-14 posture per P13-L5).
	// ANY refused/faulted → needs_input + 0 confidence. The merge
	// never reports "compliant" when ANY sub-agent failed — the
	// conservative posture prevents partial-information false
	// positives in fan-out scenarios.
	if len(refused) > 0 || len(faulted) > 0 {
		return MergedVerdict{
			Status:         verdict.DowngradeStatus, // "needs_input"
			Confidence:     0,
			Citations:      dedupAndSortCitations(nil), // empty (no OK sub-agents contribute)
			ReviewRequired: true,
			SubResults:     sorted, // include for FE audit timeline (12-07 AuditTimelineTab pattern)
			// RunPath filled by 13-03 (orchestrator context).
		}
	}

	// All-OK path: union citations across sub-agents, dedup first-
	// wins (deterministic — same input always dedups to the same
	// surviving citation).
	var allCitations []verdict.Citation
	for _, r := range ok {
		allCitations = append(allCitations, r.Citations...)
	}
	mergedCitations := dedupAndSortCitations(allCitations)

	// Min-confidence aggregation (conservative — D2-13 calibration).
	// Min is the weakest-link property: the merged verdict is only
	// as strong as the weakest grounded sub-agent. Mean would smooth
	// over the low-confidence signal; max would hide the low-
	// confidence sub-agent entirely.
	minConf := 1.0
	for _, r := range ok {
		if r.Confidence < minConf {
			minConf = r.Confidence
		}
	}

	return MergedVerdict{
		Status:         verdict.CompliantStatus, // "compliant" — multi-source agreement
		Confidence:     minConf,
		Citations:      mergedCitations,
		ReviewRequired: minConf < MergeConfidenceFloor, // D2-13 calibration
		SubResults:     sorted,
		// RunPath filled by 13-03.
	}
}

// dedupAndSortCitations deduplicates citations by (ChunkID, Span) —
// first-wins (deterministic) — then sorts by (ChunkID ASC, Span ASC)
// for the FAN-03 byte-identity contract.
//
// The helper uses a local map[string]bool for the dedup bookkeeping,
// but the OUTPUT is a sorted slice — the merge function's outer loops
// are slice-iteration only (Pitfall 3 mitigation). The map is local
// to the helper and never escapes; the source-grep regression lock
// TestMergeSourceGrepNoMapIteration explicitly allows the helper
// (the regex `map\[.*\]SubAgentResult` does NOT match
// `map[string]bool`).
//
// Empty input returns an empty (NOT nil) slice — caller's
// Citations field renders as `[]` in JSON, not `null`. Validate
// on the consumer side (the FE renderer) treats empty as "no
// citations" without nil-deref risk.
func dedupAndSortCitations(citations []verdict.Citation) []verdict.Citation {
	if len(citations) == 0 {
		return []verdict.Citation{}
	}
	// First-wins dedup by (ChunkID, Span) — deterministic.
	seen := make(map[string]bool, len(citations))
	deduped := make([]verdict.Citation, 0, len(citations))
	for _, c := range citations {
		// Null-byte separator avoids chunk_id+span key collision
		// (e.g. chunk="a" + span="bc" vs chunk="ab" + span="c").
		key := c.ChunkID + "\x00" + c.Span
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, c)
		}
	}
	// Sort by (ChunkID ASC, Span ASC) for byte-identity contract.
	sort.SliceStable(deduped, func(i, j int) bool {
		if deduped[i].ChunkID != deduped[j].ChunkID {
			return deduped[i].ChunkID < deduped[j].ChunkID
		}
		return deduped[i].Span < deduped[j].Span
	})
	return deduped
}