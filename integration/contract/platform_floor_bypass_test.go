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

//go:build contract

package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// platformFloorScanRoots are the directories scanned by
// TestNoBypassOfPlatformFloorCompose. Mirrors phase11GoScanRoots
// (no_legacy_interrupt_test.go) — Phase 11 union (6 roots) + Phase 13
// union (8 roots: + audit/sqlite + wire). Kept as a separate list
// (rather than reusing phase11GoScanRoots) so this test stays
// independent of the legacy-interrupt gate's exact root set, per the
// plan: "独立测试，不并入既有 WalkDir 测试，D8-04 同纪律".
var platformFloorScanRoots = []string{
	"internal/agent",
	"internal/agent/callbacks",
	"internal/agent/tools",
	"internal/agent/hitl",
	"internal/server",
	"internal/runtime",
	"internal/store/sqlite",
	"internal/wire",
	// Phase 14 / Plan 14-01: the platform-floor leaf package holds
	// the single canonical definition of platformFloorText and
	// platformFloorTail (Pitfall 17 root). Without it in the scan,
	// assertion 1 would report zero definitions — false positive.
	"internal/platformfloor",
}

// platformFloorSkipDirs mirrors legacyInterruptSkipDirs (Phase 8 / Plan
// 08-03). Non-product directories excluded from the scan. The gate
// file itself is skipped via filename, not directory.
var platformFloorSkipDirs = map[string]bool{
	".claude":      true,
	".git":         true,
	"node_modules": true,
	".planning":    true,
	"screenshots":  true,
}

// TestNoBypassOfPlatformFloorCompose is Plan 14-01 Task 3: the
// structural WalkDir guard for the platform floor composition
// invariant. Three assertions pin the Pitfall 17 root fix:
//
//   1. Only ONE file defines platformFloorText / platformFloorTail (the
//      single canonical home — internal/platformfloor/platform_floor.go).
//      A duplicate definition in any other file would let the two
//      copies silently drift (Pitfall 17's failure mode).
//
//   2. systemPrompt (internal/agent) + renderDeepSystemPrompt
//      (internal/runtime) each contain ≥1 ComposePlatformPrompt call
//      inside their function body. A regression that strips the wrap
//      would let the chat/deep archetype render without the floor.
//
//   3. The Skill-template constants (systemInstructionsTemplate,
//      deepArchetypeBasePrompt) are referenced ONLY in files that also
//      call ComposePlatformPrompt somewhere. A bypass function (a new
//      renderer that uses the template without wrapping) would be
//      flagged here — its file would reference the template but lack
//      the ComposePlatformPrompt call.
//
// Build tag: `//go:build contract` (matches the Phase 8/11/13 gates).
// Run with: `go test ./integration/contract -tags=contract -run
// TestNoBypassOfPlatformFloorCompose -count=1 -v`.
//
// Negative proof (gate must prove it bites): temporarily inject a
// duplicate `const platformFloorText = ...` in any other scanned file
// → this test FAILs naming that file:line → delete the probe → test is
// green again. The negative-proof evidence is recorded in
// 14-01-SUMMARY.md.
func TestNoBypassOfPlatformFloorCompose(t *testing.T) {
	root := findModuleRoot(t)

	// Collect all .go files under the scan roots, applying the skip
	// directory set (mirrors no_legacy_interrupt_test.go's discipline).
	var allGoFiles []string
	for _, rel := range platformFloorScanRoots {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue // dir not present (e.g. a phase that has not landed yet)
		}
		walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if platformFloorSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			if d.Name() == "platform_floor_bypass_test.go" {
				return nil // the gate itself carries the banned literals
			}
			allGoFiles = append(allGoFiles, path)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", abs, walkErr)
		}
	}

	// Assertion 1: exactly ONE file defines platformFloorText and
	// platformFloorTail (the single canonical home). The substring shape
	// ` = \`` matches the Go const declaration form `const X = \`...\``
	// — robust to wording/whitespace changes inside the string body.
	var textDefFiles, tailDefFiles []string
	for _, path := range allGoFiles {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		s := string(content)
		if strings.Contains(s, "platformFloorText = `") {
			rel, _ := filepath.Rel(root, path)
			textDefFiles = append(textDefFiles, rel)
		}
		if strings.Contains(s, "platformFloorTail = `") {
			rel, _ := filepath.Rel(root, path)
			tailDefFiles = append(tailDefFiles, rel)
		}
	}
	if len(textDefFiles) != 1 {
		t.Errorf("Assertion 1 FAILED: platformFloorText defined in %d files (want exactly 1): %v — duplicate definition would re-introduce Pitfall 17 drift",
			len(textDefFiles), textDefFiles)
	}
	if len(tailDefFiles) != 1 {
		t.Errorf("Assertion 1 FAILED: platformFloorTail defined in %d files (want exactly 1): %v — duplicate definition would re-introduce Pitfall 17 drift",
			len(tailDefFiles), tailDefFiles)
	}

	// Assertion 2: the two rendering functions each contain a
	// ComposePlatformPrompt call inside their body. A regression that
	// strips the wrap would let the chat/deep archetype dispatch
	// without the platform floor.
	assertComposeCallInFunc(t, root, "internal/agent/agent.go", "func systemPrompt(")
	assertComposeCallInFunc(t, root, "internal/runtime/deep_agent.go", "func renderDeepSystemPrompt(")

	// Assertion 3: the Skill-template constants (systemInstructionsTemplate,
	// deepArchetypeBasePrompt) are referenced ONLY in files that ALSO
	// DEFINE the constant or call ComposePlatformPrompt somewhere. A
	// bypass renderer would reference the template but lack the compose
	// call.
	//
	// Skip _test.go files: tests reference templates for assertions
	// (e.g. asserting the rendered prompt contains certain phrases);
	// they don't render prompts, so the bypass concern doesn't apply.
	// This mirrors no_legacy_interrupt_test.go's phase11DenyListGo
	// pattern (test files are not production code).
	for _, path := range allGoFiles {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		s := string(content)
		rel, _ := filepath.Rel(root, path)
		hasComposeCall := strings.Contains(s, "ComposePlatformPrompt(")
		for _, tmpl := range []string{"systemInstructionsTemplate", "deepArchetypeBasePrompt"} {
			if !strings.Contains(s, tmpl) {
				continue
			}
			// Files that DEFINE the constant (`const tmpl = \``) are
			// exempt — they're the canonical home for that template
			// copy. internal/server/pipeline.go has its own
			// systemInstructionsTemplate shim (Phase 2 deprecated
			// copy; kept for Phase 8/9/10 acceptance-test compile
			// compat per internal/agent/prompts.go package doc) and
			// is therefore exempt from the "must compose" check.
			// Its code does not actively render prompts in the
			// production path (the chat archetype now goes through
			// internal/agent/systemPrompt + ComposePlatformPrompt).
			if strings.Contains(s, "const "+tmpl+" = `") {
				continue
			}
			// Reference-only file: must wrap with ComposePlatformPrompt
			// (i.e., the template is composed before use). A file that
			// references the template but does NOT compose is a bypass
			// renderer — the gate's purpose.
			if !hasComposeCall {
				t.Errorf("Assertion 3 FAILED: %s references %s but contains no ComposePlatformPrompt call — potential bypass renderer", rel, tmpl)
			}
		}
	}
}

// assertComposeCallInFunc asserts the function body starting at `sig`
// in `relPath` contains a ComposePlatformPrompt call. A regression that
// strips the wrap surfaces here with a file:line pointer to the
// offending function.
//
// The function body is extracted by brace-counting from the signature
// match (a small parser, sufficient for production Go code without
// nested funcs / comments containing braces — the floor-rendering
// functions are all simple).
func assertComposeCallInFunc(t *testing.T, root, relPath, sig string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	content, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	src := string(content)
	idx := strings.Index(src, sig)
	if idx < 0 {
		t.Fatalf("Assertion 2 FAILED: function signature %q not found in %s", sig, relPath)
	}
	// Find the opening brace of the function body.
	braceIdx := strings.Index(src[idx:], "{")
	if braceIdx < 0 {
		t.Fatalf("Assertion 2 FAILED: function %q has no opening brace in %s", sig, relPath)
	}
	braceIdx += idx
	depth := 0
	endIdx := -1
	for i := braceIdx; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				endIdx = i + 1
			}
		}
		if endIdx > 0 {
			break
		}
	}
	if endIdx < 0 {
		t.Fatalf("Assertion 2 FAILED: function %q body not terminated in %s", sig, relPath)
	}
	body := src[braceIdx:endIdx]
	if !strings.Contains(body, "ComposePlatformPrompt(") {
		// Build a small file:line pointer by counting newlines up to
		// the function start — best-effort operator UX.
		line := 1
		for i := 0; i < idx; i++ {
			if src[i] == '\n' {
				line++
			}
		}
		t.Errorf("Assertion 2 FAILED: %s:%d (%s) does not call ComposePlatformPrompt — floor wrap missing, chat/deep archetype would dispatch without the platform floor",
			relPath, line, sig)
	}
}
