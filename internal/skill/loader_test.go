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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSkillFile writes a minimal-valid 7-field SKILL.md at <dir>/SKILL.md.
// The fixture body is a YAML block scalar for steps so the parser exercises
// the realistic form. Returns the absolute path written.
func writeSkillFile(t *testing.T, dir string, overrides func(m *map[string]any)) string {
	t.Helper()
	// Build YAML as a string with predictable fields, then mutate via overrides.
	body := `description: Test Skill. Trigger phrase test.
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Read tier chunks
  2. Compose verdict
  3. Refuse on zero chunks
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved"
    - "Never paraphrase critical clauses"
    - "Never modify authority tier content"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
`
	// We avoid a YAML round-trip here — the tests that need custom fields
	// pass overrides that mutate a parsed map and then re-emit. For
	// simplicity in this file, callers construct the body directly via
	// the variations below. This helper is the simple path.
	if overrides != nil {
		// For tests that mutate one field, callers use writeCustomSkill
		// below — this branch is intentionally a no-op so the simple
		// path stays obvious.
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\n"+body+"---\n\n# Test Skill Body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeCustomSkill writes a SKILL.md with arbitrary frontmatter. The
// frontmatter must already be a complete YAML document (between `---`).
// Tests that need to omit a field (e.g., for the missing-temperature case)
// use this directly so the failure mode is unambiguous.
func writeCustomSkill(t *testing.T, dir, frontmatter string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	body := "---\n" + frontmatter + "---\n\n# Body\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoader_ScanCompliance_HappyPath(t *testing.T) {
	// Test 1 — SYSK-01 happy path: drop a SKILL.md, Scan discovers it.
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	writeSkillFile(t, skillDir, nil)

	l := NewLoader(tmp)
	skills, err := l.Scan()
	require.NoError(t, err)
	require.Len(t, skills, 1, "exactly one Skill expected")

	m := skills[0]
	assert.Contains(t, m.Description, "Trigger phrase")
	assert.Equal(t, "ChatModelAgent", m.Agent)
	assert.Equal(t, []string{"authority/**"}, m.Reads)
	require.NotNil(t, m.Temperature, "pointer MUST be non-nil after Scan")
	assert.Equal(t, float32(0.0), *m.Temperature)
	// Resolved tiers pre-computed.
	assert.Equal(t, []string{"权威"}, m.ResolvedTiers(),
		"compliance Skill reads=[authority/**] must resolve to ['权威'] (CMPL-04)")
}

func TestLoader_ScanIgnoresNonSkillMD(t *testing.T) {
	// Test 2 — only SKILL.md is a Skill. README.md, notes.md, etc. are
	// ignored. A dir with no SKILL.md at all returns an empty slice.
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, "README.md"), []byte("# Notes\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, "notes.md"), []byte("free text\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755))

	l := NewLoader(tmp)
	skills, err := l.Scan()
	require.NoError(t, err)
	assert.Empty(t, skills, "no SKILL.md → empty slice, no error")
}

func TestLoader_ScanRejectsInvalidSkill(t *testing.T) {
	// Test 3 — a SKILL.md missing temperature is rejected; the error
	// propagates (NOT a silent skip). This is the SYSK-08 gate at load
	// time: operators see missing-temperature at startup, not in prod
	// logs days later.
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "broken")
	// No temperature field — Validate rejects with SYSK-08 message.
	fm := `description: Broken test.
agent: ChatModelAgent
reads: ["authority/**"]
writes: []
steps: "1. Read tier"
guardrails:
  prohibitions: ["p1", "p2", "p3"]
  required_refusal_phrases: []
  tier_limits: {}
`
	writeCustomSkill(t, skillDir, fm)

	l := NewLoader(tmp)
	_, err := l.Scan()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temperature")
	assert.Contains(t, err.Error(), "SYSK-08")
}

func TestLoader_ScanFindsMultipleSkills(t *testing.T) {
	// Test 4 — multiple Skills discovered in one Scan.
	tmp := t.TempDir()
	writeSkillFile(t, filepath.Join(tmp, "compliance"), nil)
	writeSkillFile(t, filepath.Join(tmp, "qa"), nil)

	l := NewLoader(tmp)
	skills, err := l.Scan()
	require.NoError(t, err)
	assert.Len(t, skills, 2)

	names := []string{}
	for _, s := range skills {
		names = append(names, s.Description)
	}
	// Both should be present (description is identical in our fixture,
	// but the count is the load-bearing assertion).
	assert.Contains(t, names, "Test Skill. Trigger phrase test.")
}

// TestLoader_ScanStampsManifestNameFromParentDir — UAT Test 9 regression
// helper: Scan MUST stamp Manifest.Name with the parent-directory basename
// (skills/<name>/SKILL.md → Name="<name>"). The wizard commit handler
// relies on this to verify that a freshly-committed Skill landed in the
// Loader's rescanned cache; without it, Registered would always be false
// and the FE toast would never fire.
func TestLoader_ScanStampsManifestNameFromParentDir(t *testing.T) {
	tmp := t.TempDir()
	writeSkillFile(t, filepath.Join(tmp, "compliance"), nil)
	writeSkillFile(t, filepath.Join(tmp, "qa"), nil)

	l := NewLoader(tmp)
	skills, err := l.Scan()
	require.NoError(t, err)
	require.Len(t, skills, 2)

	gotNames := map[string]bool{}
	for _, s := range skills {
		gotNames[s.Name] = true
	}
	assert.True(t, gotNames["compliance"],
		"Manifest.Name MUST be stamped from the parent dir basename; got %v", gotNames)
	assert.True(t, gotNames["qa"],
		"Manifest.Name MUST be stamped from the parent dir basename; got %v", gotNames)
}

// TestLoader_ScanAndSkillsConcurrentNoRace — UAT Test 9 regression: the fix
// made Loader.Scan reachable from a request handler (wizard commit rescan)
// while GET /api/skills reads Loader.Skills from another handler goroutine.
// This hammers Scan (writer) and Skills (reader) concurrently. Run under
// `go test -race` — it FAILS (data race on cached) if anyone removes the
// RWMutex added alongside the Test 9 fix.
func TestLoader_ScanAndSkillsConcurrentNoRace(t *testing.T) {
	tmp := t.TempDir()
	writeSkillFile(t, filepath.Join(tmp, "compliance"), nil)

	l := NewLoader(tmp)
	if _, err := l.Scan(); err != nil {
		t.Fatalf("prime scan: %v", err)
	}

	var wg sync.WaitGroup
	const writers, readers, iterations = 4, 8, 300

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if _, err := l.Scan(); err != nil {
					t.Errorf("scan: %v", err)
					return
				}
			}
		}()
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = l.Skills()
			}
		}()
	}
	wg.Wait()
}

func TestLoader_ScanRejectsSymlinkEscape(t *testing.T) {
	// Test 5 — V12 ASVS path-traversal protection. A SKILL.md symlink
	// pointing OUTSIDE the skills root is rejected. The attack is:
	// /tmp/skills/evil -> /etc/passwd  (or any file outside the root).
	//
	// On Windows, os.Symlink requires either Developer Mode or admin
	// elevation (CreateSymbolicLinkW returns "A required privilege is
	// not held by the client"). CI runners and most developer boxes
	// do not have this. The production code's symlink-rejection logic
	// is still loaded and reachable on every platform via the loader
	// path — the only thing skipped here is the symlink-creation
	// pre-condition, not the check.
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Developer Mode / admin on Windows; " +
			"the loader's symlink-rejection code is still compiled in (run on linux/macOS for full coverage)")
	}
	tmp := t.TempDir()
	// The "evil" skill dir contains a symlink whose final target is outside.
	evilDir := filepath.Join(tmp, "evil")
	require.NoError(t, os.MkdirAll(evilDir, 0o755))
	// Create a real SKILL.md OUTSIDE the root.
	outside := t.TempDir()
	outsideSkill := filepath.Join(outside, "SKILL.md")
	require.NoError(t, os.WriteFile(outsideSkill, []byte("---\ndescription: x\n"), 0o644))
	// Symlink: <tmp>/evil/SKILL.md -> <outside>/SKILL.md (escape!)
	require.NoError(t, os.Symlink(outsideSkill, filepath.Join(evilDir, "SKILL.md")))

	l := NewLoader(tmp)
	_, err := l.Scan()
	require.Error(t, err, "symlink escape must be rejected")
	assert.True(t,
		strings.Contains(err.Error(), "symlink") || strings.Contains(err.Error(), "outside") || strings.Contains(err.Error(), "V12"),
		"error must mention symlink/outside/V12 so the operator understands why the Skill was rejected, got %q", err.Error())
}

func TestLoader_ParseComplianceCanonicalSkill(t *testing.T) {
	// Test 6 — CMPL-01: the canonical compliance SKILL.md loads
	// cleanly. The repo default Skill is simple-qa (quick task
	// 260814-vga); the compliance Skill ships as the examples/
	// template downstream plans (02-03 pipeline, 02-04 GET /api/skills)
	// rely on.
	repoRoot, err := findRepoRoot(t)
	require.NoError(t, err)

	skillsDir := filepath.Join(repoRoot, "examples", "skills")
	l := NewLoader(skillsDir)
	skills, err := l.Scan()
	require.NoError(t, err)
	require.NotEmpty(t, skills, "the canonical compliance Skill MUST exist (CMPL-01)")

	var found *Manifest
	for _, m := range skills {
		if strings.HasPrefix(m.Description, "企业合规") {
			found = m
			break
		}
	}
	require.NotNil(t, found, "the canonical compliance Skill MUST be discoverable in examples/skills/")
	assert.Contains(t, found.Description, "合规", "compliance Skill description must be the canonical one")
	assert.Equal(t, []string{"authority/**"}, found.Reads)
	require.NotNil(t, found.Temperature)
	assert.Equal(t, float32(0.0), *found.Temperature,
		"compliance Skill temperature MUST be 0.0 (deterministic for compliance/data Skills)")
	assert.Equal(t, []string{"权威"}, found.ResolvedTiers(),
		"compliance Skill ResolvedTiers MUST be ['权威'] (CMPL-04)")
}

func TestLoader_ScanRejectsFrontmatterWithoutOpeningMarker(t *testing.T) {
	// Defensive: a file named SKILL.md with no `---` opener fails the
	// parse and the error wraps the file path.
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "no-frontmatter")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("# This file has no frontmatter at all\n"),
		0o644))

	l := NewLoader(tmp)
	_, err := l.Scan()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no YAML frontmatter")
}

func TestLoader_ScanReturnsEmptyForMissingDir(t *testing.T) {
	// Defensive: a non-existent skills dir is reported explicitly
	// (so the operator notices a misconfigured SERVER_SKILLS_DIR env
	// immediately rather than seeing "0 Skills" and wondering why).
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist")

	l := NewLoader(missing)
	_, err := l.Scan()
	require.Error(t, err)
	assert.True(t,
		errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file"),
		"error should be a not-exist error, got %v", err)
}

func TestParseFrontmatter_HappyPath(t *testing.T) {
	// Unit test the parseFrontmatter helper directly — no filesystem.
	raw := []byte("---\nfoo: bar\nbaz: 42\n---\n# body\n")
	front, err := parseFrontmatter(raw)
	require.NoError(t, err)
	assert.Equal(t, "foo: bar\nbaz: 42", string(front))
}

func TestParseFrontmatter_NoOpener(t *testing.T) {
	raw := []byte("# no opener\n")
	_, err := parseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no YAML frontmatter")
}

func TestParseFrontmatter_NoCloser(t *testing.T) {
	raw := []byte("---\nfoo: bar\nno closer here\n")
	_, err := parseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unterminated")
}

// findRepoRoot walks up from the test's CWD to locate go.mod. Test-only
// helper — keeps TestLoader_ParseComplianceCanonicalSkill independent of
// the cmd/spiked/wire.go:findProjectRoot helper (which lives in the
// binary package, not the library package).
func findRepoRoot(t *testing.T) (string, error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("go.mod not found within 12 levels of test CWD")
}
