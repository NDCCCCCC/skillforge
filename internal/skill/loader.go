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
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/skillforge/skillforge/internal/kpi"
	"gopkg.in/yaml.v3"
)

// Loader walks a skills directory at startup, parses each skills/*/SKILL.md
// against the 7-field schema, validates it, resolves the reads-pattern
// tier values, and caches the manifests for the HTTP handler (plan 02-04
// GET /api/skills) to expose.
//
// File-driven registration (D2-07): the single source of truth for which
// Skills exist is the filesystem. There is no DB or config file to keep in
// sync — dropping skills/<name>/SKILL.md into the directory and restarting
// the server is the entire workflow.
//
// Validation errors are NOT silently swallowed (TestLoader_ScanRejectsInvalidSkill
// pins the propagation). The author of an invalid Skill must see the error
// at startup; auto-skipping would mask the typo until a downstream query
// failed in a confusing way.
//
// Phase 5: the per-file parse+validate+tiers body was extracted into the
// unexported helper `parseAndValidate` so the new Registry.reloadOne can
// reuse the same logic (and the same V12 symlink guard) without duplicating
// ~40 lines of fragile frontmatter handling. Loader.Scan still walks the
// directory at startup; Watch lives on Registry (a separate type).
type Loader struct {
	skillsDir string       // absolute path; rejected-against root for symlinks
	cached    []*Manifest  // populated by Scan; guarded by mu
	mu        sync.RWMutex // guards cached: Scan (wizard-commit rescan, request goroutine) writes; Skills (GET /api/skills, request goroutine) reads. Required since the UAT Test 9 fix made Scan reachable post-startup.
}

// NewLoader constructs a Loader rooted at skillsDir. The directory is
// NOT touched until Scan is called (testable: callers can construct +
// point at a temp dir). The path is stored as given; relative paths are
// resolved to absolute by Scan (so callers can pass "./skills" from the
// project root).
func NewLoader(skillsDir string) *Loader {
	return &Loader{skillsDir: skillsDir}
}

// Skills returns the most recently scanned slice of validated manifests.
// Order is filesystem-walk order (sorted by WalkDir's lexical traversal).
// Returns nil if Scan has not been called yet.
//
// Concurrent-safe: takes the read lock because Scan is now reachable from
// a request handler (wizard commit rescan, UAT Test 9) while other request
// handlers read this slice via GET /api/skills.
func (l *Loader) Skills() []*Manifest {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cached
}

// Scan walks the skills directory looking for SKILL.md files, parses
// each, validates the 7-field schema, and caches the result. Returns
// the cached slice (also accessible via Skills()) or the first
// validation error encountered (wrapped with the file path).
//
// On error, the partial state of l.cached is unspecified — callers
// MUST treat the slice as invalid. In practice the server binary
// exits non-zero on the first Scan error, so partial state never
// reaches the HTTP handler.
func (l *Loader) Scan() ([]*Manifest, error) {
	root, err := filepath.Abs(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path for %q: %w", l.skillsDir, err)
	}
	// Stat the root so the error message names a missing directory
	// explicitly (rather than failing later in WalkDir with a less
	// helpful message).
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("skills dir %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills path %q is not a directory", root)
	}

	var out []*Manifest
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Only SKILL.md (exact filename, case-sensitive) inside an
		// immediate subdirectory is a Skill. README.md / notes.md /
		// sub-SKILL.md files are ignored.
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "SKILL.md" {
			return nil
		}
		// The Skill "name" is the parent directory name (e.g.,
		// "compliance" for skills/compliance/SKILL.md). This is the
		// id used by POST /api/chat body { "skill": "compliance" }.
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil || rel == "." || strings.Contains(rel, string(filepath.Separator)) {
			// SKILL.md not in an immediate subdirectory (e.g., loose
			// in the skills root, or three levels deep) is ignored —
			// directory layout is part of the contract.
			return nil
		}

		m, tiers, err := parseAndValidate(path)
		if err != nil {
			return err
		}
		m.resolvedTiers = tiers
		// Stamp the parent-dir basename onto the parsed manifest so
		// downstream consumers (the wizard commit handler verifying a
		// freshly-committed skill landed in the cache; future hot-reload
		// paths) can match a Manifest back to its on-disk name without
		// re-walking the directory. The directory layout contract
		// (skills/<name>/SKILL.md) guarantees this is the canonical id.
		// Benign for existing callers: Manifest.Name is yaml:"-" json:"-".
		m.Name = filepath.Base(filepath.Dir(path))
		out = append(out, m)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	// Publish the rebuilt slice. The filesystem walk runs lock-free (no
	// contention on disk I/O); only the field swap is guarded. Scan assigns
	// a fresh slice rather than appending, so a concurrent Skills() reader
	// holding the previous slice finishes iterating it undisturbed.
	l.mu.Lock()
	l.cached = out
	l.mu.Unlock()
	return out, nil
}

// ValidateAgainstKPI cross-checks every skill's fenced YAML routing table
// (inside steps:) against kpi.Registry.Names(). Called at server boot
// AFTER LoadKPIs and AFTER Loader.Scan (D16-C2 hermetic pattern).
// Failure = log.Fatalf (mirrors D16-08-D LoadKPIs failure mode).
func (l *Loader) ValidateAgainstKPI(reg *kpi.Registry) error {
	manifests := l.Skills()
	for _, m := range manifests {
		if !m.HasDataChannel() {
			continue
		}
		routes, err := parseRoutingTable(m.Steps)
		if err != nil {
			return fmt.Errorf("skill %q: parsing routing table: %w", m.Name, err)
		}
		if len(routes) == 0 {
			continue
		}
		for _, route := range routes {
			if _, ok := reg.Lookup(route.KPI); !ok {
				return fmt.Errorf("skill %q references unknown KPI %q in routing table", m.Name, route.KPI)
			}
		}
	}
	return nil
}

// parseRoutingTable extracts the fenced YAML block from steps and returns route entries.
func parseRoutingTable(steps string) ([]routeEntry, error) {
	start := strings.Index(steps, "```yaml")
	if start == -1 {
		return nil, nil
	}
	// Search for closing ``` in the original string, starting after the opening marker.
	afterOpen := start + 7 // skip past "```yaml"
	end := strings.Index(steps[afterOpen:], "```")
	if end == -1 {
		return nil, fmt.Errorf("unclosed fenced yaml block")
	}
	yamlContent := steps[afterOpen : afterOpen+end]
	var doc struct {
		Routes map[string][]struct {
			Triggers []string `yaml:"triggers"`
			KPI     string    `yaml:"kpi"`
		} `yaml:"routes"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	var entries []routeEntry
	for group, routes := range doc.Routes {
		for _, r := range routes {
			entries = append(entries, routeEntry{Group: group, KPI: r.KPI, Triggers: r.Triggers})
		}
	}
	return entries, nil
}

type routeEntry struct {
	Group    string
	KPI      string
	Triggers []string
}

// parseAndValidate reads a single SKILL.md file, runs the V12 symlink
// guard, parses frontmatter, unmarshals YAML, validates the 7-field
// contract, and resolves the reads-pattern tier values. It is the
// shared parse pipeline used by:
//   - Loader.Scan (Phase 2 startup scan, via WalkDir callback).
//   - Registry.reloadOne (Phase 5 hot-reload; the symlink guard is the
//     single line of defense against atomic-save+rename replacing a
//     SKILL.md with a symlink to /etc/passwd — V12 ASVS).
//
// The root for the symlink guard is computed as filepath.Dir(filepath.Dir(path))
// because the layout contract guarantees every SKILL.md lives at
// <root>/<skillname>/SKILL.md. A loose SKILL.md in the root is rejected
// by the caller (Scan's rel check, the watcher's HasSuffix filter); the
// helper does not need to defend against that case.
//
// On any error the returned Manifest is nil — callers MUST NOT use it.
func parseAndValidate(path string) (*Manifest, []string, error) {
	// V12 ASVS — symlink guard runs on EVERY parse (not just startup).
	// An editor / wizard save+rename cycle can atomically replace a real
	// SKILL.md with a symlink. Re-running this guard on each reload is
	// the only correct posture (T-5-01).
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		root := filepath.Dir(filepath.Dir(path))
		target, evalErr := filepath.EvalSymlinks(path)
		if evalErr != nil {
			return nil, nil, fmt.Errorf("resolve symlink %s: %w", path, evalErr)
		}
		absTarget, absErr := filepath.Abs(target)
		if absErr != nil {
			return nil, nil, fmt.Errorf("abs symlink target %s: %w", target, absErr)
		}
		if !strings.HasPrefix(absTarget, root+string(filepath.Separator)) && absTarget != root {
			return nil, nil, fmt.Errorf(
				"symlink escape rejected: %s -> %s is outside skills root %s (V12 ASVS path-traversal protection)",
				path, absTarget, root,
			)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	front, err := parseFrontmatter(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(front, &m); err != nil {
		return nil, nil, fmt.Errorf("unmarshal YAML in %s: %w", path, err)
	}
	if err := Validate(&m); err != nil {
		return nil, nil, fmt.Errorf("validate %s: %w", path, err)
	}
	// Pre-compute resolved tiers once — the pipeline does not need
	// to re-resolve on every request. Data-channel Skills skip
	// ResolveTiers entirely (no tier semantics — D16-C1).
	var tiers []string
	if !m.HasDataChannel() {
		var err error
		tiers, err = ResolveTiers(m.Reads)
		if err != nil {
			// Validate already rejected unknown dirs, so this is
			// belt-and-braces; treat as fatal anyway.
			return nil, nil, fmt.Errorf("resolve tiers for %s: %w", path, err)
		}
	}
	return &m, tiers, nil
}

// parseFrontmatter extracts the YAML block between the first two `---`
// markers. Returns the YAML bytes ready for yaml.Unmarshal. Pattern
// adapted from scripts/spiked/wire.go:parseSKILL (the spike-era frontmatter
// extractor) with these differences:
//   - Operates on raw bytes (not a file path) so it is testable without
//     a filesystem.
//   - The opening `---` must be on the FIRST line (no leading whitespace
//     or BOM). The spike was lenient; production is strict.
//   - The closing `---` must be at the start of a line. Bodies that
//     contain `---` elsewhere (e.g., a horizontal rule in a markdown
//     body) are NOT prematurely terminated — the closing marker is the
//     first line beginning with `\n---`.
func parseFrontmatter(raw []byte) ([]byte, error) {
	const open = "---\n"
	const close = "\n---"
	// Normalize line endings (CRLF → LF) so the structural markers match
	// on Windows-committed files. yaml.Unmarshal handles either line
	// style natively, but the HasPrefix/Index checks below compare against
	// LF-terminated markers, so CRLF input would otherwise fail with
	// "no YAML frontmatter" (regression caught by the post-merge gate in
	// the 02-02 worktree merge — the canonical compliance SKILL.md was
	// committed with CRLF on Windows).
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	if len(raw) < len(open) || !bytes.Equal(raw[:len(open)], []byte(open)) {
		return nil, errors.New("no YAML frontmatter: file must start with `---` on line 1")
	}
	body := raw[len(open):]
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		return nil, errors.New("unterminated frontmatter: missing closing `---` line")
	}
	// Trim trailing newline so yaml.Unmarshal sees clean YAML.
	front := body[:end]
	if bytes.HasSuffix(front, []byte("\n")) {
		front = front[:len(front)-1]
	}
	return front, nil
}
