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
// hand-authored policy markdown into retriever Document rows with correct
// knowledge-base tier metadata.
package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
)

// uploadAuditHook is the audit callback for upload pipeline events.
// Mirrors the indexAuditHook pattern from indexer.go.
var uploadAuditHook func(action, detail string)

// SetUploadAuditHook binds the production audit sink for the upload pipeline.
// Called once at boot, before any UploadIndex call. nil means "do not audit".
func SetUploadAuditHook(fn func(action, detail string)) { uploadAuditHook = fn }

// UploadIndex parses a document, chunks it, and returns the resulting
// []retriever.Document ready for backend Insert — running the D-53-03
// doc-level UPSERT against the doc's previous generation first.
//
// Phase 54-01: this function is now a COMPOSITION of the same shared
// mechanics the async ingest step machine runs (single-implementation
// discipline — 53-02's "two chunkers" bug class). The ingest package
// cannot be imported from here (ingest imports knowledge for the persist
// step; the reverse would cycle), so the shared mechanics live HERE as
// exports and both callers compose them:
//
//   - ChunkUploadBody      — the ONLY chunker (buildSplitter + splitBody,
//     the 53-02 CJK-aware recursive splitter).
//   - ValidateUploadTiers  — the KNOW-02 gate (hook-aware).
//   - ResolveReuseAndEmbed — the D-53-03 upsert embed half (previous-
//     generation lookup, hash-unchanged re-embed skip, ID allocation,
//     embed-only-fresh).
//
// The upload handler's synchronous call path is retired in 54-01 (the
// async ingest Runner owns the pipeline now); the function remains for
// its direct callers and tests with its signature unchanged.
//
// docID is the document-lifecycle key (Phase 53-03): the corpus-relative
// path the 53-01 persist step writes the canonical file to
// ("authority/<sha8>-<stem>.md"). After persist, an uploaded doc and a
// directory doc share the exact same doc_id semantics (53-CONTEXT). The
// caller derives it BEFORE this call — via the knowledge PersistedDocID
// helper — because the upsert lookup needs the key before any backend
// read. An EMPTY docID selects the legacy append posture (no lookup, no
// delete, every chunk fresh): it exists for direct/test callers only; the
// production upload path always passes a real docID.
//
// The upsert (D-53-03; PK stays self-incrementing, no hash64 PK change):
//
//  1. Look up the existing rows of (tier=slug, doc_id) through the
//     retriever.DocChunkStore capability (type-asserted on r, the
//     CandidateCounter precedent). A backend that does not implement the
//     capability fails the upload LOUDLY whenever docID is set — silently
//     appending a re-upload alongside its previous generation is exactly
//     the breakpoint-C defect class (superseded chunks stay searchable),
//     and the fail-closed philosophy forbids hiding that.
//  2. An incoming chunk whose content_hash matches an existing row's hash
//     REUSES that row's PK and stored vector — zero BGE-M3 calls for
//     unchanged content (the LlamaIndex refresh pattern). Reused rows
//     keep non-contiguous PKs by design; their chunk_index is rewritten
//     to the new sequence position.
//  3. Changed / new chunks get fresh PKs from GetMaxID(slug)+1 upward and
//     are the ONLY chunks that cost an embedding call. Embed failure is a
//     hard error — never a zero vector (Pitfall 8).
//  4. ALL old rows of the doc are deleted via DocChunkStore.DeleteDocRows
//     BEFORE the caller Inserts (Milvus does not deduplicate Insert by
//     PK, so delete-then-insert is the only ordering that cannot stack a
//     second searchable generation — the 2931/256 incident class). The
//     accepted transient: Milvus deletes are asynchronously visible, so a
//     brief old+new coexistence window is tolerable (53-CONTEXT). If the
//     caller's Insert then fails, the doc is absent until the next boot
//     reseed self-heals it from the canonical file — the same posture a
//     first-time-upload Insert failure already has.
//
// UploadIndex does NOT call Insert — the caller decides when to Insert
// and handles backend errors. This keeps UploadIndex testable without a
// real Milvus/Mongo backend.
//
// Behavior summary:
//   - Slug is validated against the registry before any I/O (fail-closed).
//   - validateTiers runs BEFORE the embedder (KNOW-02 fail-closed gate).
//   - The upsert deletes the old generation; the caller inserts the new
//     one — the delete+insert pair is the caller's transaction boundary.
//   - nextID starts at GetMaxID(slug)+1, or 1 if no chunks exist.
//   - No temp files: parsing operates on in-memory bytes via io.Reader.
func UploadIndex(
	ctx context.Context,
	slug string,
	fileBytes []byte,
	mimeType string,
	filename string,
	docID string,
	emb embedder.Embedder,
	reg kb.Registry,
	r retriever.Retriever,
) ([]retriever.Document, error) {
	// Step 1: slug validation (fail-closed gate).
	if err := validateSlug(slug, reg); err != nil {
		return nil, err
	}

	// Step 2: parse via the document parser registry.
	// NOTE: parser.Parse strips frontmatter internally and returns the
	// stripped body in Content. The parser's ParseResult.Frontmatter field
	// is NOT populated by ParserRegistry.Parse (it is set only by parsers
	// that call their own frontmatter split internally). We re-extract
	// frontmatter from the raw bytes below so we can parse effective_*.
	parserReg := document.NewParserRegistry()
	_, err := parserReg.Parse(ctx, mimeType, bytes.NewReader(fileBytes), filename)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	// Step 3: extract body and frontmatter from raw bytes.
	// We re-run splitFrontmatter on the original bytes because the parser
	// already stripped frontmatter from pr.Content, and ParserRegistry.Parse
	// does not propagate the frontmatter bytes to ParseResult.Frontmatter.
	body, fm := splitFrontmatter(fileBytes)
	if len(body) == 0 {
		return nil, fmt.Errorf("parse failed: document body is empty")
	}

	// Step 4: chunk via the single chunker (53-02 / D-53-02: chunkBody's
	// byte-window chunking is retired — two chunk policies inside one tier
	// made retrieval distributions incomparable; the cross-path
	// equivalence test pins byte-identical output).
	chunks, err := ChunkUploadBody(ctx, body)
	if err != nil {
		return nil, err
	}

	// Step 5: parse effective window from frontmatter (0,0 if absent).
	ef, et := parseEffectiveWindow(fm)

	// Step 6: build the doc slice with Tier + the 53-03 doc-lifecycle
	// triple set, so validateTiers can check each doc's tier against the
	// registry and the upsert can hash-match against the backend's stored
	// hashes. KNOW-02 gate runs BEFORE the embedder.
	docs := make([]retriever.Document, 0, len(chunks))
	for i, chunk := range chunks {
		docs = append(docs, retriever.Document{
			ID:            int64(i), // placeholder; will be overwritten below
			Tier:          slug,
			Content:       chunk,
			Source:        filename,
			Embedding:     nil, // filled in embed step (reused chunks: from the backend)
			EffectiveFrom: ef,
			EffectiveTo:   et,
			DocID:         docID,
			ChunkIndex:    int32(i),
			ContentHash:   contentHashHex(chunk),
		})
	}
	if err := ValidateUploadTiers(docs, reg); err != nil {
		return nil, err
	}

	// Steps 7-8 (Phase 53-03 / D-53-03 upsert): previous-generation
	// lookup, hash-unchanged re-embed skip, ID allocation, and selective
	// embedding — one shared implementation with the ingest embed step.
	docs, prevGeneration, err := ResolveReuseAndEmbed(ctx, slug, docID, docs, emb, r)
	if err != nil {
		return nil, err
	}

	// Step 9 (Phase 53-03 / D-53-03 upsert, part 2): delete the doc's
	// ENTIRE previous generation. Ordering is load-bearing: this runs
	// AFTER the embed (the slow part) so the old generation stays
	// searchable as long as possible, and BEFORE the function returns
	// because the caller Inserts immediately after — Milvus does not
	// deduplicate Insert by PK, so delete-then-insert is the only ordering
	// that cannot stack two searchable generations (the reseed
	// write-amplification incident class). The empty-docID legacy posture
	// skips this step (nothing is addressable without a doc_id).
	if docID != "" && prevGeneration > 0 {
		lifecycle := r.(retriever.DocChunkStore) // asserted non-nil in ResolveReuseAndEmbed
		if err := lifecycle.DeleteDocRows(ctx, slug, docID); err != nil {
			return nil, fmt.Errorf("doc lifecycle delete for doc_id %q: %w", docID, err)
		}
	}

	return docs, nil
}

// ChunkUploadBody chunks an upload body with the corpus loader's ONE
// chunker: buildSplitter (the CJK-aware recursive splitter, 53-02 /
// D-53-02) + splitBody. Exported so the async ingest chunk step (54-01)
// and this package's UploadIndex compose the same implementation — a
// second chunker anywhere in the upload surface is the exact defect class
// 53-02 retired (cross-path equivalence test pins byte-identical chunks).
//
// Error wrapping matches the retired inline call shape ("build splitter:"
// / "split body:") so error-text assertions survive the decomposition.
func ChunkUploadBody(ctx context.Context, body []byte) ([]string, error) {
	splitter, err := buildSplitter(ctx)
	if err != nil {
		return nil, fmt.Errorf("build splitter: %w", err)
	}
	chunks, err := splitBody(ctx, splitter, string(body))
	if err != nil {
		return nil, fmt.Errorf("split body: %w", err)
	}
	return chunks, nil
}

// ValidateUploadTiers is the KNOW-02 fail-closed gate for upload-produced
// docs (empty/unknown/deactivated tier rejected before any embedder or
// backend call — refusal-masking moat, T-3-MASK). Hook-aware exactly like
// the retired inline dispatch: a test-installed validateTiersHook wins,
// otherwise the real validateTiers runs. Exported so the ingest chunk
// step gates through the same implementation.
func ValidateUploadTiers(docs []retriever.Document, reg kb.Registry) error {
	if validateTiersHook != nil {
		return validateTiersHook(docs, reg)
	}
	return validateTiers(docs, reg)
}

// ResolveReuseAndEmbed is the D-53-03 upsert EMBED half, shared by
// UploadIndex and the ingest embed step (Phase 54-01): resolve the doc's
// previous generation through the DocChunkStore capability, reuse
// unchanged rows (hash match → same PK + stored vector, zero embed
// calls), allocate fresh IDs from the tier watermark, and embed ONLY the
// fresh chunks. Embed failure is a hard error — never a zero vector
// (Pitfall 8; D-54-01: embedding is a necessity, fail-closed).
//
// Returns the docs (IDs + embeddings assigned) and prevGeneration — the
// size of the previous generation found. UploadIndex uses it to decide
// whether a delete is due (step 9 above); the ingest path ignores it (its
// vector_insert step deletes inside the VectorSink before inserting).
//
// Empty docID = the legacy append posture (see the UploadIndex godoc):
// no lookup, no reuse, every chunk fresh.
func ResolveReuseAndEmbed(
	ctx context.Context,
	slug string,
	docID string,
	docs []retriever.Document,
	emb embedder.Embedder,
	r retriever.Retriever,
) ([]retriever.Document, int, error) {
	// Resolve the doc's previous generation (fail-closed on a backend
	// without the DocChunkStore capability whenever docID is set).
	var existing []retriever.DocChunk
	if docID != "" {
		lifecycle, ok := r.(retriever.DocChunkStore)
		if !ok {
			return nil, 0, fmt.Errorf(
				"retriever backend does not support doc lifecycle (retriever.DocChunkStore) — "+
					"required to upsert doc_id %q (fail-closed: silently appending a re-upload alongside "+
					"its previous generation would leave superseded chunks searchable)", docID)
		}
		rows, err := lifecycle.DocChunkRows(ctx, slug, docID)
		if err != nil {
			return nil, 0, fmt.Errorf("doc lifecycle lookup for doc_id %q: %w", docID, err)
		}
		existing = rows
	}
	prevGeneration := len(existing)

	// Reuse map: stored content_hash → the existing row (PK + vector).
	// First occurrence wins so a duplicated stored hash cannot alias two
	// rows; a consumed entry is removed so two IDENTICAL chunks in the new
	// generation never both reuse the same PK (they would have collided
	// on insert).
	reuse := make(map[string]retriever.DocChunk, len(existing))
	for _, row := range existing {
		if row.ContentHash == "" {
			// Legacy row (pre-53): no hash to match — it will be deleted
			// wholesale by the delete half and never reused. Skipping it
			// here is what keeps the zero-value legacy contract
			// one-directional: legacy rows stay readable, but an upsert
			// never trusts them.
			continue
		}
		if _, dup := reuse[row.ContentHash]; !dup {
			reuse[row.ContentHash] = row
		}
	}

	// ID allocation + selective embedding. Reused chunks keep their PK
	// and stored vector (NO embed call — the whole point of the re-embed
	// skip); fresh chunks draw IDs from the tier watermark and are the
	// only ones that reach the embedder.
	startID, err := getUploadNextID(ctx, slug, r)
	if err != nil {
		return nil, 0, err
	}
	nextFresh := startID
	for i := range docs {
		if old, ok := reuse[docs[i].ContentHash]; ok {
			if len(old.Embedding) == 0 {
				// A stored row without a usable vector cannot be re-inserted
				// (SeedDocuments rejects dim-mismatched embeddings loudly).
				// Fail here with the doc context instead of surfacing a bare
				// backend dimension error two steps later.
				return nil, 0, fmt.Errorf(
					"doc lifecycle reuse for doc_id %q: existing row id=%d carries no stored vector; "+
						"re-upload is still possible but this row must be re-embedded (purge the doc and retry)",
					docID, old.ID)
			}
			docs[i].ID = old.ID
			docs[i].Embedding = old.Embedding
			docs[i].ChunkIndex = int32(i) // rewritten to the new sequence position
			delete(reuse, docs[i].ContentHash)
			continue
		}
		docs[i].ID = nextFresh
		nextFresh++
	}
	// Embed ONLY the fresh chunks (reused ones carry their stored vector).
	// Embed failure is a hard error — never fall back to a zero vector.
	for i := range docs {
		if docs[i].Embedding != nil {
			continue
		}
		vec, err := emb.Embed(ctx, docs[i].Content)
		if err != nil {
			return nil, 0, fmt.Errorf("embed failed: %w", err)
		}
		docs[i].Embedding = vec
	}
	return docs, prevGeneration, nil
}

// validateSlug is the fail-closed gate for the upload target. It returns
// nil when slug is registered AND active. It returns an error for empty,
// unknown, or deactivated slugs before any I/O occurs.
//
// Mirrors the three-state decision table from WalkRegisteredDirs but
// surfaces the error to the caller rather than logging and skipping.
func validateSlug(slug string, reg kb.Registry) error {
	if slug == "" {
		return errors.New("slug is empty")
	}
	// Check whether the slug is known and active using Lookup (fail-closed).
	if _, ok := reg.Lookup(slug); !ok {
		return fmt.Errorf("kb %q is not registered or deactivated", slug)
	}
	return nil
}

// getUploadNextID queries the backend for the current maximum chunk ID
// for slug, then returns max+1. If no chunks exist (GetMaxID returns 0),
// returns 1. This ensures uploaded chunks never collide with boot-seeded
// IDs (indexer.go starts at 1 on a fresh collection; uploads continue
// from wherever the backend ceiling is).
func getUploadNextID(ctx context.Context, slug string, r retriever.Retriever) (int64, error) {
	maxID, err := r.GetMaxID(ctx, slug)
	if err != nil {
		return 0, fmt.Errorf("get max id for slug %q: %w", slug, err)
	}
	return maxID + 1, nil
}

// validateTiersHook allows tests to inject a failure for KNOW-02 gate testing.
// When non-nil, called instead of the package-level validateTiers.
var validateTiersHook func([]retriever.Document, kb.Registry) error

// SplitFrontmatterRaw exposes the corpus loader's frontmatter splitter to
// the upload persist path (internal/server, Phase 53-01 / D-53-01).
//
// Why exported: the canonical knowledge file written at upload time must
// carry the EXACT body bytes UploadIndex chunked (splitFrontmatter on the
// raw upload bytes — NOT the md parser's goldmark-rendered plain text,
// which strips markdown syntax and would re-chunk differently). Without
// this guarantee the boot reseed would diverge from the upload-time chunk
// set, silently defeating the restart-evaporation fix.
//
// Returns (frontmatter-stripped body, raw YAML block or nil), mirroring
// splitFrontmatter. The persist path ignores the YAML block: uploaded
// docs are unversioned on reseed (parseEffectiveWindow reads only the
// provenance frontmatter the persist path writes, which has no
// effective_* fields → (0,0) perpetual — the 53-CONTEXT accepted
// behavior).
func SplitFrontmatterRaw(raw []byte) (body []byte, frontmatter []byte) {
	return splitFrontmatter(raw)
}

// ParseEffectiveWindow exposes the corpus loader's effective-window
// parser to the upload persist path (53-01): the canonical knowledge file
// must carry the SOURCE document's effective_from / effective_to so a boot
// reseed reads back the same window the upload-time docs carried — without
// this passthrough an upload with a bounded window silently degrades to
// perpetual (0,0) at the next reseed, breaking restart chunk equivalence.
// nil frontmatter (non-md sources) yields the (0,0) perpetual sentinel,
// exactly matching the directory-path behavior for frontmatter-less files.
func ParseEffectiveWindow(fm []byte) (effectiveFrom, effectiveTo int64) {
	return parseEffectiveWindow(fm)
}
