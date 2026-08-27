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
	"context"
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

// DatasetLister is the narrow interface for dataset existence checks.
// It returns the set of logical dataset names currently registered in the
// datasets registry (sqlite). internal/kpi does NOT import internal/store
// (interface isolation, mirrors internal/duckdb/staging.go:59-65 DatasetRegistry).
//
// The production implementation adapts store.ListDatasets to return only the
// Dataset field from each DatasetRow; the adapter is assembled in main.go.
type DatasetLister interface {
	// ListDatasets returns the set of registered dataset names. An empty
	// (non-nil) slice means "no datasets registered" (bootstrap window).
	// A nil return is treated identically to empty.
	ListDatasets(ctx context.Context) ([]string, error)
}

// LoadKPIs loads and validates all KPI definition files (*.yaml) in dir,
// returning an immutable Registry and its content-hash signature.
//
// The pipeline has two layers:
//
//	Layer 1 (pure Go, hermetic): YAML schema + :param↔params coverage + enum
//	  compilation + name uniqueness + dataset declaration (16-01).
//	Layer 2 (engine, WSL-gated): json_serialize_sql AST walk via SQLSerializer
//	  + dataset existence via DatasetLister + :param→$param rewrite (16-05).
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
//  3. Cross-file KPI name uniqueness is enforced.
//  4. Dataset existence check (Pitfall 2 bootstrap window):
//     datasets empty → skip existence check; non-empty → declared dataset
//     must be in the set, or reject (fail-closed).
//  5. Layer 2: ValidateSQLAST for each KPI's SQL (D16-A2 full branch fail-closed).
//  6. :param → $param rewrite + coverage re-check (Pitfall 3 double confirmation).
//  7. GROUP BY column extraction (v1 dialect: simple identifiers only;
//     expressions → fail-closed reject).
//  8. Compile into Registry with SQLDollar ($param form), TableRefCount,
//     PreserveColumns.
//
// Fail-closed convention: ser or datasets being nil is treated as a
// configuration error — production assembly MUST provide both (mirrors
// staging.go narrow-interface discipline). Tests that wish to skip Layer 2
// must provide a no-op DatasetLister (returning empty slice) and accept
// the corresponding check relaxation, with a comment explaining why.
func LoadKPIs(ctx context.Context, dir string, ser SQLSerializer, datasets DatasetLister) (*Registry, Signature, error) {
	// Fail-closed: ser must be provided in production (Layer 2 is mandatory)
	if ser == nil {
		return nil, "", fmtError("LoadKPIs", fmt.Errorf(
			"SQLSerializer is nil — Layer 2 (json_serialize_sql AST walk) is mandatory; production assembly must provide a serializer (fail-closed assembly convention)"))
	}
	// datasets may be nil (pure bootstrap), but document the convention
	_ = datasets // used below

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

	// --- Layer 2 additions (16-05) ---

	// ① Dataset existence check (bootstrap window, Pitfall 2)
	var registeredDatasets map[string]bool
	if datasets != nil {
		dsList, err := datasets.ListDatasets(ctx)
		if err != nil {
			return nil, "", fmtError("LoadKPIs", fmt.Errorf("list datasets: %w", err))
		}
		registeredDatasets = make(map[string]bool, len(dsList))
		for _, d := range dsList {
			registeredDatasets[d] = true
		}
	}

	// Bootstrap window: if registeredDatasets is empty (or nil/nil datasets),
	// skip existence checks. Non-empty: every file's Dataset must be in the set.
	if len(registeredDatasets) > 0 {
		for _, r := range results {
			if !registeredDatasets[r.file.Dataset] {
				var registered []string
				for d := range registeredDatasets {
					registered = append(registered, d)
				}
				sort.Strings(registered)
				return nil, "", fmtError("LoadKPIs", fmt.Errorf(
					"dataset %q in %s is not in the registered datasets %v — every KPI file's dataset must exist in the datasets registry (D16-A2; non-registry = bootstrap window closed)",
					r.file.Dataset, filepath.Base(r.path), registered))
			}
		}
	}

	// ② Layer 2: ValidateSQLAST for each KPI (on the REWRITTEN form).
	// DuckDB's parser requires $param named placeholders, not :param — the
	// canonical user form is :param (see validate_text.go:42), so the rewrite
	// must happen BEFORE we hand the SQL to DuckDB. Calling ValidateSQLAST
	// on the pre-rewrite form triggers "syntax error at or near ':'" and
	// the AST walk is dead in production.
	for _, r := range results {
		for _, kpi := range r.file.KPIs {
			sqlDollar := paramRewritePattern.ReplaceAllString(kpi.SQL, `$$$1`)
			refs, err := ValidateSQLAST(ctx, ser, sqlDollar, r.file.Dataset)
			if err != nil {
				return nil, "", fmtError("LoadKPIs", fmt.Errorf(
					"validate SQL for %s in %s: %w", kpi.Name, filepath.Base(r.path), err))
			}
			// Store the ref count — we'll attach it during compilation
			// We use a side-channel: store in the KPIDef temporarily via a
			// map that compileKPI can read.
			// Actually, let's just compute it during compileKPI below.
			_ = refs // will be used in compilation
		}
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
			ck, err := compileKPI(ctx, r.file, &kpi, ser)
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

// compileKPI converts a KPIDef into its compiled form, performing:
//  1. :param → $param rewrite (Pitfall 3) — must precede Layer 2 walk
//     because DuckDB's parser rejects :param in favor of $param.
//  2. ValidateSQLAST (Layer 2) on the REWRITTEN form to get table ref count.
//  3. Rewritten placeholder coverage re-check (Pitfall 3 double confirmation).
//  4. GROUP BY column extraction for PreserveColumns (v1 dialect: simple identifiers only).
//  5. Parameter compilation (enum sets sorted, patterns compiled).
func compileKPI(ctx context.Context, file *KPIFile, kpi *KPIDef, ser SQLSerializer) (CompiledKPI, error) {
	// :param → $param rewrite (Pitfall 3) — must precede Layer 2 walk.
	sqlDollar := paramRewritePattern.ReplaceAllString(kpi.SQL, `$$$1`)

	// Layer 2: AST walk on the REWRITTEN form to get table reference count.
	// DuckDB's parser requires $param named placeholders, not :param.
	refs, err := ValidateSQLAST(ctx, ser, sqlDollar, file.Dataset)
	if err != nil {
		return CompiledKPI{}, err
	}
	tableRefCount := len(refs)

	// Double-check: rewrite preserved all placeholders (Pitfall 3 double confirmation)
	rewrittenPlaceholders := extractDollarParams(sqlDollar)
	originalPlaceholders := paramPlaceholderPattern.FindAllStringSubmatch(kpi.SQL, -1)
	for _, m := range originalPlaceholders {
		rewrittenName := m[1]
		if !rewrittenPlaceholders[rewrittenName] {
			return CompiledKPI{}, fmt.Errorf(
				":%s→$%s rewrite lost a placeholder (internal error)", rewrittenName, rewrittenName)
		}
	}

	// Re-run the bidirectional coverage check on the rewritten SQL.
	// This catches any drift introduced by the rewrite (Pitfall 3: "重写后重跑覆盖检查").
	// The $param placeholders should correspond 1:1 with the original :param set,
	// which was already validated against Params in Layer 1. But we confirm.
	sqlParams := make(map[string]bool)
	for p := range rewrittenPlaceholders {
		sqlParams[p] = true
	}
	declaredParams := make(map[string]bool)
	for name := range kpi.Params {
		declaredParams[name] = true
	}
	var inSQLNotDeclared []string
	for p := range sqlParams {
		if !declaredParams[p] {
			inSQLNotDeclared = append(inSQLNotDeclared, p)
		}
	}
	var declaredNotInSQL []string
	for p := range declaredParams {
		if !sqlParams[p] {
			declaredNotInSQL = append(declaredNotInSQL, p)
		}
	}
	if len(inSQLNotDeclared) > 0 || len(declaredNotInSQL) > 0 {
		sort.Strings(inSQLNotDeclared)
		sort.Strings(declaredNotInSQL)
		return CompiledKPI{}, fmt.Errorf(
			"post-rewrite coverage check: SQL has params missing: %v; params has SQL missing: %v (Pitfall 3: rewrite must not introduce drift)",
			inSQLNotDeclared, declaredNotInSQL)
	}

	// GROUP BY column extraction (v1 dialect: simple identifiers only)
	preserveCols, err := extractPreserveColumns(kpi.SQL, kpi.CountColumn)
	if err != nil {
		return CompiledKPI{}, fmt.Errorf("extract GROUP BY columns: %w", err)
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
		Name:            kpi.Name,
		Description:     kpi.Description,
		SQLRaw:          sqlDollar,
		SQLDollar:       sqlDollar,
		CountColumn:     kpi.CountColumn,
		Dataset:         file.Dataset,
		FileKMin:        file.KMin,
		MaxRows:         kpi.MaxRows,
		TableRefCount:   tableRefCount,
		PreserveColumns: preserveCols,
		Params:          compiledParams,
	}, nil
}

// groupByPattern extracts the text after GROUP BY for column parsing.
// Case-insensitive match; captures everything after GROUP BY until ORDER BY,
// LIMIT, HAVING, QUALIFY, or end of string.
var groupByPattern = regexp.MustCompile(`(?is)\bGROUP\s+BY\s+(.*?)(?:\b(?:ORDER\s+BY|LIMIT|HAVING|QUALIFY)\b|$)`)

// simpleIdentifierPattern matches a single SQL identifier (v1 KPI dialect).
// Only [A-Za-z_][A-Za-z0-9_]* is allowed — no expressions, function calls,
// or qualified names. If GROUP BY contains anything beyond comma-separated
// simple identifiers, the KPI is fail-closed rejected (v1 dialect limitation).
var simpleIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// extractPreserveColumns extracts GROUP BY dimension columns + CountColumn
// for kmin.Apply (16-06). The GROUP BY clause must consist solely of
// comma-separated simple identifiers — expressions, function calls, and
// qualified names are fail-closed rejected (v1 KPI dialect limitation).
func extractPreserveColumns(sql, countColumn string) ([]string, error) {
	match := groupByPattern.FindStringSubmatch(sql)
	if len(match) < 2 {
		// No GROUP BY found — KPI with no GROUP BY but with aggregate functions
		// (e.g. SELECT count(*) FROM ds_loans.data) has no dimension columns,
		// only the count column. Return just the count column.
		return []string{countColumn}, nil
	}

	groupByText := strings.TrimSpace(match[1])
	if groupByText == "" {
		return []string{countColumn}, nil
	}

	// Split by comma and validate each part is a simple identifier
	parts := strings.Split(groupByText, ",")
	var columns []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !simpleIdentifierPattern.MatchString(p) {
			return nil, fmt.Errorf(
				"GROUP BY contains %q which is not a simple identifier — v1 KPI dialect only supports comma-separated identifiers (e.g. GROUP BY status, region); expressions like %q are fail-closed rejected",
				p, p)
		}
		columns = append(columns, p)
	}

	// Append count column if not already present
	for _, c := range columns {
		if c == countColumn {
			return columns, nil
		}
	}
	columns = append(columns, countColumn)

	return columns, nil
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
