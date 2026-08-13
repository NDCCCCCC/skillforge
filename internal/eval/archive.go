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

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArchiveTimestampLayout is the filename-safe UTC timestamp format for
// archived runs. Colons are ILLEGAL in Windows filenames, so the time
// component uses hyphens: 2026-08-13T10-05-21Z.
const ArchiveTimestampLayout = "2006-01-02T15-04-05Z"

// IndexFileName is the trend-history manifest inside the archive dir.
const IndexFileName = "INDEX.json"

// RefusalCategory is the golden-set category whose context_recall IS
// the refusal-accuracy rate.
//
// Pattern 3 — NO NEW METRIC. metrics.go::ContextRecall returns 1.0 when
// (groundTruth == "REFUSAL" && refused) and 0.0 otherwise, so the mean
// context_recall over the refusal-required category is precisely
// "correct refusals / refusal-required cases". Introducing a separate
// refusal_accuracy metric function would be a second source of truth
// for a number we already compute.
const RefusalCategory = "refusal-required"

// IndexRun is one row of the trend history (D6-02). It is deliberately
// a FLAT, lightweight summary: the dashboard's trend line reads the
// whole index on every poll, and lazy-loads the full report only when
// the operator drills into a specific run.
type IndexRun struct {
	TS               string  `json:"ts"`
	Skill            string  `json:"skill"`
	JudgeModel       string  `json:"judge_model"`
	Faithfulness     float64 `json:"faithfulness"`
	ContextPrecision float64 `json:"context_precision"`
	AnswerRelevance  float64 `json:"answer_relevance"`
	ContextRecall    float64 `json:"context_recall"`
	RefusalAccuracy  float64 `json:"refusal_accuracy"`
	FrozenVersion    string  `json:"frozen_version"`
	ReportPath       string  `json:"report_path"`
}

// EvalIndex is the eval/reports/INDEX.json document.
//
// Version is a schema version, bumped only on a breaking shape change,
// so a future reader can migrate rather than mis-parse.
type EvalIndex struct {
	Version int        `json:"version"`
	Runs    []IndexRun `json:"runs"`
}

// IndexVersion is the current EvalIndex schema version.
const IndexVersion = 1

// WriteArchive appends one run to the file-based trend history (D6-02):
//
//  1. writes the FULL report to <dir>/<iso-ts>.json (via WriteReport, so
//     archived reports and eval/report.json share one serializer), and
//  2. appends a lightweight IndexRun to <dir>/INDEX.json.
//
// Why files and not a table: eval metadata is not audit data. Keeping
// it in git makes history diffable and reviewable by OSS contributors,
// costs zero dependencies, and keeps the INSERT-only audit store's
// semantic boundary clean.
//
// CONCURRENCY LIMITATION (documented, accepted): the read-modify-write
// of INDEX.json is atomic against a CRASH (tmp file + rename) but NOT
// against a concurrent second process — two simultaneous archiving runs
// can lose one entry. Eval runs are minute-scale and effectively
// serialized (one CI job, one local operator), and git is the canonical
// history if an entry is ever lost. Adding a lock file would buy little
// and complicate the CI path.
func WriteArchive(dir string, r Report, skill, judgeModel, frozenVersion string) error {
	if dir == "" {
		return fmt.Errorf("eval.WriteArchive: dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("eval.WriteArchive: mkdir %q: %w", dir, err)
	}

	ts := time.Now().UTC().Format(ArchiveTimestampLayout)
	reportName := ts + ".json"

	if skill == "" {
		skill = r.Skill
	}
	if r.Skill == "" {
		r.Skill = skill
	}

	if err := WriteReport(filepath.Join(dir, reportName), r); err != nil {
		return fmt.Errorf("eval.WriteArchive: %w", err)
	}

	run := IndexRun{
		TS:               ts,
		Skill:            skill,
		JudgeModel:       judgeModel,
		Faithfulness:     r.Overall["faithfulness"],
		ContextPrecision: r.Overall["context_precision"],
		AnswerRelevance:  r.Overall["answer_relevance"],
		ContextRecall:    r.Overall["context_recall"],
		RefusalAccuracy:  RefusalAccuracy(r),
		FrozenVersion:    frozenVersion,
		ReportPath:       reportName,
	}

	indexPath := filepath.Join(dir, IndexFileName)
	index, err := LoadIndex(indexPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("eval.WriteArchive: %w", err)
		}
		index = EvalIndex{Version: IndexVersion}
	}
	if index.Version == 0 {
		index.Version = IndexVersion
	}
	index.Runs = append(index.Runs, run)

	return writeIndexAtomic(indexPath, index)
}

// RefusalAccuracy derives the refusal-accuracy rate from a Report's
// per-category context_recall (see RefusalCategory). Returns 0 when the
// report has no refusal-required cases.
func RefusalAccuracy(r Report) float64 {
	cat, ok := r.PerCategory[RefusalCategory]
	if !ok {
		return 0
	}
	return cat["context_recall"]
}

// LoadIndex reads eval/reports/INDEX.json. A missing file is returned
// as an os.IsNotExist-inspectable error so callers can distinguish
// "no history yet" (initialize an empty index) from "corrupt history"
// (stop and let a human look).
func LoadIndex(path string) (EvalIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return EvalIndex{}, err
		}
		return EvalIndex{}, fmt.Errorf("eval.LoadIndex: open %q: %w", path, err)
	}
	defer f.Close()

	var index EvalIndex
	if err := json.NewDecoder(f).Decode(&index); err != nil {
		return EvalIndex{}, fmt.Errorf("eval.LoadIndex: decode %q: %w", path, err)
	}
	return index, nil
}

// writeIndexAtomic writes the index via tmp-file + rename so a crash
// mid-write cannot leave a truncated INDEX.json — the trend history is
// append-only data whose loss is silent, which is exactly the failure
// mode worth engineering against. os.Rename is atomic on POSIX and, on
// Windows, on the same volume (the tmp file is a sibling by
// construction). The tmp file is removed on any failure so no .tmp
// residue is left behind for the dashboard to trip over.
func writeIndexAtomic(path string, index EvalIndex) error {
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("eval.WriteArchive: marshal index: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("eval.WriteArchive: write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("eval.WriteArchive: rename %q -> %q: %w", tmp, path, err)
	}
	return nil
}
