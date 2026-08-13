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

package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/reprohash"
	"github.com/skillforge/skillforge/internal/skill"
)

// TestComputePromptHash_BodyDrift is THE Q1 regression test.
//
// Before Phase 6, the prompt hash covered only (skillName,
// modelVersion) — so this assertion was IMPOSSIBLE to satisfy: editing
// a Skill's prompt body while holding its name and model constant
// produced an identical hash, and the frozen-set pin (SC-6) would pass
// green over a drifted system.
func TestComputePromptHash_BodyDrift(t *testing.T) {
	const (
		skill = "compliance"
		model = "glm-5.2"
	)
	before := reprohash.PromptHash(skill, model, "You are a strict compliance auditor. Cite every claim.")
	after := reprohash.PromptHash(skill, model, "You are a strict compliance auditor. Cite every claim. Refuse when unsure.")

	assert.NotEqual(t, before, after,
		"editing ONLY the rendered prompt body MUST change the hash (Q1 — this is what makes the frozen prompt_hash pin meaningful)")
}

// TestComputePromptHash_Stable pins the determinism contract: identical
// inputs produce identical output, and each input is load-bearing.
func TestComputePromptHash_Stable(t *testing.T) {
	const (
		skill = "compliance"
		model = "glm-5.2"
		body  = "You are a strict compliance auditor."
	)

	first := reprohash.PromptHash(skill, model, body)
	second := reprohash.PromptHash(skill, model, body)
	assert.Equal(t, first, second, "same (skill, model, body) MUST hash identically across calls")
	assert.Len(t, first, 64, "SHA-256 hex digest is 64 chars")

	for name, got := range map[string]string{
		"skill drift": reprohash.PromptHash("other-skill", model, body),
		"model drift": reprohash.PromptHash(skill, "deepseek-chat", body),
		"body drift":  reprohash.PromptHash(skill, model, body+" Extra."),
	} {
		assert.NotEqual(t, first, got, "%s MUST change the hash", name)
	}

	// Field boundaries are length-prefixed: a rename that merely shifts
	// a character across the skill/body boundary must NOT collide.
	assert.NotEqual(t,
		reprohash.PromptHash("ab", model, "c"),
		reprohash.PromptHash("a", model, "bc"),
		"length-prefixed fields prevent concatenation collisions")
}

// writeLock is a test helper: writes raw JSON to a temp .lock.json and
// returns its path.
func writeLock(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "compliance.lock.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// TestLoadLock_Valid parses a well-formed sidecar into a LockFile.
func TestLoadLock_Valid(t *testing.T) {
	path := writeLock(t, `{
  "model_ver": "glm-5.2",
  "prompt_hash": "abc123",
  "seed": 42,
  "eval_set_version": "compliance-2026-08-13-v1",
  "skill_name": "compliance"
}`)

	lock, err := LoadLock(path)
	require.NoError(t, err)
	assert.Equal(t, "glm-5.2", lock.ModelVersion)
	assert.Equal(t, "abc123", lock.PromptHash)
	assert.Equal(t, int64(42), lock.Seed)
	assert.Equal(t, "compliance-2026-08-13-v1", lock.EvalSetVersion)
	assert.Equal(t, "compliance", lock.SkillName)
}

// TestLoadLock_MissingField: every required field, when absent, yields
// an error that NAMES it. A silently-defaulted field would produce a
// pin that can never fail.
func TestLoadLock_MissingField(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantMissing string
	}{
		{
			name:        "missing model_ver",
			body:        `{"prompt_hash":"h","seed":42,"eval_set_version":"v1","skill_name":"compliance"}`,
			wantMissing: "model_ver",
		},
		{
			name:        "missing prompt_hash",
			body:        `{"model_ver":"glm-5.2","seed":42,"eval_set_version":"v1","skill_name":"compliance"}`,
			wantMissing: "prompt_hash",
		},
		{
			name:        "missing eval_set_version",
			body:        `{"model_ver":"glm-5.2","prompt_hash":"h","seed":42,"skill_name":"compliance"}`,
			wantMissing: "eval_set_version",
		},
		{
			name:        "missing skill_name",
			body:        `{"model_ver":"glm-5.2","prompt_hash":"h","seed":42,"eval_set_version":"v1"}`,
			wantMissing: "skill_name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadLock(writeLock(t, tc.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMissing, "error must name the missing field")
		})
	}
}

// TestLoadLock_Missing/Malformed file paths.
func TestLoadLock_UnreadableAndMalformed(t *testing.T) {
	_, err := LoadLock(filepath.Join(t.TempDir(), "does-not-exist.lock.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eval.LoadLock: open")

	_, err = LoadLock(writeLock(t, `{not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eval.LoadLock: decode")
}

const (
	pinSkill = "compliance"
	pinModel = "glm-5.2"
	pinBody  = "You are a strict compliance auditor. Cite every claim."
)

// validPin builds a LockFile whose prompt_hash matches the (pinSkill,
// pinModel, pinBody) triple — i.e. a lock that AssertPin should accept.
func validPin() LockFile {
	return LockFile{
		ModelVersion:   pinModel,
		PromptHash:     PromptHashFor(pinSkill, pinModel, pinBody),
		Seed:           42,
		EvalSetVersion: "compliance-test-v1",
		SkillName:      pinSkill,
	}
}

// TestAssertPin_AllMatch: a live configuration identical to the pin
// passes.
func TestAssertPin_AllMatch(t *testing.T) {
	assert.NoError(t, AssertPin(validPin(), pinModel, pinSkill, pinBody))
}

// TestAssertPin_ModelDrift: a different live model is named as
// "model_ver drift".
func TestAssertPin_ModelDrift(t *testing.T) {
	err := AssertPin(validPin(), "deepseek-chat", pinSkill, pinBody)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_ver drift")
	assert.Contains(t, err.Error(), "deepseek-chat", "the error must show the live value")
}

// TestAssertPin_PromptHashDrift: the Q1 payoff — the Skill name and the
// model are UNCHANGED, only the rendered body moved, and the pin still
// catches it.
func TestAssertPin_PromptHashDrift(t *testing.T) {
	err := AssertPin(validPin(), pinModel, pinSkill, pinBody+" Also: always refuse on weekends.")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_hash drift")
}

// TestAssertPin_SkillNameDrift: running a frozen set against another
// Skill reports the skill mismatch, NOT a confusing hash drift.
func TestAssertPin_SkillNameDrift(t *testing.T) {
	err := AssertPin(validPin(), pinModel, "creative", pinBody)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill_name drift")
}

// TestActiveModelVersion covers the JUDGE_MODEL override + default.
func TestActiveModelVersion(t *testing.T) {
	t.Setenv("JUDGE_MODEL", "")
	assert.Equal(t, DefaultJudgeModel, ActiveModelVersion())

	t.Setenv("JUDGE_MODEL", "deepseek-chat")
	assert.Equal(t, "deepseek-chat", ActiveModelVersion())
}

// TestShippedFrozenLockIsInSync verifies the REAL
// eval/frozen/compliance.lock.json against the REAL Skill's rendered
// prompt — i.e. it runs AssertPin against the shipped artifacts,
// hermetically (no LLM, no network).
//
// This is what keeps the pin honest between re-freezes. Without it, an
// edit to skills/compliance/SKILL.md would leave a stale lock that only
// fails much later, inside a CI eval run, as a confusing hard stop.
// Here it fails immediately, in the unit suite, with a clear message
// telling the author to re-freeze.
func TestShippedFrozenLockIsInSync(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	lock, err := LoadLock(filepath.Join(repoRoot, "eval", "frozen", "compliance.lock.json"))
	require.NoError(t, err, "the shipped frozen lock must parse")
	assert.Equal(t, "compliance", lock.SkillName)
	assert.Equal(t, DefaultJudgeModel, lock.ModelVersion)

	loader := skill.NewLoader(filepath.Join(repoRoot, "skills"))
	_, err = loader.Scan()
	require.NoError(t, err, "scan skills/")

	var manifest *skill.Manifest
	for _, m := range loader.Skills() {
		if m.Name == "compliance" {
			manifest = m
		}
	}
	require.NotNil(t, manifest, "the compliance Skill must load")

	err = AssertPin(lock, lock.ModelVersion, "compliance", agent.RenderSystemPrompt(manifest))
	assert.NoError(t, err,
		"the shipped lock is stale: the rendered compliance prompt no longer matches the pinned prompt_hash — re-freeze eval/frozen/compliance.lock.json")
}

// TestShippedFrozenSetLoads: the pinned 50-case set must parse with the
// same reader the golden set uses (identical Case schema — D6-13).
func TestShippedFrozenSetLoads(t *testing.T) {
	cases, err := LoadGolden(filepath.Join("..", "..", "eval", "frozen", "compliance.jsonl"))
	require.NoError(t, err)
	assert.Len(t, cases, 50, "the frozen set is the pinned 50-case golden set")

	var refusalCases int
	for _, c := range cases {
		if c.Category == RefusalCategory {
			refusalCases++
		}
	}
	assert.Positive(t, refusalCases,
		"the frozen set must retain refusal-required cases — refusal_accuracy is derived from that category")
}
