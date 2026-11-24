// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 57-03 (ONT-03) -- entity-level grounding.
//
// Span grounding (grounding.go) answers "does this quote appear in the
// cited chunk?". Entity grounding answers a DIFFERENT question: "are the
// named entities real entities of the corpus this Skill is allowed to
// read?" (citation-laundering lesson: a well-quoted span can still
// smuggle an entity that exists nowhere in the authorized corpus).
//
// The two checks are complementary and both feed review_required:
//   - span failure  -> ApplyDowngrade (existing D2-02 policy)
//   - entity failure -> ApplyEntityGrounding (this file, ONT-03)
//
// NIL SEMANTICS (ADDITIVE contract): a nil *EntityGrounding means "entity
// grounding did not run" (no KG wired) and is NOT a failure. This keeps
// every legacy verdict -- and every deployment without an ontology --
// exactly as it was.
package verdict

import (
	"sort"
	"strings"
)

// EntityMatcher answers "is this name a known entity?" against the
// authorized corpus KG. Implemented by the anchor dictionary in the
// server package; declared here so the verdict policy stays free of any
// ontology import (verdict must not depend on the KG layer).
type EntityMatcher interface {
	// Known reports whether name matches an entity visible to the caller.
	Known(name string) bool
}

// MaxReportedEntities caps each bucket so an adversarial answer cannot
// inflate the wire payload (the lists are display + audit material, not a
// full inventory).
const MaxReportedEntities = 32

// minEntityRunes is the shortest name considered an entity. One- and
// two-character fragments match almost everything and would make the
// check meaningless (they would both over-verify and over-flag).
const minEntityRunes = 3

// ExtractEntityCandidates pulls the entity-like names out of an answer.
//
// Deterministic rules (no model call, no NER dependency):
//   - 「...」 and 『...』 quoted spans (the corpus' own convention);
//   - "..." / '...' quoted spans;
//   - 《...》 (document/section titles);
//   - bare sequences of 3+ CJK runes or 3+ ASCII letters between
//     punctuation/whitespace.
//
// The output is deduplicated and sorted so the report is reproducible.
func ExtractEntityCandidates(answer string) []string {
	seen := make(map[string]bool)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len([]rune(s)) < minEntityRunes {
			return
		}
		if !seen[s] {
			seen[s] = true
		}
	}

	// Quoted spans first -- they are the strongest entity signal.
	for _, pair := range [][2]string{
		{"「", "」"}, {"『", "』"}, {"《", "》"}, {"\"", "\""},
	} {
		rest := answer
		for {
			i := strings.Index(rest, pair[0])
			if i < 0 {
				break
			}
			rest = rest[i+len(pair[0]):]
			j := strings.Index(rest, pair[1])
			if j < 0 {
				break
			}
			add(rest[:j])
			rest = rest[j+len(pair[1]):]
		}
	}

	// Bare CJK/ASCII runs.
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			add(run.String())
			run.Reset()
		}
	}
	for _, r := range answer {
		isCJK := r >= 0x4E00 && r <= 0x9FFF
		isLatin := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if isCJK || isLatin {
			run.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// CheckEntityGrounding classifies the answer's entity candidates as
// verified (known to the authorized KG) or unverified.
//
// A nil matcher returns nil -- "the check did not run" -- so a
// ontology-less deployment emits an unchanged verdict payload.
//
// An empty candidate list returns a report with two empty (non-nil)
// slices when a matcher IS present: "grounding ran and found nothing to
// check" is a real fact, distinct from "grounding did not run".
func CheckEntityGrounding(answer string, matcher EntityMatcher) *EntityGrounding {
	if matcher == nil {
		return nil
	}
	verified := make([]string, 0, 8)
	unverified := make([]string, 0, 8)
	for _, name := range ExtractEntityCandidates(answer) {
		if matcher.Known(name) {
			if len(verified) < MaxReportedEntities {
				verified = append(verified, name)
			}
			continue
		}
		if len(unverified) < MaxReportedEntities {
			unverified = append(unverified, name)
		}
	}
	return &EntityGrounding{Verified: verified, Unverified: unverified}
}

// ApplyEntityGrounding applies the ONT-03 policy: a non-empty unverified
// list is a grounding failure and the verdict must SAY SO.
//
// Policy (mirrors ApplyDowngrade so the two paths are consistent):
//  1. ReviewRequired = true
//  2. Confidence capped at ConfidenceFloor (0.89)
//  3. Status forced to DowngradeStatus ("needs_input") -- an answer
//     naming entities the authorized corpus does not know is not a
//     confident answer.
//
// A nil report (grounding did not run) and an empty unverified list are
// both no-ops. The function is idempotent.
func ApplyEntityGrounding(v Verdict, report *EntityGrounding) Verdict {
	if report == nil || len(report.Unverified) == 0 {
		return v
	}
	v.ReviewRequired = true
	if v.Confidence > ConfidenceFloor {
		v.Confidence = ConfidenceFloor
	}
	if v.ComplianceStatus != "" {
		v.ComplianceStatus = DowngradeStatus
	}
	v.Status = DowngradeStatus
	return v
}
