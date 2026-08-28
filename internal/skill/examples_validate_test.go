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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExamplesValidate is the DIST-08 schema-validation gate for the
// published examples/ tree. It is the runnable proof of the README /
// CONTRIBUTING claim "every example Skill passes the 7-field validator".
//
// CONTRIBUTING.md (Plan 07-03) points contributors at this exact test as
// the canonical way to validate a new SKILL.md — there is intentionally
// NO `cmd/skillforge validate` subcommand (CONTEXT.md Open Question Q4
// resolution: lighter surface, the test is the gate).
//
// The test drives the SAME code path production uses at server startup:
// `skill.NewLoader(<dir>).Scan()` parses every <dir>/<name>/SKILL.md,
// runs Validate() (SYSK-02 / SYSK-08 / P-SKILL-2/3), and returns the
// first validation error. A nil error here means every template in
// examples/skills/ passes the schema gate.
//
// Four templates MUST be present (D7-10 originals + the Phase 13 addition):
//   - compliance  — verbatim copy of skills/compliance/SKILL.md (the only
//     runnable Skill; the canonical 7-field example).
//   - simple-qa   — a minimal RAG Skill (ChatModelAgent, reads authority/**).
//   - calculator  — a tool-type Skill that declares `reads: [draft/**]` as
//     a PLACEHOLDER tier so it passes Validate without weakening the
//     default-deny invariant (P-SKILL-3). The runtime does not yet
//     execute pure-tool Skills (D7-10); the template exists to show
//     Skill-format diversity and to prove the validator stays unchanged.
//   - compliance-orchestrator — Phase 13 / Plan 13-05's `agent:
//     orchestrator` parent Skill (reads authority+historical, writes []).
//     It is the --sub-skill-config target of the 20-case fan-out UAT
//     (eval/golden/fanout.jsonl) and proves the SAME 7-field validator
//     accepts the third archetype without schema changes — the
//     Skill-driven platform thesis extended by file, not by code.
//
// Path note: this test file lives at internal/skill/examples_validate_test.go,
// so the relative path "../../examples/skills" resolves from the package
// directory <root>/internal/skill/ to <root>/examples/skills/. `go test
// ./internal/skill` runs with the package directory as cwd, so the
// relative path works in any clone without env vars.
func TestExamplesValidate(t *testing.T) {
	loader := NewLoader("../../examples/skills")
	manifests, err := loader.Scan()
	require.NoError(t,
		err,
		"every example SKILL.md MUST pass the 7-field validator (DIST-08 gate); "+
			"a validation error here means a published example is non-conformant",
	)

	// Exactly six templates ship under examples/skills/ (D7-10's three +
	// Phase 13's compliance-orchestrator + Phase 17's kpi-dashboard +
	// Phase 18's monthly-report). Asserting the count catches both accidental
	// deletions (regression) and accidentally-shipping typos (e.g., a scratch
	// file matching SKILL.md).
	require.Len(t,
		manifests, 6,
		"examples/skills/ MUST contain exactly 6 Skill templates (compliance, simple-qa, calculator, compliance-orchestrator, kpi-dashboard, monthly-report); "+
			"got %d manifests — check for stray SKILL.md files or a missing template",
		len(manifests),
	)

	// The Skill "name" is the parent directory basename (Loader.Scan stamps
	// it from the path). Collect names so the diagnostic is useful when the
	// count is right but a template is misnamed.
	names := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		names[m.Name] = true
	}
	for _, want := range []string{"compliance", "simple-qa", "calculator", "compliance-orchestrator", "kpi-dashboard"} {
		assert.True(t,
			names[want],
			"expected examples/skills/%s/SKILL.md to be scanned; got names %v",
			want, names,
		)
	}
}
