// Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
//
// retrieve_probe: one-off diagnostic. Bypasses the LLM/agent entirely and
// drives the PRODUCTION bge embedder + milvus retriever directly, to answer
// "for query X, what does retrieval actually return and at what score?".
//
// Distinguishes two failure classes the /api/chat path conflates:
//   - LLM query-rewrite drift  (agent rewrites 年假 -> 出差费用; code-forced
//     retrieval with the ORIGINAL query would fix it)
//   - embedding discrimination (even the original query 年假 doesn't rank
//     annual_leave.md near the top; needs embedder/index work, not prompting)
//
// Usage (same env as cmd/server; NO GLM_API_KEY needed — retrieval only):
//
//	MILVUS_ADDRESS=your-milvus-host:19530 \
//	BGE_M3_URL=http://your-bge-host:11434/v1/embeddings \
//	BGE_M3_MODEL=bge-m3 \
//	go run ./scripts/retrieve_probe
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
)

func main() {
	ctx := context.Background()

	bgeURL := os.Getenv("BGE_M3_URL")
	milvusAddr := os.Getenv("MILVUS_ADDRESS")
	bgeModel := os.Getenv("BGE_M3_MODEL")
	if bgeModel == "" {
		bgeModel = "bge-m3"
	}
	if bgeURL == "" || milvusAddr == "" {
		log.Fatal("BGE_M3_URL and MILVUS_ADDRESS must be set")
	}

	emb := bge.New(bge.Config{URL: bgeURL, Model: bgeModel, Timeout: 60 * time.Second})

	mc, err := milvus.New(ctx, milvus.Config{Addr: milvusAddr, CollectionName: "skillforge_knowledge"})
	if err != nil {
		log.Fatalf("milvus.New: %v", err)
	}
	if err := mc.Ensure(ctx); err != nil {
		log.Fatalf("milvus.Ensure: %v", err)
	}

	// Three probes: a should-hit (annual_leave), a known-good (travel_expense),
	// and a should-miss control (quantum mechanics). Read-only — no re-seed.
	queries := []string{"年假有几天", "年假", "出差住宿费超过500元合规吗", "量子力学原理"}
	now := time.Now().Unix()

	for _, q := range queries {
		vec, err := emb.Embed(ctx, q)
		if err != nil {
			log.Printf("[%s] embed FAILED: %v", q, err)
			continue
		}
		hits, err := mc.Search(ctx, vec, []string{"权威"}, 3, now)
		if err != nil {
			log.Printf("[%s] search FAILED: %v", q, err)
			continue
		}
		fmt.Printf("\n=== query=%q  authority-tier top-3 (hits=%d) ===\n", q, len(hits))
		for i, h := range hits {
			c := h.Content
			if len([]rune(c)) > 70 {
				c = string([]rune(c)[:70]) + "…"
			}
			fmt.Printf("  #%d  score=%.4f  source=%s\n      %s\n", i, h.Score, h.Source, c)
		}
	}
}
