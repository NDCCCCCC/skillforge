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

package data

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/skill"
	"gopkg.in/yaml.v3"
)

// Rules is the in-memory form of a datarules/{dataset}.yaml rule file
// (D15-A1 mixed carrier: per-dataset column→rule mapping is a declarative
// file data staff can write; the standard PII identification rules live
// in Go code — normalize.go / desensitize.go — and are versioned with
// the repo).
//
// The file is a CONTROLLED file: versioned in git, changed via PR review
// (the primary defense against a passthrough misdeclaration — T-15-06),
// and its content digest (rules_sha256) rides every import-batch audit
// row so "which rule version desensitized this batch" is always
// provable (D15-A4).
type Rules struct {
	Dataset string                `yaml:"dataset"`
	Tier    string                `yaml:"tier"`
	KMin    int                   `yaml:"k_min"`
	Columns map[string]ColumnRule `yaml:"columns"`
}

// ColumnRule is the per-column decision. Exactly one of the three rule
// kinds applies; every field that is meaningless for the chosen kind
// must stay unset (ValidateRules rejects contradictory combinations —
// a silently-ignored YAML typo is exactly the error class a controlled
// file must surface at load time).
type ColumnRule struct {
	Rule       string  `yaml:"rule"`
	Profile    string  `yaml:"profile"`
	Width      float64 `yaml:"width"`
	DuckDBType string  `yaml:"duckdb_type"`
}

// Rule-kind vocabulary (D15-D4): v1 L2 is numeric fixed-width bucketing
// ONLY. A categorical sensitive field must choose hash, or passthrough
// and rely on the k_min query-side defense line — other L2 kinds are
// fail-closed rejected here, not silently degraded.
const (
	RuleHash        = "hash"
	RulePassthrough = "passthrough"
	RuleBucketFixed = "bucket_fixed"
)

// duckDBTypeVocab is the allowlist for the optional duckdb_type field
// (staging CREATE TABLE types; Appender-safe per 15-RESEARCH Pitfall 7 —
// invalid types errAppenderCreation at NewAppender time).
var duckDBTypeVocab = []string{"", "VARCHAR", "BIGINT", "DOUBLE", "TIMESTAMP"}

// LoadRules reads, parses, and validates a rule file, returning the
// rules and rulesSHA256 = hex(sha256(raw file bytes)) — the D15-A4
// audit anchor. Rule files are regular controlled files only: a
// symlink is rejected (V12 mirror of the SKILL.md loader guard — an
// atomically swapped symlink must not be silently followed).
//
// Errors carry the file path (loader.go:202-215 form); validation
// errors are NOT silently swallowed.
func LoadRules(path string) (*Rules, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("lstat %s: %w", path, err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmtError("LoadRules", fmt.Errorf(
			"%s is a symlink — rule files are regular controlled files only (V12 symlink-swap guard)", path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("read %s: %w", path, err))
	}
	sum := sha256.Sum256(raw)
	rulesSHA := hex.EncodeToString(sum[:])
	var m Rules
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("unmarshal YAML in %s: %w", path, err))
	}
	if err := ValidateRules(&m); err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("validate %s: %w", path, err))
	}
	return &m, rulesSHA, nil
}

// ValidateRules is the fail-closed load-time validator (mirror of the
// Skill 7-field validator discipline: first failing check short-circuits,
// errors name the field and the rule, vocab allowlists are default-deny).
//
// Checks:
//   - Dataset: non-empty, passes the duckdb.ValidateDatasetID grammar
//     (same ^[A-Za-z0-9_-]{1,64}$ allowlist the ATTACH layer enforces),
//     and contains no "__" — that separator is reserved for the
//     "{dataset}__{period}" composite registry name (D15-C2 / A4).
//   - Tier: one of 权威|历史|个人 via skill.TierEnumByValue; 草稿 (draft)
//     is explicitly excluded — dataset imports never produce draft
//     state. Single source of truth: internal/skill TierEnum.
//   - KMin: >= 0; 0 means "use the global KMinDefault" (D15-D3).
//   - Columns: at least one; per column the rule kind, profile, width,
//     and duckdb_type vocabs above.
//
// The CSV↔rules column-set comparison is deliberately NOT part of this
// function — it happens per import against the actual upload and lists
// the complete diff (ValidateColumnSet below), while file validation
// stays first-error-short-circuit like the Skill validator.
func ValidateRules(m *Rules) error {
	if m == nil {
		return fmtError("ValidateRules", fmt.Errorf("rules is nil — a rule file is required for every dataset (D15-A1)"))
	}
	if strings.TrimSpace(m.Dataset) == "" {
		return fmtError("ValidateRules", fmt.Errorf(
			"dataset is required and must be the logical dataset name (the {dataset}__{period} composite registry name is built from it, D15-C2)"))
	}
	if err := duckdb.ValidateDatasetID(m.Dataset); err != nil {
		return fmtError("ValidateRules", fmt.Errorf(
			"dataset %q fails the dataset-id grammar (same allowlist as the duckdb ATTACH layer, connector.go:104): %w", m.Dataset, err))
	}
	if strings.Contains(m.Dataset, "__") {
		return fmtError("ValidateRules", fmt.Errorf(
			"dataset %q contains \"__\" — \"__\" is the reserved separator of the \"{dataset}__{period}\" composite registry name; use single underscores (D15-C2 / RESEARCH A4)", m.Dataset))
	}
	dir, ok := skill.TierEnumByValue(m.Tier)
	if !ok || dir == "draft" {
		return fmtError("ValidateRules", fmt.Errorf(
			"tier %q is not a valid dataset tier (valid: 权威, 历史, 个人; 草稿/draft is excluded — dataset imports never produce draft state; single source of truth: internal/skill TierEnum)", m.Tier))
	}
	if m.KMin < 0 {
		return fmtError("ValidateRules", fmt.Errorf(
			"k_min must be >= 0 (got %d; 0 = use the global KMinDefault=%d, D15-D3)", m.KMin, KMinDefault))
	}
	if len(m.Columns) == 0 {
		return fmtError("ValidateRules", fmt.Errorf(
			"columns must declare at least one column rule — every CSV column needs an explicit decision (default-deny, D15-A2)"))
	}
	names := make([]string, 0, len(m.Columns))
	for n := range m.Columns {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic error messages across map iteration order
	for _, name := range names {
		cr := m.Columns[name]
		if strings.TrimSpace(name) == "" {
			return fmtError("ValidateRules", fmt.Errorf("column name must be non-empty"))
		}
		switch cr.Rule {
		case RuleHash:
			// Profile is optional: "" selects the default NFC+TrimSpace
			// chain (normalize.go locked spec). A non-empty profile must
			// be a registered name — unknown names are the "rule name
			// hallucination" class and fail closed here.
			if cr.Profile != "" && !containsString(KnownProfiles(), cr.Profile) {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: hash rule profile %q is unknown (known profiles: %v); the profile must appear in internal/data/normalize.go KnownProfiles() — do NOT invent profile names in the rule file (D15-A1 single source of truth)",
					name, cr.Profile, KnownProfiles()))
			}
			if cr.Width != 0 {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: hash rule must not declare width (meaningless combination — fail-closed against silently ignored typos)", name))
			}
		case RuleBucketFixed:
			if cr.Width <= 0 {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: bucket_fixed requires width > 0 (got %v; FixedBucket's width contract is owned here, desensitize.go does not re-validate)", name, cr.Width))
			}
			if cr.Profile != "" {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: bucket_fixed must not declare profile (meaningless combination — fail-closed against silently ignored typos)", name))
			}
		case RulePassthrough:
			if cr.Profile != "" || cr.Width != 0 {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: passthrough must not declare profile or width (a passthrough column is stored verbatim; either field here is a rule-file typo — fail-closed, D15-A2)", name))
			}
		default:
			return fmtError("ValidateRules", fmt.Errorf(
				"column %q: rule %q is not supported (valid rules: hash, passthrough, bucket_fixed — D15-D4: v1 L2 is numeric fixed-width bucketing only; categorical sensitive fields choose hash or passthrough + k_min)",
				name, cr.Rule))
		}
		if !containsString(duckDBTypeVocab, cr.DuckDBType) {
			return fmtError("ValidateRules", fmt.Errorf(
				"column %q: duckdb_type %q is not supported (valid: \"\", VARCHAR, BIGINT, DOUBLE, TIMESTAMP)",
				name, cr.DuckDBType))
		}
	}
	return nil
}

// UnknownColumns returns the CSV columns that have no rule-file entry,
// preserving CSV order. D15-A2: a column that is neither ruled nor
// passthrough-declared is rejected at import time (fail-closed).
func UnknownColumns(rules *Rules, csvColumns []string) []string {
	out := make([]string, 0)
	if rules == nil {
		return append(out, csvColumns...)
	}
	for _, c := range csvColumns {
		if _, ok := rules.Columns[c]; !ok {
			out = append(out, c)
		}
	}
	return out
}

// MissingColumns returns the rule-file columns absent from the CSV
// header, in sorted rule order. The reverse of UnknownColumns — schema
// drift is a two-way rejection (Pitfall 21: a dropped CSV column must
// surface, not silently narrow the import).
func MissingColumns(rules *Rules, csvColumns []string) []string {
	if rules == nil {
		return nil
	}
	have := make(map[string]bool, len(csvColumns))
	for _, c := range csvColumns {
		have[c] = true
	}
	names := make([]string, 0, len(rules.Columns))
	for n := range rules.Columns {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]string, 0)
	for _, n := range names {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

// ValidateColumnSet compares the CSV header against the rule file in
// BOTH directions and, on mismatch, reports the COMPLETE diff in one
// error (unknown + missing, every column name listed — the schema-diff
// report form of Pitfall 21; this is deliberately NOT first-error
// short-circuit, unlike ValidateRules).
func ValidateColumnSet(rules *Rules, csvColumns []string) error {
	unknown := UnknownColumns(rules, csvColumns)
	missing := MissingColumns(rules, csvColumns)
	if len(unknown) == 0 && len(missing) == 0 {
		return nil
	}
	return fmtError("ValidateColumnSet", fmt.Errorf(
		"CSV columns do not match the rule file — unknown (in CSV, not in rules): %v; missing (in rules, not in CSV): %v (fail-closed two-way schema check: the complete diff is listed, and every unknown column is rejected — D15-A2 / Pitfall 21)",
		unknown, missing))
}

// containsString is the package-local membership helper (skill.validate
// keeps an identical unexported copy for the same reason: no shared
// util package is worth the coupling for 5 lines).
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
