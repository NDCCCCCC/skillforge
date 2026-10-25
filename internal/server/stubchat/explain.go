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

// explain.go — deterministic SQL explanation for the stub LLM (260925).
//
// Operator feedback: a stub answer that just dumps raw SQL (or a lone
// `update etl_payment_due_t` line) is useless — the SQL belongs in a
// fenced code block and the answer must ANALYZE it. This file produces
// a markdown answer (概述 → ```sql fence → 逐句分析) for SQL-shaped
// chunk content.
//
// Grounding contract is intact: GroundingCheck verifies ONLY
// Citation.Span (internal/verdict/grounding.go:170) — Answer is free
// text. The span stays a verbatim substring of the chunk (firstSpan);
// the answer's fenced SQL is also verbatim chunk text, while the
// analysis prose is template-generated from parsed statement shapes —
// no LLM, no randomness, so concurrent Streams stay byte-identical
// (same property renderVerdict already guarantees).
package stubchat

import (
	"fmt"
	"sort"
	"strings"
)

// maxAnswerStmts caps the per-statement analysis list so a pathological
// 100-statement chunk cannot produce an unbounded answer.
const maxAnswerStmts = 12

// firstStatementSpan returns the verbatim FIRST complete SQL statement
// from fenced SQL text: from the first keyword-led line through its
// terminating ';'. Used as the citation span for SQL chunks — the
// generic firstSpan rune cap would cut a long statement mid-clause,
// which reads as "the SQL is truncated". The result is always a
// verbatim substring of sqlText (GroundingCheck-safe); "" when no
// keyword-led line exists.
func firstStatementSpan(sqlText string) string {
	lines := strings.Split(sqlText, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		up := strings.ToUpper(t)
		for _, kw := range []string{"UPDATE", "SELECT", "INSERT", "DELETE", "DROP", "CREATE", "ALTER", "MERGE", "TRUNCATE"} {
			if strings.HasPrefix(up, kw) {
				start = i
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(lines); i++ {
		if strings.Contains(lines[i], ";") {
			return strings.TrimSpace(strings.Join(lines[start:i+1], "\n"))
		}
	}
	// Unterminated tail: still return it verbatim (no fabricated ';').
	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

// sameDocSource reports whether two ECL source paths belong to the same
// base document: extension stripped, then the "__secNN" section suffix
// removed ("取数口径/001_机构部门科室信息更新__sec01_概述.md" and
// "...__sec03_SQL_正文.md" are siblings). Sources without the marker
// compare by their whole stripped path.
func sameDocSource(a, b string) bool {
	base := func(s string) string {
		s = strings.TrimSuffix(s, ".md")
		if i := strings.Index(s, "__sec"); i >= 0 {
			s = s[:i]
		}
		return s
	}
	return base(a) == base(b)
}

// looksLikeSQL reports whether chunk content is SQL-shaped: a ```sql /
// ``` fence, or a leading SQL keyword near the start of a line. Prose
// regulations never match; SQL manuals always do.
func looksLikeSQL(content string) bool {
	if strings.Contains(content, "```sql") || strings.Contains(content, "```SQL") {
		return true
	}
	keywords := []string{
		"UPDATE ", "SELECT ", "INSERT ", "DELETE ", "DROP ",
		"CREATE ", "ALTER ", "MERGE ", "TRUNCATE ",
	}
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		up := strings.ToUpper(t)
		for _, kw := range keywords {
			if strings.HasPrefix(up, kw) {
				return true
			}
		}
	}
	return false
}

// extractFencedSQL returns the verbatim contents of every fenced code
// block in content. Fences never appear in the extracted text (the
// answer re-fences it itself, with a guaranteed balanced pair); when
// the chunk carries no fences at all the whole content is returned —
// the caller only reaches explainSQL when looksLikeSQL matched, so the
// text is SQL regardless.
func extractFencedSQL(content string) string {
	var blocks []string
	lines := strings.Split(content, "\n")
	in := false
	var cur []string
	for _, line := range lines {
		if isFenceLine(line) {
			if in {
				blocks = append(blocks, strings.Join(cur, "\n"))
				cur = nil
			}
			in = !in
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	if in && len(cur) > 0 {
		blocks = append(blocks, strings.Join(cur, "\n")) // unterminated fence — keep what's there
	}
	if len(blocks) == 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

// sqlStmt is one parsed statement (best-effort, deterministic).
type sqlStmt struct {
	Kind   string   // UPDATE / SELECT / DROP / CREATE / …
	Table  string   // first identifier after the kind keyword
	Fields []string // UPDATE SET assignments / SELECT projection
	Where  string   // WHERE clause verbatim (rune-truncated)
}

// parseStatements splits SQL text on `;` and extracts a shallow shape
// per statement. Deliberately NOT a SQL parser: keyword/identifier
// scanning is enough for template prose, and it must never fail —
// worst case a statement is described generically.
func parseStatements(sqlText string) []sqlStmt {
	var out []sqlStmt
	for _, raw := range strings.Split(sqlText, ";") {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		lines := strings.Split(stmt, "\n")
		var kept []string
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if t == "" {
				continue
			}
			kept = append(kept, t)
		}
		if len(kept) == 0 {
			continue
		}
		flat := strings.Join(kept, " ")
		s := sqlStmt{Kind: kindOf(flat)}
		s.Table = tableOf(flat, s.Kind)
		switch s.Kind {
		case "UPDATE":
			s.Fields = setFields(flat)
		case "SELECT":
			s.Fields = selectFields(flat)
		}
		s.Where = whereOf(flat)
		out = append(out, s)
	}
	return out
}

func kindOf(stmt string) string {
	up := strings.ToUpper(stmt)
	for _, kw := range []string{"UPDATE", "SELECT", "INSERT", "DELETE", "DROP", "CREATE", "ALTER", "MERGE", "TRUNCATE"} {
		if strings.HasPrefix(up, kw) {
			return kw
		}
	}
	return "SQL"
}

func tableOf(stmt, kind string) string {
	up := strings.ToUpper(stmt)
	after := map[string]string{
		"UPDATE": "UPDATE ", "INSERT": "INSERT INTO ", "DELETE": "DELETE FROM ",
		"DROP": "DROP TABLE ", "TRUNCATE": "TRUNCATE TABLE ", "ALTER": "ALTER TABLE ",
	}
	if p, ok := after[kind]; ok {
		rest := stmt[bestIndex(up, strings.ToUpper(p)):]
		rest = strings.TrimSpace(rest[len(strings.ToUpper(p)):])
		if rest != "" {
			return firstIdent(rest)
		}
		return ""
	}
	if kind == "SELECT" || kind == "MERGE" {
		if i := strings.Index(up, " FROM "); i >= 0 {
			return firstIdent(stmt[i+6:])
		}
		if kind == "MERGE" {
			if i := strings.Index(up, " INTO "); i >= 0 {
				return firstIdent(stmt[i+6:])
			}
		}
	}
	if kind == "CREATE" {
		// CREATE TABLE AS SELECT / CREATE TABLE <name>
		fields := strings.Fields(stmt)
		for i, f := range fields {
			if strings.EqualFold(f, "TABLE") && i+1 < len(fields) {
				return strings.Trim(firstIdent(fields[i+1]), "\"`")
			}
		}
	}
	return ""
}

// bestIndex returns strings.Index(a, b) or -1 unchanged (helper kept
// explicit so tableOf stays obvious).
func bestIndex(a, b string) int {
	return strings.Index(a, b)
}

func firstIdent(s string) string {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) {
		c := s[end]
		if c == '.' || c == '_' || c == '$' ||
			(c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			end++
			continue
		}
		break
	}
	return s[:end]
}

// setFields extracts the assignment targets of an UPDATE ... SET clause.
func setFields(stmt string) []string {
	up := strings.ToUpper(stmt)
	i := strings.Index(up, " SET ")
	if i < 0 {
		return nil
	}
	rest := stmt[i+5:]
	if j := strings.Index(strings.ToUpper(rest), " WHERE "); j >= 0 {
		rest = rest[:j]
	}
	var out []string
	for _, part := range strings.Split(rest, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) != "" {
			out = append(out, strings.TrimSpace(kv[0]))
		}
	}
	return out
}

// selectFields extracts the projection of a SELECT (dedup'd, capped).
func selectFields(stmt string) []string {
	up := strings.ToUpper(stmt)
	i := strings.Index(up, "SELECT ")
	if i != 0 {
		return nil
	}
	rest := stmt[7:]
	if j := strings.Index(up, " FROM "); j >= 0 {
		rest = stmt[7:j]
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(rest, ",") {
		f := strings.TrimSpace(part)
		if f == "" || f == "*" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// whereOf returns the WHERE clause verbatim (rune-truncated to 120).
func whereOf(stmt string) string {
	up := strings.ToUpper(stmt)
	i := strings.Index(up, " WHERE ")
	if i < 0 {
		return ""
	}
	w := strings.TrimSpace(stmt[i+7:])
	runes := []rune(w)
	if len(runes) > 120 {
		w = string(runes[:120]) + "…"
	}
	return w
}

// explainStmt renders one template line for a statement.
func explainStmt(s sqlStmt, idx int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d. **%s**", idx, s.Kind)
	if s.Table != "" {
		fmt.Fprintf(&b, " 目标表 `%s`", s.Table)
	}
	switch s.Kind {
	case "UPDATE":
		if len(s.Fields) > 0 {
			fmt.Fprintf(&b, " — 更新字段:%s", backjoin(s.Fields))
		}
	case "SELECT":
		if len(s.Fields) > 0 {
			fmt.Fprintf(&b, " — 查询字段:%s", backjoin(s.Fields))
		}
	case "INSERT":
		b.WriteString(" — 向表中写入数据")
	case "DELETE":
		b.WriteString(" — 按条件删除表中数据")
	case "DROP":
		b.WriteString(" — 删除该表(通常为临时表清理)")
	case "CREATE":
		b.WriteString(" — 创建表/对象")
	default:
		b.WriteString(" — 数据库操作语句")
	}
	if s.Where != "" {
		fmt.Fprintf(&b, ";限定条件:`%s`", s.Where)
	}
	b.WriteString("\n")
	return b.String()
}

func backjoin(fields []string) string {
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, "`"+f+"`")
	}
	return strings.Join(quoted, "、")
}

// explainSQL builds the full markdown answer: 概述 → fenced verbatim
// SQL → 逐句分析. The fence is guaranteed balanced (we open and close
// it ourselves around fence-free text).
func explainSQL(content string) string {
	sqlText := extractFencedSQL(content)
	stmts := parseStatements(sqlText)
	if len(stmts) == 0 {
		return strings.TrimSpace(content)
	}
	if len(stmts) > maxAnswerStmts {
		stmts = stmts[:maxAnswerStmts]
	}

	var b strings.Builder
	kinds := map[string]int{}
	for _, s := range stmts {
		kinds[s.Kind]++
	}
	// Deterministic kind summary order.
	order := make([]string, 0, len(kinds))
	for k := range kinds {
		order = append(order, k)
	}
	sort.Strings(order)
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, fmt.Sprintf("%s ×%d", k, kinds[k]))
	}
	fmt.Fprintf(&b, "**概述**:该 SQL 片段共 %d 条语句(%s),以下为原文与逐条分析。\n\n", len(stmts), strings.Join(parts, "、"))

	b.WriteString("```sql\n")
	b.WriteString(sqlText)
	b.WriteString("\n```\n\n")

	b.WriteString("**逐句分析**\n\n")
	for i, s := range stmts {
		b.WriteString(explainStmt(s, i+1))
	}
	return strings.TrimSpace(b.String())
}

// buildStubAnswer picks the answer body for the top chunk: SQL-shaped
// content gets the structured explanation; everything else keeps the
// pre-existing behavior (the span itself as the answer).
func buildStubAnswer(content, span string) string {
	if looksLikeSQL(content) {
		if ans := explainSQL(content); ans != "" {
			return ans
		}
	}
	return span
}
