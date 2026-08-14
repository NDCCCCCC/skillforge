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

package milvus

import (
	"fmt"
	"strings"
)

// BuildMilvusFilter converts a Skill.reads slice (e.g. ["权威"]) into a
// Milvus filter expression applied PRE-similarity at the SDK Search layer,
// co-layered with the policy-version effective_to PRE-filter (Plan 05-02
// D5-15).
//
// Empty / nil tiers returns the literal string "false" which Milvus
// evaluates as a never-matching predicate (default-deny per D-09). This is
// the structural enforcement of "refuse when no tier matches": the database
// itself returns zero rows, independent of any agent policy.
//
// Effective PRE-filter (D5-15 sentinel design, NOT nullable):
//
//	tier in [<quoted tiers>] and (effective_to == 0 or effective_to >= <now>)
//
// Order: tier first (the selectivity win), effective clause second. The
// optimizer can short-circuit on tier — a default-deny Skill never
// evaluates the effective clause.
//
// Sentinel design rationale: Milvus evaluates NULL-evaluates-FALSE
// (Pitfall 2 / RESEARCH) — using nullable effective fields would let the
// filter silently match NOTHING when the field is unset (which is the
// common case for legacy versionless chunks). The sentinel-0 design
// sidesteps that pitfall entirely: existing chunks with no effective_*
// metadata get the default value 0 (perpetual) and pass the predicate.
//
// CN characters in tier names are escaped via Go's %q formatter (Pitfall 7
// mitigation): %q produces a valid double-quoted string with hostile
// characters (single quotes, double quotes, backslashes) backslash-escaped,
// so the resulting expression is injection-safe for all tier values.
//
// `now` is the epoch-seconds cutoff (computed once at pipeline entry,
// per VerdictPipeline). Tests may pass 0 to assert expression shape;
// production always passes time.Now().Unix().
func BuildMilvusFilter(tiers []string, now int64) string {
	if len(tiers) == 0 {
		// "false" is a valid Milvus boolean predicate that matches zero rows.
		// Returning "" would be a default-allow (no filter), which violates
		// the default-deny contract — so we explicitly return "false".
		// The effective clause is NOT appended here — it would never match
		// anyway (the tier clause already returned "false"), and adding it
		// would clutter the expression for no benefit.
		return "false"
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		// %q produces a double-quoted, backslash-escaped Go string literal.
		// For CN chars it yields "权威" verbatim (UTF-8 safe); for hostile
		// chars it yields "O\"Brien" / "a\\b" — both safe inside [ ... ].
		quoted[i] = fmt.Sprintf("%q", t)
	}
	tierClause := fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
	// D5-15 sentinel PRE-filter. NOT nullable (Pitfall 2 mitigation).
	effectiveClause := fmt.Sprintf("(effective_to == 0 or effective_to >= %d)", now)
	return tierClause + " and " + effectiveClause
}
