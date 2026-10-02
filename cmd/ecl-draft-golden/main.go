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

// Command ecl-draft-golden is the D-GT-01 golden-set DRAFTER for the ECL
// pipeline: it walks the sql-guides KB, reassembles the __secNN fragments
// into logical notes (ecl.WalkAndMerge), runs the deterministic regex stub
// extractor (ecl.StubExtract) over each merged note, and emits one jsonl
// draft row per note in the D-GT-01 row shape:
//
//	{"source_idx":"001","tables":[{"name":"etl_payment_due_t","kind":"update"}],"related":["006"]}
//
// D-GT-01 INVARIANT: this binary has NO LLM code path. Its only project
// import is internal/knowledge/ecl, and its only entry points are the
// deterministic WalkAndMerge + StubExtract functions. Any model/provider
// import in this directory is a D-GT-01 violation (the plan's grep gate
// checks the import block). The LLM NEVER builds the golden set
// (anti-self-grading); this draft is the starting point for the mandatory
// HUMAN review checkpoint
// (.planning/phases/39-ecl-pipeline/golden-review-checklist.md), whose
// output — not this file — becomes eval/ecl-golden/sql-guides.jsonl.
//
// The stub emits zero related-notes on the current corpus (no explicit
// numeric cross-references exist; plan 39-03 survey) — human review owns
// the relations column entirely.
//
// Usage:
//
//	go run ./cmd/ecl-draft-golden -kb knowledge/sql-guides -out eval/ecl-golden/sql-guides.jsonl
//
// Exit codes: 0 on success, 1 on error (unwalkable KB, zero merged notes,
// unwritable out path).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ecl-draft-golden:", err)
		os.Exit(1)
	}
}

func run() error {
	kb := flag.String("kb", "knowledge/sql-guides", "sql-guides KB directory to walk and merge")
	out := flag.String("out", "eval/ecl-golden/sql-guides.jsonl", "draft jsonl output path")
	flag.Parse()

	notes, _, err := ecl.WalkAndMerge(*kb)
	if err != nil {
		return err
	}
	// Corpus-path guard: a 0-note walk means the KB path is wrong or the
	// corpus lost its fragment shape. Drafting an empty golden set would be
	// indistinguishable from a real one downstream — refuse loudly.
	if len(notes) == 0 {
		return fmt.Errorf("walk %s: 0 merged notes — wrong KB path or non-ECL corpus", *kb)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	f, err := os.Create(*out)
	if err != nil {
		return fmt.Errorf("open %s: %w", *out, err)
	}
	defer f.Close()

	// WalkAndMerge already returns notes sorted by numeric source_idx;
	// emit one jsonl row per note in that order. Empty lists marshal as []
	// (never null) to match the eval/golden jsonl conventions.
	for _, n := range notes {
		ex := ecl.StubExtract(n)
		row := ecl.GoldenRow{
			SourceIdx: n.SourceIdx,
			Tables:    ex.Tables,
			Related:   make([]string, 0, len(ex.RelatedNotes)),
		}
		for _, r := range ex.RelatedNotes {
			row.Related = append(row.Related, r.SourceIdx)
		}
		if ex.Tables == nil {
			row.Tables = []ecl.ExtractedTable{}
		}
		line, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("marshal row %s: %w", n.SourceIdx, err)
		}
		if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
			return fmt.Errorf("write %s: %w", *out, err)
		}
	}
	fmt.Fprintf(os.Stdout, "drafted %d rows -> %s\n", len(notes), *out)
	return nil
}
