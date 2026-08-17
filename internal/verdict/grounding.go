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

package verdict

import (
	"strings"

	"golang.org/x/text/width"
)

// Normalize collapses runs of whitespace, applies width.Narrow (全角→半角
// per D2-05) and trims leading/trailing whitespace. The result is a
// canonical string form suitable for substring comparison.
//
// Why this matters: the LLM emits Span as a verbatim quote from a chunk,
// but the chunk text in Milvus is the verbatim .md file body
// (one-doc-one-chunk, no chunker/splitter exists), and the LLM may
// emit its Span quote with different whitespace than the source text.
// Naive substring contains would fail to match "出差住宿费
// 超过标准额度时" vs "出差住宿费超过标准额度时" if the chunk had no
// whitespace between the LLM's emitted span's words. width.Narrow
// handles the CN 全角→半角 drift (Pitfall 4 mitigation: the LLM
// occasionally emits fullwidth ASCII when the chunk has halfwidth).
//
// Normalize is exported so the contract test (integration/contract/
// contract_test.go) and downstream consumers (Phase 4 UI) can apply the
// same canonicalization to user-supplied citation queries.
func Normalize(s string) string {
	// 1. width.Narrow — 全角数字/字母/punctuation → 半角 (D2-05).
	//    width.Narrow is idempotent on already-halfwidth runes; it only
	//    converts the W/F/Wide variants.
	narrowed := width.Narrow.String(s)

	// 2. Collapse runs of whitespace to a single space, then trim.
	var b strings.Builder
	b.Grow(len(narrowed))
	prevSpace := false
	for _, r := range narrowed {
		if isWhitespace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// isWhitespace reports whether r is a whitespace rune we treat as
// collapsible. ASCII space + tab + CR + LF + the Unicode space category
// (which includes the ideographic space U+3000 全角空格).
func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	// Unicode category Zs (space separators) — includes U+3000 全角空格
	// which width.Narrow does NOT convert to halfwidth (it's already a
	// "narrow" width per Unicode, just visually fullwidth in CJK fonts).
	if r == 0x3000 {
		return true
	}
	return false
}

// canonicalChunkID normalizes a citation's ChunkID for map lookup. The
// pipeline frames chunks to the LLM as "[CHUNK <id>]" and the LLM echoes
// "CHUNK <id>" as the chunk_id; the grounding map is keyed by the bare
// "<id>". This helper strips the "CHUNK "/"chunk " prefix (case-insensitive)
// and surrounding whitespace so the lookup matches regardless of which
// form the LLM emitted.
func canonicalChunkID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > len("CHUNK ") {
		if strings.EqualFold(s[:len("CHUNK ")], "CHUNK ") {
			s = strings.TrimSpace(s[len("CHUNK "):])
		}
	}
	return s
}

// lookupChunk finds a chunk's text in chunksByID, trying the canonical
// (prefix-stripped) key first, then the raw key as a fallback. Returns
// (text, true) on hit.
func lookupChunk(chunksByID map[string]string, chunkID string) (string, bool) {
	if t, ok := chunksByID[canonicalChunkID(chunkID)]; ok {
		return t, true
	}
	if t, ok := chunksByID[chunkID]; ok { // raw fallback (verbatim key)
		return t, true
	}
	return "", false
}

// GroundingReport summarizes the result of a GroundingCheck call.
type GroundingReport struct {
	// Grounded is true when at least one citation's Span was found (after
	// normalization) in the chunk text identified by ChunkID.
	Grounded bool

	// UngroundedCitations lists the Citation entries whose Span did NOT
	// appear in their respective chunk text. Empty when Grounded is true.
	// The citations slice is in the original order so the caller can
	// correlate with the input.
	UngroundedCitations []Citation
}

// GroundingCheck verifies that every citation's Span literally appears
// in the chunk text identified by ChunkID (after normalization per D2-05).
//
// chunksByID is a lookup map from ChunkID → chunk text. The pipeline
// (02-03 cmd/server) builds this from the Milvus search result + the
// chunk's stored `content` field. If a ChunkID is not present in the
// map, the citation is treated as ungrounded (the chunk was not
// retrieved — i.e. the LLM cited something it never saw).
//
// GroundingCheck is PURE: it does not mutate v. The downgrade side
// effect (D2-02) lives in ApplyDowngrade, which is the canonical entry
// point used by the verdict pipeline:
//
//	report := verdict.GroundingCheck(v, chunksByID)
//	if !report.Grounded {
//	    v = verdict.ApplyDowngrade(v, report)
//	}
//
// Splitting the read-only check from the mutating downgrade lets us
// unit-test both independently — GroundingCheck is a pure predicate;
// ApplyDowngrade is the policy application.
func GroundingCheck(v Verdict, chunksByID map[string]string) GroundingReport {
	report := GroundingReport{Grounded: true}
	for _, c := range v.Citations {
		// The LLM emits ChunkID in the form it saw in the prompt
		// ("CHUNK 7"), but chunksByID is keyed by the bare numeric id
		// ("7", via strconv.FormatInt in the pipeline). Normalize the
		// lookup key so the moat actually verifies spans instead of
		// silently flagging every citation as a "novel claim". Tolerates
		// "CHUNK 7" / "chunk 7" / "7" / " CHUNK 7 " (LLM casing/spacing
		// drift); falls back to the raw key if the map is keyed verbatim.
		chunkText, ok := lookupChunk(chunksByID, c.ChunkID)
		if !ok {
			// Chunk not retrieved — the LLM cited something it never saw.
			// This is the "novel claim" failure mode (D2-02 / D-01).
			report.Grounded = false
			report.UngroundedCitations = append(report.UngroundedCitations, c)
			continue
		}
		normChunk := Normalize(chunkText)
		normSpan := Normalize(c.Span)
		if !strings.Contains(normChunk, normSpan) {
			report.Grounded = false
			report.UngroundedCitations = append(report.UngroundedCitations, c)
		}
	}
	return report
}

// ConfidenceFloor is the maximum Confidence a Verdict may retain after a
// grounding downgrade per D2-02. The rationale: a verdict with one or
// more ungrounded citations is, at best, partially informed; capping
// confidence at 0.89 forces the UI to render review_required=true and
// prevents downstream consumers (eval pipelines, LLM-as-judge) from
// treating the answer as fully authoritative.
const ConfidenceFloor = 0.89

// DowngradeStatus is the platform-neutral Status a downgraded Verdict is
// forced to. Per D2-02, when grounding fails the verdict is downgraded
// to Status="needs_input" so the UI + downstream consumers can render a
// clear "we don't have enough grounded context" affordance rather than
// the LLM's optimistic-but-fabricated verdict.
//
// 260817-vsm: was "需补充" (compliance-specific Chinese). Migrated to
// platform-neutral English as part of the verdict-subject-mismatch fix
// (debug session verdict-subject-mismatch.md) — the platform envelope
// no longer carries compliance vocabulary.
const DowngradeStatus = "needs_input"

// NonCompliantStatus is the Compliance-Skill-specific verdict value
// carried in Verdict.ComplianceStatus when the review concludes the
// subject fails the policy. Plan 05-06 (SC-7 / CMPL-07 / D5-14) adds the
// redline pass that fires conditionally on Verdict.ComplianceStatus
// being NonCompliantStatus OR DowngradeStatus — a Compliant verdict
// emits NO redline chunk and makes NO LLM call (cost control).
//
// 260817-vsm: was Verdict.Status="不合规" (compliance-Skill column
// bleeding into the platform envelope). Migrated to
// Verdict.ComplianceStatus="non_compliant" so the platform envelope stays
// neutral and compliance vocabulary lives only on the Compliance Skill's
// own payload field.
const NonCompliantStatus = "non_compliant"

// CompliantStatus is the Compliance-Skill-specific verdict value when
// the subject conforms to the cited policy. Used as the ComplianceStatus
// field value on Verdict for a clean "yes, this matches" review.
//
// 260817-vsm: introduced alongside NonCompliantStatus to give the
// Compliance Skill's verdict a binary (compliant / non_compliant) shape.
// The previous single-Status-enum design ("合规/不合规/需补充/unanswered")
// conflated platform-level outcomes with Compliance-Skill outcomes;
// splitting them is the platform-decoupling fix.
const CompliantStatus = "compliant"

// NotApplicableStatus is the Compliance-Skill-specific verdict value when
// the query is a factual lookup (not a compliance review). The
// Compliance Skill's system prompt instructs the LLM to set
// ComplianceStatus="not_applicable" for factual queries so the
// conclusionText renderer knows to lead with Answer (not ComplianceStatus).
//
// 260817-vsm: factual queries previously got ComplianceStatus="compliant"
// by default — semantically wrong ("compliant" doesn't mean "answered"),
// which is exactly the verdict-subject-mismatch root cause.
const NotApplicableStatus = "not_applicable"

// ComplianceStatuses is the Compliance-Skill-specific verdict enum for
// Verdict.ComplianceStatus. Distinct from ValidStatuses (the platform-
// neutral Status enum). Used by the Compliance Skill's own validators
// + tests to gate which ComplianceStatus values are acceptable.
//
// Semantics:
//
//	"compliant"     — subject matches the cited policy.
//	"non_compliant" — subject fails the cited policy.
//	"not_applicable" — query is a factual lookup (not a compliance review);
//	                   see Answer field for the actual answer.
var ComplianceStatuses = map[string]bool{
	CompliantStatus:     true,
	NonCompliantStatus:  true,
	NotApplicableStatus: true,
}

// ApplyDowngrade applies the D2-02 downgrade policy to v when report
// indicates grounding failure. Returns v unchanged when report.Grounded
// is true (the happy path).
//
// Downgrade rules (D2-02):
//  1. ReviewRequired = true (force-on regardless of LLM's emit).
//  2. Confidence = min(Confidence, ConfidenceFloor=0.89) (cap).
//  3. Status = "需补充" (overwrite LLM's optimistic status).
//
// The function is IDEMPOTENT: calling it on an already-downgraded
// Verdict is a no-op (no further lowering of Confidence, no Status flip
// away from "需补充", ReviewRequired stays true). This matters because
// the verdict pipeline may call ApplyDowngrade multiple times in
// retry/re-eval paths — non-idempotent downgrade would silently destroy
// the original Confidence value across retries.
//
// ApplyDowngrade mutates the passed value and returns the result. It
// does NOT mutate the input in-place — Go's value semantics make the
// in-place mutation invisible to the caller anyway, and returning the
// modified copy keeps the call site explicit:
//
//	v = verdict.ApplyDowngrade(v, report)  // explicit reassignment
func ApplyDowngrade(v Verdict, report GroundingReport) Verdict {
	if report.Grounded {
		return v // no-op on the happy path
	}

	// Rule 1: ReviewRequired = true (always force-on, even if LLM said false).
	v.ReviewRequired = true

	// Rule 2: Confidence cap at 0.89.
	//    Idempotent: if v.Confidence is already ≤ 0.89, min returns it unchanged.
	if v.Confidence > ConfidenceFloor {
		v.Confidence = ConfidenceFloor
	}

	// Rule 3: Status overwrite to DowngradeStatus ("needs_input").
	//    Idempotent: if v.Status is already DowngradeStatus, assignment is a no-op.
	//    260817-vsm: also propagate to ComplianceStatus when the verdict
	//    already carries a compliance verdict — the Compliance Skill's UI
	//    reads ComplianceStatus (not Status) for the verdict badge, so
	//    leaving it stale would render a mismatch (e.g. UI shows "合规"
	//    while the platform Status says "needs_input"). When ComplianceStatus
	//    is empty (non-compliance Skill / no verdict yet), leave it — the
	//    platform Status alone is enough.
	if v.ComplianceStatus != "" {
		v.ComplianceStatus = DowngradeStatus
	}

	v.Status = DowngradeStatus

	return v
}

// GroundingChecker is the canonical entry point used by the verdict
// pipeline (02-03 cmd/server). It is a convenience wrapper that combines
// GroundingCheck (pure predicate) + ApplyDowngrade (policy mutation)
// into a single call: the pipeline does NOT need to call them separately.
//
// Behavior:
//   - Calls GroundingCheck to build the report.
//   - If report.Grounded is true, returns v unchanged (no allocation,
//     no mutation, no downgrade applied).
//   - If report.Grounded is false, calls ApplyDowngrade and returns
//     the downgraded Verdict.
//
// For tests + audit paths that need the GroundingReport, call
// GroundingCheck directly instead — GroundingChecker discards the
// intermediate report after applying the downgrade. The audit trail is
// captured by the pipeline via the trace log (D-12 deliverable).
func GroundingChecker(v Verdict, chunksByID map[string]string) Verdict {
	report := GroundingCheck(v, chunksByID)
	if report.Grounded {
		return v // happy path — no allocation, no mutation
	}
	return ApplyDowngrade(v, report)
}
