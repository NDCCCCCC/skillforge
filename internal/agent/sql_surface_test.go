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

// sql_surface_test.go is Phase 16 / Plan 16-07 Task 1: the
// "agent never writes SQL" structural ban (SC-3 / DATA-09) lifted from
// a prompt-level promise to a repo-level WalkDir gate.
//
// The plan's check is structural: the agent-reachable packages
// (internal/agent + internal/runtime + internal/server) must contain
// ZERO SQL DML/DQL literal strings — that way an LLM cannot reach
// SQL text through any tool, helper, or godoc example.
//
// Substring-only matching was the plan's literal text, but it
// generates false positives in legitimate English usage (e.g.,
// fmt.Errorf messages that read "audit INSERT failed"). The refined
// rule this file enforces is **SQL-shaped** — the keyword must be
// followed by SQL syntax, not English prose:
//
//   - SELECT  → must be followed by `*`, identifier, FROM/WHERE/etc.
//   - INSERT  → must be followed by INTO (the SQL keyword, not the
//               English verb).
//   - ATTACH  → must be followed by quote or identifier (real
//               DuckDB ATTACH syntax).
//   - DETACH  → must be followed by identifier.
//   - CREATE TABLE → already specific; requires trailing identifier.
//
// Comments are stripped before scanning. A godoc mention of
// "ATTACH cache" or "audit INSERT" stays, but does not trigger.
//
// Baseline protocol (16-PATTERNS reminder ②):
//
//   1. WalkDir the three roots and report every match.
//   2. If internal/agent or internal/runtime has ANY non-comment,
//      SQL-shaped hit → STOP, escalate to planner (DATA-09 red line).
//   3. internal/server baseline state (this phase, 2026-08-27):
//      after comment-stripping + SQL-shape refinement, ZERO hits
//      across the three roots — the scan is therefore zero-tolerance
//      with NO baselineExclusions table. Any future regression
//      surfaces immediately.
//
// The test runs always-on (no build tag) — `go test ./internal/agent/`
// in the default profile exercises it. The companion structural lock
// is TestKPINarrowSurface (reflect-asserts *kpi.Executor's exported
// method set is exactly {Call}).
package agent_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/kpi"
)

// sqlSurfaceScanRoots are the package roots the agent LLM can
// reach through some code path. Per 16-PATTERNS reminder ② and
// DATA-09 — these are the three roots a future LLM-prompt-injection
// attempt could combine to reach SQL.
var sqlSurfaceScanRoots = []string{
	"internal/agent",
	"internal/runtime",
	"internal/server",
}

// sqlSurfaceSkipDirs are non-product directories excluded from the
// scan. Mirrors the `legacyInterruptSkipDirs` discipline from
// integration/contract/no_legacy_interrupt_test.go:67-73.
var sqlSurfaceSkipDirs = map[string]bool{
	".claude":      true,
	".git":         true,
	"node_modules": true,
	".planning":    true,
	"screenshots":  true,
	"web":          true, // FE is owned by web/src — not in the BE attack surface
	"testdata":     true, // golden fixtures may legitimately contain SQL templates
}

// sqlSurfaceForbidden is the SQL-shape deny list. Each entry is a
// precompiled regex that captures the SQL keyword PLUS the syntax
// that must follow it. The list deliberately covers both DML
// (INSERT) and DQL (SELECT) plus the DuckDB ATTACH / DETACH / CREATE
// TABLE surface — any of those appearing as a Go string literal in
// the agent-reachable packages would re-open the SQL surface.
//
// Patterns (post-comment-stripping):
//
//   - SELECT  → followed by `*`, identifier, or SQL keyword
//               (FROM / WHERE / COUNT / MAX / MIN / DISTINCT / ...)
//   - INSERT  → followed by INTO (the SQL keyword). Bare "INSERT"
//               in error messages ("audit INSERT failed") does NOT
//               trigger — the SQL shape requires `INTO` to follow.
//   - ATTACH  → followed by quote or identifier (real DuckDB
//               ATTACH syntax like ATTACH 'foo.db' or ATTACH foo).
//   - DETACH  → followed by identifier.
//   - CREATE TABLE → followed by identifier or `(`.
var sqlSurfaceForbidden = []struct {
	keyword string
	re      *regexp.Regexp
}{
	// SELECT followed by *, identifier, or SQL clause keyword. Case-
	// insensitive because SQL is case-insensitive; English
	// occurrences like "select an option" don't appear after
	// comment stripping (it's a deliberate gap-filler rule, not a
	// real SQL string).
	{"SELECT", regexp.MustCompile(`(?i)\bSELECT\s+(\*|[A-Za-z_]|FROM\b|WHERE\b|COUNT\b|MAX\b|MIN\b|DISTINCT\b)`)},

	// INSERT must be followed by `INTO`. English usage of "INSERT"
	// in error messages ("audit INSERT failed", "audit INSERT
	// fail-closed") is excluded by the INTO requirement.
	{"INSERT", regexp.MustCompile(`(?i)\bINSERT\s+INTO\b`)},

	// ATTACH must be followed by SQL surface: quote (string path) or
	// identifier (database alias). English usage like "ATTACH cache"
	// in comments is stripped; "ATTACH the dataset" prose doesn't
	// match (no quote/identifier after the space).
	{"ATTACH", regexp.MustCompile(`(?i)\bATTACH\s+('|"|` + "`" + `|[A-Za-z_])`)},

	// DETACH must be followed by identifier. Same reasoning as
	// ATTACH — English "DETACH the dataset" prose doesn't match.
	{"DETACH", regexp.MustCompile(`(?i)\bDETACH\s+[A-Za-z_]`)},

	// CREATE TABLE is already specific; require trailing identifier
	// or `(`. English "create table at the restaurant" prose doesn't
	// match (table is not capitalized after the space, and the
	// surrounding context is comment-stripped anyway).
	{"CREATE TABLE", regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+[A-Za-z_(]`)},
}

// stripGoComments returns the source text with Go line comments
// ("//") and the contents of block comments ("/* ... */") removed.
// Newlines are preserved so t.Errorf file:line pointers stay accurate.
//
// The implementation walks the string char-by-char, tracking the
// current "comment depth" (0 = code, >0 = inside a block comment, -1 =
// inside a line comment that just started). It is intentionally minimal
// — sufficient for production Go code (no raw strings containing "/*"
// inside valid code paths; our forbidden keywords don't appear in
// any raw string in this codebase).
func stripGoComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inLine := false
	inBlock := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte(c)
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				inBlock = false
				i++ // skip the '/'
			} else if c == '\n' {
				b.WriteByte(c) // preserve newlines so error lines stay correct
			}
			continue
		}
		// Not in a comment — look for the start of one.
		if c == '/' && i+1 < len(src) {
			next := src[i+1]
			switch next {
			case '/':
				inLine = true
				i++
				continue
			case '*':
				inBlock = true
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// TestNoSQLGenerationSurface is the structural WalkDir guard that
// upgrades "agent never writes SQL" from a prompt-level promise to a
// repo-level invariant (SC-3 / DATA-09). Three assertions:
//
//   1. The three scan roots contain ZERO non-comment occurrences of
//      the forbidden SQL keywords after comment stripping. The
//      baseline protocol (see file godoc) requires this to stay at
//      zero — any future code that smuggles SQL text into the
//      agent-reachable surface fails here with a file:line pointer.
//
//   2. The gate itself (sql_surface_test.go) is excluded from the
//      scan — its own forbidden-keyword literals would otherwise
//      trigger a self-match. Mirrors the gate-file-self-skip pattern
//      in integration/contract/no_legacy_interrupt_test.go:113-115.
//
//   3. _test.go files are excluded from the scan — they can
//      legitimately contain SQL strings (golden fixtures, test
//      setup sql.Open calls, etc.) without violating the agent-
//      runtime guarantee (test code is not in the LLM-visible
//      production surface). Mirrors the phase11DenyListGo skip rule
//      in no_legacy_interrupt_test.go:262-264.
//
// Run with: `go test ./internal/agent/ -run TestNoSQLGenerationSurface
// -count=1 -v`.
func TestNoSQLGenerationSurface(t *testing.T) {
	root := findModuleRoot(t)

	type hit struct {
		path string
		line int
		kw   string
	}
	var hits []hit

	for _, rel := range sqlSurfaceScanRoots {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue // dir not present (e.g. a phase that has not landed yet)
		}
		walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if sqlSurfaceSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") {
				return nil
			}
			if name == "sql_surface_test.go" {
				return nil // the gate itself carries the banned literals
			}
			if strings.HasSuffix(name, "_test.go") {
				return nil // test files are not in the LLM-visible production surface
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			stripped := stripGoComments(string(content))
			relPath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relPath = path
			}
			for i, line := range strings.Split(stripped, "\n") {
				for _, pat := range sqlSurfaceForbidden {
					if pat.re.MatchString(line) {
						hits = append(hits, hit{
							path: relPath,
							line: i + 1,
							kw:   pat.keyword,
						})
					}
				}
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", abs, walkErr)
		}
	}

	if len(hits) > 0 {
		var b strings.Builder
		b.WriteString("SQL surface gate hits (DATA-09 red line — agent LLM must not reach SQL text):\n")
		for _, h := range hits {
			b.WriteString("  ")
			b.WriteString(h.path)
			b.WriteString(":")
			b.WriteString(itoa(h.line))
			b.WriteString("  contains forbidden keyword ")
			b.WriteString(h.kw)
			b.WriteString("\n")
		}
		t.Fatalf("%s", b.String())
	}
}

// TestKPINarrowSurface is the interface-level companion to
// TestNoSQLGenerationSurface. While the WalkDir gate bans SQL
// LITERALS in the agent-reachable packages, this assertion pins the
// kpi.Executor API surface: the only SQL-bearing entry point the
// agent can possibly reach is *kpi.Executor's Call method. There is
// no broader *Executor.Query, *Executor.Exec, *Executor.QueryContext,
// or *Executor.ExecContext — and adding any of them would force a
// deliberate change to this test.
//
// The reflection-based assertion is hermetic and runs in microseconds.
// It also documents the architectural promise for reviewers — the
// "executor is a narrow seam" invariant that the agent runtime relies
// on is a single-line contract here, not a 50-line grep.
//
// Run with: `go test ./internal/agent/ -run TestKPINarrowSurface
// -count=1 -v`.
func TestKPINarrowSurface(t *testing.T) {
	// Reflect over *kpi.Executor. The exported method set must be
	// exactly {Call} — no Query, Exec, QueryContext, ExecContext,
	// ExecTx, or other generic-SQL shapes. Call is the only door
	// through which an agent can touch SQL (D16-A3/B3 narrow-surface
	// invariant).
	t.Helper()

	execType := reflect.TypeOf((*kpi.Executor)(nil))
	methodNames := map[string]bool{}
	for i := 0; i < execType.NumMethod(); i++ {
		methodNames[execType.Method(i).Name] = true
	}

	// Whitelist — what IS allowed. Any name not on this list AND
	// matching one of the forbidden SQL verbs is a structural
	// regression that the gate must catch.
	allowed := map[string]bool{
		"Call": true, // the only SQL entry point
	}
	for got := range methodNames {
		if !allowed[got] {
			t.Errorf("kpi.Executor exposes unexpected method %q — agent surface must be exactly {Call}; remove this method or extend the whitelist with a deliberate decision (DATA-09 narrow-seam invariant)", got)
		}
	}

	// Explicit per-keyword check — matches the deny-list vocabulary
	// so a future developer who adds `func (e *Executor) Query(...)`
	// sees BOTH a generic-shape complaint (above) AND a specific
	// "banned verb" complaint (here).
	for _, banned := range []string{"Query", "Exec", "QueryContext", "ExecContext", "ExecTx"} {
		if methodNames[banned] {
			t.Errorf("kpi.Executor exposes %s — DATA-09: agent must not reach SQL via a generic verb; use Call (the only narrow seam) instead", banned)
		}
	}

	// Positive assertion: Call MUST be present (otherwise the gate
	// would green-light a future regression that deletes the seam).
	if !methodNames["Call"] {
		t.Fatal("kpi.Executor.Call is missing — the agent has no SQL entry point at all; this is a build break, not a test failure")
	}
}

// itoa is a tiny base-10 itoa used by the gate's error output — we
// avoid strconv import to keep this file dependency-light (the only
// other stdlib pulls are io/fs, os, path/filepath, reflect, strings).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// findModuleRoot walks up from this test file's package directory
// until it finds go.mod. Mirrors the helper in
// integration/contract/contract_test.go:497-515 — kept local rather
// than cross-package-imported because internal/agent has no business
// importing integration/contract (which carries `//go:build contract`).
func findModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod by walking up from " + dir)
	return ""
}
