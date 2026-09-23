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

// Package ecl implements the Phase 39 ECL (Extract-Classify-Load) pipeline
// substrate: the section-aware walk+merge chunker for sql-guides (ONT-10),
// its ChunkMeta contract, the IsSecFragment routing predicate the knowledge
// indexer uses to divert __secNN fragments onto the ECL path (Plan 06), and
// the deterministic regex stub extractor that doubles as the golden-set
// drafter (D-GT-01).
//
// The chunker is a productionized port of VALIDATED spike 005c
// (.planning/spikes/005c-ontology-aware-chunker/main.go) — D-ECL-02 locks
// "原样生产化" (productionize as-is): the filename regex, the group/sort/
// merge algorithm, and the ChunkMeta JSON contract are kept faithful to the
// spike so the pinned corpus numbers (185 fragments → 26 notes → 69 section
// records, note 001 contains 3/3 corpus SQL targets) remain reproducible.
package ecl

// ChunkMeta is the per-section metadata record attached to every merged
// section (D-ECL-02 contract). Field names and json tags are VERBATIM from
// spike 005c's chunk_metadata.json — 69 records on the current corpus — so
// the Milvus seed writer (Plans 05/06) and any downstream consumer can key
// on the exact same field names the spike validated.
type ChunkMeta struct {
	SourceIdx   string   `json:"source_idx"`
	Title       string   `json:"title"`
	Section     string   `json:"section"`
	SectionIdx  int      `json:"section_idx"`
	Sections    int      `json:"section_count"`
	SourcePaths []string `json:"source_paths"`
	BodyBytes   int      `json:"body_bytes"`
}

// Fragment is one parsed __secNN file — a byte-bucket-split piece of a
// logical note. Fragments are the input to MergeFragments.
type Fragment struct {
	Path      string // path as passed to the walker (input-root relative)
	SecOrder  int    // numeric order from the filename __secNN
	Title     string // frontmatter title
	SourceIdx string // frontmatter source_idx (the logical note id, e.g. "001")
	Section   string // frontmatter section (the ontology boundary, e.g. "SQL / 正文")
	Body      string // body text after the frontmatter block
}

// MergedSection concatenates every fragment of a note that shares the same
// frontmatter section value, in fragment (secNN) order.
type MergedSection struct {
	Section     string
	SectionIdx  int // first-seen order of the section within its note
	Body        string
	SourcePaths []string
}

// Note is a merged logical note: all __secNN fragments sharing one
// source_idx, reassembled into ordered sections.
type Note struct {
	SourceIdx string
	Title     string
	Sections  []MergedSection // ordered by first-seen section order

	// Tier is the KB subdirectory name (e.g. "sql-guides") set during
	// WalkAndMerge. Used for belongsToTier ABox triples (D-TIER-02).
	Tier string

	// StartLine and EndLine are 1-based line numbers from the original
	// source file's frontmatter marker, used for Provenance (D-PROV-05).
	// Zero means unknown/unavailable.
	StartLine int
	EndLine   int
}
