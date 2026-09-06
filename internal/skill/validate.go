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
	"slices"
	"strings"

	"github.com/skillforge/skillforge/internal/agent/hitl"
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
// rules (reads channel XOR / ResolveTiers, guardrails.prohibitions length).
// The first failing check short-circuits — Validate does NOT return a list
// of all problems at once. Rationale: the loader surfaces errors at startup,
// the author fixes one, re-runs, sees the next — incremental feedback is
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
	// 2. agent — must be one of the runtime archetypes (Phase 12) or
	//    the legacy Eino ADK agent type names (Phase 11 alias form).
	//    The dispatch layer (runtime.DispatchAgent) routes on the
	//    short names "chat" / "deep" (or empty defaults to chat);
	//    the Eino ADK names are accepted at validation time so
	//    existing Phase 11 Skills do not need a manifest migration.
	if !validAgentTypes[m.Agent] {
		return fmt.Errorf(
			"agent type %q is not supported (SKILL_SCHEMA field 2; valid: chat, deep, orchestrator, ChatModelAgent, ReActAgent, SequentialAgent, ParallelAgent, LoopAgent, ConditionalAgent)",
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
	// 3a. reads channel-kind branching (D16-C1/C2 XOR enforcement).
	// Data channel and tier channel are mutually exclusive — two storage
	// systems, two evidence systems (citation whitelist vs data-reference
	// snapshot); mixed-Skill is v1 deferred. The XOR check lives in
	// Validate() (not Scan) so reloadOne re-runs it on every hot-reload
	// (Pitfall 6: zero additional code needed to cover the hot-reload
	// window — parseAndValidate calls Validate).
	//
	// Pre-scan: reject any data:-prefixed value that is NEITHER of the
	// two supported data-channel constants (v1.0 aggregated_only +
	// v1.1 external:authorized). This catches typos and future
	// not-yet-supported modes before they reach ResolveTiers (which
	// would produce a misleading TierEnum error). The error names BOTH
	// supported modes and echoes the offending value so the Skill
	// author can self-correct (D16-C2 fail-closed framing).
	for _, r := range m.Reads {
		if strings.HasPrefix(r, "data:") && r != ReadDataAggregatedOnly && r != ReadDataExternalAuthorized {
			return fmt.Errorf(
				"reads %q: unknown data channel mode — supported modes are %q (aggregated KPIs only) and %q (external authorized channel, requires the server to wire the channel); D16-C2 fail-closed",
				r, ReadDataAggregatedOnly, ReadDataExternalAuthorized,
			)
		}
	}
	if m.HasDataChannel() {
		// (a) Data channel must be the sole reads entry — no mixing with tiers.
		if len(m.Reads) > 1 {
			return fmt.Errorf(
				"reads declares both data channel and tier reads %v — mutually exclusive (D16-C1: two storage systems / two evidence systems), mixed Skill is v1 deferred; keep only one data-channel mode (%s | %s) or keep only tier reads",
				m.Reads, ReadDataAggregatedOnly, ReadDataExternalAuthorized,
			)
		}
		// (b) Deep archetype + data channel: conditionally allowed.
		// MODIFIED (D18-A4 / Pattern 3): deep+data is allowed ONLY when
		// writes contains "report:publish" AND Template is non-nil
		// (monthly-report Skill uses deep+data+report_publish).
		// The deep system prompt branches on HasDataChannel() && Template != nil
		// to serve the data-channel tool set {kpi_call, fan_out_dispatch,
		// report_publish, plan_approve, write_todos} — replacing the tier-channel
		// retrieve promise. Without "report:publish" + valid Template, the
		// original incompatibility stands and we fail-closed.
		if m.Agent == "deep" && m.HasDataChannel() {
			if !slices.Contains(m.Writes, "report:publish") || m.Template == nil {
				return fmt.Errorf(
					"agent type %q is incompatible with data channel unless writes contains %q AND template: is set (D18-A4); deep archetype system prompt branches to data-channel tool set only when both conditions are met",
					m.Agent, "report:publish",
				)
			}
		}
		// (c) Skip ResolveTiers — data channel has no tier semantics.
		// resolvedTiers stays empty (nil), which the retriever treats as
		// default-deny via BuildMilvusFilter.
	} else {
		// Tier channel: resolve reads patterns into Milvus tier VARCHAR values.
		// Every top-level dir must be in TierEnum (SYSK-07 + D2-12). Delegate
		// to ResolveTiers so there is exactly ONE tier-enum source of truth
		// in this package.
		if _, err := ResolveTiers(m.Reads); err != nil {
			return fmt.Errorf("reads: %w", err)
		}
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
			"temperature is required (SYSK-08 — no implicit default; missing field rejected). "+
				"Declare an explicit value, e.g. `temperature: 0.0` for compliance/data Skills, "+
				"`temperature: 0.7+` for creative Skills.",
		)
	}
	if *m.Temperature < 0.0 || *m.Temperature > 1.0 {
		return fmt.Errorf(
			"temperature must be in [0.0, 1.0] (SYSK-08), got %f",
			*m.Temperature,
		)
	}
	// 8. writes — every target must be a KnownTarget (D11-B3 shared source of
	// truth: same AllowedWriteTargets list as internal/agent/hitl/registry.go).
	// An unknown target is REJECTED at load time so the Skill author sees the
	// error at startup, not at first agent run. Empty [] (read-only) is allowed.
	// Phase 12 plan_approve is added to AllowedWriteTargets and automatically
	// accepted here WITHOUT any change to this function (D11-B3 "shared source").
	known := hitl.KnownTargets()
	for _, w := range m.Writes {
		if !containsString(known, w) {
			return fmt.Errorf(
				"writes contains unknown target %q (known targets: %v); "+
					"the target must appear in hitl.AllowedWriteTargets — "+
					"do NOT add it here, update internal/agent/hitl/registry.go instead (D11-B3 shared source)",
				w, known,
			)
		}
	}
	return nil
}

// containsString is an unexported helper local to the skill package.
// The hitl package has its own `Contains()` (exported, post-Task 6);
// we cannot reuse it without flipping the import direction (skill
// cannot import hitl's unexported helper directly). Keeping the
// package-internal copy here costs ~5 lines and avoids a cyclic
// dependency risk if hitl ever needed the helper.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
