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

package skill

import (
	"fmt"
	"strings"
)

// TierEnum maps the ASCII top-level directory name from a Skill's `reads`
// pattern (e.g., "authority") to the Milvus tier VARCHAR value (e.g.,
// "权威"). This is the ONLY place those two worlds meet — a typo here
// silently breaks the trust boundary (the retriever queries a tier that
// doesn't exist in Milvus, returning zero chunks, which the pipeline
// correctly treats as `no_authorized_source` refusal — fail-closed by
// construction).
//
// Source of the CN-char values: PROJECT.md D-07 + the spike-era Milvus
// schema proven in internal/spike/milvus_client.go (now graduated to
// internal/retriever). The four tiers are the locked set for SkillForge
// v1 — adding a new tier requires adding both an entry here AND creating
// the corresponding knowledge/<dir>/ directory.
var TierEnum = map[string]string{
	"authority":  "权威",
	"historical": "历史",
	"personal":   "个人",
	"draft":      "草稿",
}

// TierEnumByValue performs the reverse lookup of TierEnum: given a
// CN-char tier value (e.g. "权威"), it returns the ASCII dir key
// ("authority") and ok=true. Empty string and any value outside the 4
// canonical CN-char tiers return ("", false).
//
// Used by internal/knowledge.Index (KNOW-02) to validate that a
// Document's Tier field is one of the 4 canonical CN-char values; the
// indexer rejects docs where TierEnumByValue(doc.Tier) returns ok=false.
// This is the backend-agnostic tier-mandatory enforcement path per
// CONTEXT.md discretion clause (the Milvus/Mongo schema validators
// are belt-and-suspenders on top of this — see 03-RESEARCH.md §Pitfall 6).
func TierEnumByValue(value string) (dir string, ok bool) {
	if value == "" {
		return "", false
	}
	for k, v := range TierEnum {
		if v == value {
			return k, true
		}
	}
	return "", false
}

// TierEnumValues returns a freshly-allocated slice of the 4 canonical
// CN-char tier values ("权威", "草稿", "历史", "个人"), ordered by the
// ASCII dir key (authority, draft, historical, personal) so the output
// is deterministic across calls and reusable in error messages.
//
// The returned slice is freshly allocated on every call; callers may
// mutate it without affecting the package-level TierEnum map. Used by
// internal/knowledge.Index (KNOW-02) to enumerate valid tiers in its
// rejection error message so Skill authors see the canonical set.
func TierEnumValues() []string {
	dirs := knownTierDirs() // sorted ASCII dir keys
	out := make([]string, 0, len(dirs))
	for _, k := range dirs {
		out = append(out, TierEnum[k])
	}
	return out
}

// knownTierDirs returns the sorted list of valid ASCII top-level dir names.
// Used in error messages from ResolveTiers so Skill authors immediately
// see what they should have written instead of guessing.
func knownTierDirs() []string {
	out := make([]string, 0, len(TierEnum))
	for k := range TierEnum {
		out = append(out, k)
	}
	// sort for stable error messages — map iteration order is randomized
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// ResolveTiers converts a Skill's reads patterns (e.g., ["authority/**",
// "authority/policies/**"]) into the unique set of Milvus tier VARCHAR
// values that the retriever may query (e.g., ["权威"]).
//
// Behavior contract:
//   - Empty input → returns nil, no error (the caller MUST then call
//     Validate separately, which rejects empty reads with default-deny).
//     ResolveTiers does NOT enforce default-deny — that is Validate's job.
//   - Unknown top-level dir → returns an error (default-deny: refuse
//     rather than guess; "unknown/**" silently mapping to nothing would
//     mask a typo in the Skill author).
//   - Duplicate tier values from overlapping patterns (authority/** AND
//     authority/policies/**) are deduplicated so Milvus sees only the
//     unique set (BuildMilvusFilter still works on duplicates but the
//     filter expression is cleaner).
//
// ResolveTiers is pure — no I/O, no globals — so it is trivially
// unit-testable (TestResolveTiers).
func ResolveTiers(reads []string) ([]string, error) {
	if len(reads) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(TierEnum))
	out := make([]string, 0, len(reads))
	for _, r := range reads {
		// Split on the first "/" — the top-level dir is everything before.
		// "authority/policies/anti-bribery" → "authority"
		// "authority" (no slash) → "authority" (single-segment reads are
		//   valid per SKILL_SCHEMA "tier patterns" examples).
		top := r
		if i := strings.Index(r, "/"); i >= 0 {
			top = r[:i]
		}
		tier, ok := TierEnum[top]
		if !ok {
			return nil, fmt.Errorf(
				"reads %q: unknown tier dir %q (known: %v)",
				r, top, knownTierDirs(),
			)
		}
		if !seen[tier] {
			seen[tier] = true
			out = append(out, tier)
		}
	}
	return out, nil
}
