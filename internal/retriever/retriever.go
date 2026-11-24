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

// Package retriever defines the abstraction over tiered vector search
// backends (Milvus, MongoDB Atlas Vector Search, etc.). All
// implementations MUST enforce tier-filter PRE-similarity (SYSK-07, P-RAG-2)
// — this is the core trust boundary.
//
// This file holds ONLY the interface + the generic Hit / Document types.
// The Milvus implementation lives in internal/retriever/milvus; future
// backends (MongoDB, pgvector) will sit alongside it.
package retriever

import (
	"context"
	"errors"
)

// ErrChunkNotFound is returned by GetChunk when no row carries the
// requested numeric id. The server layer maps this to HTTP 404. It is
// distinct from a generic backend error (→ 500) so the FE can render
// a soft "片段不存在" state rather than a hard error.
var ErrChunkNotFound = errors.New("retriever: chunk not found by id")

// Hit is a generic search result returned by all Retriever
// implementations. Tier is the CN-char tier value (e.g. "权威") that
// the Skill resolved from its reads slice; Source is the slash-
// normalized path to the original corpus file (e.g.
// "authority/travel_expense.md"). EffectiveFrom / EffectiveTo round-
// trip from the backend so the indexer / pipeline can audit which
// version of a policy each chunk was sourced from.
//
// Phase 53-03: DocID / ChunkIndex / ContentHash are the doc-lifecycle
// projection (检索元数据投影 — the graph builder and chunk surfaces can
// display them). Same zero-value legacy contract as Document below:
// rows written before Phase 53 carry the zero values, which is a pure
// display matter and never changes filter or ranking semantics.
type Hit struct {
	ID            int64
	Score         float32
	Tier          string
	Content       string
	Source        string
	EffectiveFrom int64
	EffectiveTo   int64
	DocID         string
	ChunkIndex    int32
	ContentHash   string

	// Phase 58-01 (PROV-01/PROV-02) -- provenance RUNTIME fields. They are
	// NOT part of any backend schema (zero values mean "vector", which is
	// exactly the pre-58 wire semantics): the fusion layer (fuse.go)
	// populates them in memory, and the citation projection copies them
	// onto the wire. A backend that never sets them keeps working
	// unchanged -- the additive contract.
	//
	//	Origin    -- vector | graph | hybrid (empty == vector).
	//	ViaEntity -- the anchored entity whose graph path surfaced this hit;
	//	             empty for pure vector hits.
	//	ViaRels   -- the predicate path walked to reach it.
	Origin    string
	ViaEntity string
	ViaRels   []string
	// Channels is the RAW channel set the fusion layer saw (PROV-02).
	// A single-channel hit carries exactly one entry; a dual-channel hit
	// carries both. Nil == pre-58 producer (reads as [vector]).
	Channels []string
}

// Document is the input to Retriever.Insert (used by the knowledge
// indexer). The ID is the numeric primary key (Milvus INT64 PK; the
// convention is 1-based and monotonically increasing across tier dirs).
//
// Plan 05-02 / D5-15: EffectiveFrom + EffectiveTo are the policy-
// version window (epoch seconds). Sentinel design — NOT nullable
// fields. Zero means "open-ended" / perpetually valid:
//
//   - EffectiveFrom = 0 AND EffectiveTo = 0 → perpetually valid (legacy).
//   - EffectiveFrom = 0 AND EffectiveTo > 0 → open-ended start, ends at <to>.
//   - EffectiveFrom > 0 AND EffectiveTo = 0 → starts at <from>, perpetual.
//   - EffectiveFrom > 0 AND EffectiveTo > 0 → bounded window.
//
// KNOWN-02 (extended) at the indexer layer rejects inverted windows
// (to < from when both nonzero) and negative values (to < 0).
//
// Plan 39-02 / D-Milvus-02 — native ECL section metadata. Four fields
// extend the row schema (Milvus carries them as NATIVE columns, not JSON
// stuffed into source, so Phase 41 hybrid retrieval can filter on them).
// The legacy-producer contract is zero values: producers that do not set
// them seed rows with source_idx="", section="", section_idx=0 and
// section_count=0, and every existing filter keeps its semantics
// (D-Milvus-02). section_count > 0 is the OWNERSHIP MARKER: it marks the
// row as ECL-owned, which is what lets the marker-scoped purge
// (milvus.DeleteByExpr with `section_count > 0`) remove exactly the ECL
// rows inside a tier that also holds legacy rows.
type Document struct {
	ID            int64
	Tier          string
	Content       string
	Source        string
	Embedding     []float32
	EffectiveFrom int64
	EffectiveTo   int64

	// SourceIdx is the zero-padded logical note number (e.g. "001") the
	// ECL chunker assigns; "" for legacy (non-ECL) rows.
	SourceIdx string
	// Section is the merged logical-section title; "" for legacy rows.
	Section string
	// SectionIdx is the 1-based position of this section within its note
	// (first-seen order); 0 for legacy rows.
	SectionIdx int64
	// SectionCount is the number of sections in the owning note. Zero for
	// legacy rows; > 0 marks the row as ECL-owned (the ownership marker
	// the marker-scoped purge keys on).
	SectionCount int64

	// DocID is the document-level lifecycle key (Phase 53-03 / D-53-03).
	// Both producers assign it to the slash-normalized path of the source
	// document, so the two paths converge to ONE semantics after the 53-01
	// persist step (53-CONTEXT: 落盘后两条路径的 doc_id 语义归一):
	//
	//   - directory path (knowledge.Index): the path relative to
	//     knowledge/, e.g. "authority/travel_expense.md" — the same
	//     string as Source but an INDEPENDENT concept (Source is the
	//     citation display title; DocID is the upsert/delete key).
	//   - upload path (UploadIndex): the corpus-relative path the 53-01
	//     persist step wrote, e.g. "authority/<sha8>-<stem>.md".
	//
	// ZERO-VALUE LEGACY CONTRACT (the D-Milvus-02 analog): rows written
	// before Phase 53 carry DocID="". Legacy rows stay locatable by
	// Source+ID and every existing filter keeps its exact semantics —
	// nothing in Search / ListMetadata / GetChunk branches on DocID.
	DocID string
	// ChunkIndex is the 0-based position of this chunk within its doc_id's
	// CURRENT chunk sequence. The upsert rewrites it for every row of the
	// doc on each re-upload, so the sequence stays contiguous (0..N-1)
	// even when some chunks keep their old PKs. 0 for legacy rows.
	ChunkIndex int32
	// ContentHash is the sha256 hex digest of the chunk text — the upsert's
	// re-embed skip key: an incoming chunk whose hash matches an existing
	// row of the same doc_id reuses that row's PK and vector and skips the
	// BGE-M3 call. "" for legacy rows (they can never match, so an upsert
	// treats every chunk as new — the safe legacy posture).
	ContentHash string
}

// DocumentMeta is the metadata-only projection returned by
// Retriever.ListMetadata. The graph builder (internal/knowledge.GraphBuilder,
// Plan 05-05 / SC-5 / KNOW-06) reads these to construct the tier->doc->chunk
// tree without going through the vector search path. No embedding
// payload — keeps the network round-trip lean for the graph browser.
//
// Source is the slash-normalized corpus path (e.g. "authority/travel_expense.md").
// Tier is the CN-char VARCHAR ("权威" / "历史" / "个人" / "草稿"). ID is the
// numeric primary key (chunk_id used by citations).
//
// Phase 53-03: DocID / ChunkIndex / ContentHash join the projection (same
// zero-value legacy contract as Document) so the graph and admin surfaces
// can show the doc-lifecycle metadata without a second round-trip.
type DocumentMeta struct {
	ID            int64
	Tier          string
	Source        string
	EffectiveFrom int64
	EffectiveTo   int64
	DocID         string
	ChunkIndex    int32
	ContentHash   string
}

// ChunkContent is the by-id content projection used by the on-demand
// chunk-content fetch (Plan 260813-dno / UAT #4). It is deliberately
// lean: no Score (Hit carries a meaningless one) and no Embedding
// (Document carries it — a chunk-content fetch must NEVER leak the
// vector over the wire). Fields mirror DocumentMeta + the verbatim
// Content string the FE renders inside ChunkSourceView.
//
// This is the separate fetch path that preserves the graph payload
// contract (T-5-22): the graph endpoint stays metadata-only; this
// type is what GET /api/knowledge/chunk/:id returns. The D-09 trust
// boundary is preserved because the chunk id was already enumerated
// by the graph (the caller cannot invent an id the graph did not list).
//
// Phase 53-03: DocID / ChunkIndex / ContentHash join the payload (same
// zero-value legacy contract as Document) — additive JSON keys only, so
// the FE chunk surface can render the lifecycle metadata and pre-53
// responses simply carry the zero values.
type ChunkContent struct {
	ID            int64
	Tier          string
	Source        string
	Content       string
	EffectiveFrom int64
	EffectiveTo   int64
	DocID         string
	ChunkIndex    int32
	ContentHash   string
}

// Retriever is the abstraction over a tiered vector search backend.
//
// Tier-filter semantics: PRE-similarity at the SDK/Search layer
// (NEVER post-filter). Empty/zero tiers → backend returns 0 hits
// (refusal-by-construction, D-09).
//
// Plan 05-02 / D5-15: Search gains a `now int64` parameter that the
// pipeline computes once at entry. The backend applies the effective_to
// PRE-filter at the same layer as the tier filter (NOT a post-filter
// fallback). Sentinel-0 design — NOT nullable fields. The Milvus +
// Mongo backends implement `(effective_to == 0 OR effective_to >= now)`
// co-layered with the existing tier clause.
//
// Plan 05-05 / SC-5 / KNOW-06: ListMetadata returns the metadata of
// every document across all tiers (no vector search, no tier filter).
// Used by the knowledge graph browser; the endpoint streams tier+source
// +effective_* only — the verbatim chunk text is fetched on-demand by
// the ChunkSourceView when the operator clicks a chunk. Implementations
// must NOT impose an effective_to PRE-filter here (the graph is a
// browse surface that includes superseded chunks with the 已失效 badge).
//
// Implementations:
//
//   - internal/retriever/milvus — the production implementation against
//     a Milvus v2.4.2 gRPC server (Phase 2 reference impl).
type Retriever interface {
	// Health reports whether the backend is reachable. Used by cmd/server
	// startup to fail fast on a misconfigured Milvus address.
	Health(ctx context.Context) error

	// Search returns up to topK hits whose tier is in the tiers slice AND
	// whose effective_to is in the valid window (sentinel: 0 = perpetual).
	// The filter is applied PRE-similarity at the SDK layer; an empty
	// tiers slice MUST return zero hits (refusal-by-construction). The
	// now parameter is the epoch-seconds cutoff the backend uses for
	// the effective_to PRE-filter (compute once at pipeline entry).
	Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]Hit, error)

	// Ensure creates (or no-ops) the collection / index needed by Insert
	// and Search. Idempotent — safe to call on every startup.
	Ensure(ctx context.Context) error

	// Insert adds docs to the backend. Caller pre-assigns numeric IDs;
	// implementations do NOT auto-dedup. The Document.EffectiveFrom /
	// EffectiveTo fields are written into the schema (default 0 for
	// legacy documents).
	Insert(ctx context.Context, docs []Document) error

	// ListMetadata returns every document's metadata projection (no
	// embedding payload). Used by the SC-5 / KNOW-06 graph browser;
	// implementations must NOT apply the effective_to PRE-filter here
	// (the graph deliberately surfaces superseded chunks so the
	// operator can browse them with the 已失效 badge).
	ListMetadata(ctx context.Context) ([]DocumentMeta, error)

	// GetChunk looks up a single chunk by its numeric primary key and
	// returns the verbatim content + metadata. Used by the on-demand
	// chunk-content fetch (Plan 260813-dno / GET /api/knowledge/chunk/:id)
	// — the graph payload stays metadata-only (T-5-22), so chunk text
	// is fetched here ONLY when the operator clicks a chunk. Returns
	// ErrChunkNotFound when no row carries the requested id; any other
	// backend error is surfaced verbatim (the server maps it to 500).
	// The D-09 trust boundary is preserved: the id was already
	// enumerated by ListMetadata (graph), so this is a by-id READ of a
	// chunk the caller was already authorized to see.
	GetChunk(ctx context.Context, id int64) (ChunkContent, error)

	// GetMaxID returns the highest chunk ID currently stored for the given
	// tier (slug). Returns 0 (no error) when no chunks exist for the tier.
	// Implemented by MilvusClient and MongoClient (Plan 27-04).
	GetMaxID(ctx context.Context, tier string) (int64, error)

	// Disconnect closes the underlying client connection. After
	// Disconnect the instance is unusable.
	Disconnect() error
}

// CandidateCounter is an OPTIONAL capability interface (quick task
// 261027-ebw): retrievers that can cheaply report the pre-truncation
// candidate-set size implement it. The VerdictPipeline type-asserts the
// Retriever to this interface and, when supported, attaches the total to
// the found_candidates subprogress step so the UI can distinguish
// "top-5 of N" from "exactly 5". Backends that cannot answer cheaply
// simply do not implement it — the pipeline treats absence as "no total".
type CandidateCounter interface {
	// CountCandidates returns the number of documents matching the tier
	// slice and the effective_to window (the same PRE-similarity filter
	// Search applies) — i.e. what Search WOULD have had to rank, before
	// its topK cut. An empty tiers slice returns 0 (refusal-by-
	// construction, D-09).
	CountCandidates(ctx context.Context, tiers []string, now int64) (int, error)
}

// DocChunk is one EXISTING row of a single doc_id — the projection the
// doc-level upsert consumes (Phase 53-03 / D-53-03). It carries the three
// facts the upsert needs per old row: the PK it can reuse, the stored
// chunk_index it rewrites, the content_hash it compares against the
// incoming chunks, and the stored vector it re-inserts verbatim (the
// whole point of the re-embed skip: an unchanged chunk must not cost a
// BGE-M3 call).
type DocChunk struct {
	ID          int64
	ChunkIndex  int32
	ContentHash string
	Embedding   []float32
}

// DocChunkStore is the OPTIONAL doc-lifecycle capability interface
// (Phase 53-03 / D-53-03, mirroring the CandidateCounter precedent):
// backends that can address rows by (tier, doc_id) implement it, and the
// upsert in knowledge.UploadIndex type-asserts the Retriever to it.
//
// Callers that require the capability (the upload path does, whenever a
// doc_id is in play) MUST fail loudly when the assertion fails rather
// than silently falling back to bare-append: appending a re-uploaded
// document alongside its previous generation is exactly the breakpoint-C
// defect class (superseded chunks stay searchable next to their
// replacements — the 53-CONTEXT evidence). Implementations so far:
// milvus.MilvusClient and embedded.Client; the mongo backend predates
// this phase and does not implement it (its upload path fails closed
// until it does — out of 53-03 scope per the plan's files_modified).
type DocChunkStore interface {
	// DocChunkRows returns every row of one (tier, doc_id) as the DocChunk
	// projection. A doc_id with no rows returns an empty slice and nil
	// error (first upload, nothing to reuse or delete). A collection-wide
	// truncation guard mirrors scanMaxID (WR-02): a window-filling result
	// set errors instead of silently under-reporting the doc's rows.
	DocChunkRows(ctx context.Context, tier, docID string) ([]DocChunk, error)

	// DeleteDocRows deletes EVERY row of one (tier, doc_id) — the upsert's
	// delete-half (delete-all-then-insert keeps the PK non-deduplicated
	// Milvus collection free of double generations) and the DELETE
	// endpoint's row purge. Known Milvus semantics: deletes are
	// asynchronously visible, so a brief window where the old rows are
	// still served next to the freshly inserted ones is accepted by
	// design (53-CONTEXT). Errors are returned verbatim; the caller
	// decides retry/abort.
	DeleteDocRows(ctx context.Context, tier, docID string) error
}
