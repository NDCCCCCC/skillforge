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

package ecl

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"unicode/utf8"

	recursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/retriever"
)

// MinSplitRunes is the sub-split threshold in RUNES. Merged section bodies
// at or under this size become ONE Document; larger bodies are sub-split so
// every Milvus row stays well under the content column's silent
// truncateRunes cap (DefaultContentMaxLen = 8192, milvus/config.go) and
// inside BGE-M3's 8192-token window — RESEARCH Pitfall 2: 14/69 corpus
// sections exceed 8192 bytes, the worst at 93,993 bytes, and direct
// section-level seeding would truncate SQL mid-statement with no error.
// 3500 is the planner pin (within RESEARCH's 3000-4000 band).
const MinSplitRunes = 3500

// eclTier is the registry slug the ECL pipeline seeds into. ECL is
// sql-guides-scoped (CONTEXT §Phase Boundary / D-ECL-01) — the slug doubles
// as the retriever filter value (knowledge_bases registry semantics: Tier
// IS the slug).
const eclTier = "sql-guides"

// splitSeparators is the CJK-aware separator ladder for the recursive
// splitter: prefer structural boundaries (blank line, line, CJK/ASCII
// sentence end), then clause separators, then spaces — with "" as the
// FINAL entry. The "" is load-bearing: the splitter passes a still-oversized
// piece through UNSPLIT once its separator list is exhausted (recursive.go:
// `if len(newSeparators) == 0` branch), and strings.Split(text, "") reduces
// any text to per-rune pieces, so the ladder always terminates below the
// chunk size. LenFunc measures runes so ChunkSize counts CJK correctly.
func buildSplitter() (document.Transformer, error) {
	return recursive.NewSplitter(context.Background(), &recursive.Config{
		ChunkSize: MinSplitRunes,
		Separators: []string{
			"\n\n", "\n", "。", "！", "？", ".", "!", "?", ";", "；", "，", ",", " ", "",
		},
		LenFunc: func(s string) int { return utf8.RuneCountInString(s) },
	})
}

// subSplit returns the rune-measured parts of body. Bodies already within
// the cap come back as a single-element slice — the splitter is not invoked
// for them (no surprise whitespace normalization on the common path).
func subSplit(t document.Transformer, body string) ([]string, error) {
	if utf8.RuneCountInString(body) <= MinSplitRunes {
		return []string{body}, nil
	}
	out, err := t.Transform(context.Background(), []*schema.Document{{ID: "ecl-section", Content: body}})
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, len(out))
	for _, d := range out {
		parts = append(parts, d.Content)
	}
	return parts, nil
}

// BuildDocuments converts merged notes into Milvus-ready Documents
// (ONT-13's Milvus half). For every note section: one Document when the
// body is within the cap, else N sub-split Documents. EVERY Document —
// split or not — carries the IDENTICAL section metadata:
//
//	Tier          "sql-guides" (the ECL slug)
//	Content       section body, or one sub-split part body
//	Source        first entry of the section's SourcePaths (deterministic;
//	              ToSlash-normalized to match the citation path contract)
//	SourceIdx     the note's source_idx (e.g. "001")
//	Section       the merged section name
//	SectionIdx    the section's 1-based position within its note (the
//	              retriever.Document contract: 1-based for ECL rows, 0 =
//	              legacy — seed.go D-Milvus-02 zero-fill sentinel)
//	SectionCount  len(note.Sections) (> 0 — the ECL ownership marker)
//	EffectiveFrom/To  0 / 0 — perpetual (KNOWN-02 sentinel)
//	Embedding     nil — the seed path (Plan 06 boot / manager SeedFunc
//	              impl) embeds before insert; the legacy indexer embeds at
//	              build time, but the ECL pipeline deliberately defers it so
//	              a BGE-M3 outage fails the LOAD stage (dead-letterable per
//	              D-RT-01) instead of the chunk build
//	ID            idAlloc() per Document — callers derive it from the
//	              collection-wide watermark (GetMaxIDCollection, Plan 39-02)
//	              so ECL rows can never collide with boot-seeded PKs
//	              (RESEARCH Pitfall 5)
//
// Empty-bodied sections produce no Document (mirrors the legacy indexer's
// empty-body skip — seeding blank rows would only pollute retrieval).
//
// Post-condition (T-39-13): no returned Content exceeds MinSplitRunes
// runes. A violation is a hard ERROR, never a silent oversized row.
func BuildDocuments(notes []Note, idAlloc func() int64) ([]retriever.Document, error) {
	if idAlloc == nil {
		return nil, errors.New("ecl: chunk: idAlloc is required")
	}
	splitter, err := buildSplitter()
	if err != nil {
		return nil, fmt.Errorf("ecl: chunk: splitter: %w", err)
	}

	docs := make([]retriever.Document, 0)
	for _, n := range notes {
		for _, s := range n.Sections {
			if len(s.SourcePaths) == 0 {
				return nil, fmt.Errorf("ecl: chunk: note %s section %q has no source paths", n.SourceIdx, s.Section)
			}
			if len(s.Body) == 0 {
				continue // empty section — nothing retrievable
			}
			parts, err := subSplit(splitter, s.Body)
			if err != nil {
				return nil, fmt.Errorf("ecl: chunk: split note %s section %q: %w", n.SourceIdx, s.Section, err)
			}
			for _, body := range parts {
				if rc := utf8.RuneCountInString(body); rc > MinSplitRunes {
					return nil, fmt.Errorf(
						"ecl: chunk: post-condition violated: note %s section %q part is %d runes (cap %d) — refusing to seed a truncation hazard",
						n.SourceIdx, s.Section, rc, MinSplitRunes)
				}
				docs = append(docs, retriever.Document{
					ID:           idAlloc(),
					Tier:         eclTier,
					Content:      body,
					Source:       filepath.ToSlash(s.SourcePaths[0]),
					SourceIdx:    n.SourceIdx,
					Section:      s.Section,
					SectionIdx:   int64(s.SectionIdx) + 1,
					SectionCount: int64(len(n.Sections)),
				})
			}
		}
	}
	return docs, nil
}
