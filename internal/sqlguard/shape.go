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
	"sort"
	"strings"
)

// Shape is everything the guard can prove about one SQL text. It is
// deliberately coarse: tokens in, facts out, no AST.
type Shape struct {
	// Tables are normalized `schema.table[@dblink]` references, sorted and
	// de-duplicated. Schema and dblink are part of the identity: changing
	// `"ids_wuh".x` to `ids_other.x` must be a violation, not a rename.
	Tables []string
	// Predicates are the top-level WHERE atoms, in source order. Order is
	// part of the comparison: reordering the field list is a template
	// change the prompt forbids, so it should surface.
	Predicates []Predicate
	// Fingerprint pins the statement skeleton.
	Fingerprint Fingerprint
	// DMLCount counts data-changing keywords anywhere in the text. A
	// template that is a pure SELECT must stay at zero.
	DMLCount int
	// HasStar records a `*` projection. Field-set comparison degrades to a
	// suggestion when it is set, because `a.*` legitimately hides the list.
	HasStar bool
	// Ok is false when the text could not be shaped at all (no statement
	// keyword, or a structural failure such as unbalanced parentheses).
	Ok bool
	// Note explains !Ok, and is also used for "parsed with caveats".
	Note string
}

// Predicate is one atom of the top-level WHERE clause, reduced to the parts
// a parameter change must never touch.
type Predicate struct {
	// Column is the left-hand side, lower-cased, alias preserved
	// (`a.policy_no`). Quoting is stripped: `"POLICY_NO"` and `policy_no`
	// are the same column in DM8's default (case-insensitive) mode.
	Column string
	// Op is normalized to one of: = <> < > <= >= in between like is.
	Op string
	// ValueKind classifies the right-hand side: string, number, date,
	// list, null, expr. It catches "value replaced by a different type",
	// which a value-only diff would miss.
	ValueKind string
	// Value is the literal text of the right-hand side, used only by the
	// slot rule (R4) and by violation messages. It is NOT part of the
	// skeleton comparison: changing a slot value is allowed by design.
	Value string
}

// Fingerprint is the coarse skeleton used by rule R5.
type Fingerprint struct {
	Select  int
	From    int
	Join    int
	GroupBy int
	OrderBy int
	MaxDepth int
}

// Equal compares the structural part only.
func (f Fingerprint) Equal(o Fingerprint) bool {
	return f.Select == o.Select &&
		f.From == o.From &&
		f.Join == o.Join &&
		f.GroupBy == o.GroupBy &&
		f.OrderBy == o.OrderBy &&
		f.MaxDepth == o.MaxDepth
}

// Extract shapes one SQL text. It always returns a Shape; callers must
// check Ok before trusting it.
func Extract(sql string) Shape {
	toks := Lex(sql)
	sh := Shape{}

	// Pass 1: skeleton counters, depth, DML, star.
	depth, maxDepth := 0, 0
	for i, t := range toks {
		switch t.Kind {
		case TokPunct:
			switch t.Text {
			case "(":
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
			case ")":
				if depth > 0 {
					depth--
				} else {
					// Unbalanced close paren: tolerate one, note it.
					sh.Note = "unbalanced ')'"
				}
			}
		case TokOperator:
			if t.Text == "*" {
				sh.HasStar = true
			}
		}
		if t.IsKeyword() {
			switch t.Upper() {
			case "SELECT":
				sh.Fingerprint.Select++
			case "FROM":
				sh.Fingerprint.From++
			case "JOIN":
				sh.Fingerprint.Join++
			case "GROUP":
				if i+1 < len(toks) && toks[i+1].IsKeywordAny("BY") {
					sh.Fingerprint.GroupBy++
				}
			case "ORDER":
				if i+1 < len(toks) && toks[i+1].IsKeywordAny("BY") {
					sh.Fingerprint.OrderBy++
				}
			case "INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE",
				"ALTER", "CREATE", "MERGE", "EXECUTE":
				sh.DMLCount++
			}
		}
	}
	if depth != 0 && sh.Note == "" {
		sh.Note = "unbalanced '('"
	}
	sh.Fingerprint.MaxDepth = maxDepth
	sh.Tables = extractTables(toks)
	sh.Predicates = extractPredicates(toks)

	if sh.Fingerprint.Select == 0 && sh.Fingerprint.From == 0 && sh.DMLCount == 0 {
		// Nothing recognizable — an UPDATE statement is fine (DMLCount>0),
		// but a fragment with no statement keyword cannot be shaped.
		sh.Note = "no statement keyword found"
		return sh
	}
	sh.Ok = true
	return sh
}

// extractTables collects `schema.table[@dblink]` from FROM / JOIN sites,
// including comma-separated lists and skipping subqueries.
func extractTables(toks []Token) []string {
	set := map[string]bool{}
	add := func(name string) {
		if name == "" || name == "(" {
			return
		}
		set[normalizeTableName(name)] = true
	}

	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if !t.IsKeywordAny("FROM", "JOIN") {
			continue
		}
		// Walk the table reference list that starts right after.
		j := i + 1
		for j < len(toks) {
			// Skip `(` — a subquery: its own FROM/JOIN will be reached by
			// the outer loop, so just step over this open paren.
			if toks[j].Kind == TokPunct && toks[j].Text == "(" {
				break
			}
			// A table reference is one or more name parts joined by `.`.
			// The lexer folds `schema.table` into one token, but a QUOTED
			// schema (`"ids_wuh".nonauto_plc_element_t`) is necessarily
			// three tokens, so the join has to happen here.
			name, next := readTableRef(toks, j)
			if name == "" {
				break
			}
			add(name)
			j = next
			// Optional alias: an identifier that is not a keyword.
			if j < len(toks) && toks[j].IsKeywordAny("AS") {
				j++
			}
			if j < len(toks) && toks[j].Kind == TokIdent && !toks[j].IsKeyword() {
				j++ // alias
			}
			// Comma-separated table list: `from a, b`.
			if j < len(toks) && toks[j].Kind == TokPunct && toks[j].Text == "," {
				j++
				continue
			}
			break
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readTableRef reads one table reference starting at toks[i] and returns
// its dotted name plus the index just past it. Quoted parts are joined with
// their neighbours (`"ids_wuh" . nonauto_plc_element_t` → one name).
func readTableRef(toks []Token, i int) (string, int) {
	if i >= len(toks) {
		return "", i
	}
	if toks[i].Kind != TokIdent && toks[i].Kind != TokQuotedIdent {
		return "", i
	}
	if toks[i].IsKeyword() {
		return "", i
	}
	name := toks[i].Text
	j := i + 1
	for {
		if j+1 >= len(toks) {
			break
		}
		if toks[j].Kind == TokPunct && toks[j].Text == "." &&
			(toks[j+1].Kind == TokIdent || toks[j+1].Kind == TokQuotedIdent) &&
			!toks[j+1].IsKeyword() {
			name += "." + toks[j+1].Text
			j += 2
			continue
		}
		break
	}
	return name, j
}

// normalizeTableName lower-cases and drops identifier quoting while
// keeping schema and dblink, which are meaningful for access control.
func normalizeTableName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

// identText returns the text of an identifier token, or "" for anything
// else. Quoted identifiers are returned as-is; callers normalize later.
func identText(t Token) string {
	switch t.Kind {
	case TokIdent, TokQuotedIdent:
		return t.Text
	}
	return ""
}

// extractPredicates reads the top-level WHERE atoms.
//
// "Top level" = inside the WHERE that belongs to the outermost statement.
// Atoms inside subqueries are skipped on purpose: the corpus nests
// `(select max(...) from ... where ...)` inside the projection, and those
// conditions are part of the field list, not of the user-adjustable
// filters.
func extractPredicates(toks []Token) []Predicate {
	baseDepth := 0
	depth := 0
	start := -1
	for i, t := range toks {
		if t.Kind == TokPunct {
			switch t.Text {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			}
		}
		if start < 0 && t.IsKeywordAny("WHERE") {
			// Only the OUTERMOST WHERE is the user's filter surface.
			// Correlated subqueries in the projection carry their own WHERE
			// (`where b.policy_no=a.policy_no and b.factor_code='M9'`), and
			// those are part of the field list — not adjustable, and not a
			// predicate skeleton. Matching on depth==0 is what keeps them out.
			if depth != 0 {
				continue
			}
			start = i + 1
			baseDepth = depth
			continue
		}
		if start >= 0 && depth == baseDepth && t.IsKeywordAny("GROUP", "ORDER", "HAVING", "UNION", "INTERSECT", "MINUS", "EXCEPT", "RETURNING") {
			return parsePredicates(toks[start:i])
		}
	}
	if start >= 0 {
		return parsePredicates(toks[start:])
	}
	return nil
}

// parsePredicates splits a WHERE token slice on top-level AND/OR into
// atoms, taking care not to split the `AND` that belongs to BETWEEN.
func parsePredicates(toks []Token) []Predicate {
	var out []Predicate
	depth := 0
	segStart := 0
	pendingBetween := false

	flush := func(end int) {
		if end <= segStart {
			return
		}
		if p, ok := atomify(toks[segStart:end]); ok {
			out = append(out, p)
		}
	}

	for i, t := range toks {
		switch {
		case t.Kind == TokPunct && t.Text == "(":
			depth++
		case t.Kind == TokPunct && t.Text == ")":
			if depth > 0 {
				depth--
			}
		case depth == 0 && t.IsKeywordAny("BETWEEN"):
			pendingBetween = true
		case depth == 0 && t.IsKeywordAny("AND"):
			if pendingBetween {
				// This AND separates the BETWEEN bounds, not two atoms.
				pendingBetween = false
				continue
			}
			flush(i)
			segStart = i + 1
		case depth == 0 && t.IsKeywordAny("OR"):
			flush(i)
			segStart = i + 1
		}
	}
	flush(len(toks))
	return out
}

// atomify turns one AND/OR-free segment into a Predicate.
func atomify(seg []Token) (Predicate, bool) {
	// Strip leading parens left over from `(A or B) and C` grouping.
	for len(seg) > 0 && seg[0].Kind == TokPunct && seg[0].Text == "(" {
		seg = seg[1:]
	}
	for len(seg) > 0 && seg[len(seg)-1].Kind == TokPunct && seg[len(seg)-1].Text == ")" {
		seg = seg[:len(seg)-1]
	}
	if len(seg) < 2 {
		return Predicate{}, false
	}

	// LHS must be a column reference; a function wrapper
	// (`trunc(a.dt)`) is normalized to its argument so that the two
	// equivalent corpus spellings compare equal.
	lhs, rest := columnOf(seg)
	if lhs == "" || len(rest) == 0 {
		return Predicate{}, false
	}

	op, rest := operatorOf(rest)
	if op == "" {
		return Predicate{}, false
	}

	kind := valueKindOf(rest)
	return Predicate{
		Column:    lhs,
		Op:        op,
		ValueKind: kind,
		Value:     literalText(rest),
	}, true
}

// literalText renders the RHS back to a compact, comparable string.
// Whitespace is collapsed and quotes are kept so "2024" and 2024 stay
// distinguishable in a violation message.
func literalText(seg []Token) string {
	if len(seg) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range seg {
		if i > 0 && needsSpace(seg[i-1], t) {
			b.WriteByte(' ')
		}
		switch t.Kind {
		case TokString:
			b.WriteByte('\'')
			b.WriteString(t.Text)
			b.WriteByte('\'')
		default:
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// needsSpace inserts a space between tokens that would otherwise merge
// ambiguously (`in ('a')` stays readable, `2024-01-01` is one string token
// anyway).
func needsSpace(prev, cur Token) bool {
	if prev.Kind == TokOperator || cur.Kind == TokOperator {
		return false
	}
	if cur.Kind == TokPunct && (cur.Text == ")" || cur.Text == ",") {
		return false
	}
	if prev.Kind == TokPunct && prev.Text == "(" {
		return false
	}
	return true
}

// columnOf returns the column reference at the front of seg and the
// remaining tokens. `trunc(col)` → column "col".
func columnOf(seg []Token) (string, []Token) {
	if len(seg) == 0 {
		return "", nil
	}
	// `trunc(col)` / `to_date(col)` / `nvl(col, x)` style wrappers: if the
	// first token is an identifier followed by `(`, use the first argument
	// as the column and resume after the matching `)`.
	if len(seg) >= 2 && seg[1].Kind == TokPunct && seg[1].Text == "(" {
		if seg[0].IsKeyword() {
			// e.g. NOT EXISTS (...) — not a plain column predicate.
			return "", nil
		}
		inner, after := parenGroup(seg, 1)
		if len(inner) > 0 && len(after) > 0 {
			name := strings.ToLower(identText(inner[0]))
			if name != "" {
				return name, after
			}
		}
		return "", nil
	}

	switch seg[0].Kind {
	case TokIdent, TokQuotedIdent:
		if seg[0].IsKeyword() {
			return "", nil
		}
		return strings.ToLower(strings.ReplaceAll(identText(seg[0]), `"`, "")), seg[1:]
	}
	return "", nil
}

// parenGroup returns the tokens inside seg[open] and what follows the
// matching close paren.
func parenGroup(seg []Token, open int) (inner, after []Token) {
	depth := 0
	for i := open; i < len(seg); i++ {
		if seg[i].Kind == TokPunct && seg[i].Text == "(" {
			depth++
			continue
		}
		if seg[i].Kind == TokPunct && seg[i].Text == ")" {
			depth--
			if depth == 0 {
				return seg[open+1 : i], seg[i+1:]
			}
		}
	}
	return nil, nil
}

// operatorOf reads a comparison operator at the front of seg.
func operatorOf(seg []Token) (string, []Token) {
	if len(seg) == 0 {
		return "", nil
	}
	if seg[0].IsKeywordAny("IN") {
		return "in", seg[1:]
	}
	if seg[0].IsKeywordAny("BETWEEN") {
		return "between", seg[1:]
	}
	if seg[0].IsKeywordAny("LIKE") {
		return "like", seg[1:]
	}
	if seg[0].IsKeywordAny("IS") {
		return "is", seg[1:]
	}
	// `NOT IN` / `NOT BETWEEN` / `NOT LIKE`
	if seg[0].IsKeywordAny("NOT") && len(seg) > 1 {
		op, rest := operatorOf(seg[1:])
		switch op {
		case "in", "between", "like", "is":
			return "not " + op, rest
		}
	}
	if seg[0].Kind == TokOperator {
		switch seg[0].Text {
		case "=", "<>", "!=", "<", ">", "<=", ">=":
			op := seg[0].Text
			if op == "!=" {
				op = "<>"
			}
			return op, seg[1:]
		}
	}
	return "", nil
}

// valueKindOf classifies the RHS.
func valueKindOf(seg []Token) string {
	if len(seg) == 0 {
		return "expr"
	}
	switch seg[0].Kind {
	case TokString:
		return "string"
	case TokNumber:
		return "number"
	case TokPunct:
		if seg[0].Text == "(" {
			return "list"
		}
	case TokIdent:
		if seg[0].IsKeywordAny("NULL") {
			return "null"
		}
		if strings.Contains(seg[0].Text, "(") {
			return "expr"
		}
	}
	// A function call or a column reference on the RHS.
	if len(seg) >= 2 && seg[1].Kind == TokPunct && seg[1].Text == "(" {
		name := seg[0].Upper()
		switch name {
		case "TO_DATE", "DATE", "TRUNC", "TO_TIMESTAMP", "STR_TO_DATE":
			return "date"
		}
		return "expr"
	}
	return "expr"
}
