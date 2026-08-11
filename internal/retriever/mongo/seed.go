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

package mongo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// SeedFromKnowledgeDir mirrors internal/knowledge.Index for the MongoDB
// backend: scan knowledge/{authority,historical,personal,draft}/*.md,
// strip YAML frontmatter, embed each body via emb, and Insert the
// resulting Documents.
//
// WHY THIS EXISTS (Rule 3 deviation):
// internal/knowledge/indexer.go was supposed to be refactored to take a
// retriever.Retriever (see 02-04-REFACTOR-SUMMARY.md), but the refactor
// commit (dadd63a) left the milvus-coupled signature
// `Index(ctx, dir, emb, mc client.Client, cfg milvus.Config)` in place.
// Until that indexer is actually made backend-agnostic, the MongoDB
// path needs its own seeding routine. This is intentionally a near-copy
// of the milvus indexer's scan/embed/strip logic — duplication is the
// price of not touching the milvus tests in this plan.
//
// Behavior parity with internal/knowledge.Index:
//   - Tier dirs that don't exist on disk are skipped silently.
//   - Empty tier dirs are valid (SC-4 refusal mechanism).
//   - Files with YAML frontmatter get the frontmatter stripped before
//     embedding (same stripFrontmatter algorithm).
//   - IDs are 1-based, monotonically increasing across tier dirs (same
//     convention as Milvus so a citation's chunk_id is portable across
//     backends).
//   - Empty knowledge tree → no-op (returns nil; no Insert call).
func (c *Client) SeedFromKnowledgeDir(ctx context.Context, knowledgeDir string, emb embedder.Embedder) error {
	if knowledgeDir == "" {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: knowledgeDir is empty")
	}
	if emb == nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: embedder is nil")
	}

	// Same fixed enumeration as internal/knowledge.knownTierDirs. Sorted
	// alphabetically for deterministic ID assignment across re-runs.
	tierDirs := []string{"authority", "historical", "personal", "draft"}
	sort.Strings(tierDirs)

	var docs []retriever.Document
	nextID := int64(1)

	for _, tierDir := range tierDirs {
		tierVal, ok := skill.TierEnum[tierDir]
		if !ok {
			// Code-level drift between this list and skill.TierEnum.
			return fmt.Errorf("mongo.SeedFromKnowledgeDir: tier %q missing from skill.TierEnum", tierDir)
		}

		tierAbs := filepath.Join(knowledgeDir, tierDir)
		info, err := os.Stat(tierAbs)
		if err != nil {
			if os.IsNotExist(err) {
				continue // missing tier dir is valid
			}
			return fmt.Errorf("mongo.SeedFromKnowledgeDir: stat %s: %w", tierAbs, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("mongo.SeedFromKnowledgeDir: %s is not a directory", tierAbs)
		}

		pattern := filepath.Join(tierAbs, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("mongo.SeedFromKnowledgeDir: glob %s: %w", pattern, err)
		}
		sort.Strings(matches)

		for _, path := range matches {
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("mongo.SeedFromKnowledgeDir: read %s: %w", path, err)
			}
			body := stripFrontmatter(raw)
			if len(body) == 0 {
				continue
			}

			// Production embed — never a deterministic fallback (Pitfall 8).
			vec, err := emb.Embed(ctx, string(body))
			if err != nil {
				return fmt.Errorf("mongo.SeedFromKnowledgeDir: embed %s: %w", path, err)
			}

			rel, err := filepath.Rel(knowledgeDir, path)
			if err != nil {
				return fmt.Errorf("mongo.SeedFromKnowledgeDir: rel %s: %w", path, err)
			}
			docs = append(docs, retriever.Document{
				ID:        nextID,
				Tier:      tierVal,
				Content:   string(body),
				Source:    filepath.ToSlash(rel),
				Embedding: vec,
			})
			nextID++
		}
	}

	if len(docs) == 0 {
		// Empty knowledge tree — same behavior as milvus indexer. Returns
		// nil; the retrieval path will return no_authorized_source for every
		// query against the empty collection.
		return nil
	}
	if err := c.Insert(ctx, docs); err != nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: insert %d docs: %w", len(docs), err)
	}
	return nil
}

// stripFrontmatter is a verbatim re-implementation of the milvus-side
// internal/knowledge.stripFrontmatter. Kept local to avoid an internal
// import dependency from a *retriever backend* on the knowledge package
// (which would invert the dependency direction). The algorithm is small
// and stable.
func stripFrontmatter(raw []byte) []byte {
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	const open = "---\n"
	if !bytes.HasPrefix(raw, []byte(open)) {
		return raw
	}
	body := raw[len(open):]
	const close = "\n---"
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		return raw
	}
	after := body[end+len(close):]
	return bytes.TrimLeft(after, "\n")
}
