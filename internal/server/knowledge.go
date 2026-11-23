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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
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
//     knowledge layer returns 404 for missing IDs)
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

	// Count total chunks across all tiers.
	totalChunks := 0
	for _, tier := range g.Tiers {
		totalChunks += tier.ChunkCount
	}

	// Phase 28 (GRF-05): read limit from Deps; zero-value falls back to default.
	maxNodes := s.GraphLimits.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 50
	}
	if totalChunks > maxNodes {
		g.Degraded = true
		g.DegradedReason = fmt.Sprintf("节点数超过 %d（实际 %d）；已返回骨架图，请调高 SKILLFORGE_GRAPH_MAX_NODES 或减少知识库内容", maxNodes, totalChunks)
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
	// 2026-11-09 安全三连：chunk 直读必须校验 tier 可见性（见
	// knowledgeReadAllowed）。tier 隔离此前只在检索路径生效，直读路径
	// 不做判定，而 chunk id 是递增整数可枚举 —— 权威库原文可被匿名
	// 客户端整篇取走。判定放在取到 payload 之后、写出之前，内容不会
	// 离开服务端。
	if !s.knowledgeReadAllowed(c, payload.Tier) {
		c.JSON(http.StatusForbidden, graphResponseError{
			Error: "knowledge chunk not readable by this caller",
		})
		return
	}
	c.JSON(http.StatusOK, payload)
}

// neighborsHandler implements GET /api/knowledge/graph/neighbors?chunk_id=X&depth=N.
// Returns the N-depth neighborhood subgraph around chunk X including all edges.
// D-28-02-AUTH: public read (no admin token required).
func (s *Deps) neighborsHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized",
		})
		return
	}
	if s.Retriever == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "retriever not initialized",
		})
		return
	}

	chunkIDStr := string(c.Query("chunk_id"))
	if chunkIDStr == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "chunk_id is required"})
		return
	}
	id, ok := parseLenientChunkID(chunkIDStr)
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid chunk id"})
		return
	}
	chunkID := strconv.FormatInt(id, 10)

	maxDepth := s.GraphLimits.NeighborDepthLimit
	if maxDepth <= 0 {
		maxDepth = 2
	}
	depthStr := string(c.Query("depth"))
	depth := maxDepth
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}
	// Cap at configured max.
	if depth > maxDepth {
		depth = maxDepth
	}

	// Build EdgeGraph from metadata.
	eg, err := knowledge.EdgeGraphFromMetadata(ctx, s.Retriever)
	if err != nil {
		log.Printf("[WARN] server: neighborsHandler EdgeGraphFromMetadata: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "neighbor query failed — see server logs",
		})
		return
	}

	// Phase 28 (GRF-05): read limits from Deps; zero-value falls back to defaults.
	maxNodes := s.GraphLimits.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 50
	}
	maxEdges := s.GraphLimits.MaxEdges
	if maxEdges <= 0 {
		maxEdges = 200
	}

	result := eg.Neighbors(chunkID, depth, maxNodes)
	if result == nil {
		c.JSON(http.StatusNotFound, graphResponseError{Error: "chunk not found"})
		return
	}

	// Phase 28 (GRF-05): cap edge count first — edge limit reason takes
	// precedence over depth-limit reason when both trigger.
	if len(result.Edges) > maxEdges {
		result.Degraded = true
		result.DegradedReason = fmt.Sprintf("边数超过 %d（实际 %d），已截断。请调高 SKILLFORGE_GRAPH_MAX_EDGES", maxEdges, len(result.Edges))
		result.Edges = result.Edges[:maxEdges]
	}

	c.JSON(http.StatusOK, result)
}

// pathHandler implements GET /api/knowledge/graph/path?from=X&to=Y&max_hops=N.
// Returns the BFS shortest path between two chunks, or degraded if no path exists.
// D-28-02-AUTH: public read (no admin token required).
func (s *Deps) pathHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized",
		})
		return
	}
	if s.Retriever == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "retriever not initialized",
		})
		return
	}

	fromStr := string(c.Query("from"))
	toStr := string(c.Query("to"))
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "from and to are required"})
		return
	}
	fromID, ok := parseLenientChunkID(fromStr)
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid from chunk id"})
		return
	}
	toID, ok := parseLenientChunkID(toStr)
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid to chunk id"})
		return
	}
	from := strconv.FormatInt(fromID, 10)
	to := strconv.FormatInt(toID, 10)

	// Phase 28 (GRF-05): read hops limit from Deps; zero-value falls back to default.
	maxHopsLimit := s.GraphLimits.PathHopsLimit
	if maxHopsLimit <= 0 {
		maxHopsLimit = 4
	}
	maxHopsStr := string(c.Query("max_hops"))
	maxHops := maxHopsLimit
	requestedHops := maxHops
	if maxHopsStr != "" {
		if m, err := strconv.Atoi(maxHopsStr); err == nil && m > 0 {
			maxHops = m
			requestedHops = m
		}
	}
	// Cap at configured max.
	if maxHops > maxHopsLimit {
		maxHops = maxHopsLimit
	}

	// Build EdgeGraph from metadata.
	eg, err := knowledge.EdgeGraphFromMetadata(ctx, s.Retriever)
	if err != nil {
		log.Printf("[WARN] server: pathHandler EdgeGraphFromMetadata: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "path query failed — see server logs",
		})
		return
	}

	result := eg.ShortestPath(from, to, maxHops)
	// If max_hops was capped at the configured limit, tag as degraded.
	if requestedHops > maxHopsLimit {
		result.Degraded = true
		result.DegradedReason = fmt.Sprintf("max_hops 超过限制 %d（请求 %d），已截断。请调高 SKILLFORGE_GRAPH_PATH_HOPS_LIMIT", maxHopsLimit, requestedHops)
	}
	c.JSON(http.StatusOK, result)
}

// Compile-time check that knowledge.GraphBuilder satisfies the
// KnowledgeBuilder type alias declared in server.go.
var _ KnowledgeBuilder = (*knowledge.GraphBuilder)(nil)

// the knowledge persist root is the on-disk corpus the indexer seeds from (see
// internal/knowledge/indexer.go — the same project-relative path). The
// docs endpoints read from it directly: the graph endpoint answers
// "what is INDEXED" (Milvus rows), while /api/knowledge/docs answers
// "what is ON DISK" — the left-sidebar FILES tree. They can differ
// transiently (a file added but not yet re-indexed); the FE shows the
// disk truth for browsing and the graph truth for retrieval.
//
// Since Phase 26 the corpus tree is knowledge/<slug>/ and the slug IS the
// directory name — the registry (Deps.KBRegistry) is the only source of
// which directories exist, so there is no separate tier→directory mapping
// layer to keep in sync (the old one silently produced empty listings the
// day the two lists drifted).
//
// Package var (not a const) following the persistKnowledgeRoot pattern
// (upload.go): production keeps the default, and the Phase 53-03 DELETE
// tests redirect it via knowledge.SetPersistKnowledgeRoot so the
// file-deletion assertions never touch the repo's real knowledge/ tree.

// (retired) SetKnowledgeRootDir bound the on-disk corpus root the docs endpoints
// read and the doc DELETE handler removes files under. Production never
// calls this (the default matches the seed root); tests use it with
// t.Cleanup restore.

// docEntry is one file row in the /api/knowledge/docs response.
type docEntry struct {
	Filename string `json:"filename"`
	Bytes    int64  `json:"bytes"`
}

// docsTier is one tier block in the /api/knowledge/docs response. Tier
// carries the registry slug.
type docsTier struct {
	Tier string     `json:"tier"`
	Docs []docEntry `json:"docs"`
}

// docsHandler implements GET /api/knowledge/docs — the disk-truth file
// tree for the left-sidebar FILES panel (and the doc nodes' click target
// in the knowledge graph view). Unlike graphHandler this does NOT touch
// the retriever: it walks knowledge/<slug>/ and returns filenames +
// sizes. Frontmatter stays in the content (the doc viewer renders it as
// the metadata header block).
//
// The block set is the ACTIVE registry set (slug ascending — List() is
// sorted): a deactivated base disappears from the browsing surface exactly
// as it does from retrieval and skill validation, and a newly created base
// appears without a code change (KBM-01: the registry is the single source
// of the KB set). An empty registry is a legal degraded state → 200 with
// an empty list (the FE renders an empty tree), never a 500.
//
//	GET /api/knowledge/docs → 200 { tiers: [ { tier, docs: [{filename, bytes}] } ] }
//	                        → 503 { error: KB registry not wired }
func (s *Deps) docsHandler(ctx context.Context, c *app.RequestContext) {
	if s.KBRegistry == nil {
		// Fail-closed: an empty-looking 200 would read as "no corpus",
		// which is a different operational fact than "registry unwired".
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}
	tiers := make([]docsTier, 0)
	for _, slug := range s.KBRegistry.ActiveSlugs() {
		tiers = append(tiers, docsTierFor(slug))
	}
	c.JSON(http.StatusOK, map[string]any{"tiers": tiers})
}

// docsTierFor lists one corpus directory into a docsTier block. The slug
// doubles as the directory name under the persist corpus root, and it is already
// whitelist-validated by the registry (a slug can never carry a separator
// — kb.ValidateSlug), so joining it into a path is safe by construction.
//
// Phase 999.2: the listing delegates to knowledge.WalkDocs (the KBM-06
// single-walk core) so the docs tree sees the SAME recursive corpus view
// as the indexer — nested KB subdirectories show up here with their
// slash-normalized relative paths instead of being silently dropped.
//
// A missing directory is an EMPTY tier, not a 500: a base can exist in the
// registry before its corpus lands on disk (and a fresh checkout may ship
// no .gitkeep at all). The tree shows 0 docs for it.
func docsTierFor(slug string) docsTier {
	entries, err := knowledge.WalkDocs(filepath.Join(knowledge.PersistKnowledgeRoot(), slug))
	if err != nil {
		return docsTier{Tier: slug, Docs: []docEntry{}}
	}
	docs := make([]docEntry, 0, len(entries))
	for _, e := range entries {
		docs = append(docs, docEntry{Filename: e.RelPath, Bytes: e.Bytes})
	}
	return docsTier{Tier: slug, Docs: docs}
}

// docContentHandler implements GET /api/knowledge/doc?tier=&file= — the
// full markdown source of one corpus doc, backing the DocViewer (left
// FILES click → center panel; graph doc-node click).
//
// Path traversal is blocked by a DOUBLE gate (T-26-05-3). The tier value
// is joined into a filesystem path and the slug set is OPEN (admins create
// bases at runtime), so a 4-value lookup table is neither possible nor
// sufficient — both halves are required:
//
//  1. kb.ValidateSlug — the whitelist REGEX (lowercase/digit/hyphen, no
//     separators): traversal-shaped values never reach the registry, and
//     the regex gate holds for every slug that will ever exist;
//  2. registry Lookup — the value must be a currently-registered ACTIVE
//     base, so unregistered AND deactivated slugs refuse here too (the
//     400 names the active set, mirroring the reads-contract error form).
//
// validRelDocPath reports whether file is a safe KB-relative slash path
// for the doc-content reader. Phase 999.2 relaxed the old bare-base-name
// rule so nested subdirectories are previewable, but the traversal floor
// is unchanged: backslashes anywhere, a leading slash, an empty path, or
// any empty / "." / ".." segment is rejected before the path ever reaches
// filepath.Join. Everything else is a plain relative lookup under
// knowledge/<tier>/.
func validRelDocPath(file string) bool {
	if file == "" || strings.Contains(file, "\\") || strings.HasPrefix(file, "/") {
		return false
	}
	for seg := range strings.SplitSeq(file, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// The file half is a KB-relative slash path. Phase 999.2 relaxed the old
// bare-base-name rule (filepath.Base(file) == file) so nested KB
// subdirectories are previewable: "汽车险/011_xxx.md" is legal, while any
// traversal shape ("..", leading "/", backslashes, empty segments) is
// still rejected before Join.
//
//	GET /api/knowledge/doc → 200 { tier, file, content }
//	                       → 400 { error: "invalid tier or file" }          (regex / path gate)
//	                       → 400 { error: "unknown or deactivated KB …" }   (registry gate)
//	                       → 404 { error: "doc not found" }
func (s *Deps) docContentHandler(ctx context.Context, c *app.RequestContext) {
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}
	tier := string(c.Query("tier"))
	file := string(c.Query("file"))
	// Gate 1 — the whitelist regex + the KB-relative path rule. Same 400
	// envelope as before, so the FE error handling is unchanged.
	if kb.ValidateSlug(tier) != nil || !validRelDocPath(file) {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid tier or file"})
		return
	}
	// Gate 2 — registry membership: an unknown OR deactivated slug 400s
	// naming the active set (fail-closed with the valid set in the
	// message, the same discipline as the reads contract).
	if _, ok := s.KBRegistry.Lookup(tier); !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("unknown or deactivated knowledge base %q (active: %v)", tier, s.KBRegistry.ActiveSlugs()),
		})
		return
	}
	// Gate 3 (2026-11-09 安全三连) — tier 可见性。Gate 1 只校验 slug 形状，
	// Gate 2 只校验"这个 base 是 active" —— 都不回答"这个调用方能不能读
	// 这个 tier"。缺了这一门，authority 制度原文可被任意匿名客户端整篇
	// 取走（file 参数可枚举）。
	if !s.knowledgeReadAllowed(c, tier) {
		c.JSON(http.StatusForbidden, graphResponseError{
			Error: "knowledge document not readable by this caller",
		})
		return
	}
	content, err := os.ReadFile(filepath.Join(knowledge.PersistKnowledgeRoot(), tier, file))
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

// actionDocDeleted is the INSERT-only audit action the doc DELETE endpoint
// writes (Phase 53-03 / ING-03). It lands in the same audit table as the
// kb admin mutations (action_kb_created / action_kb_deactivated) and the
// upload pipeline (action_doc_uploaded), so GET /api/admin/audit surfaces
// the full document lifecycle: uploaded → (re-uploaded) → deleted.
const actionDocDeleted = "action_doc_deleted"

// DocDeleteAudit is the FrozenArgs payload for action_doc_deleted.
type DocDeleteAudit struct {
	Slug       string `json:"slug"`
	DocID      string `json:"doc_id"`
	ChunkCount int    `json:"chunk_count"`
}

// docDeleteHandler implements DELETE /api/admin/kb/:slug/docs/:doc_id —
// the document-level delete verb of the doc-lifecycle phase (Phase 53-03 /
// ING-03 / D-53-03).
//
// Wire shape: the URL carries the doc_id VERBATIM (slash path, no
// %-encoding), e.g.
//
//	DELETE /api/admin/kb/authority/docs/authority/travel_expense.md
//
// The trailing segment is therefore registered as a Hertz WILDCARD
// (*doc_id, the /reports/*filepath precedent) rather than a single-segment
// :param — a doc_id IS a slash path ("authority/<sha8>-<stem>.md"), and a
// single-segment param would force %2F-encoding whose decode behavior is
// router-version-dependent. The wildcard keeps the URL unencoded and the
// handler receives the leading slash, which it strips.
//
// Semantics (D-53-03 / D-53-01 — two deletes, one transaction intent):
//
//  1. Vector rows: DeleteDocRows(tier=slug, doc_id) through the
//     retriever.DocChunkStore capability — every chunk of the document
//     leaves the retrieval surface immediately.
//  2. The canonical FILE under knowledge/<slug>/ — the D-53-01 single
//     source of truth. Deleting rows without the file would resurrect the
//     document at the next boot reseed (the file IS the corpus), so the
//     file MUST go with the rows; the raw-bytes archive under
//     data/uploads/ is provenance, not corpus, and is deliberately kept.
//
// CHECK ORDER mirrors kb_admin.go / upload.go: 403 before anything that
// could leak KB registry state; 503 for the nil registry; 400 for the
// slug regex; 404 for the unknown/deactivated slug.
//
// doc_id is gated TWICE before it ever becomes a filesystem path: it must
// start with "<slug>/" (a caller can never delete another base's file —
// the slug in the URL and the slug in the doc_id must agree) and its
// remainder must pass validRelDocPath (the traversal floor: no leading
// slash, no backslashes, no "." / ".." / empty segments).
//
// Lifecycle keys only: the delete addresses rows by doc_id, NEVER by
// source. Pre-53 collections whose rows still carry doc_id="" answer 404
// ("no lifecycle rows") — and a source-keyed fallback would be actively
// WRONG, not merely incomplete: on the ECL-cohabiting tier an ECL
// fragment row's source can equal the note path, so deleting by source
// could cross producer generations (the exact failure the D-Milvus-02
// ownership marker exists to prevent). Operators migrate a pre-53
// collection with the documented one-shot SKILLFORGE_MILVUS_RECREATE=1
// rebuild, after which every row carries a doc_id.
//
//	DELETE /api/admin/kb/:slug/docs/:doc_id → 200 { deleted_chunks: N }
//	                                        → 403 (no admin token)
//	                                        → 503 { error: KB registry not wired }
//	                                        → 400 { error: invalid slug / invalid doc_id }
//	                                        → 404 { error: kb not found / doc_id has no lifecycle rows }
//	                                        → 500 { error: delete failed — see server logs } (W-3)
func (s *Deps) docDeleteHandler(ctx context.Context, c *app.RequestContext) {
	// STEP 1: admin token gate — 403 must NOT leak KB registry state.
	if !requireAdmin(c) {
		return
	}

	// STEP 2: KB registry nil → 503.
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}

	// STEP 3: validate slug param (whitelist regex — same 400 as upload).
	slug := c.Param("slug")
	if err := kb.ValidateSlug(slug); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}

	// STEP 4: slug active check via registry Lookup — an unknown OR
	// deactivated base has no deletable surface.
	if _, ok := s.KBRegistry.Lookup(slug); !ok {
		c.JSON(http.StatusNotFound, graphResponseError{Error: "kb " + slug + " not found or deactivated"})
		return
	}

	// STEP 5: doc_id extraction + the two-path gate. Hertz wildcard params
	// carry the leading slash of the matched remainder.
	docID := strings.TrimPrefix(string(c.Param("doc_id")), "/")
	if docID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "doc_id is required (the corpus-relative path, e.g. " + slug + "/file.md)"})
		return
	}
	if !strings.HasPrefix(docID, slug+"/") {
		// Cross-base guard: the URL's slug and the doc_id's slug segment
		// must agree, so a request scoped to one base can never address
		// another base's rows or file.
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "doc_id must live under " + slug + "/ (got " + docID + ")"})
		return
	}
	rel := strings.TrimPrefix(docID, slug+"/")
	if !validRelDocPath(rel) {
		// Traversal floor: identical rule the doc-content reader applies.
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid doc_id path"})
		return
	}

	// STEP 6: retriever + capability gate. The delete is a doc-lifecycle
	// operation; a backend without retriever.DocChunkStore cannot honor it
	// and silently answering 200 would be a lie.
	if s.Retriever == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "retriever not initialized"})
		return
	}
	lifecycle, ok := s.Retriever.(retriever.DocChunkStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "retriever backend does not support doc-level delete (retriever.DocChunkStore)",
		})
		return
	}

	// STEP 7: existence check via the same projection the upsert reads —
	// the count doubles as the response's deleted_chunks, and an empty
	// result is the 404 (no lifecycle rows for this doc_id: either the
	// doc never existed or the collection predates Phase 53 — the error
	// says which remedy applies).
	rows, err := lifecycle.DocChunkRows(ctx, slug, docID)
	if err != nil {
		log.Printf("[WARN] server: doc delete lookup failed (slug=%s doc_id=%s): %v", slug, docID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "doc delete failed — see server logs"})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, graphResponseError{
			Error: "doc_id " + docID + " has no lifecycle rows (not indexed, or the collection predates Phase 53 and needs the one-shot SKILLFORGE_MILVUS_RECREATE rebuild)",
		})
		return
	}

	// STEP 8 (a): vector rows out — every chunk of the document leaves the
	// retrieval surface.
	if err := lifecycle.DeleteDocRows(ctx, slug, docID); err != nil {
		log.Printf("[WARN] server: doc delete rows failed (slug=%s doc_id=%s): %v", slug, docID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "doc delete failed — see server logs"})
		return
	}

	// STEP 8 (b): the canonical file out — D-53-01: the file IS the
	// corpus, so rows-without-file would resurrect the doc at the next
	// boot reseed. A missing file is success (idempotent: a doc whose
	// rows exist but file never persisted — the pre-53-01 upload shape —
	// still deletes cleanly). Any OTHER removal failure is a 500 AFTER
	// the rows are gone: reseed would resurrect the doc, so this must
	// fail loudly and name the half-state, not masquerade as success.
	// filePath uses the FULL docID (slug segment included): the canonical
	// file lives at <persist root>/<slug>/<name> (PersistUploadedDoc),
	// and `rel` here is docID minus the slug prefix — Joining on rel
	// would remove a file one level up from where 53-01 wrote it, and
	// os.Remove's IsNotExist would then be swallowed by the idempotent
	// branch, returning 200 while the corpus file survives (caught by
	// TestDocDeleteHappyPath in the 53-03 review). rel remains the
	// traversal-check surface: the check above already proves docID =
	// slug + "/" + validRelDocPath(rel), so Join(root, FromSlash(docID))
	// cannot escape the corpus root.
	filePath := filepath.Join(knowledge.PersistKnowledgeRoot(), filepath.FromSlash(docID))
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("[WARN] server: doc deleted from index but file removal failed (slug=%s doc_id=%s file=%s): %v — the next reseed will resurrect it", slug, docID, filePath, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "rows deleted but file removal failed — the next reseed will resurrect this doc; fix the filesystem and retry",
		})
		return
	}

	// STEP 9: INSERT-only audit row (best-effort, putKBAdminAuditRow
	// posture: a failed write is logged and never rolls the delete back).
	s.putDocDeletedAuditRow(ctx, DocDeleteAudit{
		Slug:       slug,
		DocID:      docID,
		ChunkCount: len(rows),
	})

	c.JSON(http.StatusOK, map[string]any{"deleted_chunks": len(rows)})
}

// putDocDeletedAuditRow writes the action_doc_deleted audit row. Mirrors
// putKBAdminAuditRow (kb_admin.go): Store-nil = degraded mode (skip), a
// failed write is logged at WARN and never rolls the mutation back — the
// rows and the file are already gone, and turning a successful delete
// into a 500 because its audit row failed would desync the operator's
// intent from the system state.
func (s *Deps) putDocDeletedAuditRow(ctx context.Context, payload DocDeleteAudit) {
	if s.Store == nil {
		return
	}
	frozen, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WARN] server: doc delete audit row marshal (slug=%s doc_id=%s): %v (row skipped)", payload.Slug, payload.DocID, err)
		return
	}
	if _, err := s.Store.PutAudit(ctx, store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "admin",
		Action:     actionDocDeleted,
		ActorType:  "admin",
		FrozenArgs: frozen,
	}); err != nil {
		log.Printf("[WARN] server: doc delete audit row write (slug=%s doc_id=%s): %v (best-effort, delete not rolled back)", payload.Slug, payload.DocID, err)
	}
}
