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

// bannedLegacyInterruptIdentifiers is the SC-3 ban list (Phase 8 / Plan
// 08-03, RESEARCH Pitfall 5). Identifiers come from eino@v0.9.14
// compose/interrupt.go:44-117: the deprecated no-address interrupt
// constructors still compile in v0.9.14, so nothing but a repo-level gate
// keeps them out of future HITL code.
//
// Why banned: these APIs carry NO interrupt address, so a Resume cannot
// target them deterministically — mixing them with the v0.9 address-based
// Interrupt/Resume semantics (tool.Interrupt / compose.Interrupt /
// adk.Interrupt + InterruptContexts) causes resume-to-wrong-node bugs.
// v0.9 address semantics is the ONLY interrupt semantics allowed here.
//
// NOT banned: compose.WrapInterruptAndRerunIfNeeded — it is the official
// migration helper eino ships precisely to adapt legacy errors to the new
// address-based semantics.
//
// Run this gate with:
//
//	go test ./integration/contract -tags=contract -run TestNoLegacyInterruptAPI -v
var bannedLegacyInterruptIdentifiers = []string{
	"NewInterruptAndRerunErr",     // deprecated func (compose/interrupt.go)
	"compose.InterruptAndRerun",   // deprecated sentinel var (compose/interrupt.go)
	"deprecatedInterruptAndRerun", // internal name — listed to prevent bypass
}

// legacyInterruptSkipDirs are non-product directories excluded from the
// scan. .claude/ contains worktrees with their own go.mod (not part of
// this module's build); web/ is the frontend tree; .git/ is VCS internals;
// node_modules is third-party JS; .planning/ is GSD docs; screenshots/ is
// binary artifacts. Everything else — including _test.go files — is in
// scope: a future test misusing a banned API fails this gate too.
var legacyInterruptSkipDirs = map[string]bool{
	".claude":      true,
	"web":          true,
	".git":         true,
	"node_modules": true,
	".planning":    true,
	"screenshots":  true,
}

// TestNoLegacyInterruptAPI is the SC-3 repo-level lint ban (test-as-gate,
// same layer as TestContractVersionPin). It walks every .go source file
// under the module root and fails on the first occurrence of any banned
// legacy interrupt identifier.
//
// The gate file itself (no_legacy_interrupt_test.go) is skipped: its ban
// list necessarily contains the banned literals.
//
// Scope note (Plan 11-02): the new `internal/agent/hitl/` directory is
// NOT in legacyInterruptSkipDirs, so it is scanned automatically by this
// WalkDir. The new hitl files (approval_decision.go, registry.go,
// hitl_memory_facts.go) use the address-based tool.StatefulInterrupt +
// tool.GetResumeContext API exclusively; running this test with `-count=1`
// confirms zero hits across the four roots: internal/agent/,
// internal/agent/callbacks/, internal/agent/tools/, internal/agent/hitl/.
//
// Negative proof protocol (gate must prove it bites): temporarily drop a
// .go file containing e.g. a comment with "compose.NewInterruptAndRerunErr"
// anywhere inside the scan scope → this test FAILs naming that file:line →
// delete the probe → test is green again. Evidence is recorded in
// 08-03-SUMMARY.md.
func TestNoLegacyInterruptAPI(t *testing.T) {
	root := findModuleRoot(t)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if legacyInterruptSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if name == "no_legacy_interrupt_test.go" {
			return nil // the gate itself carries the banned literals
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		for i, line := range strings.Split(string(content), "\n") {
			for _, ident := range bannedLegacyInterruptIdentifiers {
				if strings.Contains(line, ident) {
					t.Fatalf("%s:%d 使用了被禁的 legacy interrupt API: %s — v0.9 地址语义 Interrupt/Resume 唯一（SC-3 / Pitfall 5）；迁移用官方 WrapInterruptAndRerunIfNeeded 或地址式 tool.Interrupt / compose.Interrupt",
						rel, i+1, ident)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}
