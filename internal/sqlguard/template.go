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

// ExtractTemplates pulls every SQL statement out of a retrieved note.
//
// The corpus is messier than it looks (see .planning/v2/sql-corpus-analysis.md):
// chunks are 4KB slices of a larger document, so a fenced block can start in
// one chunk and end in the next; some fences are unclosed; and a few notes
// write SQL outside any fence. The extractor therefore:
//
//  1. prefers fenced blocks, including ones whose closing fence was cut off
//     (an unterminated fence that starts a statement is kept — a truncated
//     template is still a usable whitelist for tables and predicates);
//  2. falls back to a bare statement scan when no fence is present;
//  3. never returns an empty-string entry, and drops blocks without any
//     statement keyword.
//
// Returned texts keep their original line breaks (literal `\r` splitters are
// expanded to newlines here so the caller can render them); the lexer is
// tolerant either way.
func ExtractTemplates(note string) []string {
	if strings.TrimSpace(note) == "" {
		return nil
	}
	note = normalizeSplitters(note)

	var out []string
	for _, block := range fencedBlocks(note) {
		if looksLikeSQL(block) {
			out = append(out, strings.TrimSpace(block))
		}
	}
	if len(out) == 0 {
		if stmt := bareStatement(note); looksLikeSQL(stmt) {
			out = append(out, strings.TrimSpace(stmt))
		}
	}
	return out
}

// normalizeSplitters turns literal `\r` / `\n` / `\t` sequences into real
// newlines. The corpus uses them as line breaks that a CSV/JSON import
// escaped, which is why a 17-line recipe sometimes arrives as one line.
func normalizeSplitters(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	s = strings.ReplaceAll(s, `\r\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\n")
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, " ")
	return s
}

// fencedBlocks walks the text and returns the contents of every ``` fence.
// An unterminated fence contributes the rest of the text.
func fencedBlocks(s string) []string {
	lines := strings.Split(s, "\n")
	var blocks []string
	var cur []string
	inFence := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		isFence := strings.HasPrefix(trimmed, "```")
		if isFence {
			if inFence {
				blocks = append(blocks, strings.Join(cur, "\n"))
				cur = nil
				inFence = false
				continue
			}
			inFence = true
			cur = nil
			// A fence that opens another fence inside an open one
			// (`042__sec07`) is treated as a close-then-open by the branch
			// above; this branch only runs when not already inside.
			continue
		}
		if inFence {
			cur = append(cur, ln)
		}
	}
	if inFence && len(cur) > 0 {
		blocks = append(blocks, strings.Join(cur, "\n"))
	}
	return blocks
}

// looksLikeSQL is a cheap gate: the block must contain a statement keyword.
func looksLikeSQL(s string) bool {
	u := strings.ToUpper(s)
	for _, kw := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "CREATE", "MERGE", "DECLARE"} {
		if strings.Contains(u, kw) {
			return true
		}
	}
	return false
}

// bareStatement returns the text from the first statement keyword to the
// end, used when a note has no fences at all.
func bareStatement(s string) string {
	u := strings.ToUpper(s)
	best := -1
	for _, kw := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH"} {
		if i := strings.Index(u, kw); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return s[best:]
}

// MinTableOverlap is the Jaccard threshold a candidate template must clear
// before it may be used as the reference for a generated answer.
//
// Why a threshold and not "best match wins": live regression (2026-11-11)
// showed two different recipes that share one table
// (`wuh_dw.etl_nonauto_policy_t` is the common fact table for 学平险 and
// 承保清单). Picking the highest-overlap template regardless of how low
// that overlap is made the guard reject a perfectly good answer — the
// wrong reference produced R1 (missing table), R3 (3 vs 2 predicates) and
// R5 (different skeleton) failures at once.
//
// A correct template overlaps on essentially every table it names
// (Jaccard ≈ 1.0); an unrelated-but-same-fact-table recipe lands around
// 0.3. 0.5 separates them without being fragile.
const MinTableOverlap = 0.5

// BestTemplate picks the template that most plausibly produced the
// generated answer.
//
// A candidate qualifies only when its table set is at least MinTableOverlap
// similar (Jaccard) to the generated SQL's table set. Sharing one table out
// of three is not evidence of derivation, and using such a template as the
// yardstick turns the guard into a false-positive machine.
//
// Returns ok=false when no candidate clears the threshold — the caller must
// then skip the check rather than compare against an unrelated recipe.
func BestTemplate(note string, generated string) (sql string, tmpl Shape, ok bool) {
	cands := ExtractTemplates(note)
	if len(cands) == 0 {
		return "", Shape{}, false
	}
	gen := Extract(generated)
	if !gen.Ok || len(gen.Tables) == 0 {
		return "", Shape{}, false
	}
	genSet := map[string]bool{}
	for _, t := range gen.Tables {
		genSet[t] = true
	}

	type scored struct {
		sql   string
		shape Shape
		jacc  float64
		hit   int
		size  int
	}
	list := make([]scored, 0, len(cands))
	for _, c := range cands {
		sh := Extract(c)
		if !sh.Ok {
			continue
		}
		hit := 0
		union := map[string]bool{}
		for _, t := range sh.Tables {
			union[t] = true
			if genSet[t] {
				hit++
			}
		}
		for _, t := range gen.Tables {
			union[t] = true
		}
		if hit == 0 {
			continue
		}
		jacc := float64(hit) / float64(len(union))
		list = append(list, scored{sql: c, shape: sh, jacc: jacc, hit: hit, size: len(c)})
	}
	if len(list) == 0 {
		return "", Shape{}, false
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].jacc != list[j].jacc {
			return list[i].jacc > list[j].jacc
		}
		if list[i].hit != list[j].hit {
			return list[i].hit > list[j].hit
		}
		return list[i].size > list[j].size
	})
	top := list[0]
	if top.jacc < MinTableOverlap {
		return "", Shape{}, false
	}
	return top.sql, top.shape, true
}
