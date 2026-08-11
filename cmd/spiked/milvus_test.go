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

// Package main — milvus subcommand tests.
//
// TestBuildMilvusFilter is a fast (<1s) smoke test that runs on every
// commit. It re-checks the spike.BuildMilvusFilter contract from the cmd
// package's perspective (verifying the import works + the exported symbol
// produces the exact filter strings SC-2 depends on).
//
// TestSeedAndFilterIntegration is the SC-2 integration test. It is gated
// behind MILVUS_INTEGRATION=1 because it requires a live Milvus instance
// (internal-api strategy: set MILVUS_ADDRESS to your internal cluster).
// When the env var is unset, the test skips so the quick-run path stays
// green without network access. See SUMMARY.md for the human-verify gate.
package main

import (
	"context"
	"os"
	"testing"

	"github.com/skillforge/skillforge/internal/spike"
	"github.com/stretchr/testify/assert"
)

// TestBuildMilvusFilter re-asserts the BuildMilvusFilter contract from the
// cmd package. The canonical table-driven suite lives in
// internal/spike/milvus_client_test.go; this is a thin smoke test that
// catches import-wiring regressions and verifies the few cases the bleed
// assertion in runMilvus depends on.
func TestBuildMilvusFilter(t *testing.T) {
	// Default-deny: nil/empty → "false" (D-09 structural enforcement).
	assert.Equal(t, "false", spike.BuildMilvusFilter(nil))
	assert.Equal(t, "false", spike.BuildMilvusFilter([]string{}))

	// Single CN tier — exact bytes (the filter SC-2 asserts on).
	assert.Equal(t, `tier in ["权威"]`, spike.BuildMilvusFilter([]string{"权威"}))
	assert.Equal(t, `tier in ["历史"]`, spike.BuildMilvusFilter([]string{"历史"}))
	assert.Equal(t, `tier in ["个人"]`, spike.BuildMilvusFilter([]string{"个人"}))
	assert.Equal(t, `tier in ["草稿"]`, spike.BuildMilvusFilter([]string{"草稿"}))

	// Multi-tier ordering preserved.
	assert.Equal(t, `tier in ["权威","历史"]`,
		spike.BuildMilvusFilter([]string{"权威", "历史"}))

	// Hostile characters do not crash (Pitfall 7 — injection safety).
	assert.NotPanics(t, func() {
		_ = spike.BuildMilvusFilter([]string{"O'Brien"})
	})
}

// TestSeedAndFilterIntegration is the SC-2 integration test. It connects to
// a live Milvus instance (MILVUS_ADDRESS), seeds the 12-fixture corpus,
// runs 4 single-tier + 1 default-deny query, and asserts zero cross-tier
// bleed. Skipped unless MILVUS_INTEGRATION=1 is set.
//
// To run: set MILVUS_ADDRESS, BGE_M3_URL (optional — deterministic fallback
// used if unset), then:
//
//	MILVUS_INTEGRATION=1 go test ./cmd/spiked -run TestSeedAndFilterIntegration -v -count=1 -timeout 60s
func TestSeedAndFilterIntegration(t *testing.T) {
	if os.Getenv("MILVUS_INTEGRATION") != "1" {
		t.Skip("set MILVUS_INTEGRATION=1 + MILVUS_ADDRESS to run live Milvus tier-bleed test")
	}

	addr := os.Getenv("MILVUS_ADDRESS")
	if addr == "" {
		t.Fatal("MILVUS_INTEGRATION=1 but MILVUS_ADDRESS is empty — set MILVUS_ADDRESS to your internal Milvus gRPC endpoint")
	}

	ctx := context.Background()
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	cfg := spike.MilvusConfig{
		Addr:           addr,
		Token:          os.Getenv("MILVUS_TOKEN"),
		CollectionName: spike.DefaultCollectionName,
		EmbedDim:       spike.DefaultEmbedDim,
		BGEURL:         os.Getenv("BGE_M3_URL"),
	}

	// Load + embed fixtures (deterministic fallback if BGE_M3_URL empty).
	docs, embedMode, err := loadAndEmbedFixtures(ctx, root, cfg.BGEURL)
	if err != nil {
		t.Fatalf("loadAndEmbedFixtures: %v", err)
	}
	assert.Len(t, docs, 12, "expected 12 fixtures (3 per tier × 4 tiers)")
	t.Logf("embed mode: %s", embedMode)

	// Connect to Milvus.
	mc, err := spike.NewMilvusClient(ctx, cfg)
	if err != nil {
		t.Fatalf("NewMilvusClient: %v", err)
	}
	defer mc.Close()

	// Clean slate.
	_ = mc.DropCollection(ctx, cfg.CollectionName)

	// Seed.
	if err := spike.SeedDocuments(ctx, mc, cfg, docs); err != nil {
		t.Fatalf("SeedDocuments: %v", err)
	}

	// Run 4 single-tier queries — assert zero bleed.
	zeroVec := make([]float32, cfg.EmbedDim)
	for _, tier := range []string{"权威", "历史", "个人", "草稿"} {
		filter := spike.BuildMilvusFilter([]string{tier})
		hits, err := spike.SearchWithTierFilter(ctx, mc, cfg, zeroVec, filter, 10)
		assert.NoError(t, err, "search tier=%s", tier)
		assert.NotEmpty(t, hits, "tier=%s: expected at least 1 row from 3 seeded docs", tier)
		bleed := findBleed(hits, []string{tier})
		assert.Empty(t, bleed, "tier=%s: cross-tier bleed detected (filter=%s)", tier, filter)
		for _, h := range hits {
			assert.Equal(t, tier, h.Tier, "tier=%s: result row has wrong tier", tier)
		}
		t.Logf("tier=%s: %d rows, zero bleed", tier, len(hits))
	}

	// Default-deny: empty tier list → 0 results.
	emptyFilter := spike.BuildMilvusFilter(nil)
	emptyHits, err := spike.SearchWithTierFilter(ctx, mc, cfg, zeroVec, emptyFilter, 10)
	assert.NoError(t, err, "default-deny search")
	assert.Empty(t, emptyHits, "default-deny: empty tier list must return 0 rows (D-09), filter=%s", emptyFilter)
}
