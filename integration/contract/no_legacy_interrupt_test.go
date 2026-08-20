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
	"fmt"
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
// this module's build); .git/ is VCS internals; node_modules is
// third-party JS; .planning/ is GSD docs; screenshots/ is binary
// artifacts. Everything else — including _test.go files — is in scope:
// a future test misusing a banned API fails this gate too.
//
// NOTE (Plan 11-08): web/ is REMOVED from the skip set so the Phase 11
// frontend surface (web/src/**) is scanned by the companion Phase 11
// sub-test below. The .go walk will simply find no .go files under
// web/ — zero cost, but the directory traversal proves the gate's
// scope covers the frontend tree.
var legacyInterruptSkipDirs = map[string]bool{
	".claude":      true,
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

// phase11DenyListGo is the Plan 11-08 deny-list for Phase 11 Go
// sources. Each entry is a substring that must NOT appear in any
// scanned .go file (with the file-scope exceptions encoded in
// phase11AllowedStatefulInterruptFiles).
//
// Patterns:
//
//   - Runner.Query(                — the deprecated re-routing API.
//   - .Resume(ctx,                 — the deprecated no-params resume;
//                                    Phase 11 uses ResumeWithParams.
//   - tool.StatefulInterrupt(      — allowed ONLY inside
//                                    internal/agent/hitl/ (the HITL
//                                    Tool files).
//   - tool.Interrupt(              — the deprecated no-args alias.
var phase11DenyListGo = []string{
	"Runner.Query(",
	".Resume(ctx,",
	"tool.StatefulInterrupt(",
	"tool.Interrupt(",
}

// phase11GoScanRoots are the Phase 11 directories scanned by
// TestNoLegacyInterruptAPIPhase11. The union covers the Runner-ization
// (11-01), the HITL Tool infra (11-02..05), and the server approval
// endpoints (11-04).
var phase11GoScanRoots = []string{
	"internal/agent",
	"internal/agent/callbacks",
	"internal/agent/tools",
	"internal/agent/hitl",
	"internal/server",
	"internal/runtime",
}

// phase11AllowedStatefulInterruptFiles lists the .go files permitted
// to call tool.StatefulInterrupt(. Any OTHER .go file using the API
// surfaces as a gate failure — this is the D11-C3 fail-closed
// posture's code-level anchor (the audit_writer.go file MUST NOT
// raise the interrupt itself).
var phase11AllowedStatefulInterruptFiles = map[string]bool{
	"hitl_memory_facts.go":  true,
	"hitl_review_report.go": true,
	"hitl_plan_approve.go":  true, // Phase 12 placeholder — allowed when it lands
}

// phase11LegacyRunnerQueryCarveOut lists the .go files where the
// `s.Runner.Query(` pattern is a PRE-PHASE-11 legacy seam (the local
// server.Runner interface declared in internal/server/server.go:79),
// NOT the eino adk.Runner. These files predate the Runner-ization
// (11-01) and were intentionally left on the legacy path; Phase 11
// scope was the /api/chat handler + approval endpoints, not the
// verdict-pipeline fallback. Documenting the carve-out here keeps the
// gate honest — the pattern is banned in Phase 11 code while the
// pre-existing legacy seam is explicitly acknowledged (and tracked
// for a future Phase 12+ migration).
var phase11LegacyRunnerQueryCarveOut = map[string]bool{
	"pipeline.go": true, // internal/server/pipeline.go:438 — verdict-pipeline fallback on legacy server.Runner
}

// phase11WebDenyList is the Plan 11-08 deny-list for the frontend
// TypeScript tree (web/src/**). Each entry is a substring that must
// NOT appear in any scanned .ts / .tsx file.
var phase11WebDenyList = []string{
	"Runner.Query(",
}

// TestNoLegacyInterruptAPIPhase11 is the Plan 11-08 extension of the
// WalkDir guard. It scans the Phase 11 Go surface
// (internal/agent/{callbacks,tools,hitl}/ + internal/server/) for the
// Phase 11 deny-list, and the frontend tree (web/src/**) for the FE
// deny-list.
//
// This test complements (does NOT replace) TestNoLegacyInterruptAPI:
// the Phase 8 test bans the v0.9 legacy constructors across the whole
// repo; this test bans the Phase 11-specific deprecated patterns
// (Runner.Query, no-params Resume, misplaced StatefulInterrupt, the
// no-args tool.Interrupt alias) across the Phase 11 surface.
//
// Run with:
//
//	go test ./integration/contract -tags=contract -run TestNoLegacyInterruptAPIPhase11 -v
func TestNoLegacyInterruptAPIPhase11(t *testing.T) {
	root := findModuleRoot(t)
	var hits []string

	// ---- Go scan across Phase 11 dirs ----
	for _, rel := range phase11GoScanRoots {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue // dir not present (e.g. a phase that has not landed yet)
		}
		_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") {
				return nil
			}
			// Skip _test.go files: the deny-list patterns legitimately
			// appear in test source as string literals (e.g. this file's
			// own deny-list). Test files are not production code.
			if strings.HasSuffix(name, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(b)
			relPath, _ := filepath.Rel(root, path)
			for _, pat := range phase11DenyListGo {
				if !strings.Contains(s, pat) {
					continue
				}
				// tool.StatefulInterrupt is allowed inside the HITL
				// Tool files only.
				if pat == "tool.StatefulInterrupt(" && phase11AllowedStatefulInterruptFiles[name] {
					continue
				}
				// Runner.Query( is a pre-Phase-11 legacy seam in the
				// verdict-pipeline fallback (carve-out documented at
				// phase11LegacyRunnerQueryCarveOut).
				if pat == "Runner.Query(" && phase11LegacyRunnerQueryCarveOut[name] {
					continue
				}
				hits = append(hits, fmt.Sprintf("%s: %s", relPath, pat))
			}
			return nil
		})
	}

	// ---- Frontend scan across web/src/** ----
	webSrc := filepath.Join(root, "web", "src")
	if _, err := os.Stat(webSrc); err == nil {
		_ = filepath.WalkDir(webSrc, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(b)
			relPath, _ := filepath.Rel(root, path)
			for _, pat := range phase11WebDenyList {
				if strings.Contains(s, pat) {
					hits = append(hits, fmt.Sprintf("%s: %s", relPath, pat))
				}
			}
			return nil
		})
	}

	if len(hits) > 0 {
		t.Fatalf("Phase 11 legacy interrupt API gate hits:\n%s", strings.Join(hits, "\n"))
	}
}
