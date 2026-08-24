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

// Param type vocabulary (D16-A3): v1 supports string (pattern-constrained)
// and enum (value-set-constrained) only. A type not in this vocabulary is
// fail-closed rejected by ValidateKPIFile — no silent degradation.
// Mirrors internal/data/rules.go:66-70 RuleHash/RulePassthrough/RuleBucketFixed
// vocabulary pattern (closed-set constants with explicit default-deny).
const (
	ParamTypeString = "string" // pattern-constrained free text (e.g. period: '^2[0-9]{3}-(0[1-9]|1[0-2])$')
	ParamTypeEnum   = "enum"   // explicit value allowlist (e.g. status: [正常, 逾期])
)

// KPIFile is the in-memory form of a kpis/{dataset}.yaml file.
// A single YAML file declares all KPIs for one dataset.
//
// The file is a CONTROLLED file: versioned in git, changed via PR review
// (D16-A1 — YAML is the single source of truth), and its content digest
// rides every kpi_calls audit row so "which KPI version produced this result"
// is always provable (KPI-02 signature field).
type KPIFile struct {
	// Dataset is the dataset identifier this file's KPIs reference.
	// Must match a dataset name in the datasets registry (D16-A2).
	// SQL table references use the alias ds_{Dataset}.data (staging table
	// name is always "data" — internal/data/importer.go:1049).
	Dataset string `yaml:"dataset"`

	// Description is a human-readable summary of what this file's KPIs measure.
	Description string `yaml:"description"`

	// KMin is the per-file default for k_min row-level suppression.
	// 0 = defer to the dataset-level k_min_override, then global KMinDefault=5
	// (three-tier chain, mirroring internal/data/desensitize.go:28-39 semantics).
	KMin int `yaml:"k_min"`

	// KPIs is the list of KPI definitions in this file.
	// Must be non-empty; names must be unique within the file.
	KPIs []KPIDef `yaml:"kpis"`
}

// KPIDef is a single KPI definition: a named SQL template with declared
// parameters and metadata. The SQL must be a single SELECT statement
// that aggregates (not row-level detail) and references only the
// declared dataset via the ds_{Dataset}.data alias.
type KPIDef struct {
	// Name is the KPI identifier used in kpi_call(name, params).
	// Must match kpiNamePattern: ^[A-Za-z][A-Za-z0-9_]{0,63}$.
	// Mirrors internal/duckdb/connector.go:109-117 ValidateDatasetID
	// lexical intent but stricter (KPI names forbid hyphens to avoid
	// ambiguity with composite names).
	Name string `yaml:"name"`

	// Description is a human-readable explanation of what this KPI measures.
	// Required — shown in the kpi_call tool catalog (D16-B1).
	Description string `yaml:"description"`

	// SQL is the query template with :param named placeholders.
	// Loader rewrites :param to $param for go-duckdb named binding
	// (Pitfall 3). Must be a single SELECT (Layer 1 text pre-check
	// + Layer 2 AST verification in 16-05).
	SQL string `yaml:"sql"`

	// CountColumn names the SELECT-list alias that holds the aggregate
	// count for k_min row-level suppression (D16-D1). The value must
	// appear as "AS {CountColumn}" in the SQL (Layer 1 text check;
	// Layer 2 AST verification in 16-05).
	CountColumn string `yaml:"count_column"`

	// MaxRows is the per-KPI result set size limit (0 = use package
	// default from executor, 16-06). Prevents unbounded snapshots
	// (Pitfall 7: KPI must aggregate, not return row-level detail).
	MaxRows int `yaml:"max_rows"`

	// Params declares the named parameters referenced by :param
	// placeholders in SQL. Each parameter has a type and value-domain
	// constraint (enum values or string pattern). Placeholders and
	// declarations must correspond 1:1 (D16-A2 bidirectional check).
	Params map[string]ParamSpec `yaml:"params"`
}

// ParamSpec declares the type and value domain of a KPI parameter.
// Exactly one of enum (Values) or pattern (Pattern) applies; specifying
// both is fail-closed rejected (D16-A3 mutual exclusivity).
type ParamSpec struct {
	// Type is the parameter type: ParamTypeString or ParamTypeEnum.
	// Vocabulary default-deny: any other value is rejected at load time.
	Type string `yaml:"type"`

	// Values is the enum value allowlist. Required and non-empty when
	// Type==ParamTypeEnum; must be false when Type==ParamTypeString.
	// Also serves as the differential whitelist (D16-D2): data staff
	// control parameter granularity via this declaration.
	Values []string `yaml:"values"`

	// Pattern is the regex constraint for Type==ParamTypeString parameters.
	// Must compile successfully at load time; failure is fail-closed.
	// Mutually exclusive with Values (enum + pattern = reject).
	Pattern string `yaml:"pattern"`

	// Required controls whether the parameter must be provided at call time.
	// false = optional; if omitted the placeholder condition is typically
	// guarded by SQL like "(:param IS NULL OR col = :param)".
	Required bool `yaml:"required"`
}
