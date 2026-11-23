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

// Phase 54 / Plan 54-01 (ING-04/05) — the eight pipeline step functions.
//
// Each step is a faithful decomposition of the synchronous chain that
// lived in internal/server/upload.go (STEP 7-9) and
// internal/knowledge/upload_index.go through Phase 53: the semantics,
// ordering, and error-text contracts are carried over verbatim, only the
// step boundaries are new. Where a mechanic predates this phase it is
// NOT re-implemented here — the step calls the shared knowledge-package
// implementation (ChunkUploadBody / ValidateUploadTiers /
// ResolveReuseAndEmbed / PersistUploadedDoc), so there is exactly one
// chunker, one tier gate, one upsert-embed half, and one naming home
// (the 53-02 "two implementations" defect class is structurally
// impossible).
//
// Step-boundary ownership (54-01 plan): parse / vector_insert / ecl_*
// belong to this plan; chunk / embed to 54-02 (this plan ships the
// minimal-but-real versions so the chain runs end to end on fake seams).
//
// Red lines carried from 53 / D-54-01:
//   - persist (inside parse) runs BEFORE any vector insert — CONTEXT 坑 4
//     fail-closed: "insert succeeded but no file on disk" is unreachable
//     by step ordering.
//   - doc_id comes from knowledge.PersistUploadedDoc's return (the single
//     naming home — 53-03 drift fix; no second literal of the rule).
//   - the upsert's hash-unchanged re-embed skip is preserved inside
//     knowledge.ResolveReuseAndEmbed.
//   - embed failure = job failed, fail-closed (D-54-01: embedding is a
//     necessity, never degraded); the ecl steps are enhancements — their
//     absence (skipped) never blocks job=done.
package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"syscall"

	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// pipelineState is the in-memory state handed from step to step within
// one job run. Everything it carries is re-derivable from durable state
// (the job row + the 53-01 archive) EXCEPT raw — the raw bytes ride in
// memory this phase because step-product staging lands in 54-02
// (D-54-02: staging first, retry second); from 54-02 on, a retrying run
// re-reads raw from the job row's source_path archive instead.
type pipelineState struct {
	raw         []byte
	body        []byte // parse output (frontmatter-stripped)
	fm          []byte // frontmatter block (nil for non-md sources)
	ef, et      int64  // effective window parsed from fm (0,0 perpetual when absent)
	parserName  string // parse output (ParseResult.ParserName)
	docID       string // parse output (the 53-01 persist relative path)
	chunks      []string
	docs        []retriever.Document
	pks         []int64          // vector_insert output (the written PK set)
	failedItems []FailedItem     // embed output: the chunks whose embed failed (SC 7 partial semantics)
	skipPKs     map[string]int64 // embed output: content_hash -> kept PK (unchanged chunks; vector_insert preserves these verbatim)
	cacheKeys   []string         // embed output: the cache keys actually used (embeds.json content)
}

// stepParse — plan 54-01 owns this step. Verbatim migration of the sync
// chain's parse+persist sequence:
//
//  1. parserReg.Parse — format validation + ParserName capture (written
//     to the job row for 54-04 consumption). Failure = parse failed.
//  2. splitFrontmatter on the RAW bytes — the md body comes from the raw
//     bytes, NOT the parser output (the parser's goldmark-rendered plain
//     text strips markdown syntax and would re-chunk differently at the
//     next reseed — D-53-01). Empty body = parse failed.
//  3. knowledge.PersistUploadedDoc — the durable-corpus write (53-01),
//     archived-then-canonical ordering preserved inside it. Its returned
//     doc_id IS the single naming source (53-03): the upsert's delete key
//     and the canonical file's frontmatter cannot drift.
//  4. Record doc_id / parser_name / source_path on the job row.
//
// A persist failure fails THIS step (D-53-01 fail-closed in the async
// world: a document with no file on disk is not a fact). Known accepted
// half-state (documented in the plan): a LATER step failing leaves the
// canonical file on disk with a failed job — the self-healing direction
// (next reseed indexes it; 54-03 retry resumes).
//
// artifact_ref = the doc_id (the canonical file IS the durable parse
// product; 54-02 may refine the ref vocabulary).
func (r *Runner) stepParse(ctx context.Context, j Job, st *pipelineState) (string, error) {
	pr, err := r.parserReg.Parse(ctx, j.Mime, bytes.NewReader(st.raw), j.Filename)
	if err != nil {
		return "", fmt.Errorf("parse failed: %w", err)
	}
	st.parserName = pr.ParserName

	body, fm := knowledge.SplitFrontmatterRaw(st.raw)
	if len(body) == 0 {
		return "", errors.New("parse failed: document body is empty")
	}
	st.body, st.fm = body, fm

	docID, err := knowledge.PersistUploadedDoc(ctx, j.Slug, j.Filename, j.Mime, st.raw)
	if err != nil {
		return "", err // already carries the "persist failed:" prefix
	}
	st.docID = docID
	st.ef, st.et = knowledge.ParseEffectiveWindow(fm)

	sourcePath := knowledge.ArchivePath(j.SHA256, j.Filename)
	if err := r.jobs.SetJobDocInfo(ctx, j.JobID, docID, pr.ParserName, sourcePath, nowTime()); err != nil {
		return "", fmt.Errorf("record doc info on job row: %w", err)
	}
	// 54-02: stage the parse product (the frontmatter-stripped body a
	// 54-03 retry re-reads instead of re-parsing the archive).
	r.writeArtifact(j.JobID, "parsed.txt", st.body)
	return docID, nil
}

// stepChunk — minimal real implementation (54-02 adds product staging):
// the single chunker + doc-skeleton build + the KNOW-02 tier gate, in the
// same order UploadIndex ran them (chunk BEFORE the tier gate, both
// before the embedder).
//
// artifact_ref = "chunks=N" (provisional inline count; 54-02 replaces it
// with the on-disk chunk staging ref per D-54-02).
func (r *Runner) stepChunk(ctx context.Context, j Job, st *pipelineState) (string, error) {
	chunks, err := knowledge.ChunkUploadBody(ctx, st.body)
	if err != nil {
		return "", err
	}
	st.chunks = chunks

	docs := make([]retriever.Document, 0, len(chunks))
	for i, chunk := range chunks {
		docs = append(docs, retriever.Document{
			ID:            0, // assigned by the embed step (reuse keeps old PKs)
			Tier:          j.Slug,
			Content:       chunk,
			Source:        j.Filename,
			Embedding:     nil,
			EffectiveFrom: st.ef,
			EffectiveTo:   st.et,
			DocID:         st.docID,
			ChunkIndex:    int32(i),
			ContentHash:   knowledge.ContentHashHex(chunk),
		})
	}
	if err := knowledge.ValidateUploadTiers(docs, r.reg); err != nil {
		return "", err
	}
	st.docs = docs

	// 54-02: stage the chunk manifest ({chunk_index, content, content_hash}
	// per line) — the RAGFlow Task.chunk_ids equivalent lives on the
	// vector_insert side, which appends the "pk" field after the sink
	// allocates. json.Marshal for the lines: content is arbitrary user
	// text and must never break the line-oriented format.
	type chunkLine struct {
		ChunkIndex  int    `json:"chunk_index"`
		Content     string `json:"content"`
		ContentHash string `json:"content_hash"`
	}
	lines := make([]chunkLine, 0, len(docs))
	for _, doc := range docs {
		lines = append(lines, chunkLine{ChunkIndex: int(doc.ChunkIndex), Content: doc.Content, ContentHash: doc.ContentHash})
	}
	if buf, err := json.Marshal(lines); err == nil {
		r.writeArtifact(j.JobID, "chunks.jsonl_lines.json", buf)
	} else {
		log.Printf("[WARN] ingest: chunks manifest marshal: %v", err)
	}
	return fmt.Sprintf("chunks=%d", len(chunks)), nil
}

// stepEmbed — the D-53-03 upsert embed half via the shared
// knowledge.ResolveReuseAndEmbed: previous-generation lookup, hash-
// unchanged re-embed skip (red line: preserved verbatim), ID allocation,
// embed-only-fresh, fail-fast. An embed failure fails the job (D-54-01:
// embedding is a necessity; never a zero vector, Pitfall 8).
//
// artifact_ref = "embedded=N" (provisional; 54-02 wires the embed-cache
// key + per-chunk item ledger).
func (r *Runner) stepEmbed(ctx context.Context, j Job, st *pipelineState) (string, error) {
	// 54-02 three-tier lookup (D-54-02):
	//
	//  Tier 1 -- previous generation (ExistingChunks seam): unchanged
	//  hashes keep their PK AND their vector (re-inserted verbatim at
	//  vector_insert) and never touch the embedding endpoint. This is the
	//  53-03 refresh judgment moved to BEFORE embedding, its correct
	//  position per the LlamaIndex refresh pattern.
	//  Tier 2 -- the embed cache: (model, content_hash) hits resolve at
	//  zero endpoint cost (BLOB decode behind the EmbedCache seam).
	//  Tier 3 -- a fresh emb.Embed, PutEmbedding for the next run.
	//
	// An embed failure fails the job (D-54-01: embedding is a necessity;
	// never a zero vector, Pitfall 8).
	//
	// artifact_ref = embeds.json (the cache-key set actually used).
	st.skipPKs = map[string]int64{}

	if r.existing != nil {
		prev, err := r.existing(ctx, j.Slug, st.docID)
		if err != nil {
			log.Printf("[WARN] ingest: previous-generation lookup %s/%s: %v (every chunk embeds fresh)", j.Slug, st.docID, err)
			prev = nil
		}
		for i := range st.docs {
			if old, ok := prev[st.docs[i].ContentHash]; ok {
				st.skipPKs[st.docs[i].ContentHash] = old.ID
				st.docs[i].ID = old.ID
				st.docs[i].Embedding = old.Embedding
			}
		}
	}

	var (
		emb, missCount int
		cacheKeys      []string
	)
	for i := range st.docs {
		if st.docs[i].Embedding != nil {
			continue // Tier 1 hit: vector carried in from the previous generation
		}
		emb++
		cacheKeys = append(cacheKeys, st.docs[i].ContentHash)
	}

	if len(cacheKeys) > 0 && r.embedCache != nil && r.embedderModel != "" {
		hits, err := r.embedCache.GetEmbeddings(ctx, r.embedderModel, cacheKeys)
		if err != nil {
			log.Printf("[WARN] ingest: embed cache lookup: %v (every remaining chunk embeds fresh)", err)
			hits = nil
		}
		for i := range st.docs {
			if st.docs[i].Embedding != nil {
				continue
			}
			if vec, ok := hits[st.docs[i].ContentHash]; ok {
				st.docs[i].Embedding = vec // Tier 2 hit
			}
		}
	}

	for i := range st.docs {
		if st.docs[i].Embedding != nil {
			continue // Tier 1 or 2
		}
		missCount++
		vec, err := r.emb.Embed(ctx, st.docs[i].Content)
		if err != nil {
			// 54-03 SC 7 item tolerance: ONE chunk's embed failure does not
			// fail the step — the failure is LEDGERED (failedItems -> the
			// partial projection + failed_items in the GET response) and the
			// chunk's Embedding stays nil, which the vector_insert step
			// filters OUT (a failed chunk never enters the store: no fake
			// vector, no half document — D-54-01 honored at the item
			// granularity). An ALL-failed embed still fails the step below.
			st.failedItems = append(st.failedItems, FailedItem{
				Key:   st.docs[i].ContentHash,
				Error: "embed failed: " + err.Error(),
			})
			continue
		}
		st.docs[i].Embedding = vec
		if r.embedCache != nil && r.embedderModel != "" {
			if err := r.embedCache.PutEmbedding(ctx, r.embedderModel, st.docs[i].ContentHash, vec, nowTime()); err != nil {
				log.Printf("[WARN] ingest: embed cache put: %v", err)
			}
		}
	}

	if buf, err := json.Marshal(cacheKeys); err == nil {
		r.writeArtifact(j.JobID, "embeds.json", buf)
	}
	if len(st.failedItems) > 0 {
		// 54-03 SC 7: the failure ledger rides the artifacts too, so the
		// GET endpoint's failed_items projection has a durable source
		// (the in-memory slice dies with the goroutine).
		if buf, err := json.Marshal(st.failedItems); err == nil {
			r.writeArtifact(j.JobID, "embed_failed.json", buf)
		}
	}
	if len(st.failedItems) == len(st.docs) {
		// EVERY chunk failed: there is nothing to insert and nothing to
		// show for the step — fail it outright (partial needs at least one
		// success to be partial).
		return "", fmt.Errorf("embed failed: all %d chunks failed (see failed_items)", len(st.failedItems))
	}
	return fmt.Sprintf("embedded=%d cached=%d kept=%d failed=%d", emb, len(cacheKeys)-emb, len(st.skipPKs), len(st.failedItems)), nil
}

func (r *Runner) stepVectorInsert(ctx context.Context, j Job, st *pipelineState) (string, error) {
	if _, ok := r.reg.Lookup(j.Slug); !ok {
		return "", errors.New("slug deactivated during upload")
	}
	if r.sink == nil {
		return "", errors.New("vector sink not wired")
	}
	// 54-02: the sink receives the FULL doc set — the Tier 1 kept chunks
	// carry their PRESERVED PKs and original vectors, so the sink's own
	// delete+insert stays idempotent (a kept row re-inserts under its
	// same PK; nothing to insert happens only when the doc is brand new
	// and empty, unreachable -- parse rejects empty bodies).
	// 54-03 SC 7: chunks whose embed FAILED (Embedding nil, ledgered in
	// failedItems) never enter the store -- no fake vector, no half doc.
	live := make([]retriever.Document, 0, len(st.docs))
	for i := range st.docs {
		if st.docs[i].Embedding != nil {
			live = append(live, st.docs[i])
		}
	}
	pks, err := r.sink(ctx, j.Slug, st.docID, live)
	if err != nil {
		if errors.Is(err, retriever.ErrIndexerDisabled) || IsBackendUnavailable(err) {
			return "", fmt.Errorf("backend unavailable: %w", err)
		}
		return "", fmt.Errorf("insert failed: %w", err)
	}
	st.pks = pks
	return "pks=" + formatPKs(pks), nil
}

func (r *Runner) stepECLSkipped(_ context.Context, _ Job, _ *pipelineState) (string, error) {
	const reason = "ecl auto-trigger not wired (54-04)"
	if r.eclTrigger != nil {
		// Unreachable in 54-01 (main.go wires nil; 54-04 owns the wired
		// path). Defensive: a wired trigger without wiring code must not
		// silently claim done.
		return "", fmt.Errorf("ecl trigger wired but step not implemented until 54-04")
	}
	return reason, nil
}

// formatPKs renders the vector_insert artifact list ("pks=<csv>"). Empty
// inserts render "pks=" (a zero-chunk doc cannot reach this step — the
// parse step rejects empty bodies — but the renderer stays total).
func formatPKs(pks []int64) string {
	out := make([]byte, 0, len(pks)*8)
	for i, pk := range pks {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, fmt.Sprintf("%d", pk)...)
	}
	return string(out)
}

// IsBackendUnavailable reports whether err is a network-level backend
// failure (the retired server/upload.go helper, migrated verbatim — the
// step machine now owns the classification since the HTTP handler no
// longer inserts). Phase 32 (REALBUG-03) replaced strings.Contains
// matching with errors.Is + net.Error.Timeout(); that shape is preserved.
func IsBackendUnavailable(err error) bool {
	if err == nil {
		return false
	}
	// DNS resolution failure (caller may have renamed host)
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// Connection refused / network unreachable / host down
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	// Context timeout / cancellation propagates from request lifecycle
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return true
	}
	// Generic net.Error timeout (covers *net.OpError wrapping)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// logStepFailure is the runner-side WARN for a failed step — never silent
// (the ecl goroutine posture): the step row is the queryable record, the
// log line is the operator's immediate signal.
func logStepFailure(jobID, step string, err error) {
	log.Printf("[WARN] ingest: job %s step %s failed: %v", jobID, step, err)
}
