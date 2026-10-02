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

	"github.com/skillforge/skillforge/internal/kb"
)

// ResolveTiers converts a Skill's reads patterns (e.g., ["authority/**",
// "authority/policies/**"]) into the unique set of knowledge-base slugs
// that the retriever may query (e.g., ["authority"]).
//
// ResolveTiers is a historical function name kept as a compatibility
// shim — the real source of truth is kb.Registry.ResolveReads (Phase 26,
// KBM-01: the D17-era compiled-in enumeration is deleted, and the slug
// set, the semantic roles and the deactivation switches live only in the
// registry). Keeping the name shrinks the blast radius: every existing
// caller site keeps its call shape and only gains the registry argument.
//
// Behavior contract (unchanged by the delegation except for the value
// source):
//   - Empty input → returns nil, no error (the caller MUST then call
//     Validate separately, which rejects empty reads with default-deny).
//     ResolveTiers does NOT enforce default-deny — that is Validate's job.
//   - Unknown or deactivated knowledge base → returns an error listing
//     the active slug set (default-deny: refuse rather than guess;
//     "unknown/**" silently mapping to nothing would mask a Skill-author
//     typo).
//   - Duplicate slugs from overlapping patterns (authority/** AND
//     authority/policies/**) are deduplicated so the retriever filter
//     sees only the unique set.
//   - reg == nil with non-empty reads → error (fail-closed): without the
//     registry the reads contract cannot be evaluated, and a silent empty
//     set would be indistinguishable from a deliberate default-deny.
//
// ResolveTiers is pure — no I/O, no globals — so it is trivially
// unit-testable (TestResolveTiersDelegatesToRegistry).
func ResolveTiers(reads []string, reg kb.Registry) ([]string, error) {
	if len(reads) == 0 {
		return nil, nil
	}
	// Defense-in-depth guard: data-channel values must never reach
	// ResolveTiers. Validate() branches before calling ResolveTiers
	// when HasDataChannel() is true, so this branch should never be
	// hit in normal flow. It prevents future callers from bypassing
	// Validate and calling ResolveTiers directly, which would produce
	// a misleading unknown-KB error for "data:" prefixed values
	// (D16-C2).
	for _, r := range reads {
		if strings.HasPrefix(r, "data:") {
			return nil, fmt.Errorf(
				"reads %q: data channel values are validated by Validate (D16-C2), not ResolveTiers — this is a defense-in-depth guard",
				r,
			)
		}
	}
	if reg == nil {
		return nil, fmt.Errorf(
			"reads %v: no knowledge-base registry wired — reads resolve against kb.Registry (KBM-01), so a nil registry fails closed instead of resolving to a silent empty set",
			reads,
		)
	}
	return reg.ResolveReads(reads)
}
