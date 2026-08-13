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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archiveReport builds a Report with the given overall faithfulness and
// a refusal-required category whose context_recall (= refusal accuracy)
// is refusalAcc.
func archiveReport(skill string, faithfulness, refusalAcc float64) Report {
	return Report{
		Skill: skill,
		Overall: map[string]float64{
			"faithfulness":      faithfulness,
			"context_precision": 0.85,
			"answer_relevance":  0.88,
			"context_recall":    0.79,
		},
		PerCategory: map[string]map[string]float64{
			RefusalCategory: {"context_recall": refusalAcc},
		},
		Cases: []CaseResult{{ID: "compliance-001", Category: RefusalCategory}},
	}
}

// TestWriteArchive_CreatesIndexAndReport: the first archive call must
// create BOTH artifacts — the full report copy and the trend index.
func TestWriteArchive_CreatesIndexAndReport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reports") // not pre-created: WriteArchive must mkdir
	r := archiveReport("compliance", 0.92, 0.95)

	require.NoError(t, WriteArchive(dir, r, "compliance", "glm-5.2", "compliance-2026-08-13-v1"))

	index, err := LoadIndex(filepath.Join(dir, IndexFileName))
	require.NoError(t, err)
	assert.Equal(t, IndexVersion, index.Version)
	require.Len(t, index.Runs, 1)

	run := index.Runs[0]
	reportPath := filepath.Join(dir, run.ReportPath)
	require.FileExists(t, reportPath, "the full report copy must be archived alongside the index")

	archived, err := LoadReport(reportPath)
	require.NoError(t, err)
	assert.Equal(t, r.Overall, archived.Overall, "the archived copy is the FULL report")
	assert.Equal(t, "compliance", archived.Skill)
}

// TestWriteArchive_AppendsAtomically: a second archive call appends
// without corrupting the first entry, and leaves no .tmp residue behind
// (the atomic tmp+rename must complete, not litter).
func TestWriteArchive_AppendsAtomically(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, WriteArchive(dir, archiveReport("compliance", 0.92, 0.95), "compliance", "glm-5.2", "v1"))
	require.NoError(t, WriteArchive(dir, archiveReport("compliance", 0.81, 0.90), "compliance", "deepseek-chat", "v1"))

	index, err := LoadIndex(filepath.Join(dir, IndexFileName))
	require.NoError(t, err)
	require.Len(t, index.Runs, 2, "the second run appends; it does not replace")

	assert.InDelta(t, 0.92, index.Runs[0].Faithfulness, 1e-9, "the first entry survives intact")
	assert.Equal(t, "glm-5.2", index.Runs[0].JudgeModel)
	assert.InDelta(t, 0.81, index.Runs[1].Faithfulness, 1e-9)
	assert.Equal(t, "deepseek-chat", index.Runs[1].JudgeModel)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "atomic write must leave no .tmp residue")
	}
}

// TestWriteArchive_IndexRunShape pins all 10 INDEX.json fields — the
// dashboard's trend line and the Phase 7 README badge both read this
// shape, so a silent field drop would break consumers that are not in
// this repo yet.
func TestWriteArchive_IndexRunShape(t *testing.T) {
	dir := t.TempDir()
	r := archiveReport("compliance", 0.92, 0.95)

	require.NoError(t, WriteArchive(dir, r, "compliance", "glm-5.2", "compliance-2026-08-13-v1"))

	index, err := LoadIndex(filepath.Join(dir, IndexFileName))
	require.NoError(t, err)
	require.Len(t, index.Runs, 1)
	run := index.Runs[0]

	assert.NotEmpty(t, run.TS)
	assert.NotContains(t, run.TS, ":", "the timestamp must be filename-safe (Windows rejects ':')")
	assert.Equal(t, "compliance", run.Skill)
	assert.Equal(t, "glm-5.2", run.JudgeModel)
	assert.InDelta(t, 0.92, run.Faithfulness, 1e-9)
	assert.InDelta(t, 0.85, run.ContextPrecision, 1e-9)
	assert.InDelta(t, 0.88, run.AnswerRelevance, 1e-9)
	assert.InDelta(t, 0.79, run.ContextRecall, 1e-9)
	assert.InDelta(t, 0.95, run.RefusalAccuracy, 1e-9,
		"refusal_accuracy is DERIVED from per_category['refusal-required']['context_recall'] — no new metric")
	assert.Equal(t, "compliance-2026-08-13-v1", run.FrozenVersion)
	assert.Equal(t, run.TS+".json", run.ReportPath)
}

// TestRefusalAccuracy_MissingCategory: a report with no
// refusal-required cases scores 0 rather than panicking on a nil map.
func TestRefusalAccuracy_MissingCategory(t *testing.T) {
	assert.Equal(t, 0.0, RefusalAccuracy(Report{}))
	assert.Equal(t, 0.0, RefusalAccuracy(Report{PerCategory: map[string]map[string]float64{}}))
}

// TestWriteArchive_EmptyDirRejected: an empty dir is a caller bug, not
// a silent write to the process CWD.
func TestWriteArchive_EmptyDirRejected(t *testing.T) {
	err := WriteArchive("", archiveReport("compliance", 0.9, 0.9), "compliance", "glm-5.2", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dir is empty")
}

// TestLoadIndex_MissingIsNotExist: a missing INDEX.json must be
// distinguishable from a corrupt one — WriteArchive initializes an
// empty index for the former and refuses to proceed on the latter.
func TestLoadIndex_MissingIsNotExist(t *testing.T) {
	_, err := LoadIndex(filepath.Join(t.TempDir(), IndexFileName))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err), "a missing index must be os.IsNotExist-inspectable")
}

// TestLoadIndex_Corrupt: malformed history surfaces a decode error
// instead of being silently reset (which would erase trend history).
func TestLoadIndex_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, IndexFileName)
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"runs":`), 0o644))

	_, err := LoadIndex(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eval.LoadIndex: decode")

	err = WriteArchive(dir, archiveReport("compliance", 0.9, 0.9), "compliance", "glm-5.2", "v1")
	require.Error(t, err, "a corrupt index must stop the archive, not overwrite the history")
}
