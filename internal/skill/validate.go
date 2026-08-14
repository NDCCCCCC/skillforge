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

// MinProhibitions is the lower bound on guardrails.prohibitions length.
// P-SKILL-2 (02-RESEARCH.md) requires explicit safety enumeration: <3
// prohibitions signals an under-specified Skill and is rejected at load
// time so the operator (not the LLM) is the gate.
const MinProhibitions = 3

// Validate enforces the 7-field SKILL_SCHEMA.md v1.0 contract (SYSK-02).
// Returns nil on pass; an error whose message names the failing field +
// the rule (each error is suitable for inclusion in the operator's startup
// log + the API response from plan 02-04's POST /api/skills).
//
// Order of checks matters: cheap field-presence checks first (so the
// operator sees the most basic mistake immediately), then domain-level
// rules (reads → ResolveTiers, guardrails.prohibitions → length). The first
// failing check short-circuits — Validate does NOT return a list of all
// problems at once. Rationale: the loader surfaces errors at startup, the
// author fixes one, re-runs, sees the next — incremental feedback is
// better than a wall of errors on a first attempt.
//
// SYSK-08 enforcement lives in this function via the Temperature pointer
// check (Pitfall 4 — Go zero-value trap). A missing YAML key decodes as
// nil; a present key (even at 0.0) decodes as a non-nil pointer. This is
// the only correct way to distinguish "author explicitly set 0.0" from
// "author forgot the field entirely".
func Validate(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil (SYSK-02: 7-field SKILL.md required)")
	}
	// 1. description — non-empty intent-routing text (SKILL_SCHEMA field 1)
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf(
			"description is required and must be non-empty (SKILL_SCHEMA field 1)",
		)
	}
	// 2. agent — must be one of the 6 Eino ADK agent types
	if !validAgentTypes[m.Agent] {
		return fmt.Errorf(
			"agent %q is not a supported Eino ADK type (SKILL_SCHEMA field 2; valid: ChatModelAgent, ReActAgent, SequentialAgent, ParallelAgent, LoopAgent, ConditionalAgent)",
			m.Agent,
		)
	}
	// 3. reads — non-empty (default-deny), no "*" wildcard (P-SKILL-3)
	if len(m.Reads) == 0 {
		return fmt.Errorf(
			"reads is required and must be non-empty (default-deny: P-SKILL-3 / SYSK-07)",
		)
	}
	for _, r := range m.Reads {
		if r == "*" {
			return fmt.Errorf(
				`reads %q is rejected: '*' wildcard is the P-SKILL-3 anti-pattern (bypasses tier-based access control; default-deny per SYSK-07)`,
				r,
			)
		}
	}
	// 3a. reads — every top-level dir must be in TierEnum (SYSK-07 +
	// D2-12). Delegate to ResolveTiers so there is exactly ONE tier-enum
	// source of truth in this package.
	if _, err := ResolveTiers(m.Reads); err != nil {
		return fmt.Errorf("reads: %w", err)
	}
	// 4. writes — must be present (nil = missing); empty [] is the valid
	// read-only form per SKILL_SCHEMA field 4.
	if m.Writes == nil {
		return fmt.Errorf(
			"writes is required (use [] for read-only Skills; nil means the field is absent — SKILL_SCHEMA field 4)",
		)
	}
	// 5. steps — non-empty markdown ordered list (YAML block scalar form
	// accepted per Open Q2 resolution in 02-02 plan Task 1).
	if strings.TrimSpace(m.Steps) == "" {
		return fmt.Errorf(
			"steps is required and must be non-empty (SKILL_SCHEMA field 5: declarative markdown ordered list)",
		)
	}
	// 6. guardrails.prohibitions — ≥3 entries (P-SKILL-2)
	if len(m.Guardrails.Prohibitions) < MinProhibitions {
		return fmt.Errorf(
			"guardrails.prohibitions must have at least %d entries (P-SKILL-2 — explicit safety enumeration), got %d",
			MinProhibitions, len(m.Guardrails.Prohibitions),
		)
	}
	// 7. temperature — REQUIRED pointer (SYSK-08 — no implicit default).
	// The pointer is the entire reason Manifest.Temperature is *float32
	// instead of float32 (Pitfall 4).
	if m.Temperature == nil {
		return fmt.Errorf(
			"temperature is required (SYSK-08 — no implicit default; missing field rejected). " +
				"Declare an explicit value, e.g. `temperature: 0.0` for compliance/data Skills, " +
				"`temperature: 0.7+` for creative Skills.",
		)
	}
	if *m.Temperature < 0.0 || *m.Temperature > 1.0 {
		return fmt.Errorf(
			"temperature must be in [0.0, 1.0] (SYSK-08), got %f",
			*m.Temperature,
		)
	}
	return nil
}
