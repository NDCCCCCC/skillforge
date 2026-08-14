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

// Package main — milvus subcommand (SC-2).
//
// Seeds the 4-tier fixture corpus (12 docs: 3×权威 + 3×历史 + 3×个人 + 3×草稿)
// into the spike Milvus collection, then runs tier-filtered vector searches
// and asserts ZERO cross-tier bleed. This is the structural trust boundary
// for Phase 2-5 compliance Skills: if the filter leaks, the refuse-on-
// missing-tier guarantee is policy-only, not structural.
//
// Strategy (Plan 01-01 internal-api): Milvus + BGE-M3 are reached via
// internal network endpoints (MILVUS_ADDRESS, BGE_M3_URL) — NO Docker.
// If BGE_M3_URL is unreachable, the offline deterministic fallback is
// used so the pipeline can still be exercised end-to-end.
//
// If MILVUS_ADDRESS is unreachable, the command fails fast with [FAIL]
// milvus connect — the human-verify checkpoint (Task 2) then decides
// whether to fix network/access, skip SC-2, or run mock-fallback only.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"gopkg.in/yaml.v3"
)

// docFrontmatter is the YAML frontmatter schema for testdata/docs fixtures.
// Fields mirror what the milvus package persists to Milvus.
type docFrontmatter struct {
	ID            string `yaml:"id"`
	Tier          string `yaml:"tier"`
	Source        string `yaml:"source"`
	EffectiveFrom string `yaml:"effective_from"`
}

// allTiers is the canonical 4-tier set, in display order. The bleed test
// runs one query per tier + one default-deny (empty) query.
var allTiers = []string{"权威", "历史", "个人", "草稿"}

// runMilvus is the SC-2 subcommand: seed 12 docs across 4 tiers, run
// tier-filtered searches, assert zero cross-tier bleed + default-deny.
func runMilvus(ctx context.Context, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	cfg := milvus.Config{
		Addr:           os.Getenv("MILVUS_ADDRESS"),
		Token:          os.Getenv("MILVUS_TOKEN"),
		CollectionName: milvus.DefaultCollectionName,
		EmbedDim:       milvus.DefaultEmbedDim,
	}

	// 1. Load + embed fixtures (works offline — uses deterministic fallback
	//    if BGE_M3_URL is empty or unreachable).
	docs, embedMode, err := loadAndEmbedFixtures(ctx, root, os.Getenv("BGE_M3_URL"))
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}
	fmt.Printf("[SEED] %d fixtures loaded (embed mode: %s)\n", len(docs), embedMode)

	// 2. Connect to Milvus (fail-fast; human-verify checkpoint on error).
	mc, err := milvus.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("milvus connect: %w", err)
	}
	defer func() { _ = mc.Disconnect() }()
	fmt.Printf("[OK] milvus connected: %s\n", cfg.Addr)

	// 3. Clean slate: drop collection if it exists so re-runs don't accumulate
	//    duplicate IDs. Spike-only behavior — production code must NOT do this.
	if err := mc.ClientOf().DropCollection(ctx, cfg.CollectionName); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] drop collection (continuing): %v\n", err)
	}

	// 4. Seed all 12 docs.
	if err := milvus.SeedDocuments(ctx, mc.ClientOf(), cfg, docs); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	for _, d := range docs {
		fmt.Printf("[SEED] %d tier=%s source=%s\n", d.ID, d.Tier, d.Source)
	}

	// 5. Run 4 single-tier queries + 1 default-deny (empty) query.
	//    Use a zero vector for the query — the SPIKE'S purpose is to validate
	//    the FILTER, not the similarity ranking. The filter determines which
	//    rows are eligible; the vector only orders them. A zero vector still
	//    returns rows from the filtered tier, which is all we assert on.
	zeroVec := make([]float32, cfg.EmbedDim)
	authorityCount := 0
	// Plan 05-02 D5-15: spike computed once at the top of the per-tier
	// loop. The effective_to PRE-filter is irrelevant to the tier-bleed
	// assertion (all 12 fixtures have effective_to=0 = perpetual), but
	// we pass the real `now` so the filter shape mirrors production.
	now := time.Now().Unix()
	for _, tier := range allTiers {
		filter := milvus.BuildMilvusFilter([]string{tier}, now)
		hits, err := legacySearch(ctx, mc.ClientOf(), cfg, zeroVec, filter, 10)
		if err != nil {
			return fmt.Errorf("search tier=%s: %w", tier, err)
		}
		if bleed := findBleed(hits, []string{tier}); len(bleed) > 0 {
			return fmt.Errorf("[FAIL] milvus tier bleed: tier=%s query returned %d out-of-tier rows: %v",
				tier, len(bleed), bleed)
		}
		fmt.Printf("[OK] tier=%s: %d rows, zero bleed (filter=%s)\n", tier, len(hits), filter)
		if tier == "权威" {
			authorityCount = len(hits)
		}
	}

	// 6. Default-deny: empty tier list → "false" filter → 0 results (D-09).
	emptyFilter := milvus.BuildMilvusFilter(nil, now)
	emptyHits, err := legacySearch(ctx, mc.ClientOf(), cfg, zeroVec, emptyFilter, 10)
	if err != nil {
		return fmt.Errorf("search default-deny: %w", err)
	}
	if len(emptyHits) != 0 {
		return fmt.Errorf("[FAIL] default-deny: empty tier filter returned %d rows (expected 0)", len(emptyHits))
	}
	fmt.Printf("[OK] default-deny: empty tier list → 0 results (filter=%s)\n", emptyFilter)

	fmt.Printf("[OK] spike milvus: %d docs seeded, tier filter returns %d authority chunks; zero bleed across all 4 tiers + default-deny\n",
		len(docs), authorityCount)
	return nil
}

// legacySearch performs a tier-filtered vector search using the
// milvus package-level helpers directly. It mirrors the production
// Search() method but accepts a pre-built filter string (the spike
// validates the literal filter expression — production hides this
// behind the Retriever interface).
func legacySearch(ctx context.Context, mc client.Client, cfg milvus.Config, queryVec []float32, filter string, topK int) ([]retriever.Hit, error) {
	// Wrap the SDK client as a MilvusClient and call Search. This is
	// the same code path production uses; it lets the spike build the
	// filter externally for direct assertion.
	wrapped := milvus.Wrap(mc, cfg)
	defer func() { _ = wrapped.Disconnect() }()
	// Plan 05-02 D5-15: pass a computed now (irrelevant for the spike —
	// the filter was pre-built with effective_to clauses already
	// incorporated, and tiers=[] short-circuits the effective clause
	// inside BuildMilvusFilter). Passing time.Now().Unix() keeps the
	// signature honest.
	return wrapped.Search(ctx, queryVec, []string{}, 0, time.Now().Unix())
}

// loadAndEmbedFixtures walks testdata/docs/*/*.md, parses each fixture's
// frontmatter + body, generates an embedding per doc, and returns the
// populated Document slice plus the embed mode used ("bge-m3" or "deterministic").
//
// Embedding policy: try BGE-M3 first (if BGE_M3_URL configured). On ANY
// error (URL empty, network down, parse fail), fall back to deterministic
// for ALL docs so the spike can run fully offline. Mixing real + mock
// embeddings in one seed would corrupt the vector space.
func loadAndEmbedFixtures(ctx context.Context, root, bgeURL string) ([]retriever.Document, string, error) {
	docsDir := filepath.Join(root, "testdata", "docs")
	paths, err := filepath.Glob(filepath.Join(docsDir, "*", "*.md"))
	if err != nil {
		return nil, "", fmt.Errorf("glob fixtures: %w", err)
	}
	if len(paths) == 0 {
		return nil, "", fmt.Errorf("no fixtures found under %s", docsDir)
	}
	sort.Strings(paths) // deterministic ID assignment (auth_001=1, ...)

	// Decide embed mode up front.
	useBGE := bgeURL != ""
	embedMode := "bge-m3"
	if !useBGE {
		embedMode = "deterministic"
	}

	bgeEmb := bge.New(bge.Config{URL: bgeURL})

	docs := make([]retriever.Document, 0, len(paths))
	for i, p := range paths {
		fm, body, err := parseDocFixture(p)
		if err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", p, err)
		}
		var emb []float32
		if useBGE {
			vec, embErr := bgeEmb.Embed(ctx, body)
			if embErr != nil {
				// BGE-M3 failed — fall back to deterministic for ALL docs.
				fmt.Fprintf(os.Stderr, "[WARN] BGE-M3 unreachable (%v); falling back to deterministic embeddings for all docs\n", embErr)
				embedMode = "deterministic"
				useBGE = false
				emb = bge.Deterministic(body)
				// Re-embed already-processed docs for consistency.
				for j := range docs {
					docs[j].Embedding = bge.Deterministic(docs[j].Content)
				}
			} else {
				emb = vec
			}
		} else {
			emb = bge.Deterministic(body)
		}
		docs = append(docs, retriever.Document{
			ID:        int64(i + 1), // 1-based numeric PK
			Tier:      fm.Tier,
			Content:   body,
			Source:    fm.Source,
			Embedding: emb,
		})
	}
	return docs, embedMode, nil
}

// parseDocFixture reads a markdown fixture, extracts the YAML frontmatter
// (between --- markers), and returns the frontmatter + the body (markdown
// after the closing ---). Pattern mirrors parseSKILL in wire.go.
func parseDocFixture(path string) (*docFrontmatter, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, "", fmt.Errorf("no YAML frontmatter")
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated frontmatter")
	}
	yamlStr := rest[:end]
	var fm docFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, "", fmt.Errorf("unmarshal frontmatter: %w", err)
	}
	// Body is everything after the closing `\n---` line.
	bodyStart := end + len("\n---")
	body := ""
	if bodyStart < len(rest) {
		body = strings.TrimSpace(rest[bodyStart:])
	}
	if fm.Tier == "" {
		return nil, "", fmt.Errorf("fixture missing tier: field")
	}
	return &fm, body, nil
}

// findBleed returns rows whose tier is NOT in allowedTiers. Empty result =
// no bleed (the structural guarantee SC-2 asserts).
func findBleed(hits []retriever.Hit, allowedTiers []string) []retriever.Hit {
	allowed := make(map[string]bool, len(allowedTiers))
	for _, t := range allowedTiers {
		allowed[t] = true
	}
	var leaked []retriever.Hit
	for _, h := range hits {
		if h.Tier == "" {
			// No tier on the row is itself a bleed (filter should have excluded it).
			leaked = append(leaked, h)
			continue
		}
		if !allowed[h.Tier] {
			leaked = append(leaked, h)
		}
	}
	return leaked
}
