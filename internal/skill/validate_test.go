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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validManifest returns a known-good baseline Manifest that satisfies all
// 7 SKILL_SCHEMA fields + all dependent rules. Tests mutate one field
// per subtest to isolate each SYSK-02 / SYSK-08 / P-SKILL-* check.
func validManifest() *Manifest {
	temp := float32(0.0)
	return &Manifest{
		Description: "企业合规/制度审查。意图路由示例。",
		Agent:       "ChatModelAgent",
		Reads:       []string{"authority/**"},
		Writes:      []string{}, // empty (read-only) is valid
		Steps:       "1. Read tier chunks\n2. Compose verdict\n3. Refuse on zero chunks",
		Guardrails: Guardrails{
			Prohibitions: []string{
				"Never cite clauses not retrieved",
				"Never paraphrase critical clauses",
				"Never modify authority tier content",
			},
			RequiredRefusalPhrases: []string{"no_authorized_source"},
			TierLimits:             map[string][]string{"authority": {"read-only"}},
		},
		Temperature: &temp,
	}
}

func TestValidate_CompliantSkill_HappyPath(t *testing.T) {
	// Test 1 — the canonical 7-field SKILL.md passes Validate.
	m := validManifest()
	require.NoError(t, Validate(m, testKBReg()),
		"the canonical 7-field compliance Skill MUST validate (the whole SYSK-02 contract depends on this passing)")
}

func TestValidate_NilManifest(t *testing.T) {
	// Defensive guard — Loader.Scan guards against this too, but
	// Validate itself must reject nil so callers from HTTP handlers
	// (plan 02-04 POST /api/skills) get a clear error.
	var nilM *Manifest
	err := Validate(nilM, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidate_MissingTemperature_SYSK08(t *testing.T) {
	// Test 2 — SYSK-08: pointer-based detection. Missing YAML key
	// decodes to nil; Validate rejects nil.
	m := validManifest()
	m.Temperature = nil
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temperature",
		"the error MUST name the failing field so the Skill author can fix it")
	assert.Contains(t, err.Error(), "SYSK-08",
		"the error MUST cite SYSK-08 so the rationale is in the audit trail")
}

func TestValidate_WildcardReads_PSKILL3(t *testing.T) {
	// Test 3 — P-SKILL-3 anti-pattern. The wildcard would bypass
	// tier-based access control entirely.
	m := validManifest()
	m.Reads = []string{"*"}
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wildcard")
	assert.Contains(t, err.Error(), "P-SKILL-3")
}

func TestValidate_EmptyReads_DefaultDeny(t *testing.T) {
	// Test 4 — empty reads is rejected (default-deny). A Skill with
	// no reads must not silently fall through to "no filter".
	m := validManifest()
	m.Reads = []string{}
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reads")
	assert.True(t,
		strings.Contains(err.Error(), "non-empty") || strings.Contains(err.Error(), "default-deny"),
		"error must mention non-empty or default-deny, got %q", err.Error())
}

func TestValidate_FewerThan3Prohibitions_PSKILL2(t *testing.T) {
	// Test 5 — P-SKILL-2. <3 prohibitions signals an under-specified
	// Skill; the validator is the gate, not the LLM.
	m := validManifest()
	m.Guardrails.Prohibitions = []string{"only one prohibition", "and another"}
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prohibitions")
	assert.Contains(t, err.Error(), "P-SKILL-2")
}

func TestValidate_UnknownAgent_PSKILL_SchemaField2(t *testing.T) {
	// Test 6 — SKILL_SCHEMA field 2: agent must be a known runtime
	// archetype (Phase 12: chat / deep) or a Phase 11 Eino ADK
	// alias (ChatModelAgent / etc.). A typo like "FooAgent" would
	// otherwise silently bind to a no-op at runtime.
	m := validManifest()
	m.Agent = "FooAgent"
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent")
	assert.Contains(t, err.Error(), "ChatModelAgent",
		"error should name at least one valid alternative so the typo is easy to spot")
}

// TestValidate_DeepAgent_P12Archetype is the Phase 12 / Plan 12-01
// regression lock for the "deep" archetype whitelist addition. A
// Skill declaring `agent: deep` MUST validate so the loader accepts
// the file at startup; the runtime.DispatchAgent layer routes the
// construction to runtime.BuildDeepAgent (P12-L2 MaxIteration: 10).
func TestValidate_DeepAgent_P12Archetype(t *testing.T) {
	// Case 1: canonical Phase 12 short name "deep".
	m := validManifest()
	m.Agent = "deep"
	require.NoError(t, Validate(m, testKBReg()),
		"manifest with agent=deep MUST validate (Phase 12 / Plan 12-01 adds 'deep' to the validAgentTypes whitelist so the DeepAgent archetype is accepted at load time)")

	// Case 2: canonical Phase 12 short name "chat".
	m = validManifest()
	m.Agent = "chat"
	require.NoError(t, Validate(m, testKBReg()),
		"manifest with agent=chat MUST validate (Phase 12 archetype name; maps to the Phase 11 ChatModelAgent path)")

	// Case 3: legacy Phase 11 Eino ADK type still validates.
	m = validManifest()
	m.Agent = "ChatModelAgent"
	require.NoError(t, Validate(m, testKBReg()),
		"manifest with agent=ChatModelAgent MUST still validate (Phase 11 alias form preserved for backwards compat with Skills written before 12-01)")

	// Case 4: empty Agent is still rejected (default-deny posture).
	m = validManifest()
	m.Agent = ""
	require.Error(t, Validate(m, testKBReg()),
		"empty agent MUST be rejected (no implicit default; matches the existing 'unknown agent type' check)")
}

// TestValidate_RejectsUnknownAgent is the Phase 12 / Plan 12-01
// negative-path regression lock. A Skill declaring an unknown agent
// type MUST fail Validate() so the loader surfaces the error at
// startup (D11-B3 fail-closed posture).
func TestValidate_RejectsUnknownAgent(t *testing.T) {
	m := validManifest()
	m.Agent = "bogus"
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent",
		"error MUST name the failing field")
	assert.Contains(t, err.Error(), "agent type",
		"error MUST mention 'agent type' so the message matches the runtime.DispatchAgent switch-default error string")
}

func TestValidate_TemperatureOutOfRange_SYSK08(t *testing.T) {
	// Test 7 — SYSK-08 range. 1.5 and -0.1 both rejected.
	for _, bad := range []float32{1.5, -0.1, 1.0001, -0.0001} {
		bad := bad
		name := fmt.Sprintf("temp_%g", float64(bad))
		t.Run(name, func(t *testing.T) {
			m := validManifest()
			m.Temperature = &bad
			err := Validate(m, testKBReg())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "temperature")
			assert.Contains(t, err.Error(), "[0.0, 1.0]")
		})
	}
}

func TestValidate_NilWrites_PSKILL_SchemaField4(t *testing.T) {
	// Test 8 — SKILL_SCHEMA field 4: nil writes = missing field.
	// Empty [] is the valid read-only form; nil is the failure mode.
	m := validManifest()
	m.Writes = nil
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writes")
	assert.Contains(t, err.Error(), "[]",
		"error should hint at `[]` as the read-only form so the author knows the fix")
}

// TestValidate_UnknownKBInReads_SYSK07 — the KBM-04 form of the old
// D2-12 unknown-dir check: a reads pattern naming an unregistered KB is
// rejected at load time and the error lists the active slug set.
func TestValidate_UnknownKBInReads_SYSK07(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"unknown/**"}
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown",
		"error should name the unknown KB")
	assert.Contains(t, err.Error(), "authority",
		"error should list at least one active slug so the author can fix it")
}

// --- Phase 26 / Plan 26-04 Task 1: the reads contract is registry-driven ---

// TestValidateFailsClosedNilRegistry — a tier-channel Skill cannot be
// validated without the knowledge-base registry: the wiring gap must
// surface as a load failure, never as a silent pass (T-26-04-1).
func TestValidateFailsClosedNilRegistry(t *testing.T) {
	m := validManifest() // tier channel: reads [authority/**]
	err := Validate(m, nil)
	require.Error(t, err,
		"tier-channel Skill + nil registry MUST fail closed (no silent pass)")
	assert.Contains(t, err.Error(), "registry",
		"error MUST name the missing registry so the operator fixes the wiring")

	// The same Skill validates once the registry is wired.
	require.NoError(t, Validate(m, testKBReg()),
		"the identical Skill MUST validate against the baseline registry")
}

// TestValidateDataChannelIgnoresRegistry — the data channel has no KB
// semantics (D16-C1 XOR), so a data-channel Skill validates with a nil
// registry: requiring one there would make the registry a hard
// dependency of every data Skill.
func TestValidateDataChannelIgnoresRegistry(t *testing.T) {
	m := validManifest()
	m.Reads = []string{ReadDataAggregatedOnly}
	require.NoError(t, Validate(m, nil),
		"data-channel Skill MUST validate without a registry (D16-C1: data and tier channels are mutually exclusive)")
}

// TestValidateUnknownKBRejected — the KBM-04 load-time gate: a reads
// declaration naming a KB that is not in the registry fails validation.
func TestValidateUnknownKBRejected(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"ghost/**"}
	err := Validate(m, testKBReg())
	require.Error(t, err,
		"reads naming an unregistered KB MUST fail validation at load time (KBM-04)")
	assert.Contains(t, err.Error(), "ghost")
}

// TestValidateDeactivatedKBRejected — deactivation (KBM-02) removes the
// base from the validation surface: skills referencing it stop loading
// instead of silently narrowing to the remaining bases.
func TestValidateDeactivatedKBRejected(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"historical/**"}
	err := Validate(m, testKBRegHistoricalDeactivated())
	require.Error(t, err,
		"reads naming a deactivated KB MUST fail validation (KBM-02: deactivated = invisible)")
	assert.Contains(t, err.Error(), "historical")
}

// TestValidateRoleWildcardAccepted — the KBM-04 role wildcard is part of
// the reads vocabulary: it validates and expands to the role's active
// retrieval-eligible slugs.
func TestValidateRoleWildcardAccepted(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"role:authority"}
	require.NoError(t, Validate(m, testKBReg()),
		"the role wildcard MUST be accepted by the reads contract (KBM-04)")

	got, err := ResolveTiers(m.Reads, testKBReg())
	require.NoError(t, err)
	assert.Equal(t, []string{"authority"}, got,
		"the wildcard MUST expand to the authority-role slug set")
}

// TestLegacyCNLiteralsRejected — the destructive-switch evidence
// (T-26-04-3): the old CN-char tier names are no longer legal slugs, so
// a skill carrying them fails the registry lookup instead of silently
// mapping onto the enum.
func TestLegacyCNLiteralsRejected(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"权威/**"}
	err := Validate(m, testKBReg())
	require.Error(t, err,
		"the legacy CN-char tier literal MUST be rejected — it is not a registered slug (KBM-04 breaking switch)")
	assert.Contains(t, err.Error(), "unknown or deactivated KB")
}

// TestExistingSkillsLoad — the compatibility half of the breaking
// switch: every shipped SKILL.md (skills/ + examples/skills/) keeps
// loading unchanged, because the seed slugs ARE the ASCII directory
// names the files already declare. No spelling migration is required.
func TestExistingSkillsLoad(t *testing.T) {
	repoRoot, err := findRepoRoot(t)
	require.NoError(t, err)

	for _, dir := range []string{"skills", filepath.Join("examples", "skills")} {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			loader := NewLoader(filepath.Join(repoRoot, dir), testKBReg())
			manifests, err := loader.Scan()
			require.NoError(t, err,
				"every shipped SKILL.md MUST load against the seed registry without a spelling migration (KBM-04)")
			require.NotEmpty(t, manifests, "%s must ship at least one Skill", dir)
		})
	}
}

func TestValidate_EmptyDescription_SchemaField1(t *testing.T) {
	// Test 10a — SKILL_SCHEMA field 1: description must be non-empty
	// after TrimSpace (whitespace-only is rejected).
	m := validManifest()
	m.Description = "   \t  "
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestValidate_EmptySteps_SchemaField5(t *testing.T) {
	// Test 10b — SKILL_SCHEMA field 5: steps must be non-empty
	// after TrimSpace (YAML block scalar with only whitespace is
	// rejected).
	m := validManifest()
	m.Steps = ""
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps")
}

// --- Phase 16 / Plan 16-02 Task 2: Channel guard XOR + data+deep rejection ---

// TestChannelGuardXOR enforces D16-C1/C2: data channel and tier channel are
// mutually exclusive at load time. Table-driven with 7 sub-cases covering
// both directions of XOR, unknown data modes, deep+data rejection, and the
// hot-reload window (reloadOne re-runs Validate — Pitfall 6 zero-additional-
// code coverage).
func TestChannelGuardXOR(t *testing.T) {
	temp := float32(0.0)

	cases := []struct {
		name       string
		manifest   *Manifest
		wantErr    bool
		errSubstrs []string // all must appear in the error message
	}{
		{
			name: "mixed_data_and_tier_rejected",
			manifest: &Manifest{
				Description: "mixed data+tier Skill",
				Agent:       "chat",
				Reads:       []string{ReadDataAggregatedOnly, "authority/**"},
				Writes:      []string{},
				Steps:       "1. Call KPI\n2. Read tier",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr: true,
			errSubstrs: []string{
				ReadDataAggregatedOnly, // data channel value named
				"authority/**",         // conflicting tier value named
				"mutually exclusive",   // XOR keyword
				"D16-C1",               // decision citation
			},
		},
		{
			name: "pure_data_channel_passes",
			manifest: &Manifest{
				Description: "pure data Skill",
				Agent:       "chat",
				Reads:       []string{ReadDataAggregatedOnly},
				Writes:      []string{},
				Steps:       "1. Call KPI",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr:    false,
			errSubstrs: nil,
		},
		{
			// v1.1.1 (DATA-07 second half): the external channel read is
			// now a supported mode — it must VALIDATE, not reject.
			name: "external_authorized_data_mode_passes",
			manifest: &Manifest{
				Description: "external authorized data Skill",
				Agent:       "chat",
				Reads:       []string{ReadDataExternalAuthorized},
				Writes:      []string{},
				Steps:       "1. Ask the data officer for approval",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr:    false,
			errSubstrs: nil,
		},
		{
			// The fail-closed default for UNKNOWN data:* modes stays
			// locked: adding a second supported mode must not loosen the
			// allowlist into a prefix-accepts-anything rule.
			name: "truly_unknown_data_mode_rejected",
			manifest: &Manifest{
				Description: "truly unknown data mode",
				Agent:       "chat",
				Reads:       []string{"data:totally_unknown"},
				Writes:      []string{},
				Steps:       "1. Call KPI",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr: true,
			errSubstrs: []string{
				"unknown data channel mode", // the fail-closed framing
				"data:totally_unknown",      // must echo the bad value
				"data:external:authorized",  // must name BOTH supported modes
			},
		},
		{
			name: "deep_plus_data_rejected",
			manifest: &Manifest{
				Description: "deep data Skill",
				Agent:       "deep",
				Reads:       []string{ReadDataAggregatedOnly},
				Writes:      []string{},
				Steps:       "1. Call KPI",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr: true,
			errSubstrs: []string{
				"deep",           // must name the conflicting archetype
				"report:publish", // D18-A4: the conditional-allow error names the exception
				"template:",      // D18-A4: template is part of the condition
			},
		},
		{
			name: "reverse_mixed_tier_and_data_rejected",
			manifest: &Manifest{
				Description: "reversed tier+data Skill",
				Agent:       "chat",
				Reads:       []string{"authority/**", ReadDataAggregatedOnly},
				Writes:      []string{},
				Steps:       "1. Read tier\n2. Call KPI",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_data"},
				},
				Temperature: &temp,
			},
			wantErr: true,
			errSubstrs: []string{
				ReadDataAggregatedOnly,
				"authority/**",
				"mutually exclusive",
			},
		},
		{
			name: "existing_tier_skill_passes_regression",
			manifest: &Manifest{
				Description: "existing compliance Skill",
				Agent:       "ChatModelAgent",
				Reads:       []string{"authority/**"},
				Writes:      []string{},
				Steps:       "1. Read tier chunks\n2. Compose verdict",
				Guardrails: Guardrails{
					Prohibitions:           []string{"p1", "p2", "p3"},
					RequiredRefusalPhrases: []string{"no_authorized_source"},
				},
				Temperature: &temp,
			},
			wantErr:    false,
			errSubstrs: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.manifest, testKBReg())
			if tc.wantErr {
				require.Error(t, err, "Validate MUST reject this manifest")
				for _, sub := range tc.errSubstrs {
					assert.Contains(t, err.Error(), sub,
						"error MUST contain %q (D16-C2 conflict-naming discipline)", sub)
				}
			} else {
				require.NoError(t, err, "Validate MUST accept this manifest")
			}
		})
	}
}

// TestChannelGuardXOR_PureDataChannel_SkipsResolveTiers verifies that a
// data-channel Skill passes Validate with resolvedTiers left empty (nil),
// confirming that the KB registry is never consulted for data-channel
// reads.
func TestChannelGuardXOR_PureDataChannel_SkipsResolveTiers(t *testing.T) {
	m := validManifest()
	m.Reads = []string{ReadDataAggregatedOnly}
	require.NoError(t, Validate(m, testKBReg()))
	assert.Nil(t, m.ResolvedTiers(),
		"data channel Skill MUST have nil resolvedTiers after Validate (the tier resolution path is skipped)")
}

// TestChannelGuardXOR_ReloadOne_HotReloadWindow verifies that the channel
// guard covers the hot-reload path: a Skill file changed to mixed reads
// after initial load is rejected by reloadOne, and the old valid manifest
// is retained in the snapshot (D16-C2 / Pitfall 6).
func TestChannelGuardXOR_ReloadOne_HotReloadWindow(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "dataskill")
	writeCustomSkill(t, skillDir, `description: Valid data Skill.
agent: chat
reads: ["data:aggregated_only"]
writes: []
steps: |
  1. Call KPI
guardrails:
  prohibitions: ["p1", "p2", "p3"]
  required_refusal_phrases: ["no_data"]
  tier_limits: {}
temperature: 0.0
`)

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb, testKBReg())
	require.NoError(t, err)

	// Confirm initial load succeeded.
	snap := r.Snapshot()
	require.Contains(t, snap, "dataskill", "initial load MUST succeed")
	goodDesc := snap["dataskill"].Manifest.Description
	goodVersion := snap["dataskill"].Version

	// Capture load errors via the hook.
	var loadErr atomic.Value // error
	r.SetLoadErrorHook(func(name string, err error) {
		loadErr.Store(err)
	})

	// Overwrite with mixed reads (data+tier) — must be rejected.
	skillPath := filepath.Join(skillDir, "SKILL.md")
	writeCustomSkill(t, skillDir, `description: Mixed data+tier Skill.
agent: chat
reads: ["data:aggregated_only", "authority/**"]
writes: []
steps: |
  1. Call KPI
  2. Read tier
guardrails:
  prohibitions: ["p1", "p2", "p3"]
  required_refusal_phrases: ["no_data"]
  tier_limits: {}
temperature: 0.0
`)

	r.Reload(skillPath)

	// Snapshot MUST retain the old valid manifest (keep-old contract).
	snap = r.Snapshot()
	require.Contains(t, snap, "dataskill",
		"reloadOne MUST NOT remove the skill on validation failure")
	assert.Equal(t, goodDesc, snap["dataskill"].Manifest.Description,
		"reload rejection MUST keep the old description")
	assert.Equal(t, goodVersion, snap["dataskill"].Version,
		"reload rejection MUST NOT bump the version")

	// LoadError hook called with the XOR rejection error.
	gotErr, _ := loadErr.Load().(error)
	require.Error(t, gotErr, "LoadError hook MUST be called on reload rejection")
	assert.Contains(t, gotErr.Error(), "mutually exclusive",
		"reload error MUST contain XOR keyword from Validate()")
}
