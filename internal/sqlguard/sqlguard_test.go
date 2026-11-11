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

package sqlguard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templateSQL is a real recipe from the corpus (030 承保清单 / 学平险),
// trimmed but structurally intact: projection with Chinese aliases, two
// correlated subqueries with DECODE, comma-less FROM/WHERE, and the four
// adjustable predicates the user actually changes.
const templateSQL = `select a.policy_no 保单号,a.policy_application_no 投保单号,a.department_group_name 部门组,
       a.agent_code 代理点,a.product_name 险种名称,a.premium_amount 含税保费,
       (select max(b.factor_value) 
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code='M9') 学校名称,
       (select decode(max(d.factor_value),'O2000','大中专学校学生','O2001','中学生',max(d.factor_value)) 
           from "ids_wuh".nonauto_plc_element_t d
         where d.policy_no=a.policy_no
           and d.factor_code in ('O2')) 学生类别
from wuh_dw.etl_nonauto_policy_t a
where 
a.agent_code in ('431','100AG2')
and a.product_name in ('监护人责任保险','教职工校方责任保险','学生幼儿人身意外伤害保险（2023版）')
and a.department_group_name = '黄冈中心支公司'
and a.inception_date between DATE('2025-01-01') and date('2025-12-31')`

func TestExtractTables(t *testing.T) {
	sh := Extract(templateSQL)
	require.True(t, sh.Ok, "template must parse: %s", sh.Note)
	assert.Equal(t, []string{
		"ids_wuh.nonauto_plc_element_t",
		"wuh_dw.etl_nonauto_policy_t",
	}, sh.Tables, "schema-qualified names, de-duplicated and sorted")
}

func TestExtractPredicates(t *testing.T) {
	sh := Extract(templateSQL)
	require.True(t, sh.Ok)
	require.Len(t, sh.Predicates, 4, "four top-level filters; the AND inside BETWEEN must not split")

	assert.Equal(t, Predicate{Column: "a.agent_code", Op: "in", ValueKind: "list"}, stripValue(sh.Predicates[0]))
	assert.Equal(t, Predicate{Column: "a.product_name", Op: "in", ValueKind: "list"}, stripValue(sh.Predicates[1]))
	assert.Equal(t, Predicate{Column: "a.department_group_name", Op: "=", ValueKind: "string"}, stripValue(sh.Predicates[2]))
	assert.Equal(t, Predicate{Column: "a.inception_date", Op: "between", ValueKind: "date"}, stripValue(sh.Predicates[3]))
}

func stripValue(p Predicate) Predicate {
	p.Value = ""
	return p
}

func TestFingerprint(t *testing.T) {
	sh := Extract(templateSQL)
	require.True(t, sh.Ok)
	assert.Equal(t, 3, sh.Fingerprint.Select, "outer select + two correlated subqueries")
	assert.Equal(t, 3, sh.Fingerprint.From)
	assert.Equal(t, 0, sh.Fingerprint.Join)
	assert.Equal(t, 0, sh.DMLCount)
}

// The user's exact complaint: the model changed a field. That is R3.
func TestVerifyRejectsChangedColumn(t *testing.T) {
	gen := replace(templateSQL, "a.department_group_name = '黄冈中心支公司'", "a.department_name = '黄冈中心支公司'")
	rep := Verify(gen, Extract(templateSQL))
	require.True(t, rep.Checked)
	assert.False(t, rep.OK)
	require.Len(t, rep.Failures(), 1)
	assert.Equal(t, "R3", rep.Failures()[0].Rule)
}

// The authorized change: only the organization and date literals move.
func TestVerifyAllowsSlotValueChange(t *testing.T) {
	gen := replace(templateSQL, "'黄冈中心支公司'", "'襄阳中心支公司'")
	gen = replace(gen, "DATE('2025-01-01')", "DATE('2024-01-01')")
	gen = replace(gen, "date('2025-12-31')", "date('2024-12-31')")
	rep := Verify(gen, Extract(templateSQL))
	require.True(t, rep.Checked)
	assert.True(t, rep.OK, "slot changes must pass: %s", rep.Summary())
	assert.Empty(t, rep.Violations)
}

// A literal change on a non-slot column is a warning, not a block.
func TestVerifyWarnsOnNonSlotLiteral(t *testing.T) {
	gen := replace(templateSQL, "b.factor_code='M9'", "b.factor_code='M8'")
	// That predicate is inside a subquery (not top-level), so R3/R4 do not
	// see it — but the projection change alone must still pass. This test
	// pins that subquery filters are intentionally out of scope.
	rep := Verify(gen, Extract(templateSQL))
	assert.True(t, rep.OK, "%s", rep.Summary())
}

func TestVerifyRejectsExtraTable(t *testing.T) {
	gen := replace(templateSQL,
		"from wuh_dw.etl_nonauto_policy_t a",
		"from wuh_dw.etl_nonauto_policy_t a\nleft join wuh_dw.secret_salary_t s on s.policy_no=a.policy_no")
	rep := Verify(gen, Extract(templateSQL))
	assert.False(t, rep.OK)
	rules := ruleSet(rep.Failures())
	assert.Contains(t, rules, "R1")
	assert.Contains(t, rules, "R5", "adding a join also changes the fingerprint")
}

func TestVerifyRejectsDML(t *testing.T) {
	gen := templateSQL + "\n; delete from wuh_dw.etl_nonauto_policy_t"
	rep := Verify(gen, Extract(templateSQL))
	assert.False(t, rep.OK)
	assert.Contains(t, ruleSet(rep.Failures()), "R2")
}

// Dropping a whole subquery leaves the predicates untouched but must still
// be caught — that is what the fingerprint is for.
func TestVerifyRejectsDroppedSubquery(t *testing.T) {
	gen := replace(templateSQL,
		`(select max(b.factor_value) 
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code='M9') 学校名称,`,
		`'' 学校名称,`)
	rep := Verify(gen, Extract(templateSQL))
	assert.False(t, rep.OK, "dropping a correlated subquery must not pass")
}

func TestVerifyUnparseableTemplateSkips(t *testing.T) {
	rep := Verify("select 1 from t", Shape{Ok: false, Note: "no statement keyword found"})
	assert.False(t, rep.Checked)
	assert.False(t, rep.OK)
	assert.Contains(t, rep.Skipped, "unparseable")
}

func TestVerifyUnparseableGeneratedFails(t *testing.T) {
	rep := Verify("this is not sql at all", Extract(templateSQL))
	require.True(t, rep.Checked)
	assert.False(t, rep.OK)
	assert.Equal(t, "R0", rep.Failures()[0].Rule)
}

func TestExtractTemplatesFencedAndBare(t *testing.T) {
	note := "## 说明\n\n```sql\nselect 1 from dual\n```\n\n正文里也有：\nselect a from b\n"
	got := ExtractTemplates(note)
	require.NotEmpty(t, got)
	assert.Contains(t, got[0], "select 1 from dual")

	bare := "说明文字\nupdate t set x = 1 where id = 2\n"
	got = ExtractTemplates(bare)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "update t")
}

func TestExtractTemplatesUnterminatedFence(t *testing.T) {
	// A 4KB chunk boundary can cut the closing fence away.
	note := "```sql\nselect a from b where c = 1\n"
	got := ExtractTemplates(note)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "select a from b")
}

func TestExtractTemplatesExpandsLiteralCarriageReturns(t *testing.T) {
	note := "```sql\nselect a \\rfrom b \\rwhere c = 1\n```"
	got := ExtractTemplates(note)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "\nfrom b")
}

func TestBestTemplatePicksMatchingTables(t *testing.T) {
	note := "```sql\nselect x from unrelated_t where y = 1\n```\n" +
		"```sql\nselect a.policy_no from wuh_dw.etl_nonauto_policy_t a where a.agent_code = '1'\n```"
	gen := "select a.policy_no from wuh_dw.etl_nonauto_policy_t a where a.agent_code = '2'"
	sql, sh, ok := BestTemplate(note, gen)
	require.True(t, ok)
	assert.Contains(t, sql, "etl_nonauto_policy_t")
	assert.True(t, sh.Ok)
}

func TestBestTemplateNoOverlap(t *testing.T) {
	note := "```sql\nselect x from unrelated_t\n```"
	_, _, ok := BestTemplate(note, "select a from other_t")
	assert.False(t, ok, "an unrelated template must not be used as a reference")
}

// 2026-11-11 live regression: 学平险 and 承保清单 share the fact table
// `wuh_dw.etl_nonauto_policy_t` but join different people tables. Picking
// 承保清单 because it had the highest (only) overlap made the guard reject a
// correct answer with R1+R3+R5. Low overlap must disqualify a candidate.
func TestBestTemplateRejectsLowOverlap(t *testing.T) {
	note := "```sql\n" +
		"select a.policy_no, c.wardname from wuh_dw.etl_nonauto_policy_t a\n" +
		"left join ids_wuh.nonauto_people_t c on a.policy_no = c.policy_no\n" +
		"where a.department_group_name = '襄阳中心支公司' and a.inception_year = '2025'\n" +
		"```"
	gen := "select a.policy_no, (select max(b.factor_value) from \"ids_wuh\".nonauto_plc_element_t b where b.policy_no=a.policy_no) 学校名称\n" +
		"from wuh_dw.etl_nonauto_policy_t a\n" +
		"where a.department_group_name = '襄阳中心支公司'"

	_, _, ok := BestTemplate(note, gen)
	assert.False(t, ok,
		"sharing only the fact table is not evidence of derivation; overlap %.2f is below the threshold", MinTableOverlap)
}

func TestBestTemplateAcceptsNearIdentical(t *testing.T) {
	note := "```sql\n" + templateSQL + "\n```"
	gen := replace(templateSQL, "'黄冈中心支公司'", "'襄阳中心支公司'")
	sql, sh, ok := BestTemplate(note, gen)
	require.True(t, ok)
	assert.True(t, sh.Ok)
	assert.Contains(t, sql, "etl_nonauto_policy_t")
}

// --- helpers ---

func replace(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

func ruleSet(vs []Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Rule)
	}
	return out
}
