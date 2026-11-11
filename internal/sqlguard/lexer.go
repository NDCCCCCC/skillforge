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

// Package sqlguard turns the "preserve the template verbatim, only change
// parameter slots" rule from a prompt instruction into a code check.
//
// Why a hand-written lexer instead of a full SQL parser: the corpus this
// guards is DM8 (达梦) extraction recipes — nested DECODE with dozens of
// arguments, `(+)` outer joins, dblink suffixes (`@ora_247`), Chinese
// identifiers, and a handful of samples that are simply not valid SQL
// (`to_date('2026-06-31')`, full-width parentheses). A strict parser would
// reject the templates themselves, so the guard would never run. We only
// need token-level facts (which tables, which predicate skeleton, how many
// joins), never a full AST, so the lexer stays deliberately forgiving and
// the *shape* extraction decides what it can prove.
package sqlguard

import (
	"strings"
)

// TokenKind is the coarse class of a lexed token. Keywords are not a
// separate kind: an identifier whose upper-cased text is in the keyword
// table is treated as one at the shape layer, which keeps the lexer
// dialect-agnostic.
type TokenKind int

const (
	// TokIdent covers bare identifiers, schema-qualified names
	// (`wuh_dw.etl_nonauto_policy_t`), dblink-suffixed names
	// (`t@ora_247`), and "quoted identifiers rendered unquoted.
	TokIdent TokenKind = iota
	// TokQuotedIdent is `"ids_wuh"` or `` `ids_wuh` `` — quoted so that
	// case and reserved words are preserved. Text keeps the inner value.
	TokQuotedIdent
	// TokString is a single-quoted literal. Text is the inner value with
	// doubled quotes collapsed.
	TokString
	// TokNumber is an integer or decimal literal.
	TokNumber
	// TokOperator is a comparison/arithmetic/concatenation operator.
	TokOperator
	// TokPunct is one of ( ) , ; .
	TokPunct
)

// Token is one lexed unit. Pos is the byte offset in the ORIGINAL string
// (comments included), so callers can report actionable positions.
type Token struct {
	Kind TokenKind
	Text string
	Pos  int
}

// Upper returns the upper-cased text, for keyword comparison.
func (t Token) Upper() string { return strings.ToUpper(t.Text) }

// keywords that matter to shape extraction. Not exhaustive — only the
// words the guard reasons about.
var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true,
	"JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true,
	"OUTER": true, "FULL": true, "CROSS": true, "ON": true,
	"GROUP": true, "ORDER": true, "BY": true, "HAVING": true,
	"UNION": true, "INTERSECT": true, "MINUS": true, "EXCEPT": true,
	"WITH": true, "AS": true, "AND": true, "OR": true, "NOT": true,
	"IN": true, "BETWEEN": true, "LIKE": true, "IS": true, "NULL": true,
	"INSERT": true, "UPDATE": true, "DELETE": true, "DROP": true,
	"TRUNCATE": true, "ALTER": true, "CREATE": true, "MERGE": true,
	"EXECUTE": true, "CALL": true, "DECLARE": true, "BEGIN": true,
	"CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true,
	"DISTINCT": true, "ALL": true, "EXISTS": true, "ROWNUM": true,
}

// IsKeyword reports whether the token is a bare keyword.
func (t Token) IsKeyword() bool {
	return t.Kind == TokIdent && keywords[t.Upper()]
}

// IsKeywordAny reports whether the token is a bare keyword equal to one of
// the given (upper-case) words.
func (t Token) IsKeywordAny(want ...string) bool {
	if t.Kind != TokIdent {
		return false
	}
	u := t.Upper()
	for _, w := range words(want) {
		if u == w {
			return true
		}
	}
	return false
}

// ParenDepth feeds the structural fingerprint: the depth after each token.
// Callers compare the SEQUENCE of depths at statement boundaries, which
// pins subquery nesting without needing to parse it.
//
// Lex returns the token stream with comments and whitespace dropped.
//
// Comments are removed, not recorded: the corpus uses comments to carry
// business caveats, and the "verbatim" contract for the *generated* SQL is
// enforced on the query shape, not on comment text. Keeping comments would
// make every template-vs-generated comparison fail on an editorial change
// that cannot affect execution.
//
// The lexer never returns an error. Malformed input (unterminated string,
// stray quote) is lexed as far as possible and the offending byte is kept
// inside the token it belongs to; the shape layer reports "unparseable"
// separately. Failing open here is deliberate — see the package comment.
func Lex(sql string) []Token {
	// 字面 `\r` / `\n` / `\t` (backslash + letter) show up in this corpus
	// as a split marker that never made it through a JSON/CSV import. They
	// are not escapes to the database — the recipe text literally contains
	// them — so expanding them to whitespace is what makes the SQL parse at
	// all. This mirrors excerptBody in internal/server/pipeline.go.
	sql = expandLiteralSplitters(sql)

	toks := make([]Token, 0, len(sql)/4)
	i := 0
	n := len(sql)
	for i < n {
		c := sql[i]
		switch {
		case isSpace(c):
			i++
		case c == '-' && i+1 < n && sql[i+1] == '-':
			// line comment
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			// block comment (not nested — matches the corpus)
			i += 2
			for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
		case c == '\'':
			start := i
			i++
			var b strings.Builder
			for i < n {
				if sql[i] == '\'' {
					if i+1 < n && sql[i+1] == '\'' { // escaped quote
						b.WriteByte('\'')
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteByte(sql[i])
				i++
			}
			toks = append(toks, Token{Kind: TokString, Text: b.String(), Pos: start})
		case c == '"' || c == '`':
			quote := c
			start := i
			i++
			var b strings.Builder
			for i < n {
				if sql[i] == quote {
					if i+1 < n && sql[i+1] == quote {
						b.WriteByte(quote)
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteByte(sql[i])
				i++
			}
			toks = append(toks, Token{Kind: TokQuotedIdent, Text: b.String(), Pos: start})
		case isDigit(c):
			start := i
			for i < n && (isDigit(sql[i]) || sql[i] == '.') {
				// A dot followed by another dot is not part of a number;
				// this also stops `1.` from swallowing a trailing period.
				if sql[i] == '.' && i+1 < n && !isDigit(sql[i+1]) {
					break
				}
				i++
			}
			toks = append(toks, Token{Kind: TokNumber, Text: sql[start:i], Pos: start})
		case isIdentStart(c):
			start := i
			for i < n && isIdentPart(sql[i]) {
				// Allow `.` only when a name follows, so `a.*` yields
				// ident("a") punct(".") op("*") rather than ident("a.").
				if sql[i] == '.' && i+1 < n && !isIdentStart(sql[i+1]) {
					break
				}
				i++
			}
			toks = append(toks, Token{Kind: TokIdent, Text: sql[start:i], Pos: start})
		case c == '(' || c == ')' || c == ',' || c == ';' || c == '.':
			toks = append(toks, Token{Kind: TokPunct, Text: string(c), Pos: i})
			i++
		default:
			// operators, including two-char forms
			two := ""
			if i+1 < n {
				two = sql[i : i+2]
			}
			switch two {
			case "<>", "!=", ">=", "<=", "||", "=>", ":=":
				toks = append(toks, Token{Kind: TokOperator, Text: two, Pos: i})
				i += 2
			default:
				// Unknown single byte (e.g. a stray full-width character):
				// keep it as an operator-ish token so positions stay true.
				toks = append(toks, Token{Kind: TokOperator, Text: string(c), Pos: i})
				i++
			}
		}
	}
	return toks
}

// expandLiteralSplitters turns the two-character sequences `\r`, `\n`, `\t`
// into a space. See the body comment in Lex.
func expandLiteralSplitters(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'r', 'n', 't':
				b.WriteByte(' ')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c == '#' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		c >= 0x80 // UTF-8 lead byte: Chinese identifiers
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '@' || c == '.' || c == '$' || c == '#'
}

// words upper-cases a variadic list for keyword comparison.
func words(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = strings.ToUpper(w)
	}
	return out
}
