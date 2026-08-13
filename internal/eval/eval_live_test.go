//go:build liveeval

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

// This file is the JITTER layer of the SC-6 reproducibility gate
// (D6-15). It is behind the `liveeval` build tag ON PURPOSE.
//
// Pitfall 3 counter-measure: this test makes REAL GLM API calls. If it
// ran in the default suite, every `go test ./...` — every local save,
// every unrelated CI job — would burn quota and take minutes. The tag
// means the file is not even compiled unless explicitly requested:
//
//	go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility
//
// It is a PHASE-GATE check (run once with GLM_API_KEY set before
// /gsd:verify-work), not a per-commit or per-wave check.
//
// The build tag MUST stay at the top of a SEPARATE file: Go requires
// build constraints to precede the package clause, so this cannot live
// mid-file inside eval_test.go.

package eval

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunnerLiveReproducibility runs the frozen compliance set through
// a REAL judge twice and asserts the scores agree within
// LiveReproducibilityTolerance (1e-3).
//
// What this proves that the strict layer cannot: the system is stable
// in PRACTICE. The strict layer stubs the judge, so it can only prove
// our arithmetic is deterministic — it says nothing about whether a
// real LLM judge returns comparable numbers run over run. That second
// property is the one SC-6 actually promises to an operator, and it is
// the one a temperature regression or a silent model swap would break.
//
// Skips (rather than fails) when GLM_API_KEY is unset so a contributor
// who opts into the tag without credentials gets a clear skip instead
// of a confusing auth error.
func TestRunnerLiveReproducibility(t *testing.T) {
	if os.Getenv("GLM_API_KEY") == "" {
		t.Skip("GLM_API_KEY unset — the live reproducibility layer needs a real judge")
	}

	ctx := context.Background()

	// The frozen set is the pinned baseline (D6-13). Path is relative to
	// this package dir: internal/eval → repo root → eval/frozen/.
	setPath := filepath.Join("..", "..", frozenDirFromPackage, "compliance.jsonl")
	cases, err := LoadGolden(setPath)
	require.NoError(t, err, "load frozen set %s", setPath)

	// Assert the pin before spending any tokens: scores produced under a
	// drifted model or prompt are not comparable to the baseline, so
	// there is nothing to learn from running them.
	lock, err := LoadLock(filepath.Join("..", "..", frozenDirFromPackage, "compliance.lock.json"))
	require.NoError(t, err)
	require.Equal(t, lock.ModelVersion, ActiveModelVersion(),
		"live model must match the pinned model_ver (set JUDGE_MODEL=%s)", lock.ModelVersion)

	// Cap the case count: this layer proves stability, not coverage, and
	// 50 live cases x 2 runs x 4 metrics is a lot of quota for a
	// property that shows up in the first handful.
	if len(cases) > liveCaseCap {
		cases = cases[:liveCaseCap]
	}

	samples := buildFrozenSamples(cases)

	cm, err := NewGLMChatModel(ctx)
	require.NoError(t, err)
	judge := NewJudge(cm)
	emb := newDeterministicEmbedder()

	run1 := computeAllMetrics(ctx, t, samples, judge, emb)
	run2 := computeAllMetrics(ctx, t, samples, judge, emb)

	for i, s := range samples {
		for _, m := range []string{"faith", "arel", "cprec", "crecall"} {
			v1 := run1[i][m].(float64)
			v2 := run2[i][m].(float64)
			assert.True(t, math.Abs(v1-v2) < LiveReproducibilityTolerance,
				"case %s metric %s: live drift %v exceeds %v (run1=%v, run2=%v)",
				s.ID, m, math.Abs(v1-v2), LiveReproducibilityTolerance, v1, v2)
		}
	}
}

const (
	// frozenDirFromPackage mirrors cmd/eval's frozenDir constant. It is
	// duplicated rather than imported because cmd/eval is package main.
	frozenDirFromPackage = "eval/frozen"

	// liveCaseCap bounds the quota this test can consume in one run.
	liveCaseCap = 5
)
