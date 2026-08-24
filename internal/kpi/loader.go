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

package kpi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// paramRewritePattern matches :param named placeholders for rewriting to $param
// (Pitfall 3: DuckDB uses $param + sql.Named; :param is the human-readable file syntax).
// The rewrite only captures [a-zA-Z_][a-zA-Z0-9_]* after the colon.
var paramRewritePattern = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

// LoadKPIs loads and validates all KPI definition files (*.yaml) in dir,
// returning an immutable Registry and its content-hash signature.
//
// Loading discipline (mirrors internal/data/rules.go LoadRules five-stage
// pipeline per file + internal/skill/loader.go Scan directory-walk pattern):
//
//  1. Directory does not exist or is empty → returns empty Registry + empty
//     signature + nil error (empty directory is a valid bootstrap state).
//  2. Per file: Lstat symlink guard → ReadFile → sha256 → yaml.Unmarshal →
//     ValidateKPIFile (Layer 1). Any single file failing any step prevents
//     the entire directory from loading — partial state is unspecified and
//     must not occur (D16-A1 no-registration-backdoor invariant).
//  3. Cross-file KPI name uniqueness is enforced (two files declaring the
//     same KPI name is rejected with both file paths named).
//  4. Signature is the directory aggregate hash: all file contents sorted
//     by filename, concatenated, then sha256 hex (matches the registry
//     whole-atomically-replace semantics — Discretion 8).
//
// Layer 2 (json_serialize_sql AST verification) and dataset-existence checks
// are NOT connected in this plan (16-01). The pipeline reserves an anchor:
//
//	// Layer 2 injection point — 16-05
//
// SQL is stored as the raw :param form; :param→$param rewrite happens at
// compile time (after all validation passes), producing SQLRaw for the
// compiled KPI entry.
func LoadKPIs(dir string) (*Registry, Signature, error) {
	// Check directory existence — empty/missing is a valid bootstrap state
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return newEmptyRegistry(), "", nil
		}
		return nil, "", fmtError("LoadKPIs", fmt.Errorf("stat %s: %w", dir, err))
	}
	if !info.IsDir() {
		return nil, "", fmtError("LoadKPIs", fmt.Errorf("%s is not a directory", dir))
	}

	// Collect *.yaml files (non-recursive, immediate children only)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", fmtError("LoadKPIs", fmt.Errorf("readdir %s: %w", dir, err))
	}

	var yamlFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") {
			yamlFiles = append(yamlFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(yamlFiles) == 0 {
		return newEmptyRegistry(), "", nil
	}

	// Sort for deterministic signature computation
	sort.Strings(yamlFiles)

	// Per-file five-stage pipeline (mirrors rules.go LoadRules)
	type fileResult struct {
		path    string
		file    *KPIFile
		content []byte
		sha     string
	}
	results := make([]fileResult, 0, len(yamlFiles))

	for _, path := range yamlFiles {
		// Stage 1: Lstat symlink guard (V12 mirror)
		fi, err := os.Lstat(path)
		if err != nil {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf("lstat %s: %w", path, err))
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf(
				"%s is a symlink — KPI files are regular controlled files only (V12 symlink-swap guard)", path))
		}

		// Stage 2: ReadFile
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf("read %s: %w", path, err))
		}

		// Stage 3: sha256
		sum := sha256.Sum256(raw)
		sha := hex.EncodeToString(sum[:])

		// Stage 4: yaml.Unmarshal
		var m KPIFile
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf("unmarshal YAML in %s: %w", path, err))
		}

		// Stage 5: ValidateKPIFile (Layer 1)
		if err := ValidateKPIFile(&m); err != nil {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf("validate %s: %w", path, err))
		}

		// Layer 2 injection point — 16-05
		// Dataset existence check injection point — 16-05/16-06

		results = append(results, fileResult{
			path:    path,
			file:    &m,
			content: raw,
			sha:     sha,
		})
	}

	// Cross-file KPI name uniqueness check
	nameToFiles := make(map[string][]string) // kpi name → file paths
	for _, r := range results {
		for _, kpi := range r.file.KPIs {
			nameToFiles[kpi.Name] = append(nameToFiles[kpi.Name], filepath.Base(r.path))
		}
	}
	var dupes []string
	for name, files := range nameToFiles {
		if len(files) > 1 {
			dupes = append(dupes, fmt.Sprintf("%s (in %s)", name, strings.Join(files, ", ")))
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		return nil, "", fmtError("LoadKPIs", fmt.Errorf(
			"KPI names must be globally unique; cross-file duplicates: %v (D16-A1: every KPI must be unambiguously callable by name)",
			dupes))
	}

	// Compute directory aggregate signature (Discretion 8)
	h := sha256.New()
	for _, r := range results {
		h.Write(r.content)
	}
	sig := hex.EncodeToString(h.Sum(nil))

	// Compile into immutable Registry
	compiled := make(map[string]CompiledKPI)
	var order []string
	for _, r := range results {
		for _, kpi := range r.file.KPIs {
			ck, err := compileKPI(r.file, &kpi)
			if err != nil {
				return nil, "", fmtError("LoadKPIs", fmt.Errorf("compile %s/%s: %w", filepath.Base(r.path), kpi.Name, err))
			}
			compiled[kpi.Name] = ck
			order = append(order, kpi.Name)
		}
	}
	sort.Strings(order)

	return &Registry{defs: compiled, order: order, sig: sig}, Signature(sig), nil
}

// compileKPI converts a KPIDef into its compiled form, rewriting :param to
// $param (Pitfall 3) and pre-compiling enum sets and pattern regexps.
func compileKPI(file *KPIFile, kpi *KPIDef) (CompiledKPI, error) {
	// Rewrite :param → $param
	sqlRaw := paramRewritePattern.ReplaceAllString(kpi.SQL, `$$$1`)

	// Verify rewrite preserved all placeholders (double-check after rewrite)
	rewrittenPlaceholders := extractDollarParams(sqlRaw)
	originalPlaceholders := paramPlaceholderPattern.FindAllStringSubmatch(kpi.SQL, -1)
	for _, m := range originalPlaceholders {
		rewrittenName := m[1]
		if !rewrittenPlaceholders[rewrittenName] {
			return CompiledKPI{}, fmt.Errorf(
				":%s→$%s rewrite lost a placeholder (internal error)", rewrittenName, rewrittenName)
		}
	}

	// Compile params
	compiledParams := make(map[string]CompiledParam, len(kpi.Params))
	for name, ps := range kpi.Params {
		cp := CompiledParam{
			Type:     ps.Type,
			Required: ps.Required,
		}
		switch ps.Type {
		case ParamTypeEnum:
			// Sorted copy for deterministic catalog output
			cp.EnumValues = make([]string, len(ps.Values))
			copy(cp.EnumValues, ps.Values)
			sort.Strings(cp.EnumValues)
		case ParamTypeString:
			if ps.Pattern != "" {
				re, err := regexp.Compile(ps.Pattern)
				if err != nil {
					return CompiledKPI{}, fmt.Errorf(
						"params.%s: pattern %q compile failed: %w", name, ps.Pattern, err)
				}
				cp.Pattern = re
			}
		}
		compiledParams[name] = cp
	}

	return CompiledKPI{
		Name:        kpi.Name,
		Description: kpi.Description,
		SQLRaw:      sqlRaw,
		CountColumn: kpi.CountColumn,
		Dataset:     file.Dataset,
		FileKMin:    file.KMin,
		MaxRows:     kpi.MaxRows,
		Params:      compiledParams,
	}, nil
}

// extractDollarParams extracts $param names from rewritten SQL.
func extractDollarParams(sql string) map[string]bool {
	re := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	out := make(map[string]bool, len(matches))
	for _, m := range matches {
		out[m[1]] = true
	}
	return out
}

// newEmptyRegistry returns a valid but empty Registry.
func newEmptyRegistry() *Registry {
	return &Registry{defs: make(map[string]CompiledKPI), order: nil, sig: ""}
}

// isSymlink checks whether the given directory entry is a symlink.
func isSymlink(e fs.DirEntry) bool {
	return e.Type()&os.ModeSymlink != 0
}
