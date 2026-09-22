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

// Phase 39 / Plan 39-05 Task 2 — BuildDocuments tests. Synthetic notes only
// (no corpus dependency): the oversized-section sub-split contract
// (RESEARCH Pitfall 2 / T-39-13), metadata consistency across split parts,
// the 3500-rune cap post-condition, and deterministic ID allocation.
package ecl

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// seqAllocator returns an idAlloc closure counting from first (mirrors the
// Plan 06 watermark counter shape: base + sequential increments).
func seqAllocator(first int64) func() int64 {
	next := first
	return func() int64 {
		id := next
		next++
		return id
	}
}

// updateTableRegex mirrors the corpus SQL-target contract the sub-split
// must preserve (no `update etl_\w+` occurrence lost across parts).
var updateTableRegex = regexp.MustCompile(`update etl_\w+`)

// updateTargetSet extracts the distinct update etl_* targets of a body.
func updateTargetSet(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range updateTableRegex.FindAllString(body, -1) {
		out[m] = true
	}
	return out
}

// cjkSQLBody synthesizes a CJK+SQL body over minRunes runes with exactly
// tableCount distinct `update etl_*` line-anchored statements, spread
// through the body. Lines carry no trailing separator dependence: every
// statement starts at a line start (the merge contract).
func cjkSQLBody(minRunes, tableCount int) string {
	var b strings.Builder
	tables := make([]string, 0, tableCount)
	line := 0
	for utf8.RuneCountInString(b.String()) < minRunes {
		if line < tableCount {
			name := fmt.Sprintf("etl_oversize_%03d_t", line)
			tables = append(tables, name)
			b.WriteString("update " + name + " set status = 'A' where id = " + fmt.Sprint(line) + ";\n")
		}
		b.WriteString("这是一个用于测试的中文段落，包含足够的汉字以覆盖 CJK 码点计数路径。\n")
		line++
	}
	// Ensure the LAST table statement exists even if the rune budget was
	// reached before line == tableCount.
	for i := len(tables); i < tableCount; i++ {
		name := fmt.Sprintf("etl_oversize_%03d_t", i)
		tables = append(tables, name)
		b.WriteString("update " + name + " set status = 'A' where id = " + fmt.Sprint(i) + ";\n")
	}
	return b.String()
}

// TestBuildDocumentsShortSection: a within-cap section becomes exactly ONE
// Document carrying the full section metadata; IDs come from idAlloc in
// order; EffectiveFrom/To are the perpetual sentinels; Embedding is nil
// (the seed path embeds).
func TestBuildDocumentsShortSection(t *testing.T) {
	notes := []Note{
		{
			SourceIdx: "001",
			Title:     "保费试算",
			Sections: []MergedSection{
				{
					Section:     "概述",
					SectionIdx:  0,
					Body:        "update etl_pricing_t;\n",
					SourcePaths: []string{"knowledge/sql-guides/费率/001_保费试算__sec01_概述.md", "knowledge/sql-guides/费率/001_保费试算__sec02_概述.md"},
				},
				{
					Section:     "SQL / 正文",
					SectionIdx:  1,
					Body:        "select * from etl_pricing_t;\n",
					SourcePaths: []string{"knowledge/sql-guides/费率/001_保费试算__sec03_SQL_正文.md"},
				},
			},
		},
	}

	docs, err := BuildDocuments(notes, seqAllocator(100))
	require.NoError(t, err)
	require.Len(t, docs, 2)

	first := docs[0]
	assert.Equal(t, int64(100), first.ID, "IDs come from idAlloc sequentially")
	assert.Equal(t, int64(101), docs[1].ID)
	assert.Equal(t, "sql-guides", first.Tier)
	assert.Equal(t, "update etl_pricing_t;\n", first.Content, "within-cap body passes through unsplit")
	assert.Equal(t, "001", first.SourceIdx)
	assert.Equal(t, "概述", first.Section)
	assert.Equal(t, int64(1), first.SectionIdx, "retriever.Document contract: 1-based section position")
	assert.Equal(t, int64(2), first.SectionCount)
	assert.Equal(t, "knowledge/sql-guides/费率/001_保费试算__sec01_概述.md", first.Source,
		"Source = FIRST SourcePaths entry, deterministic")
	assert.Equal(t, int64(0), first.EffectiveFrom, "perpetual sentinel")
	assert.Equal(t, int64(0), first.EffectiveTo, "perpetual sentinel")
	assert.Nil(t, first.Embedding, "seed path embeds, not the chunker")

	assert.Equal(t, int64(2), docs[1].SectionIdx, "second section is 1-based position 2")
	assert.Equal(t, int64(2), docs[1].SectionCount)
}

// TestBuildDocumentsOversizedSubSplit: an oversized CJK+SQL section yields
// multiple parts, all ≤ MinSplitRunes runes, all carrying IDENTICAL section
// metadata, with zero `update etl_*` targets lost or duplicated across the
// parts (RESEARCH Pitfall 2 designed out; T-39-13 post-condition).
func TestBuildDocumentsOversizedSubSplit(t *testing.T) {
	const (
		minRunes   = 10000
		tableCount = 5
	)
	body := cjkSQLBody(minRunes, tableCount)
	require.Greater(t, utf8.RuneCountInString(body), minRunes)

	notes := []Note{
		{
			SourceIdx: "039",
			Title:     "超大节",
			Sections: []MergedSection{
				{
					Section:     "SQL / 正文",
					SectionIdx:  0,
					Body:        body,
					SourcePaths: []string{"knowledge/sql-guides/对账/039_超大__sec02_SQL_正文.md"},
				},
			},
		},
	}

	docs, err := BuildDocuments(notes, seqAllocator(1))
	require.NoError(t, err)
	require.Greater(t, len(docs), 1, "oversized section must be sub-split")

	var rebuilt strings.Builder
	for i, d := range docs {
		assert.LessOrEqual(t, utf8.RuneCountInString(d.Content), MinSplitRunes,
			"part %d exceeds the rune cap — silent truncateRunes hazard", i)
		// IDENTICAL section metadata on every part.
		assert.Equal(t, "sql-guides", d.Tier)
		assert.Equal(t, "039", d.SourceIdx)
		assert.Equal(t, "SQL / 正文", d.Section)
		assert.Equal(t, int64(1), d.SectionIdx)
		assert.Equal(t, int64(1), d.SectionCount)
		assert.Equal(t, int64(i+1), d.ID, "sequential IDs from idAlloc")
		rebuilt.WriteString(d.Content)
		rebuilt.WriteString("\n")
	}

	// No table target lost — and none duplicated by split boundaries.
	want := updateTargetSet(body)
	got := updateTargetSet(rebuilt.String())
	assert.Equal(t, tableCount, len(want), "synthetic body must contain %d distinct targets", tableCount)
	assert.Equal(t, want, got, "sub-split must preserve every `update etl_*` target exactly once")
}

// TestBuildDocumentsMixedNotes: per-note metadata independence across
// multiple notes in one call — SectionIdx restarts per note, SectionCount
// is per-note, mixed short+oversized sections coexist, and SectionCount is
// never zero (the ECL ownership marker the marker-scoped purge keys on).
func TestBuildDocumentsMixedNotes(t *testing.T) {
	big := cjkSQLBody(8000, 2)
	notes := []Note{
		{
			SourceIdx: "002",
			Title:     "短笔记",
			Sections: []MergedSection{
				{Section: "概述", SectionIdx: 0, Body: "短正文", SourcePaths: []string{"k/002__sec01_概述.md"}},
				{Section: "SQL", SectionIdx: 1, Body: big, SourcePaths: []string{"k/002__sec02_SQL.md"}},
			},
		},
		{
			SourceIdx: "003",
			Title:     "另一笔记",
			Sections: []MergedSection{
				{Section: "概述", SectionIdx: 0, Body: "另一短正文", SourcePaths: []string{"k/003__sec01_概述.md"}},
			},
		},
	}

	docs, err := BuildDocuments(notes, seqAllocator(50))
	require.NoError(t, err)

	byNote := map[string][]retriever.Document{}
	for _, d := range docs {
		byNote[d.SourceIdx] = append(byNote[d.SourceIdx], d)
	}
	require.GreaterOrEqual(t, len(byNote["002"]), 3,
		"short section = 1 doc, oversized = ceil(8000/3500) docs")
	require.Len(t, byNote["003"], 1)

	// Note 002: overview doc first (section order), then the split SQL
	// docs sharing the SQL section's metadata.
	n2 := byNote["002"]
	assert.Equal(t, int64(2), n2[0].SectionCount)
	assert.Equal(t, int64(1), n2[0].SectionIdx)
	assert.Equal(t, "概述", n2[0].Section)
	for _, d := range n2[1:] {
		assert.Equal(t, int64(2), d.SectionCount)
		assert.Equal(t, int64(2), d.SectionIdx)
		assert.Equal(t, "SQL", d.Section)
		assert.LessOrEqual(t, utf8.RuneCountInString(d.Content), MinSplitRunes)
	}
	// Note 003 restarts its own 1-based section indexing.
	assert.Equal(t, int64(1), byNote["003"][0].SectionIdx)
	assert.Equal(t, int64(1), byNote["003"][0].SectionCount)

	for _, d := range docs {
		assert.Greater(t, d.SectionCount, int64(0), "ECL rows always carry the ownership marker")
	}
}

// TestBuildDocumentsGuards: empty sections emit nothing; missing idAlloc
// and source-less sections are errors, never silent.
func TestBuildDocumentsGuards(t *testing.T) {
	docs, err := BuildDocuments([]Note{
		{
			SourceIdx: "004",
			Sections: []MergedSection{
				{Section: "空节", SectionIdx: 0, Body: "", SourcePaths: []string{"k/004.md"}},
			},
		},
	}, seqAllocator(1))
	require.NoError(t, err)
	assert.Empty(t, docs, "empty-bodied sections must not seed blank rows")

	_, err = BuildDocuments(nil, nil)
	assert.Error(t, err, "nil idAlloc must error — IDs cannot be minted")

	_, err = BuildDocuments([]Note{
		{
			SourceIdx: "005",
			Sections: []MergedSection{
				{Section: "无路径", SectionIdx: 0, Body: "正文"},
			},
		},
	}, seqAllocator(1))
	assert.Error(t, err, "section without SourcePaths must error, not silently seed Source=\"\"")
}
