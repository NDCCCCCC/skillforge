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

package server

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
)

// recallCase is one live recall probe: a natural-language query with the
// title fragment of the sql-guides note that SHOULD appear in the top-5.
type recallCase struct {
	query      string
	targetFrag string // contains-match against hit text OR source path
}

// recallCases is the Phase 41 recall probe set — every query names an
// unambiguous target document in the sql-guides corpus. The contains
// match tolerates title renames (the fragment appears verbatim in the
// source file name too).
var recallCases = []recallCase{
	{query: "根据保单号查询支付方式和开户行名称", targetFrag: "支付方式"},
	{query: "三责险和车损险的保费保额取数口径", targetFrag: "三责险"},
	{query: "查询指定保单号的未决已决赔款", targetFrag: "已决赔款"},
	{query: "Linux 上怎么启动 FineBI", targetFrag: "FineBI"},
	{query: "新能源车签单口径是什么", targetFrag: "新能源车签单口径"},
	{query: "承保清单的取数范围", targetFrag: "承保清单"},
}

// TestHybridSearchRecall is the Phase 41 live recall gate (42-03): run
// HybridSearchScoped over N real queries against the LIVE Milvus +
// BGE-M3 endpoints and report the share of queries whose target document
// lands in the top-5. The 42-operator-bootstrap.sh L5 gate greps
// "recall=" and requires >= 0.85.
//
// Env gate: SKILLFORGE_LIVE_HYBRID=1 (requires live Milvus + BGE). Any
// endpoint connectivity failure is an env-STOP skip, never a FAIL (per
// env-intranet-service-flakiness: 断线记环境 STOP 不刷分).
//
// The KG side is built from the D-GT-01 golden set (human-reviewed
// tables/related) over the real merged corpus — deterministic ground
// truth, no LLM in the loop. Milvus supplies the content hits.
func TestHybridSearchRecall(t *testing.T) {
	if os.Getenv("SKILLFORGE_LIVE_HYBRID") != "1" {
		t.Skip("live hybrid gate: set SKILLFORGE_LIVE_HYBRID=1 with live Milvus+BGE env")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1. KG from golden + corpus (deterministic).
	golden, err := ecl.LoadGolden("../../eval/ecl-golden/sql-guides.jsonl")
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	notes, _, err := ecl.WalkAndMerge("../../knowledge/sql-guides")
	if err != nil {
		t.Fatalf("WalkAndMerge: %v", err)
	}
	byIdx := make(map[string]ecl.Note, len(notes))
	for _, n := range notes {
		byIdx[n.SourceIdx] = n
	}
	const ns = "http://skillforge.local/ontology#"
	kg := store.New()
	for _, row := range golden {
		note, ok := byIdx[row.SourceIdx]
		if !ok {
			t.Fatalf("golden row %q has no corpus note (corpus drift)", row.SourceIdx)
		}
		ex := ecl.Extraction{Tables: row.Tables}
		for _, rel := range row.Related {
			ex.RelatedNotes = append(ex.RelatedNotes, ecl.RelatedNote{SourceIdx: rel})
		}
		for _, tr := range ecl.BuildTriples(ns, note, ex) {
			kg.Insert(tr)
		}
	}

	// 2. Live embedder + retriever (env-STOP on unreachable).
	addr := os.Getenv("MILVUS_ADDRESS")
	if addr == "" {
		t.Skip("live hybrid gate: MILVUS_ADDRESS not set")
	}
	bgeURL := os.Getenv("BGE_M3_URL")
	if bgeURL == "" {
		t.Skip("live hybrid gate: BGE_M3_URL not set")
	}
	emb := bge.New(bge.Config{URL: bgeURL, Model: "bge-m3", Timeout: 60 * time.Second})
	mc, err := milvus.New(ctx, milvus.Config{Addr: addr, CollectionName: "skillforge_knowledge"})
	if err != nil {
		t.Skipf("live hybrid gate: env-STOP — milvus unreachable (%s): %v", addr, err)
	}
	if err := mc.Ensure(ctx); err != nil {
		t.Skipf("live hybrid gate: env-STOP — milvus ensure: %v", err)
	}

	tiers := []string{"sql-guides"}
	if probe, err := emb.Embed(ctx, "连通性探测"); err != nil {
		t.Skipf("live hybrid gate: env-STOP — BGE unreachable (%s): %v", bgeURL, err)
	} else if _, err := mc.Search(ctx, probe, tiers, 1, time.Now().Unix()); err != nil {
		t.Skipf("live hybrid gate: env-STOP — milvus search: %v", err)
	}

	// 3. Run the recall probes.
	hits := 0
	for _, tc := range recallCases {
		resp, err := HybridSearchScoped(HybridQuery{Query: tc.query}, tiers, kg, mc, emb, nil)
		if err != nil {
			t.Skipf("live hybrid gate: env-STOP — hybrid search %q: %v", tc.query, err)
		}
		top5 := resp.MilvusHits
		if len(top5) > 5 {
			top5 = top5[:5]
		}
		found := false
		for _, h := range top5 {
			if strings.Contains(h.Text, tc.targetFrag) || strings.Contains(h.SourcePath, tc.targetFrag) {
				found = true
				break
			}
		}
		t.Logf("query=%q target=%q hit=%v top5_sources=%v", tc.query, tc.targetFrag, found, sourcesOf(top5))
		if found {
			hits++
		}
	}

	r := float64(hits) / float64(len(recallCases))
	// "recall=" token is the 42-operator-bootstrap.sh L5 grep contract — keep.
	t.Logf("recall=%.4f (%d/%d queries hit target in top-5)", r, hits, len(recallCases))

	if r < 0.85 {
		t.Errorf("hybrid recall = %.4f < 0.85 (Phase 41 gate) — inspect per-query log above", r)
	}
}

// sourcesOf renders the top hit source paths for the per-query log.
func sourcesOf(hits []HybridMilvusHit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		p := h.SourcePath
		if i := strings.LastIndexByte(p, '/'); i >= 0 {
			p = p[i+1:]
		}
		out = append(out, p)
	}
	return out
}
