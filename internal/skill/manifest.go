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

import "strings"

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
	// Template is the optional report template block (D18-A4 — monthly-report
	// Skill uses template: section to declare the report structure + slot types).
	// Stored as any; callers type-assert to *report.Template after yaml.Unmarshal.
	// YAML tag uses inline struct parsing; the field is validated by
	// ValidateAgainstKPI (loader.go) after the full parse.
	Template any `yaml:"template,omitempty" json:"template,omitempty"`
	// Name is the Skill's directory name (e.g. "compliance"). It is
	// NOT a YAML field — the loader derives it from the parent dir
	// basename on read. The wizard (Plan 05-04) sets it before Commit
	// so the on-disk path can be constructed; production loaders leave
	// it empty and rely on the directory layout.
	Name          string   `yaml:"-" json:"-"`
	resolvedTiers []string // not serialized — populated by Loader after Validate
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

// ReadDataAggregatedOnly is the v1.0 data-channel reads value (DATA-07).
// A Skill declaring reads: ["data:aggregated_only"] enters the data channel,
// which is mutually exclusive with tier reads (D16-C1: two storage systems,
// two evidence systems — citation whitelist vs data-reference snapshot;
// mixed-Skill is v1 deferred).
//
// This constant is the sole owner of the data:aggregated_only string literal.
// All channel-kind branching MUST go through HasDataChannel() / Channel()
// (single source of truth discipline — mirrors the validAgentTypes comment
// narrative). v1.1 may add data:external:authorized here, but it must NOT
// appear in any allowlist until explicitly added — unknown data:* values are
// rejected at Validate (fail-closed, D16-C2).
const ReadDataAggregatedOnly = "data:aggregated_only"

// ChannelKind distinguishes the two mutually-exclusive read channels a Skill
// may use (D16-C1). Tier channel reads from Milvus knowledge tiers; data
// channel reads from DuckDB aggregated KPIs. Mixed-Skill is v1 deferred.
type ChannelKind int

const (
	// ChannelKindTier is the default channel: reads resolve to Milvus tier
	// VARCHAR values via ResolveTiers. The retrieve Tool is attached at
	// runtime (D16-C3).
	ChannelKindTier ChannelKind = iota
	// ChannelKindData is the aggregated-KPI channel: reads contain only
	// ReadDataAggregatedOnly; ResolveTiers is skipped (no tier semantics).
	// The kpi_call Tool is attached at runtime (D16-C3).
	ChannelKindData
)

// HasDataChannel reports whether this Skill's reads field declares a data
// channel value (any read with the "data:" prefix). This is the single
// decision point for channel-kind branching — 16-07's BuildRunner tool
// assembly MUST call HasDataChannel() rather than re-parsing reads strings
// (single source of truth, mirrors validAgentTypes enforcement narrative).
//
// Broad detection (any "data:" prefix) rather than exact-match on
// ReadDataAggregatedOnly ensures that unknown data:* modes like
// "data:external:authorized" are routed into the data-channel branch of
// Validate, where the exact-match check produces the correct "unknown data
// channel mode" error with the supported-mode hint — rather than falling
// through to the tier-channel branch and hitting ResolveTiers' defense-in-
// depth guard (which would produce a confusing TierEnum error).
func (m *Manifest) HasDataChannel() bool {
	for _, r := range m.Reads {
		if strings.HasPrefix(r, "data:") {
			return true
		}
	}
	return false
}

// Channel returns the ChannelKind for this Skill's reads. Data channel if
// HasDataChannel() is true; tier channel otherwise (including empty reads,
// which Validate rejects separately).
func (m *Manifest) Channel() ChannelKind {
	if m.HasDataChannel() {
		return ChannelKindData
	}
	return ChannelKindTier
}

// validAgentTypes is the whitelist of agent-type strings accepted in
// the `agent:` field. Source: .planning/SKILL_SCHEMA.md field 2 +
// runtime archetype names (Phase 12 dispatch table).
//
// The Phase 11 list mirrors the Eino ADK package surface
// (chatmodel.go, reactagent.go, sequential.go, parallel.go, loop.go,
// conditional.go). Phase 12 / Plan 12-01 collapses the runtime surface
// to two archetypes — "chat" and "deep" — and routes through
// internal/runtime.DispatchAgent. The Eino ADK agent type names
// ("ChatModelAgent" / "ReActAgent" / etc.) are retained as alias
// values so existing Phase 11 Skills with `agent: ChatModelAgent`
// keep validating without a manifest migration.
//
// "deep" enables the DeepAgent archetype (Phase 12 / Plan 12-01) —
// dispatched by runtime.DispatchAgent to runtime.BuildDeepAgent
// (P12-L2 MaxIteration: 10 hardcoded; WithoutGeneralSubAgent: true;
// WithoutWriteTodos: true).
//
// "orchestrator" enables the FanOutOrchestrator archetype (Phase 13 /
// P8.4 — sub-agent fan-out with structural tier-closure). Dispatched
// by runtime.DispatchAgent to runtime.NewOrchestrator. The orchestrator
// constructs sub-agents per-Run via runtime.BuildSubAgent (P13-L2 +
// FAN-02); sub-agents are read-only by structural design
// (subManifest.Writes = [] — P13-L4). Orchestrator Skill manifests MUST
// declare non-empty Reads (orchestrator itself may declare writes:[],
// since the orchestrator fans out to read-only sub-agents).
//
// The whitelist is a hard filter (D11-B3 pattern): unknown agent
// types are rejected at Validate() so the Skill author sees the
// error at startup, not at first agent run. The runtime dispatch
// layer is defense-in-depth — it re-checks the agent type and
// returns "unknown agent type" if Validate somehow let it through.
var validAgentTypes = map[string]bool{
	// Phase 12 archetype names (canonical).
	"chat":         true,
	"deep":         true,
	"orchestrator": true, // Phase 13 / P8.4 — FanOutOrchestrator (per-Run sub-agent fan-out)
	// Phase 11 Eino ADK names (alias form — kept for backwards
	// compatibility with Skills written before 12-01).
	"ChatModelAgent":   true,
	"ReActAgent":       true,
	"SequentialAgent":  true,
	"ParallelAgent":    true,
	"LoopAgent":        true,
	"ConditionalAgent": true,
}
