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

// Package skill implements the Skill-as-platform substrate for SkillForge:
// the 7-field SKILL.md manifest, the loader that scans skills/*/SKILL.md on
// startup, and the tier-name → Milvus tier-VARCHAR mapper that the retriever
// consumes.
//
// Scope of this package (Phase 2 plan 02-02):
//   - Manifest + Guardrails structs (the 7-field validation contract).
//   - Validate(*Manifest) enforcing SYSK-02 + SYSK-08.
//   - TierEnum + ResolveTiers (dir-name → CN-char tier resolution; the only
//     place the ASCII reads-pattern world meets the CN-char Milvus world).
//   - Loader.Scan (file-driven registration) + parseFrontmatter.
//
// Package ownership inversion (Wave-1 parallel-safety, per 02-02 plan):
// TierEnum and ResolveTiers live HERE, not in internal/retriever, even
// though the retriever ultimately consumes the resolved tier values.
// Rationale: keep internal/skill → internal/retriever import direction
// forbidden so plan 02-01 and plan 02-02 can run in parallel without
// file conflicts. The retriever takes pre-resolved tier values
// ([]string of CN-char VARCHARs) — it never needs to know the ASCII dir
// names.
package skill

// Manifest is the 7-field SKILL.md v1.0 schema. Field-by-field source of
// truth: .planning/SKILL_SCHEMA.md. All seven fields are REQUIRED and
// validated by Validate(*Manifest) (SYSK-02).
//
// The Temperature field uses *float32 (pointer) on purpose — Pitfall 4
// (02-RESEARCH.md) calls out the Go zero-value trap: a plain float32
// decodes YAML's missing key as 0.0, silently defaulting the Skill to
// deterministic mode without the author knowing. The pointer preserves
// "absent" so Validate can reject it (SYSK-08 — no implicit default).
//
// The resolvedTiers field is NOT yaml-decoded. The Loader populates it
// post-Validate by calling ResolveTiers(m.Reads), so the pipeline in
// plan 02-03 doesn't have to re-resolve on every request.
//
// DescriptionVec is the in-memory cached vector of `Description`, populated
// by Registry.reloadOne via the embedded embedder (Phase 5 D5-01 + D5-06
// for hot-reload). It is in-memory only (json:"-") — the on-disk SKILL.md
// never carries the vector and a JSON round-trip does not preserve it.
type Manifest struct {
	Description    string     `yaml:"description"             json:"description"` // 1 — intent routing text
	Agent          string     `yaml:"agent"                   json:"agent"`       // 2 — Eino ADK agent type
	Reads          []string   `yaml:"reads"                   json:"reads"`       // 3 — tier patterns (no `*` wildcard)
	Writes         []string   `yaml:"writes"                  json:"writes"`      // 4 — operation patterns (`[]` = read-only)
	Steps          string     `yaml:"steps"                   json:"steps"`       // 5 — markdown ordered list (YAML block scalar)
	Guardrails     Guardrails `yaml:"guardrails"              json:"guardrails"`  // 6 — prohibitions + refusal phrases + tier limits
	Temperature    *float32   `yaml:"temperature"             json:"temperature"` // 7 — [0.0, 1.0], pointer = required presence (SYSK-08)
	DescriptionVec []float32  `yaml:"-"                       json:"-"`           // Phase 5 D5-01: cached embedding, in-memory only
	resolvedTiers  []string   // not serialized — populated by Loader after Validate
}

// SetDescriptionVec injects the cached embedding vector for `Description`.
// The setter is the only sanctioned way for Registry code to populate the
// in-memory vector — direct field assignment is reserved for tests (see
// SetResolvedTiersForTest). Production code paths MUST route through this
// setter so a future audit grep for "DescriptionVec =" catches unintended
// persisters.
// (SetDescriptionVec implementation below — see field doc on Manifest.)

// Guardrails is the structured form of SKILL.md field 6. Three sub-fields:
//   - Prohibitions: forbidden actions the Skill must never take.
//   - RequiredRefusalPhrases: phrases that MUST appear in refusal messages.
//   - TierLimits: per-tier operation limits (e.g., authority: [read-only]).
type Guardrails struct {
	Prohibitions           []string            `yaml:"prohibitions"             json:"prohibitions"`
	RequiredRefusalPhrases []string            `yaml:"required_refusal_phrases" json:"required_refusal_phrases"`
	TierLimits             map[string][]string `yaml:"tier_limits"              json:"tier_limits"`
}

// ResolvedTiers returns the unique set of Milvus tier VARCHAR values that
// this Skill may query (e.g., ["权威"] for a Skill with reads:[authority/**]).
// The Loader populates this once after Validate; downstream consumers
// (plan 02-03's pipeline, plan 02-04's GET /api/skills handler) read it
// instead of re-resolving on every request.
//
// Safe to call before Loader.Scan: returns nil (which the retriever treats as
// default-deny via BuildMilvusFilter).
func (m *Manifest) ResolvedTiers() []string {
	return m.resolvedTiers
}

// SetResolvedTiersForTest injects a resolved tier slice directly into the
// manifest, bypassing the file-system Loader. This is the test affordance
// introduced by Plan 03-03 (internal/agent unit tests): those tests need a
// Manifest with a specific resolved-tier slice without paying the disk-I/O
// cost of writing a SKILL.md to a temp dir + invoking Loader.Scan.
//
// Production code paths MUST continue to populate resolvedTiers via the
// Loader (Loader.Scan → Validate → ResolveTiers → assignment). This method
// has the `ForTest` suffix to flag any non-test caller in review.
//
// Calling with nil clears the cache; downstream consumers treat nil as
// default-deny via BuildMilvusFilter.
func (m *Manifest) SetResolvedTiersForTest(tiers []string) {
	m.resolvedTiers = tiers
}

// SetDescriptionVec assigns the cached description embedding vector for
// Phase 5 routing (D5-01, D5-06 hot-reload). The vector is owned by the
// Manifest's lifetime — the caller may hand off its slice (we do not copy).
func (m *Manifest) SetDescriptionVec(v []float32) {
	m.DescriptionVec = v
}

// validAgentTypes is the whitelist of Eino ADK agent type strings accepted
// in the `agent:` field. Source: .planning/SKILL_SCHEMA.md field 2 +
// eino/adk package surface (chatmodel.go:212, reactagent.go, sequential.go,
// parallel.go, loop.go, conditional.go).
var validAgentTypes = map[string]bool{
	"ChatModelAgent":   true,
	"ReActAgent":       true,
	"SequentialAgent":  true,
	"ParallelAgent":    true,
	"LoopAgent":        true,
	"ConditionalAgent": true,
}
