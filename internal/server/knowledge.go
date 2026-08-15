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

// Plan 05-05 / SC-5 (KNOW-06) — GET /api/knowledge/graph endpoint.
//
// The graph endpoint exposes the metadata-only tier -> doc -> chunk tree
// to the FE knowledge browser. Implementation lives behind a dedicated
// /api/knowledge route group (separate from /api/skills) so future
// knowledge-related endpoints (e.g. /api/knowledge/chunks/{id} for the
// future on-demand chunk-content fetch) can register alongside without
// crowding the skills surface.
//
// Wire contract:
//
//	GET /api/knowledge/graph → 200 { tiers: [{ tier, docCount, chunkCount, docs: [...] }] }
//	                         503 { error: "knowledge graph not initialized" }
//	                         500 { error: "knowledge graph build failed — see server logs" }
//
// The 503 mirrors the wizard handler posture: when Deps.Knowledge is
// nil (a test that bypasses the graph layer, or a misconfigured server)
// the endpoint refuses the request rather than returning a misleading
// empty array (which the FE would mount as the empty-state hero — a
// valid signal only when the corpus is genuinely empty).
//
// The 500 envelope deliberately carries a generic message rather than
// err.Error(): ErrInvalidTier / ErrEmptySource embed internal metadata
// (numeric row IDs, source-path taxonomy) that an attacker could probe
// via hand-edited retriever fixtures if the endpoint is exposed. The
// full error is logged at [WARN] server-side (W-3).

package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/knowledge"
)

// graphResponseError is the {error: "..."} envelope returned with 4xx /
// 5xx responses on /api/knowledge/*. Mirrors the wizard + commit
// envelope so the FE error-handling path is uniform.
type graphResponseError struct {
	Error string `json:"error"`
}

// parseLenientChunkID extracts a numeric chunk ID from LLM-emitted or
// human-typed strings. The pipeline frames chunks as `[ref-<id>]` in the
// prompt and instructs the LLM to output digits-only in `chunk_id`, but
// the LLM occasionally echoes the label prefix verbatim
// (e.g. "CHUNK 7" / "ref-7" / "[ref-7]"). The earlier verdict-layer
// fix (verdict.canonicalChunkID) handles the in-process grounding map;
// this helper is the URL-path analogue for /api/knowledge/chunk/:id.
//
// Accepted forms (returns (id, true)):
//   - "7"              → 7
//   - "CHUNK 7"        → 7
//   - "chunk 7"        → 7 (case-insensitive prefix)
//   - "CHUNK7"         → 7 (no space also accepted)
//   - "ref-7"          → 7
//   - "[ref-7]"        → 7
//   - "  7  "          → 7 (whitespace trimmed)
//   - `"7"` / `'7'`    → 7 (quotes stripped)
//   - "-7"             → -7 (negative numbers pass through; the
//                         knowledge layer returns 404 for missing IDs)
//
// Rejected (returns (0, false)):
//   - "abc"
//   - ""
//   - "CHUNK abc"      (prefix stripped but remainder non-numeric)
//   - "7abc"           (no substring extraction; full string must be digits)
//
// Implementation: trim + strip matching pairs of brackets/parens/quotes
// (handles nested forms like "[[7]]") + strip a "CHUNK"/"REF" prefix
// (case-insensitive, with optional separator) + re-trim. The remainder
// is then run through strconv.ParseInt.
func parseLenientChunkID(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	for {
		changed := false
		switch {
		case len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']':
			s = strings.TrimSpace(s[1 : len(s)-1])
			changed = true
		case len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')':
			s = strings.TrimSpace(s[1 : len(s)-1])
			changed = true
		case len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"':
			s = strings.TrimSpace(s[1 : len(s)-1])
			changed = true
		case len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'':
			s = strings.TrimSpace(s[1 : len(s)-1])
			changed = true
		}
		if !changed {
			break
		}
	}
	for _, prefix := range []string{"CHUNK", "REF"} {
		if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			rest := s[len(prefix):]
			// Accept the prefix as a label iff what follows is clearly a
			// numeric payload: space, dash, or digit. Reject "CHUNKabc"
			// / "REFnote" — those are not label+digit forms.
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '-' || (rest[0] >= '0' && rest[0] <= '9') {
				s = strings.TrimLeft(rest, " -")
				break
			}
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// graphHandler implements GET /api/knowledge/graph. The Deps.Knowledge
// field holds the GraphBuilder. When nil (test mode or unconfigured
// production), the handler returns 503 with the standard envelope.
// On success it returns 200 + the Graph JSON the FE renders as a 3-level
// Accordion tree.
//
// Plan 05-05 / T-5-22 mitigation: the Graph JSON carries metadata only
// (id / tier / source / effective_*). Verbatim chunk text opens
// separately via ChunkSourceView — the graph endpoint deliberately
// does NOT carry content into the browser.
func (s *Deps) graphHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized (set Deps.Knowledge at server construction)",
		})
		return
	}
	g, err := s.Knowledge.Build(ctx)
	if err != nil {
		// W-3 (Phase 5 review): log the full err.Error() server-side at
		// WARN — ErrInvalidTier / ErrEmptySource embed internal metadata
		// (numeric row IDs, source-path taxonomy) that an attacker could
		// probe via hand-edited retriever fixtures if /api/knowledge/graph
		// is exposed. Return a generic message to the FE; the operator
		// finds the full error in the server logs. Mirrors the [WARN]
		// convention used in pipeline.go for non-fatal build failures.
		log.Printf("[WARN] server: knowledge graph build failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "knowledge graph build failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, g)
}

// chunkHandler implements GET /api/knowledge/chunk/:id. Plan 260813-dno
// / UAT #4 — the on-demand chunk-content fetch. The graph endpoint stays
// metadata-only (T-5-22); this handler returns the verbatim chunk text
// + metadata when the operator clicks a graph chunk (the FE fires it
// only on a fixture-miss in chunk-lookup, so the verdict-citation path
// is untouched).
//
// Posture mirrors graphHandler exactly so the FE error handling is
// uniform across /api/knowledge/*:
//
//	GET /api/knowledge/chunk/:id → 200 { id, tier, source, content,
//	                                  effectiveFrom, effectiveTo, version }
//	                             → 503 { error: "knowledge graph not initialized..." }  (nil Knowledge)
//	                             → 400 { error: "invalid chunk id" }                    (non-int64 path)
//	                             → 404 { error: "chunk not found" }                     (ErrChunkNotFound)
//	                             → 500 { error: "knowledge chunk fetch failed — see server logs" }  (other err, W-3)
//
// W-3 (Phase 5 review, mirrored from graphHandler): the 500 envelope
// carries a generic message — NEVER err.Error(). The full error is
// logged at [WARN] server-side so the operator can diagnose without the
// FE leaking internal detail to a potential attacker.
func (s *Deps) chunkHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized (set Deps.Knowledge at server construction)",
		})
		return
	}
	idStr := c.Param("id")
	id, ok := parseLenientChunkID(idStr)
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "invalid chunk id",
		})
		return
	}
	if idStr != strconv.FormatInt(id, 10) {
		// Lenient parse: the URL segment was not a bare integer. Log
		// the original form so ops can spot LLM-output drift on chunk_id
		// (live Phase 7 symptom: verdict JSON emits "CHUNK 7" and the
		// BE rejected it). The acceptance is intentional — see
		// parseLenientChunkID godoc.
		log.Printf("[WARN] server: chunkHandler lenient parse: %q → %d (LLM likely echoed label prefix)", idStr, id)
	}
	payload, err := s.Knowledge.GetChunk(ctx, id)
	if errors.Is(err, knowledge.ErrChunkNotFound) {
		c.JSON(http.StatusNotFound, graphResponseError{
			Error: "chunk not found",
		})
		return
	}
	if err != nil {
		// W-3: log the full err.Error() server-side at WARN; return a
		// generic message to the FE. Mirrors graphHandler's [WARN]
		// convention so a future refactor stays consistent.
		log.Printf("[WARN] server: knowledge chunk fetch failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "knowledge chunk fetch failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, payload)
}

// Compile-time check that knowledge.GraphBuilder satisfies the
// KnowledgeBuilder type alias declared in server.go.
var _ KnowledgeBuilder = (*knowledge.GraphBuilder)(nil)

// knowledgeRootDir is the on-disk corpus the indexer seeds from (see
// internal/knowledge/indexer.go — the same project-relative path). The
// docs endpoints read from it directly: the graph endpoint answers
// "what is INDEXED" (Milvus rows), while /api/knowledge/docs answers
// "what is ON DISK" — the left-sidebar FILES tree. They can differ
// transiently (a file added but not yet re-indexed); the FE shows the
// disk truth for browsing and the graph truth for retrieval.
const knowledgeRootDir = "knowledge/"

// validKnowledgeTiers gates the `tier` query param on /api/knowledge/doc.
// Everything else 400s — the value is concatenated into a filesystem
// path, so an open tier would let a caller probe arbitrary directories
// (e.g. tier=../cmd). Keep in lockstep with the indexer's 4-tier set.
var validKnowledgeTiers = map[string]bool{
	"权威": true, "历史": true, "个人": true, "草稿": true,
}

// tierDirEn maps a canonical tier to its on-disk subdirectory. The
// corpus tree uses ENGLISH directory names (knowledge/authority/...) —
// the Chinese tier names are wire/UI vocabulary only. Joining the tier
// name directly into the path silently produced empty listings (the
// live "文件树为空" trap: knowledge/权威 does not exist on disk).
var tierDirEn = map[string]string{
	"权威": "authority", "历史": "historical", "个人": "personal", "草稿": "draft",
}

// docEntry is one file row in the /api/knowledge/docs response.
type docEntry struct {
	Filename string `json:"filename"`
	Bytes    int64  `json:"bytes"`
}

// docsTier is one tier block in the /api/knowledge/docs response.
type docsTier struct {
	Tier string     `json:"tier"`
	Docs []docEntry `json:"docs"`
}

// docsHandler implements GET /api/knowledge/docs — the disk-truth file
// tree for the left-sidebar FILES panel (and the doc nodes' click target
// in the knowledge graph view). Unlike graphHandler this does NOT touch
// the retriever: it walks knowledge/<tier>/ and returns filenames +
// sizes. Frontmatter stays in the content (the doc viewer renders it as
// the metadata header block).
//
//	GET /api/knowledge/docs → 200 { tiers: [ { tier, docs: [{filename, bytes}] } ] }
//	                        → 500 { error: "knowledge docs scan failed — see server logs" }
func (s *Deps) docsHandler(ctx context.Context, c *app.RequestContext) {
	tiers := make([]docsTier, 0, len(validKnowledgeTiers))
	for _, tier := range []string{"权威", "历史", "个人", "草稿"} {
		entries, err := os.ReadDir(filepath.Join(knowledgeRootDir, tierDirEn[tier]))
		if err != nil {
			// Missing tier dir (e.g. a fresh checkout with only .gitkeep
			// gone) is an empty tier, not a 500 — the tree shows 0 docs.
			tiers = append(tiers, docsTier{Tier: tier, Docs: []docEntry{}})
			continue
		}
		docs := make([]docEntry, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || e.Name() == ".gitkeep" || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			docs = append(docs, docEntry{Filename: e.Name(), Bytes: info.Size()})
		}
		tiers = append(tiers, docsTier{Tier: tier, Docs: docs})
	}
	c.JSON(http.StatusOK, map[string]any{"tiers": tiers})
}

// docContentHandler implements GET /api/knowledge/doc?tier=&file= — the
// full markdown source of one corpus doc, backing the DocViewer (left
// FILES click → center panel; graph doc-node click). Path traversal is
// blocked structurally: tier must be in validKnowledgeTiers AND file
// must be a bare base name (filepath.Base(file) == file — rejects any
// separator, so "../" prefixes and nested paths never reach Join).
//
//	GET /api/knowledge/doc → 200 { tier, file, content }
//	                       → 400 { error: "invalid tier or file" }
//	                       → 404 { error: "doc not found" }
func (s *Deps) docContentHandler(ctx context.Context, c *app.RequestContext) {
	tier := string(c.Query("tier"))
	file := string(c.Query("file"))
	if !validKnowledgeTiers[tier] || file == "" || filepath.Base(file) != file || strings.Contains(file, "\\") {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid tier or file"})
		return
	}
	content, err := os.ReadFile(filepath.Join(knowledgeRootDir, tierDirEn[tier], file))
	if err != nil {
		// Missing file (deleted between docs listing and click) → 404, not
		// a 500 with a path-bearing error message (W-3 posture).
		c.JSON(http.StatusNotFound, graphResponseError{Error: "doc not found"})
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"tier":    tier,
		"file":    file,
		"content": string(content),
	})
}
