// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ecl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ExtractedTable is one table reference produced by an extractor
// (deterministic stub or Plan 04's LLM implementation — both emit this
// type). Kind is the D-Ext-01 enum: update|select|insert|create.
type ExtractedTable struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// RelatedNote is one note-to-note relation. Relation is a free-form tag
// ("mentions-note" is the stub's only value today).
type RelatedNote struct {
	SourceIdx string `json:"source_idx"`
	Relation  string `json:"relation"`
}

// Extraction is the D-Ext-01 fixed JSON schema produced per merged note:
// {"tables":[{"name","kind"}],"related_notes":[{"source_idx","relation"}]}.
type Extraction struct {
	Tables       []ExtractedTable `json:"tables"`
	RelatedNotes []RelatedNote    `json:"related_notes"`
}

// tableNamePattern is the corpus-contract gate for extracted table names
// (WR-03 / T-39-06 defense in depth): the D-Ext-01 prompt asks only for
// etl_-prefixed identifiers ("Include only tables whose name starts with
// etl_") and every detection regex (sqlUpdateRegex etc.) captures
// `etl_\w+` — so an extraction outside ^etl_\w+$ is a hallucination or
// injection, and rejecting it here keeps arbitrary LLM strings from
// minting skf:tbl_ ABox individuals. `\w` is [0-9A-Za-z_] — exactly the
// characters SanitizeLocal passes through unchanged — so gated names are
// also collision-free under the sanitizer.
var tableNamePattern = regexp.MustCompile(`^etl_\w+$`)

// ValidateExtraction enforces the D-Ext-01 contract on an Extraction: the
// kind enum AND the etl_ table-name corpus contract. Plan 04's LLM
// extractor reuses this to gate model output before it can reach the ABox
// writer or the golden set (defense in depth alongside llm.ExtractJSON).
func ValidateExtraction(ex Extraction) error {
	for _, t := range ex.Tables {
		switch t.Kind {
		case "update", "select", "insert", "create":
			// legal
		default:
			return fmt.Errorf("ecl: invalid extraction kind %q for table %q (legal: update|select|insert|create)", t.Kind, t.Name)
		}
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("ecl: extraction has empty table name (kind %q)", t.Kind)
		}
		if !tableNamePattern.MatchString(t.Name) {
			return fmt.Errorf("ecl: extraction table name %q violates the corpus contract (want ^etl_\\w+$ — D-Ext-01 gates extractions to etl_-prefixed identifiers)", t.Name)
		}
	}
	return nil
}

// SQL detection regexes. Base patterns are the VALIDATED spike 005c
// regexes (main.go:205-209) extended to the D-Ext-01 kind enum:
//
//	sqlUpdateRegex  — spike sqlUpdateRegex verbatim
//	sqlStartRegex   — spike sqlStartRegex's alternation, split per kind
//
// Keyword case-insensitivity is scoped with (?i:…) so the etl_ table-name
// prefix stays STRICTLY lowercase (corpus contract) while captured names
// keep their original case (etl_NONAUTO_POLICY_T stays uppercase).
// All patterns are line-anchored — the merge appends "\n" between fragment
// bodies so statement starts are always at line starts.
var (
	sqlUpdateRegex   = regexp.MustCompile(`(?im)^\s*(?i:update)\s+(etl_\w+)\b`)
	sqlInsertRegex   = regexp.MustCompile(`(?im)^\s*(?i:insert)\s+(?i:into)\s+(etl_\w+)\b`)
	sqlCreateLine    = regexp.MustCompile(`(?im)^\s*(?i:create)\s+(?i:or)\s+(?i:replace)\b[^\n]*`)
	sqlDeleteRegex   = regexp.MustCompile(`(?im)^\s*(?i:delete)\s+(?i:from)\s+(etl_\w+)\b`)
	sqlMergeRegex    = regexp.MustCompile(`(?im)^\s*(?i:merge)\s+(?i:into)\s+(etl_\w+)\b`)
	sqlSelectInto    = regexp.MustCompile(`(?im)^\s*(?i:select)\b[^\n]*?\s+(?i:into)\s+(etl_\w+)\b`)
	sqlTableName     = regexp.MustCompile(`\betl_\w+`)
	relatedNoteRegex = regexp.MustCompile(`(?:见|参见|详见|参照|参阅)\s*([0-9]{3})\b`)
)

// sqlRule pairs a line-anchored detection regex with the D-Ext-01 kind it
// maps to. DELETE FROM and MERGE INTO normalize to "update": "delete" is
// NOT in the D-Ext-01 enum, and both statements rewrite existing rows —
// the closest legal enum member (pinned in plan 39-03 Task 2 <behavior>).
var sqlRules = []struct {
	re   *regexp.Regexp
	kind string
}{
	{sqlUpdateRegex, "update"},
	{sqlInsertRegex, "insert"},
	{sqlCreateLine, "create"}, // table name captured separately (first etl_* on the line)
	{sqlDeleteRegex, "update"},
	{sqlMergeRegex, "update"},
	{sqlSelectInto, "select"},
}

// StubExtract runs the deterministic regex extractor over a merged note
// (D-GT-01: this stub IS the golden-set drafter — the LLM never builds the
// golden set). The full merged note body (all sections concatenated) is
// scanned; hits are ordered by first occurrence in the body and deduped by
// table name.
//
// Related-notes detection: a note "relates to" note N when its body
// contains a 参考/见-style word immediately followed by N's 3-digit
// source_idx. Corpus survey (2026-09-22, plan 39-03 Task 2) found NO
// explicit numeric cross-references in knowledge/sql-guides today — the
// two nearest non-reference usages that motivated the strict
// word+number+boundary anchor are:
//
//	knowledge/sql-guides/取数口径/041_数据提取字段解读__sec06_SQL_正文.md:14
//	  "…冲开门红业绩…" (开门红 is a business term here, not note 029)
//	knowledge/sql-guides/财务对账/010_财务现金流小程序实现方案__sec04_SQL_正文.md:12
//	  "2. 定时任务配置本项目需要配置两个定时任务…" (generic noun, not note 012)
//
// and the only prose reference found cites a sibling note by TITLE, not id:
//
//	knowledge/sql-guides/操作手册/002_Linux中启动FineBI.md:11
//	  "详情参见：Linux 中修改 JVM 内存"
//
// So on the current corpus the drafter emits zero related_notes (zero
// false positives); human review in the D-GT-01 flow owns relations the
// regex cannot see. The anchor is pinned so future corpus edits that add
// explicit "见 0NN" references are captured deterministically.
func StubExtract(n Note) Extraction {
	var body strings.Builder
	for _, s := range n.Sections {
		body.WriteString(s.Body)
	}
	return Extraction{
		Tables:       extractTables(body.String()),
		RelatedNotes: extractRelatedNotes(body.String(), n.SourceIdx),
	}
}

// hit is one raw regex match waiting for positional ordering.
type hit struct {
	offset int
	table  ExtractedTable
}

// extractTables scans the body with every sqlRule, orders hits by their
// position in the body, and dedupes by table name (first-seen wins).
func extractTables(body string) []ExtractedTable {
	var hits []hit
	for _, rule := range sqlRules {
		locs := rule.re.FindAllStringSubmatchIndex(body, -1)
		for _, loc := range locs {
			// group 1 = captured table name; for the create rule the
			// table name is NOT a capture group — search the matched
			// line for the first etl_* token.
			var name string
			if rule.kind == "create" {
				line := body[loc[0]:loc[1]]
				m := sqlTableName.FindString(line)
				if m == "" {
					continue // CREATE line without an etl_ target: nothing to emit
				}
				name = m
			} else {
				name = body[loc[2]:loc[3]]
			}
			hits = append(hits, hit{offset: loc[0], table: ExtractedTable{Name: name, Kind: rule.kind}})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].offset < hits[j].offset })
	seen := map[string]bool{}
	out := []ExtractedTable{}
	for _, h := range hits {
		if seen[h.table.Name] {
			continue
		}
		seen[h.table.Name] = true
		out = append(out, h.table)
	}
	return out
}

// extractRelatedNotes finds explicit numeric note references, excluding
// self-references and duplicates, preserving first-seen order.
func extractRelatedNotes(body string, selfSourceIdx string) []RelatedNote {
	matches := relatedNoteRegex.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	out := []RelatedNote{}
	for _, m := range matches {
		idx := m[1]
		if idx == selfSourceIdx || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, RelatedNote{SourceIdx: idx, Relation: "mentions-note"})
	}
	return out
}
