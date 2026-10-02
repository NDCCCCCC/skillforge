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

package runtime

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/verdict"
)

// ============================================================================
// Test fixtures
// ============================================================================

// newCitation constructs a verdict.Citation with Tier="权威" (any
// tier works for the merge function — the merge dedups on
// (ChunkID, Span), not Tier).
func newCitation(chunkID, span string) verdict.Citation {
	return verdict.Citation{
		ChunkID: chunkID,
		Span:    span,
		Tier:    "权威",
	}
}

// newVerdict constructs a *verdict.Verdict with the supplied confidence
// + citations. Status defaults to "" (the merge function does not
// inspect per-sub-agent Verdict.Status — only the SubAgentResult.Status
// field matters for the merge decision).
func newVerdict(confidence float64, citations ...verdict.Citation) *verdict.Verdict {
	return &verdict.Verdict{
		Status:           "",
		Confidence:       confidence,
		Citations:        citations,
		ReviewRequired:   confidence < MergeConfidenceFloor,
		Answer:           "",
		ComplianceStatus: "",
	}
}

// newRefusal constructs a *verdict.Refusal for the conservative
// partial-failure path. Reason defaults to no_authorized_source.
func newRefusal(reason string) *verdict.Refusal {
	if reason == "" {
		reason = verdict.RefusalReasonNoAuthorizedSource
	}
	return &verdict.Refusal{
		Reason:                    reason,
		CanRephrase:               true,
		SuggestedAlternativeQuery: "",
	}
}

// newSubResult constructs a SubAgentResult with the supplied id +
// status. opts is a chain of mutator functions (functional options
// pattern) so tests can configure individual fields without
// proliferation of constructors.
func newSubResult(id, status string, opts ...func(*SubAgentResult)) SubAgentResult {
	r := SubAgentResult{
		ID:       id,
		Status:   status,
		SubQuery: id + " query",
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// ============================================================================
// FAN-03 SC-3 — 10× byte-identity contract test (regression lock for
// Pitfall 3 — map iteration randomness). The test runs the merge
// function N times with shuffled permutations of the same input and
// asserts all outputs are byte-identical via bytes.Equal.
// ============================================================================

// TestMergeSubAgentResultsByteIdentity — the FAN-03 SC-3 contract test.
// Iterates 10 permutations of 3 sub-agent results, asserts all 10
// marshaled JSON outputs are byte-identical. This is the load-bearing
// regression lock against accidental map iteration (Pitfall 3) or
// accidental clock-dependent ordering (time.Now, rand).
func TestMergeSubAgentResultsByteIdentity(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Verdict = newVerdict(0.95, newCitation("c1", "span1"))
			r.Confidence = 0.95
			r.Citations = []verdict.Citation{newCitation("c1", "span1")}
		}),
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Verdict = newVerdict(0.88, newCitation("c2", "span2"))
			r.Confidence = 0.88
			r.Citations = []verdict.Citation{newCitation("c2", "span2")}
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Verdict = newVerdict(0.91, newCitation("c3", "span3"))
			r.Confidence = 0.91
			r.Citations = []verdict.Citation{newCitation("c3", "span3")}
		}),
	}

	// 10 different permutations of the same 3 results.
	perms := [][]int{
		{0, 1, 2},
		{2, 0, 1},
		{1, 2, 0},
		{2, 1, 0},
		{1, 0, 2},
		{0, 2, 1},
		{1, 0, 2},
		{0, 1, 2},
		{2, 0, 1},
		{1, 2, 0},
	}

	var canonicalBytes []byte
	for i, perm := range perms {
		shuffled := make([]SubAgentResult, 0, len(results))
		for _, j := range perm {
			shuffled = append(shuffled, results[j])
		}
		merged := mergeSubAgentResults(shuffled)
		bz, err := json.Marshal(merged)
		if err != nil {
			t.Fatalf("perm %d: marshal: %v", i, err)
		}
		if i == 0 {
			canonicalBytes = bz
			continue
		}
		if !bytes.Equal(bz, canonicalBytes) {
			t.Errorf("perm %d: bytes differ\nfirst: %s\nlater: %s", i, canonicalBytes, bz)
		}
	}
}

// TestMergeSubAgentResultsByteIdentity_SingleCall_10x — the regression
// lock for accidental map-iteration introduction. Same input slice,
// 10 identical calls; all 10 must produce byte-identical output.
func TestMergeSubAgentResultsByteIdentity_SingleCall_10x(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c1", "span1"),
				newCitation("c2", "span2"),
			}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.85
			r.Citations = []verdict.Citation{
				newCitation("c2", "span2"),
				newCitation("c3", "span3"),
			}
		}),
	}

	var canonicalBytes []byte
	for i := 0; i < 10; i++ {
		merged := mergeSubAgentResults(results)
		bz, err := json.Marshal(merged)
		if err != nil {
			t.Fatalf("iter %d: marshal: %v", i, err)
		}
		if i == 0 {
			canonicalBytes = bz
			continue
		}
		if !bytes.Equal(bz, canonicalBytes) {
			t.Errorf("iter %d: bytes differ\nfirst: %s\nlater: %s", i, canonicalBytes, bz)
		}
	}
}

// ============================================================================
// FAN-03 SC-4 + D2-14 — conservative partial-failure merge tests.
// ============================================================================

// TestMergeConservativePartialFailure_Refused — 2 OK + 1 refused
// → merged verdict is needs_input + 0 confidence + review_required
// (P13-L5 conservative posture).
func TestMergeConservativePartialFailure_Refused(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
		}),
		newSubResult("sa2", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonNoAuthorizedSource)
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.DowngradeStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.DowngradeStatus)
	}
	if merged.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Error("ReviewRequired = false, want true")
	}
	if len(merged.SubResults) != 3 {
		t.Errorf("SubResults len = %d, want 3 (all preserved for audit timeline)", len(merged.SubResults))
	}
}

// TestMergeConservativePartialFailure_Faulted — 2 OK + 1 faulted
// → same conservative outcome (Pitfall 4 mitigation: faults are
// refusal-equivalent).
func TestMergeConservativePartialFailure_Faulted(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
		}),
		newSubResult("sa2", SubAgentStatusFaulted, func(r *SubAgentResult) {
			r.FaultedReason = "context deadline exceeded"
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.DowngradeStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.DowngradeStatus)
	}
	if merged.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Error("ReviewRequired = false, want true")
	}
}

// TestMergeConservativePartialFailure_Both — 2 OK + 1 refused + 1
// faulted → same conservative outcome.
func TestMergeConservativePartialFailure_Both(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
		}),
		newSubResult("sa2", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonLowRetrievalScore)
		}),
		newSubResult("sa3", SubAgentStatusFaulted, func(r *SubAgentResult) {
			r.FaultedReason = "retriever error"
		}),
		newSubResult("sa4", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.DowngradeStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.DowngradeStatus)
	}
	if merged.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Error("ReviewRequired = false, want true")
	}
}

// TestMergeConservativePartialFailure_AllRefused — 3 refused (no OK)
// → no citations, Status = needs_input, Confidence = 0.
func TestMergeConservativePartialFailure_AllRefused(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonNoAuthorizedSource)
		}),
		newSubResult("sa2", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonLowRetrievalScore)
		}),
		newSubResult("sa3", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonOutOfScope)
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.DowngradeStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.DowngradeStatus)
	}
	if merged.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Error("ReviewRequired = false, want true")
	}
	// Citations should be empty (no OK sub-agents contributed).
	if len(merged.Citations) != 0 {
		t.Errorf("Citations len = %d, want 0", len(merged.Citations))
	}
	// But Citations must be non-nil (empty slice, not nil).
	if merged.Citations == nil {
		t.Error("Citations is nil, want empty slice (NOT nil)")
	}
}

// TestMergeConservativePartialFailure_PreservesSubResultsForAudit —
// even on conservative failure, SubResults contains the FULL input
// (all 3 OK + 1 refused) so the FE audit timeline (12-07
// AuditTimelineTab pattern) can render the fan-out progression.
// The merge downgrades Status + Confidence, but the per-sub-agent
// trace stays.
func TestMergeConservativePartialFailure_PreservesSubResultsForAudit(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.88
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
		}),
		newSubResult("sa4", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonNoAuthorizedSource)
		}),
	}
	merged := mergeSubAgentResults(results)

	if len(merged.SubResults) != 4 {
		t.Fatalf("SubResults len = %d, want 4 (all input preserved)", len(merged.SubResults))
	}
	// SubResults must be sorted by ID ASC (byte-identity contract).
	for i := 1; i < len(merged.SubResults); i++ {
		if merged.SubResults[i-1].ID >= merged.SubResults[i].ID {
			t.Errorf("SubResults not sorted by ID: [%d]=%q >= [%d]=%q",
				i-1, merged.SubResults[i-1].ID, i, merged.SubResults[i].ID)
		}
	}
}

// ============================================================================
// Citation dedup tests (FAN-03 — deterministic first-wins).
// ============================================================================

// TestMergeCitationDedup — 3 sub-agents each citing the same 2
// (ChunkID, Span) tuples → merged verdict has 2 unique citations
// (not 6); first-wins means the alphabetically-first sub-agent's
// citation is appended to the dedup buffer first.
func TestMergeCitationDedup(t *testing.T) {
	// sa1, sa2, sa3 all cite the same c1 + c2 with IDENTICAL spans
	// (the only way dedup by (ChunkID, Span) can collapse 6 → 2).
	// After dedup, only 2 citations survive.
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c1", "shared span for c1"),
				newCitation("c2", "shared span for c2"),
			}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c1", "shared span for c1"),
				newCitation("c2", "shared span for c2"),
			}
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c1", "shared span for c1"),
				newCitation("c2", "shared span for c2"),
			}
		}),
	}
	merged := mergeSubAgentResults(results)

	if len(merged.Citations) != 2 {
		t.Fatalf("Citations len = %d, want 2 (dedup of 6 → 2)", len(merged.Citations))
	}
	// Sorted by ChunkID ASC.
	if merged.Citations[0].ChunkID != "c1" {
		t.Errorf("Citations[0].ChunkID = %q, want c1", merged.Citations[0].ChunkID)
	}
	if merged.Citations[1].ChunkID != "c2" {
		t.Errorf("Citations[1].ChunkID = %q, want c2", merged.Citations[1].ChunkID)
	}
	// Spans must be the shared spans (dedup preserved the first
	// encountered tuple per (ChunkID, Span)).
	if merged.Citations[0].Span != "shared span for c1" {
		t.Errorf("Citations[0].Span = %q, want %q", merged.Citations[0].Span, "shared span for c1")
	}
	if merged.Citations[1].Span != "shared span for c2" {
		t.Errorf("Citations[1].Span = %q, want %q", merged.Citations[1].Span, "shared span for c2")
	}
}

// TestMergeCitationDedup_PartialOverlap — sa1 cites c1+c2, sa2 cites
// c2+c3, sa3 cites c3+c4 → merged has c1, c2, c3, c4 (4 unique,
// sorted by ChunkID ASC). The c2 and c3 are deduped because sa1 +
// sa2 both cite c2 with the same span, and sa2 + sa3 both cite c3
// with the same span.
func TestMergeCitationDedup_PartialOverlap(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c1", "span-c1"),
				newCitation("c2", "span-c2"),
			}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c2", "span-c2"),
				newCitation("c3", "span-c3"),
			}
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("c3", "span-c3"),
				newCitation("c4", "span-c4"),
			}
		}),
	}
	merged := mergeSubAgentResults(results)

	if len(merged.Citations) != 4 {
		t.Fatalf("Citations len = %d, want 4 (dedup of 6 → 4)", len(merged.Citations))
	}
	// Sorted by ChunkID ASC.
	want := []string{"c1", "c2", "c3", "c4"}
	for i, w := range want {
		if merged.Citations[i].ChunkID != w {
			t.Errorf("Citations[%d].ChunkID = %q, want %q", i, merged.Citations[i].ChunkID, w)
		}
	}
	// All spans are the canonical ones (single tuple per dedup key).
	for i, c := range merged.Citations {
		expectedSpan := "span-" + want[i]
		if c.Span != expectedSpan {
			t.Errorf("Citations[%d].Span = %q, want %q", i, c.Span, expectedSpan)
		}
	}
}

// ============================================================================
// All-OK merge path (min-confidence aggregation + review_required gating).
// ============================================================================

// TestMergeAllOKConfirmedStatus — 3 OK results with confidence
// 0.95, 0.88, 0.91 → Status = compliant, Confidence = 0.88 (min).
// Note: 0.88 < 0.89 → ReviewRequired = true. Use 0.95, 0.93, 0.91
// to test ReviewRequired=false separately.
func TestMergeAllOKConfirmedStatus(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.93
			r.Citations = []verdict.Citation{newCitation("c1", "s1")}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
			r.Citations = []verdict.Citation{newCitation("c2", "s2")}
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
			r.Citations = []verdict.Citation{newCitation("c3", "s3")}
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.CompliantStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.CompliantStatus)
	}
	if merged.Confidence != 0.91 {
		t.Errorf("Confidence = %v, want 0.91 (min across sub-agents)", merged.Confidence)
	}
	if merged.ReviewRequired {
		t.Error("ReviewRequired = true, want false (0.91 >= 0.89)")
	}
}

// TestMergeAllOK_ReviewRequired_Threshold — 3 OK with confidence
// 0.85 → ReviewRequired = true (below MergeConfidenceFloor).
func TestMergeAllOK_ReviewRequired_Threshold(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.90
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.85
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.88
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.CompliantStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.CompliantStatus)
	}
	if merged.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85 (min)", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Error("ReviewRequired = false, want true (0.85 < 0.89)")
	}
}

// TestMergeAllOK_MinConfidenceFloor — 3 OK with confidence exactly at
// the threshold → ReviewRequired = false (boundary test).
func TestMergeAllOK_MinConfidenceFloor(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = MergeConfidenceFloor
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.CompliantStatus {
		t.Errorf("Status = %q, want %q", merged.Status, verdict.CompliantStatus)
	}
	if merged.ReviewRequired {
		t.Errorf("ReviewRequired = true at threshold %v, want false (boundary)", MergeConfidenceFloor)
	}
}

// ============================================================================
// Edge cases — empty input + input order independence.
// ============================================================================

// TestMergeEmptyResults — empty input → MergedVerdict.SubResults is
// empty slice (NOT nil). The conservative-partial-failure branch is
// NOT triggered (no refused/faulted), so Status defaults to compliant.
func TestMergeEmptyResults(t *testing.T) {
	merged := mergeSubAgentResults(nil)

	if merged.SubResults == nil {
		t.Error("SubResults is nil, want empty slice (NOT nil panic)")
	}
	if len(merged.SubResults) != 0 {
		t.Errorf("SubResults len = %d, want 0", len(merged.SubResults))
	}
	// No refused/faulted → conservative-partial-failure branch
	// skipped. The all-OK path runs with empty OK slice. minConf
	// initialization (1.0) is never updated → Confidence = 1.0.
	if merged.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0 (no OK → min unchanged)", merged.Confidence)
	}
	if merged.Status != verdict.CompliantStatus {
		t.Errorf("Status = %q, want %q (no failures → compliant)", merged.Status, verdict.CompliantStatus)
	}
}

// TestMergeSubAgentOrderIndependent — 3 random shuffles of the same
// 3 sub-agent results → byte-identical output across all 3 (Pitfall 3
// mitigation: the sort absorbs the shuffle).
func TestMergeSubAgentOrderIndependent(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
			r.Citations = []verdict.Citation{newCitation("c1", "s1")}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.91
			r.Citations = []verdict.Citation{newCitation("c2", "s2")}
		}),
		newSubResult("sa3", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.93
			r.Citations = []verdict.Citation{newCitation("c3", "s3")}
		}),
	}

	shuffles := [][]int{
		{0, 1, 2},
		{2, 1, 0},
		{1, 0, 2},
	}

	var canonicalBytes []byte
	for i, shuffle := range shuffles {
		shuffled := make([]SubAgentResult, 0, len(results))
		for _, j := range shuffle {
			shuffled = append(shuffled, results[j])
		}
		merged := mergeSubAgentResults(shuffled)
		bz, err := json.Marshal(merged)
		if err != nil {
			t.Fatalf("shuffle %d: marshal: %v", i, err)
		}
		if i == 0 {
			canonicalBytes = bz
			continue
		}
		if !bytes.Equal(bz, canonicalBytes) {
			t.Errorf("shuffle %d: bytes differ\nfirst: %s\nlater: %s", i, canonicalBytes, bz)
		}
	}
}

// TestMergeSubAgentResult_FieldsPreservedInSubResults — SubResults
// field of MergedVerdict preserves all input fields verbatim (no
// transformation).
func TestMergeSubAgentResult_FieldsPreservedInSubResults(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
			r.Citations = []verdict.Citation{newCitation("c1", "s1")}
			r.SubQuery = "what is X?"
		}),
		newSubResult("sa2", SubAgentStatusRefused, func(r *SubAgentResult) {
			r.Refusal = newRefusal(verdict.RefusalReasonNoAuthorizedSource)
		}),
	}
	merged := mergeSubAgentResults(results)

	if len(merged.SubResults) != 2 {
		t.Fatalf("SubResults len = %d, want 2", len(merged.SubResults))
	}
	// sa1 fields.
	if merged.SubResults[0].ID != "sa1" {
		t.Errorf("SubResults[0].ID = %q, want sa1", merged.SubResults[0].ID)
	}
	if merged.SubResults[0].SubQuery != "what is X?" {
		t.Errorf("SubResults[0].SubQuery = %q, want %q", merged.SubResults[0].SubQuery, "what is X?")
	}
	if merged.SubResults[0].Confidence != 0.95 {
		t.Errorf("SubResults[0].Confidence = %v, want 0.95", merged.SubResults[0].Confidence)
	}
	// sa2 fields (refusal preserved).
	if merged.SubResults[1].ID != "sa2" {
		t.Errorf("SubResults[1].ID = %q, want sa2", merged.SubResults[1].ID)
	}
	if merged.SubResults[1].Refusal == nil {
		t.Error("SubResults[1].Refusal is nil, want non-nil")
	}
	if merged.SubResults[1].Refusal.Reason != verdict.RefusalReasonNoAuthorizedSource {
		t.Errorf("SubResults[1].Refusal.Reason = %q, want %q",
			merged.SubResults[1].Refusal.Reason, verdict.RefusalReasonNoAuthorizedSource)
	}
}

// ============================================================================
// Source-grep regression locks (P13-L4 + P13-L5 — keep merge pure).
// ============================================================================

// TestMergeSourceGrepNoMapIteration — read merge.go and verify zero
// hits for `map[.*]SubAgentResult` (Pitfall 3 mitigation: prevents
// future executors from introducing map iteration in merge function).
// The dedupAndSortCitations helper is allowed to use
// `map[string]bool` (the regex below excludes bool).
func TestMergeSourceGrepNoMapIteration(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	src := string(data)
	// Match map keyed on SubAgentResult (the regression target).
	// Use a precise regex — \b ensures we match the type name
	// exactly, not substring matches.
	re := regexp.MustCompile(`map\[[^\]]*\]SubAgentResult`)
	hits := re.FindAllString(src, -1)
	if len(hits) > 0 {
		t.Errorf("merge.go contains %d map[...SubAgentResult] patterns: %v (Pitfall 3 violation)", len(hits), hits)
	}
}

// TestMergeSourceGrepNoAuditCall — grep for audit write patterns in
// merge.go (PutAudit, InsertAudit, etc.) → returns 0 hits (merge
// must NOT write audit rows).
func TestMergeSourceGrepNoAuditCall(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	src := string(data)
	for _, pattern := range []string{"PutAudit", "InsertAudit", "s.PutAudit", "store.PutAudit"} {
		if strings.Contains(src, pattern) {
			t.Errorf("merge.go contains %q — merge must NOT write audit rows", pattern)
		}
	}
}

// TestMergeSourceGrepNoSSEEmit — grep for SSE emit patterns in
// merge.go (EmitDecision, EmitChunk, emit() etc.) → returns 0 hits
// (merge must NOT emit SSE).
func TestMergeSourceGrepNoSSEEmit(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	src := string(data)
	for _, pattern := range []string{"EmitDecision", "EmitChunk", "EmitFanoutDecision"} {
		if strings.Contains(src, pattern) {
			t.Errorf("merge.go contains %q — merge must NOT emit SSE", pattern)
		}
	}
}

// TestMergeSourceGrepNoTimeNow — grep for time.Now() (with parens —
// actual call site, NOT comment text) in merge.go → 0 hits (merge
// must be wall-clock-independent; byte-identity contract would fail
// if merge used clock).
func TestMergeSourceGrepNoTimeNow(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	// Match "time.Now(" (the call site). "time.Now" alone would
	// match comments like "no time.Now".
	if strings.Contains(string(data), "time.Now(") {
		t.Error("merge.go contains time.Now( call site — merge must be wall-clock-independent (FAN-03 determinism)")
	}
}

// TestMergeSourceGrepSortUsage — grep for sort.SliceStable in
// merge.go → must show ≥ 2 uses (sub-result sort + citation sort).
func TestMergeSourceGrepSortUsage(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	count := strings.Count(string(data), "sort.SliceStable")
	if count < 2 {
		t.Errorf("sort.SliceStable used %d times, want ≥ 2 (sub-result sort + citation sort)", count)
	}
}

// TestMergeSourceGrepVerdictEnumUsage — grep for verdict.CompliantStatus
// + verdict.DowngradeStatus in merge.go → must show both (REUSE of
// Phase 2 verdict enum constants — NOT new type).
func TestMergeSourceGrepVerdictEnumUsage(t *testing.T) {
	data, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatalf("read merge.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "verdict.CompliantStatus") {
		t.Error("merge.go does not contain verdict.CompliantStatus — must REUSE Phase 2 enum")
	}
	if !strings.Contains(src, "verdict.DowngradeStatus") {
		t.Error("merge.go does not contain verdict.DowngradeStatus — must REUSE Phase 2 enum")
	}
}

// TestMergeSourceGrepDeterministicJSON — re-runs the byte-identity
// contract test 10 times in the same Go process. Different goroutine
// scheduling must NOT affect output (regression lock against
// accidental map iteration or clock dependence).
func TestMergeSourceGrepDeterministicJSON(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{newCitation("c1", "s1")}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.85
			r.Citations = []verdict.Citation{newCitation("c2", "s2")}
		}),
	}
	var first []byte
	for i := 0; i < 10; i++ {
		merged := mergeSubAgentResults(results)
		bz, err := json.Marshal(merged)
		if err != nil {
			t.Fatalf("iter %d: marshal: %v", i, err)
		}
		if i == 0 {
			first = bz
			continue
		}
		if !bytes.Equal(bz, first) {
			t.Errorf("iter %d: bytes differ", i)
		}
	}
}

// TestMergeCitationDedup_NullByteSeparator — verifies the dedup key
// uses null-byte separator so chunk_id="a" + span="bc" doesn't
// collide with chunk_id="ab" + span="c".
func TestMergeCitationDedup_NullByteSeparator(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("a", "bc"),
			}
		}),
		newSubResult("sa2", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.9
			r.Citations = []verdict.Citation{
				newCitation("ab", "c"),
			}
		}),
	}
	merged := mergeSubAgentResults(results)

	// Both should survive (different (ChunkID, Span) tuples despite
	// shared string concatenation).
	if len(merged.Citations) != 2 {
		t.Errorf("Citations len = %d, want 2 (null-byte separator prevents false collision)", len(merged.Citations))
	}
}

// TestMergeUnknownStatusIgnored — sub-agent with unknown status
// (e.g. "pending") → silently ignored (defensive). The remaining OK
// sub-agent proceeds normally.
func TestMergeUnknownStatusIgnored(t *testing.T) {
	results := []SubAgentResult{
		newSubResult("sa1", SubAgentStatusOK, func(r *SubAgentResult) {
			r.Confidence = 0.95
		}),
		newSubResult("sa2", "pending", func(r *SubAgentResult) {
			// Defensive: unknown status.
		}),
	}
	merged := mergeSubAgentResults(results)

	if merged.Status != verdict.CompliantStatus {
		t.Errorf("Status = %q, want %q (unknown status ignored)", merged.Status, verdict.CompliantStatus)
	}
	if merged.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95 (only OK counts)", merged.Confidence)
	}
	// SubResults includes the unknown-status entry (audit timeline
	// records it as-is).
	if len(merged.SubResults) != 2 {
		t.Errorf("SubResults len = %d, want 2", len(merged.SubResults))
	}
}