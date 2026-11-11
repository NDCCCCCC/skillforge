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
	"fmt"
	"strings"
)

// Severity tells the caller whether a violation must block the answer.
type Severity string

const (
	// SeverityFail blocks: the generated SQL departs from the template in a
	// way that can change what data the query touches.
	SeverityFail Severity = "fail"
	// SeverityWarn is recorded but does not block. Used where the corpus
	// itself is ambiguous and blocking would produce false positives.
	SeverityWarn Severity = "warn"
)

// Violation is one rule failure, worded so it can be shown to an operator.
type Violation struct {
	Rule     string
	Severity Severity
	Detail   string
}

func (v Violation) String() string {
	return fmt.Sprintf("[%s/%s] %s", v.Rule, v.Severity, v.Detail)
}

// Report is the outcome of comparing a generated SQL against its template.
type Report struct {
	// OK is true when no SeverityFail violation was found.
	OK bool
	// Checked is false when the guard could not run at all (template
	// unparseable, or the generated text is not SQL). Callers must treat
	// Checked=false as "not verified", never as "verified clean".
	Checked bool
	// Skipped explains Checked=false.
	Skipped string
	// Violations is ordered by rule.
	Violations []Violation
}

// Failures returns only the blocking violations.
func (r Report) Failures() []Violation {
	out := make([]Violation, 0, len(r.Violations))
	for _, v := range r.Violations {
		if v.Severity == SeverityFail {
			out = append(out, v)
		}
	}
	return out
}

// Summary renders the report as one operator-facing line.
func (r Report) Summary() string {
	if !r.Checked {
		return "sqlguard: not verified — " + r.Skipped
	}
	if len(r.Violations) == 0 {
		return "sqlguard: verified, template preserved"
	}
	parts := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		parts = append(parts, v.String())
	}
	return "sqlguard: " + strings.Join(parts, "; ")
}

// TableRelation describes how a generated table set relates to a template's.
//
// The distinction matters because a template lifted out of a 4KB chunk can
// be genuinely incomplete (the corpus analysis found fences cut mid-string
// and statements whose tail landed in the next chunk). "Generated SQL is
// missing a table the template names" is therefore ambiguous — truncated
// template, or a model that dropped a join. "Generated SQL adds a table"
// is not ambiguous: no truncation can invent a table.
type TableRelation int

const (
	// TablesUnrelated: the sets share nothing, or differ on both sides.
	TablesUnrelated TableRelation = iota
	// TablesEqual: same set — the only relation safe to verify against.
	TablesEqual
	// GeneratedAdds: the generated set is a strict superset of the
	// template's. The model reached for data the recipe never authorized.
	GeneratedAdds
	// GeneratedDrops: the generated set is a strict subset of the
	// template's. Usually a truncated template; cannot be judged.
	GeneratedDrops
)

func (r TableRelation) String() string {
	switch r {
	case TablesEqual:
		return "equal"
	case GeneratedAdds:
		return "generated adds tables"
	case GeneratedDrops:
		return "generated drops tables"
	default:
		return "unrelated"
	}
}

// RelateTables compares two table sets.
func RelateTables(gen, tmpl []string) TableRelation {
	genSet := make(map[string]bool, len(gen))
	for _, t := range gen {
		genSet[t] = true
	}
	tmplSet := make(map[string]bool, len(tmpl))
	for _, t := range tmpl {
		tmplSet[t] = true
	}
	if len(genSet) == 0 || len(tmplSet) == 0 {
		return TablesUnrelated
	}

	adds, drops := false, false
	for t := range genSet {
		if !tmplSet[t] {
			adds = true
		}
	}
	for t := range tmplSet {
		if !genSet[t] {
			drops = true
		}
	}
	switch {
	case !adds && !drops:
		return TablesEqual
	case adds && !drops:
		return GeneratedAdds
	case !adds && drops:
		return GeneratedDrops
	default:
		return TablesUnrelated
	}
}

// AdjustableColumns is the set of columns whose VALUES the user is allowed
// to change. Everything else in a predicate must be preserved verbatim.
//
// Source: the sql-guides corpus analysis (2026-11-11) which classified
// every recurring literal in the reviewed recipes. The three families are
// organization (机构/部门/人员), product (险种), and time (日期). Adding a
// column here is a deliberate widening of the authorization surface — it
// should follow a corpus review, not a guess.
var AdjustableColumns = map[string]bool{
	// 机构 / 部门 / 人员
	"department_group_name": true,
	"department_group_code": true,
	"department_name":       true,
	"section_office_name":   true,
	"agent_code":            true,
	"agent_name":            true,
	"operator_code":         true,
	"operator_name":         true,
	"kq":                    true,
	// 险种 / 产品
	"product_name": true,
	"product_code": true,
	// 时间
	"inception_date":   true,
	"application_date": true,
	"issue_date":       true,
	"planned_end_date": true,
	// 单笔单据标识（用户指定保单号时常见）
	"policy_no": true,
	"policy_application_no": true,
}

// adjustableColumn reports whether the predicate's column (with its alias
// prefix stripped) is a legitimate slot.
func adjustableColumn(col string) bool {
	if i := strings.LastIndex(col, "."); i >= 0 {
		col = col[i+1:]
	}
	return AdjustableColumns[strings.ToLower(col)]
}

// Verify compares a generated SQL text against the template it was derived
// from and reports every departure.
//
// The five rules are ordered by how much damage a miss would do:
//
//	R1 table closure   — a different table is a different dataset
//	R2 DML ban         — a SELECT template must never start writing
//	R3 predicate skeleton — the user's complaint "模型改错字段"
//	R5 structure fingerprint — catches equal-but-different rewrites
//	R4 slot placement  — informational; the corpus has legit oddities
//
// R1/R2/R3/R5 are blocking. R4 is a warning because the corpus cannot
// distinguish "the operator meant this literal" from "the model invented
// it" at token level.
func Verify(generated string, tmpl Shape) Report {
	rep := Report{}

	if !tmpl.Ok {
		rep.Skipped = "template unparseable (" + tmpl.Note + ")"
		return rep
	}
	gen := Extract(generated)
	if !gen.Ok {
		// The generated text failed to shape. That is itself suspicious:
		// we cannot prove it derives from the template, so refuse.
		rep.Checked = true
		rep.Violations = append(rep.Violations, Violation{
			Rule:     "R0",
			Severity: SeverityFail,
			Detail:   "生成的 SQL 无法解析（" + gen.Note + "），无法证明它沿用了模板",
		})
		return rep
	}
	rep.Checked = true

	// R1 — table closure.
	rep.Violations = append(rep.Violations, checkTables(gen.Tables, tmpl.Tables)...)

	// R2 — DML ban.
	if gen.DMLCount > tmpl.DMLCount {
		rep.Violations = append(rep.Violations, Violation{
			Rule:     "R2",
			Severity: SeverityFail,
			Detail: fmt.Sprintf("出现了模板里没有的写操作关键字（生成 %d 处，模板 %d 处）",
				gen.DMLCount, tmpl.DMLCount),
		})
	}

	// R3 — predicate skeleton + R4 slot placement.
	rep.Violations = append(rep.Violations, checkPredicates(gen.Predicates, tmpl.Predicates)...)

	// R5 — structure fingerprint.
	if !gen.Fingerprint.Equal(tmpl.Fingerprint) {
		rep.Violations = append(rep.Violations, Violation{
			Rule:     "R5",
			Severity: SeverityFail,
			Detail: fmt.Sprintf("语句结构变了（select %d→%d, from %d→%d, join %d→%d, group by %d→%d, order by %d→%d, 最大括号深度 %d→%d）",
				tmpl.Fingerprint.Select, gen.Fingerprint.Select,
				tmpl.Fingerprint.From, gen.Fingerprint.From,
				tmpl.Fingerprint.Join, gen.Fingerprint.Join,
				tmpl.Fingerprint.GroupBy, gen.Fingerprint.GroupBy,
				tmpl.Fingerprint.OrderBy, gen.Fingerprint.OrderBy,
				tmpl.Fingerprint.MaxDepth, gen.Fingerprint.MaxDepth),
		})
	}

	rep.OK = len(rep.Failures()) == 0
	return rep
}

// checkTables implements R1: the generated table set must be exactly the
// template's. A subset would silently drop a join; a superset reaches for
// data the recipe never authorized.
func checkTables(gen, tmpl []string) []Violation {
	tmplSet := map[string]bool{}
	for _, t := range tmpl {
		tmplSet[t] = true
	}
	genSet := map[string]bool{}
	for _, t := range gen {
		genSet[t] = true
	}

	var extra, missing []string
	for _, t := range gen {
		if !tmplSet[t] {
			extra = append(extra, t)
		}
	}
	for _, t := range tmpl {
		if !genSet[t] {
			missing = append(missing, t)
		}
	}
	if len(extra) == 0 && len(missing) == 0 {
		return nil
	}

	var parts []string
	if len(extra) > 0 {
		parts = append(parts, "多出模板里没有的表: "+strings.Join(extra, ", "))
	}
	if len(missing) > 0 {
		parts = append(parts, "丢失了模板里的表: "+strings.Join(missing, ", "))
	}
	return []Violation{{
		Rule:     "R1",
		Severity: SeverityFail,
		Detail:   strings.Join(parts, "；"),
	}}
}

// checkPredicates implements R3 (skeleton must match) and R4 (literal
// changes must land on an adjustable column).
func checkPredicates(gen, tmpl []Predicate) []Violation {
	if len(tmpl) == 0 {
		// A template with no top-level predicates cannot be skeleton-checked.
		// Not a violation: UPDATE-style recipes have none.
		return nil
	}
	if len(gen) != len(tmpl) {
		return []Violation{{
			Rule:     "R3",
			Severity: SeverityFail,
			Detail:   fmt.Sprintf("过滤条件数量变了（模板 %d 条，生成 %d 条）", len(tmpl), len(gen)),
		}}
	}

	var out []Violation
	for i := range tmpl {
		g, t := gen[i], tmpl[i]
		if g.Column != t.Column || g.Op != t.Op || g.ValueKind != t.ValueKind {
			out = append(out, Violation{
				Rule:     "R3",
				Severity: SeverityFail,
				Detail: fmt.Sprintf("第 %d 个过滤条件被改动：模板 (%s %s %s) → 生成 (%s %s %s)",
					i+1, t.Column, t.Op, t.ValueKind, g.Column, g.Op, g.ValueKind),
			})
			continue
		}
		// R4: same column/operator/type, different literal.
		if g.Value != t.Value && !adjustableColumn(t.Column) {
			out = append(out, Violation{
				Rule:     "R4",
				Severity: SeverityWarn,
				Detail: fmt.Sprintf("第 %d 个条件 %s 的字面量从 %s 改成了 %s，该字段不在可调槽位白名单里",
					i+1, t.Column, t.Value, g.Value),
			})
		}
	}
	return out
}
