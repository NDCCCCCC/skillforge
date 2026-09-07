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

package main

// Wiring locks for the production assemblies that consume the
// knowledge-base registry (CR-01 / CR-03, 26-REVIEW).
//
// Rationale: every one of these failures is INVISIBLE to unit tests.
// The importer tests construct ImporterConfig{KBRegistry: reg} directly,
// so an assembly site that omits the field compiles green and then fails
// only in a real boot (ValidateRules fails closed on a nil registry, so
// every Preview/Commit/Push/DryRun of the loans dataset returns an error
// that looks like a data problem). The lock therefore greps the assembly
// source itself, the same discipline TestBootAssemblyOrder uses for the
// boot order.

import (
	"os"
	"strings"
	"testing"
)

// structLiteral returns the source text of the struct composite literal
// introduced by opener (e.g. "data.ImporterConfig{") up to its MATCHING
// closing brace (nesting-aware), so a wiring lock asserts on a field
// inside ONE specific literal instead of anywhere in the file.
func structLiteral(src, opener string) (string, bool) {
	i := strings.Index(src, opener)
	if i < 0 {
		return "", false
	}
	depth := 0
	for j := i + len(opener) - 1; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1], true
			}
		}
	}
	return "", false
}

// mainGoSource reads cmd/server/main.go once for the wiring locks.
func mainGoSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	return string(raw)
}

// TestImporterConfigWiresKBRegistry is the CR-01 (26-REVIEW) lock: the
// ONLY production construction of data.ImporterConfig must carry the
// registry. Phase 26 made ValidateRules fail closed when the registry is
// nil ("a rule whose trust tier cannot be checked must not load"), so an
// assembly that omits the field regresses the shipped Phase 15 import
// surface — every Preview/Commit/Push/DryRun of datarules/loans.yaml
// fails in a real boot while the unit tests stay green.
func TestImporterConfigWiresKBRegistry(t *testing.T) {
	src := mainGoSource(t)

	lit, ok := structLiteral(src, "data.ImporterConfig{")
	if !ok {
		t.Fatal("main.go no longer constructs data.ImporterConfig — if the import assembly moved, update this lock to the new construction site")
	}
	if !strings.Contains(lit, "KBRegistry: kbReg,") {
		t.Fatal("data.ImporterConfig in main.go does not wire KBRegistry — ValidateRules fails closed on a nil registry, so EVERY import of the loans dataset fails in production (CR-01)")
	}
}
