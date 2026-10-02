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
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TestLiveLLMExtractionF1 is the SC-2 LIVE half (42-03): the production
// LLMExtractor driven by a real GLM chat model scores against the same
// D-GT-01 golden set the hermetic stub harness (extract_golden_test.go)
// uses. The 42-operator-bootstrap.sh L1 gate greps this test's
// "F1=" output and requires macro tables F1 >= 0.8.
//
// Env gate: SKILLFORGE_LIVE_F1=1 makes this test run live; without it the
// test skips so the hermetic suite (CI, `go test ./internal/...`) stays
// green with zero network dependency. GLM_API_KEY must also be set.
//
// Endpoint resolution is GLM-ONLY (GLM_API_KEY / GLM_BASE_URL / GLM_MODEL,
// model default glm-5.3-flash) — deliberately NOT eval.ResolveLLM, whose
// LOCAL_* precedence would silently route this measurement at the dead
// self-hosted endpoint in envs where LOCAL_* lingers in .env.
//
// Scoring convention: identical to the stub harness — agreeOrScore treats
// joint emptiness (read-only notes where extractor and human golden both
// emit nothing) as true-negative agreement (1,1,1), while the underlying
// ScoreTables/ScoreRelated keep f1.go's pinned zero-denominator→0.0
// convention (any LLM hallucination on a read-only note scores 0). Using
// raw ScoreTables here would make the gate mathematically unreachable:
// 22 of 26 golden rows are writes-only with empty tables, so two-sided
// empties would score 0 for CORRECT behavior.
func TestLiveLLMExtractionF1(t *testing.T) {
	if os.Getenv("SKILLFORGE_LIVE_F1") != "1" {
		t.Skip("live F1 gate: set SKILLFORGE_LIVE_F1=1 with LLM env")
	}
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		t.Skip("live F1 gate: GLM_API_KEY not set")
	}
	baseURL := os.Getenv("GLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	}
	modelName := os.Getenv("GLM_MODEL")
	if modelName == "" {
		modelName = "glm-5.3-flash"
	}

	golden, err := LoadGolden(goldenPath)
	if err != nil {
		t.Fatalf("golden set is REQUIRED by the SC-2 live gate: %v", err)
	}
	notes, _, err := WalkAndMerge(testCorpusDir)
	if err != nil {
		t.Fatalf("WalkAndMerge: %v", err)
	}
	byIdx := make(map[string]Note, len(notes))
	for _, n := range notes {
		byIdx[n.SourceIdx] = n
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	// Per-call timeout is load-bearing: the GLM Coding Plan endpoint
	// occasionally wedges an HTTP/2 stream indefinitely (observed in the
	// quick-261001-cz7 live runs — the default here is NO timeout, so one
	// wedged call froze the whole measurement). A wedged call becomes a
	// retriable error, not a frozen run.
	const perCallTimeout = 2 * time.Minute
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   modelName,
		Timeout: perCallTimeout,
	})
	if err != nil {
		t.Fatalf("live F1 gate: chat model construction (%s@%s): %v", modelName, baseURL, err)
	}
	ext := NewLLMExtractor(cm)

	// Pre-flight probe (env-intranet-service-flakiness: probe before live;
	// a disconnected endpoint is an env-STOP skip, not a quality FAIL —
	// the operator re-runs when the service is back).
	probeCtx, probeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer probeCancel()
	if _, err := cm.Generate(probeCtx, []*schema.Message{
		schema.UserMessage("ping — reply with the single word: pong"),
	}, model.WithTemperature(0.0)); err != nil {
		t.Skipf("live F1 gate: env-STOP — LLM endpoint unreachable (%s@%s): %v", modelName, baseURL, err)
	}

	var tableScores, relatedScores []NoteScore
	for _, row := range golden {
		note, ok := byIdx[row.SourceIdx]
		if !ok {
			t.Fatalf("golden row %q has no matching corpus note (corpus drift)", row.SourceIdx)
		}
		pred, err := ext.ExtractNote(ctx, note)
		if err != nil && ctx.Err() == nil {
			// One retry per note: a per-call timeout / transient 5xx is
			// infra flake, not extractor quality — retry once so the
			// measurement measures the extractor, not the network.
			t.Logf("note %s: extraction attempt failed (%v) — retrying once", row.SourceIdx, err)
			pred, err = ext.ExtractNote(ctx, note)
		}
		if err != nil {
			// A live extraction error scores as a zero row (honest: the
			// gate measures what actually happened) and is logged — but
			// does not abort the remaining notes.
			t.Logf("note %s: extraction error (%v) — scoring 0", row.SourceIdx, err)
			tableScores = append(tableScores, NoteScore{})
			relatedScores = append(relatedScores, NoteScore{})
			continue
		}

		ts := agreeOrScore(len(pred.Tables), len(row.Tables), func() NoteScore {
			p, r, f1 := ScoreTables(pred.Tables, row.Tables)
			return NoteScore{P: p, R: r, F1: f1}
		})
		tableScores = append(tableScores, ts)

		rs := agreeOrScore(len(pred.RelatedNotes), len(row.Related), func() NoteScore {
			p, r, f1 := ScoreRelated(pred.RelatedNotes, row.Related)
			return NoteScore{P: p, R: r, F1: f1}
		})
		relatedScores = append(relatedScores, rs)

		if ts.F1 < 1.0 {
			// Lowercase "f1=" on per-note lines: the bootstrap L1 gate
			// greps the FIRST "F1=" token as THE macro score, so the
			// per-note transparency log must not emit that token.
			t.Logf("note %s: tables P=%.4f R=%.4f f1=%.4f (live %d tables vs golden %d)",
				row.SourceIdx, ts.P, ts.R, ts.F1, len(pred.Tables), len(row.Tables))
		}
	}

	tp, tr, tf1 := MacroF1(tableScores)
	rp, rr, rf1 := MacroF1(relatedScores)
	// "F1=" token is the 42-operator-bootstrap.sh L1 grep contract — keep.
	t.Logf("SC-2 live macro-F1 vs reviewed golden (%d notes, model=%s): tables P=%.4f R=%.4f F1=%.4f | related P=%.4f R=%.4f F1=%.4f",
		len(golden), modelName, tp, tr, tf1, rp, rr, rf1)
	t.Logf("F1=%.4f", tf1)

	if tf1 < harnessF1Threshold {
		t.Fatalf("live tables macro-F1 = %.4f < %.2f gate (SC-2 live half) — inspect the per-note log above", tf1, harnessF1Threshold)
	}
	if rf1 < harnessF1Threshold {
		t.Fatalf("live related macro-F1 = %.4f < %.2f gate (SC-2 live half)", rf1, harnessF1Threshold)
	}
}
