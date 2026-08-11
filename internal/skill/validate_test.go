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
	require.NoError(t, Validate(m),
		"the canonical 7-field compliance Skill MUST validate (the whole SYSK-02 contract depends on this passing)")
}

func TestValidate_NilManifest(t *testing.T) {
	// Defensive guard — Loader.Scan guards against this too, but
	// Validate itself must reject nil so callers from HTTP handlers
	// (plan 02-04 POST /api/skills) get a clear error.
	var nilM *Manifest
	err := Validate(nilM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidate_MissingTemperature_SYSK08(t *testing.T) {
	// Test 2 — SYSK-08: pointer-based detection. Missing YAML key
	// decodes to nil; Validate rejects nil.
	m := validManifest()
	m.Temperature = nil
	err := Validate(m)
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
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wildcard")
	assert.Contains(t, err.Error(), "P-SKILL-3")
}

func TestValidate_EmptyReads_DefaultDeny(t *testing.T) {
	// Test 4 — empty reads is rejected (default-deny). A Skill with
	// no reads must not silently fall through to "no filter".
	m := validManifest()
	m.Reads = []string{}
	err := Validate(m)
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
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prohibitions")
	assert.Contains(t, err.Error(), "P-SKILL-2")
}

func TestValidate_UnknownAgent_PSKILL_SchemaField2(t *testing.T) {
	// Test 6 — SKILL_SCHEMA field 2: agent must be a known Eino ADK
	// agent type. A typo like "ChatModleAgent" would otherwise silently
	// bind to a no-op at runtime.
	m := validManifest()
	m.Agent = "FooAgent"
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent")
	assert.Contains(t, err.Error(), "ChatModelAgent",
		"error should name at least one valid alternative so the typo is easy to spot")
}

func TestValidate_TemperatureOutOfRange_SYSK08(t *testing.T) {
	// Test 7 — SYSK-08 range. 1.5 and -0.1 both rejected.
	for _, bad := range []float32{1.5, -0.1, 1.0001, -0.0001} {
		bad := bad
		name := fmt.Sprintf("temp_%g", float64(bad))
		t.Run(name, func(t *testing.T) {
			m := validManifest()
			m.Temperature = &bad
			err := Validate(m)
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
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writes")
	assert.Contains(t, err.Error(), "[]",
		"error should hint at `[]` as the read-only form so the author knows the fix")
}

func TestValidate_UnknownTierInReads_SYSK07_D212(t *testing.T) {
	// Test 9 — SYSK-07 + D2-12: unknown top-level dir → rejection.
	// A typo in the Skill author's reads pattern must NOT silently
	// resolve to "no tier" (fail-open); it must error so the operator
	// sees it at startup.
	m := validManifest()
	m.Reads = []string{"unknown/**"}
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown",
		"error should name the unknown tier dir")
	// error message should list the known dirs so the author can fix it
	assert.Contains(t, err.Error(), "authority")
}

func TestValidate_EmptyDescription_SchemaField1(t *testing.T) {
	// Test 10a — SKILL_SCHEMA field 1: description must be non-empty
	// after TrimSpace (whitespace-only is rejected).
	m := validManifest()
	m.Description = "   \t  "
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestValidate_EmptySteps_SchemaField5(t *testing.T) {
	// Test 10b — SKILL_SCHEMA field 5: steps must be non-empty
	// after TrimSpace (YAML block scalar with only whitespace is
	// rejected).
	m := validManifest()
	m.Steps = ""
	err := Validate(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps")
}

func TestResolveTiers_KnownDirs(t *testing.T) {
	// TierEnum must map all four top-level dirs correctly. This pins
	// the D2-12 ASCII → CN-char mapping against future drift.
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"authority", []string{"authority/**"}, []string{"权威"}},
		{"historical", []string{"historical/**"}, []string{"历史"}},
		{"personal", []string{"personal/**"}, []string{"个人"}},
		{"draft", []string{"draft/**"}, []string{"草稿"}},
		// Multi-pattern dedup: overlapping patterns collapse to one tier.
		{"dedup_overlap", []string{"authority/**", "authority/policies/**"}, []string{"权威"}},
		// Multi-tier Skills: each unique top-level dir produces one entry.
		{"multi_tier", []string{"authority/**", "historical/**"}, []string{"权威", "历史"}},
		// Reads without trailing "**" still resolve via top-level dir.
		{"bare_dir", []string{"authority"}, []string{"权威"}},
		// Deep path: only the top-level dir matters.
		{"deep_path", []string{"authority/policies/anti-bribery/v3"}, []string{"权威"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTiers(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveTiers_UnknownDirErrors(t *testing.T) {
	// Default-deny on unknown tier dir (Pitfall 5 / D2-12). The
	// error must name both the offending pattern and the known set
	// so the Skill author can self-correct without reading code.
	_, err := ResolveTiers([]string{"unknown/**"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown/**",
		"error should echo the bad pattern")
	assert.Contains(t, err.Error(), "authority",
		"error should list at least one known dir as a hint")

	// "admin" looks plausible but is not in the enum.
	_, err = ResolveTiers([]string{"admin/**"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "admin")
}

func TestResolveTiers_EmptyReturnsNilNoError(t *testing.T) {
	// Empty input returns nil, no error — ResolveTiers does NOT
	// enforce default-deny (that is Validate's job). This split keeps
	// each function single-responsibility.
	got, err := ResolveTiers([]string{})
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = ResolveTiers(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}