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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs — minimal model.ToolCallingChatModel for wizard tests
// ---------------------------------------------------------------------------

// stubWizardChatModel returns canned responses in order (one per
// Generate call). Tests that need to assert retry semantics pass two
// responses (first invalid → second valid).
type stubWizardChatModel struct {
	responses []string
	errs      []error

	calls int
	mu    sync.Mutex
}

var _ model.ToolCallingChatModel = (*stubWizardChatModel)(nil)

func (m *stubWizardChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.calls
	m.calls++
	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	content := "{}"
	if idx < len(m.responses) {
		content = m.responses[idx]
	}
	if content == "" {
		content = "{}"
	}
	return &schema.Message{Role: schema.Assistant, Content: content}, nil
}

func (m *stubWizardChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *stubWizardChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// CallCount returns the number of Generate invocations (for the retry
// assertion — the first invalid + first retry = 2).
func (m *stubWizardChatModel) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// validWizardManifestJSON is a complete 7-field manifest that passes
// Validate. Wizard.Generate tests use it as the canned LLM response so
// the resulting Manifest passes the 7-field gate.
const validWizardManifestJSON = `{
  "description": "审批流合规审计 Skill，依据权威 tier 制度给出可核验判定",
  "agent": "ChatModelAgent",
  "reads": ["authority/**"],
  "writes": [],
  "steps": "1. 检索权威制度条款\n2. 逐条比对并给出判定\n3. 输出结构化 verdict 或 refusal",
  "guardrails": {
    "prohibitions": [
      "Never cite clauses not retrieved from authority tier",
      "Never paraphrase critical clauses (quote verbatim span)",
      "Never modify authority tier content"
    ],
    "required_refusal_phrases": ["no_authorized_source"],
    "tier_limits": {"authority": ["read-only"]}
  },
  "temperature": 0.0
}`

// invalidWizardManifestJSON is missing temperature (the SYSK-08 trap).
// Tests use it to drive the retry path — first call returns invalid
// (no temperature), second call returns the valid form.
const invalidWizardManifestJSON_NoTemperature = `{
  "description": "审批流合规审计 Skill，依据权威 tier 制度给出可核验判定",
  "agent": "ChatModelAgent",
  "reads": ["authority/**"],
  "writes": [],
  "steps": "1. step",
  "guardrails": {
    "prohibitions": ["a", "b", "c"],
    "required_refusal_phrases": [],
    "tier_limits": {}
  }
}`

// ---------------------------------------------------------------------------
// TestWizard_Name_Pattern — T-5-16 gate. Wizard.Commit rejects names
// that don't match ^[a-z0-9-]+$. Path-escape names like "../../etc/passwd"
// and human-readable names like "Foo Bar" must be rejected BEFORE any
// directory operation (the FIRST gate per UI-SPEC §Security).
// ---------------------------------------------------------------------------
func TestWizard_Name_Pattern(t *testing.T) {
	tmp := t.TempDir()
	w := NewWizard(&stubWizardChatModel{}, tmp)

	cases := []struct {
		name      string
		skillName string
		wantErr   string
	}{
		{"path_escape_dotdot", "../../etc/passwd", "does not match"},
		{"path_escape_slash", "foo/bar", "does not match"},
		{"uppercase_rejected", "Compliance", "does not match"},
		{"underscore_rejected", "compliance_review", "does not match"},
		{"space_rejected", "Foo Bar", "does not match"},
		{"chinese_rejected", "合规审查", "does not match"},
		{"valid_simple", "compliance-audit", ""},
		{"valid_alphanumeric", "qa123", ""},
		{"valid_only_dash", "a-b-c", ""},
		{"empty_rejected", "", "Name is empty"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Name: tc.skillName}
			// Need a Valid manifest for the third gate to pass on
			// valid names; for invalid names the regex gate fires first.
			if tc.wantErr == "" {
				require.NoError(t, json.Unmarshal([]byte(validWizardManifestJSON), m))
				m.Name = tc.skillName
			}
			_, err := w.Commit(context.Background(), m)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr,
					"error should mention the regex constraint; got %q", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWizard_Generate_ValidatesOutput — T-5-17 happy path.
// Generate parses the LLM's JSON, runs Validate, returns the manifest.
// ---------------------------------------------------------------------------
func TestWizard_Generate_ValidatesOutput(t *testing.T) {
	w := NewWizard(&stubWizardChatModel{
		responses: []string{validWizardManifestJSON},
	}, t.TempDir())

	m, err := w.Generate(context.Background(), Answers{
		Purpose:    "审批流合规审计",
		Reads:      []string{"authority/**"},
		SkillType:  "compliance",
		Guardrails: "额外的禁止事项：不要修改权威 tier 内容",
	})
	require.NoError(t, err)
	require.NotNil(t, m)

	// The generated Manifest MUST pass Validate (D5-11).
	require.NoError(t, Validate(m), "the wizard's output must pass the 7-field validator")
	assert.Contains(t, m.Description, "审批流")
	assert.Equal(t, "ChatModelAgent", m.Agent)
	assert.Equal(t, []string{"authority/**"}, m.Reads)
	require.NotNil(t, m.Temperature)
	assert.Equal(t, float32(0.0), *m.Temperature,
		"compliance Skill must use the type-default 0.0 (D5-12)")
	assert.GreaterOrEqual(t, len(m.Guardrails.Prohibitions), MinProhibitions,
		"the validator requires at least %d prohibitions (P-SKILL-2)", MinProhibitions)

	// LLM called exactly once (no retry on a valid first call).
	assert.Equal(t, 1, w.cm.(*stubWizardChatModel).CallCount())
}

// ---------------------------------------------------------------------------
// TestWizard_Generate_RetriesOnValidationFailure — T-5-18.
// First call returns an invalid manifest (no temperature); Generate
// retries once with the validator's error in the prompt. The second
// call returns a valid manifest — Generate returns it.
// ---------------------------------------------------------------------------
func TestWizard_Generate_RetriesOnValidationFailure(t *testing.T) {
	cm := &stubWizardChatModel{
		responses: []string{invalidWizardManifestJSON_NoTemperature, validWizardManifestJSON},
	}
	w := NewWizard(cm, t.TempDir())

	m, err := w.Generate(context.Background(), Answers{
		Purpose:   "审批流合规审计",
		Reads:     []string{"authority/**"},
		SkillType: "compliance",
	})
	require.NoError(t, err, "Generate should retry + succeed on the second call")
	require.NotNil(t, m)
	require.NotNil(t, m.Temperature, "the second-call manifest must include temperature")
	assert.Equal(t, float32(0.0), *m.Temperature)
	assert.Equal(t, 2, cm.CallCount(),
		"first invalid + first retry = 2 LLM calls (T-5-18 bounded retry)")
}

// ---------------------------------------------------------------------------
// TestWizard_Generate_FailsAfterTwoInvalidAttempts — T-5-18 negative.
// Both LLM calls return invalid manifests. Generate returns the
// validation error so the FE can surface it inline per UI-SPEC §SC-3.
// ---------------------------------------------------------------------------
func TestWizard_Generate_FailsAfterTwoInvalidAttempts(t *testing.T) {
	cm := &stubWizardChatModel{
		responses: []string{invalidWizardManifestJSON_NoTemperature, invalidWizardManifestJSON_NoTemperature},
	}
	w := NewWizard(cm, t.TempDir())

	_, err := w.Generate(context.Background(), Answers{
		Purpose: "x", Reads: []string{"authority/**"}, SkillType: "compliance",
	})
	require.Error(t, err, "two invalid attempts must surface the error")
	assert.Contains(t, err.Error(), "validation",
		"the error should mention validation so the FE can render the right copy")
	assert.Equal(t, 2, cm.CallCount(),
		"first attempt + exactly ONE retry = 2 (T-5-18 bounded)")
}

// ---------------------------------------------------------------------------
// TestWizard_Generate_HandlesMarkdownFence — defensive: if the model
// wraps the JSON in ```json ... ``` despite the instruction, parse
// still succeeds (parseWizardManifest strips the fence).
// ---------------------------------------------------------------------------
func TestWizard_Generate_HandlesMarkdownFence(t *testing.T) {
	fenced := "```json\n" + validWizardManifestJSON + "\n```"
	w := NewWizard(&stubWizardChatModel{
		responses: []string{fenced},
	}, t.TempDir())

	m, err := w.Generate(context.Background(), Answers{
		Purpose: "x", Reads: []string{"authority/**"}, SkillType: "compliance",
	})
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "ChatModelAgent", m.Agent)
}

// ---------------------------------------------------------------------------
// TestWizard_Generate_HandlesLLMError — non-retryable network error
// from the first call is surfaced verbatim (no retry on a call-level
// error; only on validation failure).
// ---------------------------------------------------------------------------
func TestWizard_Generate_HandlesLLMError(t *testing.T) {
	cm := &stubWizardChatModel{
		errs: []error{context.DeadlineExceeded},
	}
	w := NewWizard(cm, t.TempDir())

	_, err := w.Generate(context.Background(), Answers{
		Purpose: "x", Reads: []string{"authority/**"}, SkillType: "compliance",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call",
		"call-level errors should propagate with a wrapping context")
	assert.Equal(t, 1, cm.CallCount(),
		"a non-validation error must NOT trigger the retry")
}

// ---------------------------------------------------------------------------
// TestWizard_Commit_AtomicWrite — D5-11: Commit writes a temp file +
// renames atomically. The file at the returned path exists and
// matches the rendered frontmatter.
// ---------------------------------------------------------------------------
func TestWizard_Commit_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	w := NewWizard(&stubWizardChatModel{}, tmp)

	m := &Manifest{Name: "compliance-audit"}
	require.NoError(t, json.Unmarshal([]byte(validWizardManifestJSON), m))
	m.Name = "compliance-audit"

	path, err := w.Commit(context.Background(), m)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// The returned path must be inside tmp and point at SKILL.md.
	assert.True(t, strings.HasPrefix(path, tmp),
		"commit path must live inside the configured skillsDir; got %q", path)
	assert.True(t, strings.HasSuffix(path, "SKILL.md"),
		"commit path must end in SKILL.md; got %q", path)

	// File exists + content has the frontmatter opener and the
	// description field (the most basic round-trip).
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	gotStr := string(got)
	assert.True(t, strings.HasPrefix(gotStr, "---\n"),
		"the written file must start with the YAML frontmatter marker")
	assert.Contains(t, gotStr, "description:")
	assert.Contains(t, gotStr, "temperature:")
	assert.Contains(t, gotStr, m.Description)
}

// ---------------------------------------------------------------------------
// TestWizard_Commit_RejectsPathEscape — T-5-16 second gate.
// Even after the regex gate catches "../../etc/passwd", Commit must
// also confirm the resolved target path lives inside skillsRoot. The
// regex gate is the FIRST line of defense; the resolved-path check is
// the SECOND (V12 ASVS reuse from internal/skill/loader.go).
// ---------------------------------------------------------------------------
func TestWizard_Commit_RejectsPathEscape(t *testing.T) {
	tmp := t.TempDir()
	w := NewWizard(&stubWizardChatModel{}, tmp)

	m := &Manifest{Name: "../../etc/passwd"}
	require.NoError(t, json.Unmarshal([]byte(validWizardManifestJSON), m))
	m.Name = "../../etc/passwd"

	_, err := w.Commit(context.Background(), m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match",
		"the regex gate fires first; error should mention the pattern constraint")

	// Confirm nothing was written.
	entries, readErr := os.ReadDir(tmp)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "the path-escape rejection must not create any directories/files")
}

// ---------------------------------------------------------------------------
// TestWizard_Commit_RejectsInvalidManifest — Validate is the third
// gate. A manifest that fails Validate is rejected BEFORE the temp
// file is written.
// ---------------------------------------------------------------------------
func TestWizard_Commit_RejectsInvalidManifest(t *testing.T) {
	tmp := t.TempDir()
	w := NewWizard(&stubWizardChatModel{}, tmp)

	// Valid name + invalid manifest (no temperature).
	m := &Manifest{
		Name: "broken-skill",
		Description: "broken test",
		Agent: "ChatModelAgent",
		Reads: []string{"authority/**"},
		Writes: []string{},
		Steps: "1. step",
		Guardrails: Guardrails{
			Prohibitions: []string{"a", "b", "c"},
			RequiredRefusalPhrases: []string{},
			TierLimits: map[string][]string{},
		},
		Temperature: nil, // SYSK-08 violation
	}

	_, err := w.Commit(context.Background(), m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation",
		"Validate failure must surface to the caller (D5-11 closed-loop contract)")

	// No file written.
	entries, _ := os.ReadDir(tmp)
	assert.Empty(t, entries, "Validate failure must not create any files")
}

// ---------------------------------------------------------------------------
// TestWizard_Commit_FsnotifyPathClosesLoop — D5-11: the file written
// by Commit lives at skills/<name>/SKILL.md — the exact shape the
// Registry's fsnotify watcher (Plan 05-01) watches. We don't start a
// real Registry here (that's covered by the registry tests), but we
// assert the on-disk path layout matches what Registry.watchLoop
// resolves to (skills/<name>/SKILL.md, basename == SKILL.md).
// ---------------------------------------------------------------------------
func TestWizard_Commit_FsnotifyPathClosesLoop(t *testing.T) {
	tmp := t.TempDir()
	w := NewWizard(&stubWizardChatModel{}, tmp)

	m := &Manifest{Name: "qa-review"}
	require.NoError(t, json.Unmarshal([]byte(validWizardManifestJSON), m))
	m.Name = "qa-review"

	path, err := w.Commit(context.Background(), m)
	require.NoError(t, err)

	// The path layout must be <skillsDir>/<name>/SKILL.md — same
	// layout the Loader's WalkDir + Registry's resolveSkillPath expect.
	assert.Equal(t, filepath.Join(tmp, "qa-review", "SKILL.md"), path,
		"wizard commit path must match the existing Loader/Registry contract")

	// And the parent dir basename + filename must satisfy
	// skillNameFromPath (Plan 05-01 registry helper).
	name := skillNameFromPath(path)
	assert.Equal(t, "qa-review", name,
		"the fsnotify-driven reload path must derive the Skill name from the path")
}
