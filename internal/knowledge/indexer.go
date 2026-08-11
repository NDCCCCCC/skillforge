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

// Package knowledge contains the corpus loader that turns a tree of
// hand-authored policy markdown into Milvus Document rows with correct
// tier metadata.
//
// Index is the canonical Pattern 7 (02-RESEARCH.md §lines 576-607)
// implementation: scan knowledge/{authority,historical,personal,draft}/*.md,
// strip YAML frontmatter, embed the body via BGE-M3 (production — NOT
// the deterministic spike-only fallback per Pitfall 8), and insert into
// Milvus with the dir-name's CN-char tier value (resolved through
// internal/skill.TierEnum so the loader cannot drift from the retriever).
//
// One-doc-one-chunk is the Phase 2 minimal model — each .md file becomes
// exactly one Document row. The corpus is hand-authored, each clause is
// 300-800 bytes, well under BGE-M3's 8192-token context window, and
// further splitting would fragment a clause and hurt retrieval precision.
// The hierarchical markdown splitter is deferred to Phase 3 (CMPL-06).
package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// maxDocBytes is a defensive cap on the body size we will embed and insert.
// BGE-M3's embedding quality degrades past roughly 4K characters on CJK
// text (the BGE-M3 8K-token context window is a ceiling, not a sweet spot).
// The Phase 2 hand-authored corpus is well under 1 KB per file — this cap
// is a guardrail against an operator dropping a 100 KB policy PDF into
// knowledge/authority/ and silently degrading retrieval for everyone else.
// When a file exceeds the cap we emit a warning and skip it; the operator
// can either split the doc or opt-in via the splitter in Phase 3.
const maxDocBytes = 4000

// knownTierDirs is the fixed enumeration of tier subdirectories we scan.
// Mirrors internal/skill.TierEnum keys but ordered for deterministic iteration
// (Go map iteration is randomized). Adding a new tier requires adding to
// both this slice AND skill.TierEnum (the loader uses TierEnum for the
// CN-char mapping).
var knownTierDirs = []string{"authority", "historical", "personal", "draft"}

// Index scans knowledgeDir/{authority,historical,personal,draft}/*.md,
// embeds the body of each .md file via BGE-M3 (retriever.EmbedQuery —
// the production HTTP client, NOT EmbedQueryDeterministic which is
// spike-only fallback per Pitfall 8), and seeds the resulting Documents
// into Milvus with the tier metadata derived from the dir name.
//
// Behavior:
//   - Tier directories that do not exist on disk are skipped silently.
//     Empty tier dirs ARE valid: knowledge/personal/ being empty is the
//     SC-4 refusal mechanism — a Skill whose reads=[authority/**] can
//     never query personal/, AND a contact-info query against an empty
//     personal/ returns 0 chunks → no_authorized_source refusal.
//   - .md files without YAML frontmatter are still indexed (the body is
//     the whole file). Production Skill files have frontmatter; corpus
//     policy files typically don't, but the loader is permissive.
//   - Files exceeding maxDocBytes are logged via fmt.Fprintf(os.Stderr, ...)
//     and SKIPPED (not silently truncated — the operator needs to see it).
//   - Documents are assigned numeric IDs starting at 1, monotonically
//     increasing across tier dirs. Re-running on the same collection
//     inserts duplicates (Milvus INT64 PK is not auto-deduped); the
//     startup hook may want to drop+recreate for clean re-seeds.
//
// Returns:
//   - nil on success (collection seeded with N>=0 documents).
//   - wrapped error on Milvus or embed failure. Embed failure is a HARD
//     error in production — we never fall back to EmbedQueryDeterministic
//     (Pitfall 8 / T-2-25): the deterministic fallback exists for cmd/spiked
//     regression only and would silently mask a BGE-M3 outage with
//     semantically meaningless vectors.
//
// knowledgeDir is expected to be the project-relative "knowledge/" path
// but any directory with the standard 4-tier layout works. bgeURL points
// at the internal BGE-M3 service (e.g. http://bge-m3.internal/v1/embeddings).
func Index(ctx context.Context, knowledgeDir string, mc client.Client, cfg retriever.MilvusConfig, bgeURL string) error {
	if knowledgeDir == "" {
		return errors.New("knowledge.Index: knowledgeDir is empty")
	}
	if bgeURL == "" {
		return errors.New("knowledge.Index: bgeURL is empty (Pitfall 8 — production refuses on BGE-M3 outage)")
	}
	// mc==nil is rejected only when the test hook is not active; in test
	// mode the hook intercepts before any Milvus SDK call, so a nil
	// client is acceptable.
	if mc == nil && indexSeedHook == nil {
		return errors.New("knowledge.Index: milvus client is nil")
	}

	var docs []retriever.Document
	nextID := int64(1)

	// Stable order for the tier scan: alphabetical. Go map iteration is
	// randomized; sorting the dir list here makes the IDs assigned in
	// re-seeds line up as much as Milvus allows (no PK auto-dedup, but at
	// least the order is predictable for human inspection).
	sortedTiers := make([]string, len(knownTierDirs))
	copy(sortedTiers, knownTierDirs)
	sort.Strings(sortedTiers)

	for _, tierDir := range sortedTiers {
		tierVal, ok := skill.TierEnum[tierDir]
		if !ok {
			// This would indicate a code-level bug — the two lists drifted.
			// Fail fast; do not silently skip.
			return fmt.Errorf("knowledge.Index: tier %q missing from skill.TierEnum (loader + skill must stay in sync)", tierDir)
		}

		tierAbs := filepath.Join(knowledgeDir, tierDir)
		info, err := os.Stat(tierAbs)
		if err != nil {
			if os.IsNotExist(err) {
				// Empty / missing tier dir IS valid (SC-4 mechanism). Skip silently.
				continue
			}
			return fmt.Errorf("knowledge.Index: stat %s: %w", tierAbs, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("knowledge.Index: %s exists but is not a directory", tierAbs)
		}

		// Glob *.md in this tier dir. Files are processed in lexical order
		// for deterministic nextID assignment across re-runs.
		pattern := filepath.Join(tierAbs, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("knowledge.Index: glob %s: %w", pattern, err)
		}
		sort.Strings(matches)

		for _, path := range matches {
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("knowledge.Index: read %s: %w", path, err)
			}

			body := stripFrontmatter(raw)
			if len(body) == 0 {
				// Empty body (frontmatter-only file or 0-byte file). Skip
				// silently — the operator probably wrote a placeholder.
				continue
			}
			if len(body) > maxDocBytes {
				fmt.Fprintf(os.Stderr,
					"[WARN] knowledge.Index: %s body=%d bytes exceeds cap=%d; skipping. "+
						"Split the doc or wait for the Phase 3 hierarchical splitter (CMPL-06).\n",
					path, len(body), maxDocBytes)
				continue
			}

			// PRODUCTION embed: retriever.EmbedQuery calls the BGE-M3 HTTP
			// endpoint. We never call EmbedQueryDeterministic here — that
			// fallback exists solely for cmd/spiked regression paths and
			// would silently hide a BGE-M3 outage (Pitfall 8).
			vec, err := retriever.EmbedQuery(ctx, bgeURL, string(body))
			if err != nil {
				return fmt.Errorf("knowledge.Index: embed %s: %w", path, err)
			}

			// Source path is the slash-normalized relative path (no
			// knowledgeDir prefix) — keeps the Citation title short and
			// portable. e.g. "authority/travel_expense.md".
			rel, err := filepath.Rel(knowledgeDir, path)
			if err != nil {
				return fmt.Errorf("knowledge.Index: relative path for %s: %w", path, err)
			}
			relSlash := filepath.ToSlash(rel)

			docs = append(docs, retriever.Document{
				ID:        nextID,
				Tier:      tierVal,
				Content:   string(body),
				Source:    relSlash,
				Embedding: vec,
			})
			nextID++
		}
	}

	// Seed in one batched Insert. An empty knowledge/ tree (all tier dirs
	// empty) is valid — SeedDocuments rejects empty input so we skip
	// the call entirely in that case (returning nil = "no corpus to index
	// right now, but startup continues"; the retrieval path will still
	// produce no_authorized_source refusals).
	if len(docs) == 0 {
		fmt.Fprintln(os.Stderr, "[WARN] knowledge.Index: no documents found; Milvus collection left empty (every query → no_authorized_source refusal)")
		return nil
	}

	if err := seedDocs(ctx, mc, cfg, docs); err != nil {
		return fmt.Errorf("knowledge.Index: seed %d docs into %s: %w", len(docs), cfg.CollectionName, err)
	}
	return nil
}

// seedDocs is the package-internal indirection point that tests use to
// intercept the Milvus write without standing up a real Milvus server.
//
// Production behavior (indexSeedHook == nil): delegate to
// retriever.SeedDocuments verbatim.
//
// Test behavior (indexSeedHook != nil): invoke the hook with the docs;
// the hook returns the error that production would have surfaced. This
// keeps the production code path identical to the test path (no mock
// branch inside Index) while letting unit tests assert on the docs
// that would have been inserted.
var indexSeedHook func(ctx context.Context, mc client.Client, cfg retriever.MilvusConfig, docs []retriever.Document) error

func seedDocs(ctx context.Context, mc client.Client, cfg retriever.MilvusConfig, docs []retriever.Document) error {
	if indexSeedHook != nil {
		return indexSeedHook(ctx, mc, cfg, docs)
	}
	return retriever.SeedDocuments(ctx, mc, cfg, docs)
}

// stripFrontmatter extracts the markdown body after the YAML frontmatter
// block (delimited by --- lines). Returns raw unchanged when no
// frontmatter is present.
//
// Pattern adapted from internal/skill/loader.go:parseFrontmatter — the
// skill loader is strict (opening --- must be on line 1, missing opener
// is an error). The knowledge indexer is permissive: most corpus files
// do NOT have frontmatter; rejecting them would break every Phase 2
// corpus file. The frontmatter IS stripped when present so a future
// indexer can add metadata fields (effective_date, owner, etc.) without
// poisoning the embedded body.
func stripFrontmatter(raw []byte) []byte {
	// Normalize CRLF → LF for marker matching across Windows-committed
	// files (mirrors the skill loader behavior).
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	const open = "---\n"
	if !bytes.HasPrefix(raw, []byte(open)) {
		return raw
	}
	body := raw[len(open):]
	// Closing marker is "\n---" — the body continues after the next newline.
	const close = "\n---"
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		// Unterminated frontmatter — treat as no frontmatter (don't fail;
		// the file is still indexable, just the frontmatter will be
		// embedded as part of the body. Better than refusing to index).
		return raw
	}
	// Skip the close line itself ("\n---") + the trailing newline.
	after := body[end+len(close):]
	return bytes.TrimLeft(after, "\n")
}