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
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// SeedFromKnowledgeDir mirrors internal/knowledge.Index for the MongoDB
// backend: walk the ACTIVE knowledge/<slug>/ directories under
// knowledgeDir (through the SHARED walker, knowledge.WalkRegisteredDirs),
// strip YAML frontmatter, embed each body via emb, and Insert the
// resulting Documents with the slug as the tier value.
//
// WHY THIS EXISTS (Rule 3 deviation):
// internal/knowledge/indexer.go was supposed to be refactored to take a
// retriever.Retriever (see 02-04-REFACTOR-SUMMARY.md), but the refactor
// commit (dadd63a) left the milvus-coupled signature
// `Index(ctx, dir, emb, mc client.Client, cfg milvus.Config)` in place.
// Until that indexer is actually made backend-agnostic, the MongoDB
// path needs its own seeding routine. Phase 26 removed the last reason
// this routine could drift from the indexer: the directory walk (which
// used to be a hand-copied list of the four tier directories here AND in
// the indexer — 26-RESEARCH Pitfall 4) now lives in ONE place,
// knowledge.WalkRegisteredDirs, and both backends consume its verdict.
//
// Behavior parity with internal/knowledge.Index (same walker, same
// three-state table — see internal/knowledge/walk.go):
//   - active slug dir        → seeded (tier = the slug)
//   - deactivated slug dir   → skipped with a WARN, no audit row
//   - unregistered dir       → skipped with a WARN + an INSERT-only audit
//     row delivered through seedAuditHook
//   - the service never fails on an unknown directory (SC-5); an empty or
//     absent knowledge root is the legal degraded state (SC-4)
//   - Files with YAML frontmatter get the frontmatter stripped before
//     embedding (same stripFrontmatter algorithm).
//   - IDs are 1-based, monotonically increasing across slug dirs (same
//     convention as Milvus so a citation's chunk_id is portable across
//     backends).
//   - Empty knowledge tree → no-op (returns nil; no write phase at all).
//
// reg is required (fail-closed): a nil registry would make every
// directory look unregistered and silently seed nothing — indistinguishable
// from a healthy empty collection.
func (c *Client) SeedFromKnowledgeDir(ctx context.Context, knowledgeDir string, emb embedder.Embedder, reg kb.Registry) error {
	if knowledgeDir == "" {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: knowledgeDir is empty")
	}
	if emb == nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: embedder is nil")
	}
	if reg == nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: registry is required (fail-closed; mirrors knowledge.Index)")
	}

	// Shared walker (KBM-06 / Pitfall 4 convergence): the mongo backend no
	// longer keeps its own directory enumeration. Audit rows for
	// unregistered directories ride the seedAuditHook callback — the same
	// seam shape as the indexer's indexAuditHook, so neither backend needs
	// a store parameter on its seed function.
	dirs, err := knowledge.WalkRegisteredDirs(knowledgeDir, reg, seedAuditHook)
	if err != nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: walk %s: %w", knowledgeDir, err)
	}

	var docs []retriever.Document
	nextID := int64(1)

	for _, slug := range dirs {
		tierAbs := filepath.Join(knowledgeDir, slug)

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
				Tier:      slug,
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
	// Idempotent re-seed: clear prior rows so repeated server starts do
	// not accumulate duplicates. Without this, top-K retrieval fills with
	// repeated copies of the same chunk (live-verify SC-3 regression).
	if seedInsertHook != nil {
		// Test seam: intercepts the whole write phase (Reset + Insert),
		// mirroring knowledge.indexSeedHook. Production never takes this
		// branch — the hook is nil unless a test set it.
		return seedInsertHook(ctx, docs)
	}
	if err := c.Reset(ctx); err != nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: reset: %w", err)
	}
	if err := c.Insert(ctx, docs); err != nil {
		return fmt.Errorf("mongo.SeedFromKnowledgeDir: insert %d docs: %w", len(docs), err)
	}
	return nil
}

// seedInsertHook is the package-internal indirection hermetic tests use to
// intercept the write phase (Reset + Insert) without a live MongoDB. nil
// (production) means the real Reset + Insert pair runs unchanged.
var seedInsertHook func(ctx context.Context, docs []retriever.Document) error

// seedAuditHook is the audit callback handed to the shared walker for the
// unregistered-directory decision (KBM-06). It is deliberately a callback
// seam and NOT a store parameter on SeedFromKnowledgeDir (Pitfall 7): the
// mongo seed must stay usable from cmd/eval and the spike binaries without
// dragging the sqlite store in.
//
// Production binds an INSERT-only wrapper over store.PutAudit at boot via
// SetSeedAuditHook; hermetic tests bind an in-memory collector; nil (the
// default) means "do not audit" — the walker's WARN still fires.
var seedAuditHook func(action, detail string)

// SetSeedAuditHook binds the production audit sink for the unregistered-
// directory decision on the mongo seed path. The wrapper the caller hands
// in MUST be INSERT-only. Called once at boot, before any seed call, so no
// synchronization is needed afterwards.
func SetSeedAuditHook(fn func(action, detail string)) { seedAuditHook = fn }

// stripFrontmatter is a verbatim re-implementation of the milvus-side
// internal/knowledge.splitFrontmatter. Kept local so the retriever backend
// depends on the knowledge package only through its exported walker (and
// not on an unexported helper). The algorithm is small and stable.
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
