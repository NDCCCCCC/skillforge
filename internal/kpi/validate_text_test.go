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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validKPIFile returns a KPIFile that passes all ValidateKPIFile checks.
func validKPIFile() *KPIFile {
	return &KPIFile{
		Dataset:    "loans",
		KMin:       0,
		KPIs: []KPIDef{
			{
				Name:        "loan_overdue_rate_by_status",
				Description: "贷款各状态逾期率",
				SQL:         "SELECT status AS status, count(*) AS row_count FROM ds_loans.data WHERE (:status IS NULL OR status = :status) GROUP BY status",
				CountColumn: "row_count",
				Params: map[string]ParamSpec{
					"status": {
						Type:     ParamTypeEnum,
						Values:   []string{"正常", "逾期"},
						Required: false,
					},
				},
			},
		},
	}
}

func TestValidateKPIFile_Valid(t *testing.T) {
	err := ValidateKPIFile(validKPIFile())
	assert.NoError(t, err, "valid KPI file should pass")
}

func TestValidateKPIFile_NilFile(t *testing.T) {
	err := ValidateKPIFile(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kpi file is nil")
}

func TestValidateKPIFile_DatasetEmpty(t *testing.T) {
	m := validKPIFile()
	m.Dataset = ""
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dataset is required")
}

func TestValidateKPIFile_DatasetBadGrammar(t *testing.T) {
	m := validKPIFile()
	m.Dataset = "my-dataset" // hyphens not allowed
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fails the kpi-name grammar")
}

func TestValidateKPIFile_DatasetDoubleUnderscore(t *testing.T) {
	m := validKPIFile()
	m.Dataset = "loans__period" // "__" is reserved
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "\"__\"")
	assert.Contains(t, err.Error(), "reserved separator")
}

func TestValidateKPIFile_KPIsEmpty(t *testing.T) {
	m := validKPIFile()
	m.KPIs = nil
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one KPI")
}

func TestValidateKPIFile_KPINameEmpty(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Name = ""
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestValidateKPIFile_KPINameBadGrammar(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Name = "bad-name" // hyphens not allowed
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fails the kpi-name grammar")
}

func TestValidateKPIFile_KPINameDuplicate(t *testing.T) {
	m := validKPIFile()
	m.KPIs = append(m.KPIs, KPIDef{
		Name:        "loan_overdue_rate_by_status", // same as first
		Description: "duplicate",
		SQL:         "SELECT status AS status, count(*) AS row_count FROM ds_loans.data GROUP BY status",
		CountColumn: "row_count",
	})
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates")
	assert.Contains(t, err.Error(), "loan_overdue_rate_by_status")
}

func TestValidateKPIFile_DescriptionEmpty(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Description = ""
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description is required")
}

func TestValidateKPIFile_SQLEmpty(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].SQL = ""
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sql is required")
}

func TestValidateKPIFile_AliasCrossDataset(t *testing.T) {
	// SQL references ds_cards but dataset is loans
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT count(*) AS row_count FROM ds_cards.data"
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ds_cards")
	assert.Contains(t, err.Error(), "not matching declared dataset")
}

func TestValidateKPIFile_AliasBareComposite(t *testing.T) {
	// SQL references ds_loans__x (composite alias fragment)
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT count(*) AS row_count FROM ds_loans__x.data"
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ds_loans__x")
}

func TestValidateKPIFile_MissingTableReference(t *testing.T) {
	// SQL has ds_loans but no .data table reference
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT count(*) AS row_count FROM ds_loans.other_table GROUP BY status"
	m.KPIs[0].CountColumn = "row_count"
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ds_loans.data")
}

func TestValidateKPIFile_CountColumnEmpty(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].CountColumn = ""
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count_column is required")
}

func TestValidateKPIFile_CountColumnNotInAlias(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].CountColumn = "nonexistent_col"
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear as")
	assert.Contains(t, err.Error(), "nonexistent_col")
}

func TestValidateKPIFile_PlaceholderInSQLNotInParams(t *testing.T) {
	// SQL has :region but params only declares :status
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT region AS region, count(*) AS row_count FROM ds_loans.data WHERE (:status IS NULL OR status = :status) AND (:region IS NULL OR region = :region) GROUP BY region"
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL has params missing")
	assert.Contains(t, err.Error(), "region")
}

func TestValidateKPIFile_ParamsDeclaredNotInSQL(t *testing.T) {
	// params declares :extra but SQL only has :status
	m := validKPIFile()
	m.KPIs[0].Params["extra"] = ParamSpec{
		Type:     ParamTypeEnum,
		Values:   []string{"a", "b"},
		Required: false,
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "params has SQL missing")
	assert.Contains(t, err.Error(), "extra")
}

func TestValidateKPIFile_PlaceholderBidirectionalDiff(t *testing.T) {
	// Both directions: SQL has :region not in params, params has :extra not in SQL
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT region AS region, count(*) AS row_count FROM ds_loans.data WHERE (:region IS NULL OR region = :region) GROUP BY region"
	m.KPIs[0].Params = map[string]ParamSpec{
		"extra": {Type: ParamTypeEnum, Values: []string{"a", "b"}, Required: false},
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	errMsg := err.Error()
	// Both sides must be listed (full diff, not first-error)
	assert.Contains(t, errMsg, "region", "SQL-side missing param must be listed")
	assert.Contains(t, errMsg, "extra", "params-side missing placeholder must be listed")
}

func TestValidateKPIFile_ParamTypeEnumEmptyValues(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:   ParamTypeEnum,
		Values: []string{}, // empty
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enum type requires non-empty values")
}

func TestValidateKPIFile_ParamTypeEnumDuplicateValues(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:   ParamTypeEnum,
		Values: []string{"正常", "正常"}, // duplicate
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates")
}

func TestValidateKPIFile_ParamTypeStringWithValues(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:   ParamTypeString,
		Values: []string{"should_not_be_here"},
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "string type must not declare values")
}

func TestValidateKPIFile_ParamTypeStringBadPattern(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:    ParamTypeString,
		Pattern: "[invalid regex", // unbalanced bracket
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not compile")
	assert.Contains(t, err.Error(), "[invalid regex")
}

func TestValidateKPIFile_ParamTypeEnumWithPattern(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:    ParamTypeEnum,
		Values:  []string{"正常", "逾期"},
		Pattern: "^.*$", // mutually exclusive
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enum type must not declare pattern")
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidateKPIFile_ParamTypeUnknown(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type: "boolean", // not in vocabulary
	}
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
	assert.Contains(t, err.Error(), "boolean")
}

func TestValidateKPIFile_NoGroupByNoAggFunc(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT status AS status, balance AS balance FROM ds_loans.data WHERE (:status IS NULL OR status = :status)"
	m.KPIs[0].CountColumn = "status" // must appear as alias
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GROUP BY")
	assert.Contains(t, err.Error(), "aggregate function")
}

func TestValidateKPIFile_GroupByWithoutAggFunc(t *testing.T) {
	// GROUP BY present — passes aggregation heuristic
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT status AS status, count(*) AS row_count FROM ds_loans.data WHERE (:status IS NULL OR status = :status) GROUP BY status"
	err := ValidateKPIFile(m)
	assert.NoError(t, err)
}

func TestValidateKPIFile_AggFuncWithoutGroupBy(t *testing.T) {
	// count() present without GROUP BY — still passes heuristic
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT count(*) AS row_count FROM ds_loans.data WHERE (:status IS NULL OR status = :status)"
	err := ValidateKPIFile(m)
	assert.NoError(t, err)
}

func TestValidateKPIFile_KMinNegative(t *testing.T) {
	m := validKPIFile()
	m.KMin = -1
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k_min must be >= 0")
}

func TestValidateKPIFile_MaxRowsNegative(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].MaxRows = -1
	err := ValidateKPIFile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_rows must be >= 0")
}

func TestValidateKPIFile_StringParamWithValidPattern(t *testing.T) {
	m := validKPIFile()
	m.KPIs[0].Params["status"] = ParamSpec{
		Type:     ParamTypeString,
		Pattern:  `^[A-Za-z]+$`,
		Required: true,
	}
	err := ValidateKPIFile(m)
	assert.NoError(t, err)
}

func TestValidateKPIFile_EmptyParamsAllowed(t *testing.T) {
	// KPI with no parameters is valid (empty params, no :param in SQL)
	m := validKPIFile()
	m.KPIs[0].SQL = "SELECT balance AS balance_bucket, count(*) AS row_count FROM ds_loans.data GROUP BY balance ORDER BY balance_bucket"
	m.KPIs[0].Params = map[string]ParamSpec{}
	err := ValidateKPIFile(m)
	assert.NoError(t, err)
}

func TestValidateKPIFile_MultipleKPIsValid(t *testing.T) {
	m := validKPIFile()
	m.KPIs = append(m.KPIs, KPIDef{
		Name:        "loan_balance_distribution",
		Description: "贷款余额分布",
		SQL:         "SELECT balance AS balance_bucket, count(*) AS row_count FROM ds_loans.data GROUP BY balance",
		CountColumn: "row_count",
		Params:      map[string]ParamSpec{},
	})
	err := ValidateKPIFile(m)
	assert.NoError(t, err)
}

// TestParamSpecCompile verifies that ParamSpec type vocabulary constants are stable.
func TestParamSpecCompile(t *testing.T) {
	assert.Equal(t, "string", ParamTypeString)
	assert.Equal(t, "enum", ParamTypeEnum)
}

// TestKPINamePattern validates the kpiNamePattern grammar.
func TestKPINamePattern(t *testing.T) {
	valid := []string{"a", "abc", "a_b", "a1", "Abc_123", "loan_overdue_rate"}
	for _, name := range valid {
		assert.True(t, kpiNamePattern.MatchString(name), "should match: %s", name)
	}
	invalid := []string{"", "1abc", "-dash", "a-b", "a b", strings.Repeat("a", 65)}
	for _, name := range invalid {
		assert.False(t, kpiNamePattern.MatchString(name), "should not match: %s", name)
	}
}
