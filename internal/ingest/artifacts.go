// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"os"
	"path/filepath"
)

// ArtifactStore is the per-job staging root (54-02 / D-54-02: staging
// BEFORE retry — a retry without locatable step products can only re-run
// the whole chain). Products land under <root>/<job_id>/ as small files,
// one conventional name per step:
//
//	parse        → parsed.txt    (the extracted body)
//	chunk        → chunks.jsonl  (one {chunk_index, content, content_hash}
//	                               per line; the vector_insert step appends
//	                               the "pk" field back onto each line — the
//	                               RAGFlow Task.chunk_ids equivalent, so a
//	                               retry can purge stragglers by PK)
//	embed        → embeds.json   (the [{model, content_hash}] cache-key set
//	                               actually used; vectors live in the
//	                               embed_cache table, not on disk)
//	vector_insert→ pks.json      (the written PK set — a retry purges by
//	                               this list BEFORE re-inserting)
//	ecl_*        → ecl_<step>.json
//
// Write order is deliberately "file first, step-row done second": a crash
// between the two leaves a file without a done marker, which is harmless —
// every step is idempotent and a re-run overwrites. There is NO automatic
// GC in this phase (products are small, dirs are job-isolated, job rows
// are queryable); the DELETE endpoint (53-03) calls PurgeArtifactsByDoc so
// the tree stays bounded.
//
// The directory convention mirrors internal/duckdb/staging.go (same volume,
// same git-ignore semantics for data/ingest); the MECHANISM does not —
// duckdb staging is whole-file rename-commit for one big dataset file,
// while ingest products are many-and-small with step-level bookkeeping
// already living in the ingest_steps table (no third ledger).
type ArtifactStore struct {
	root string
}

// NewArtifactStore binds the staging root (production: "data/ingest" —
// same volume and git-ignore posture as data/staging and data/datasets;
// tests: t.TempDir()).
func NewArtifactStore(root string) *ArtifactStore {
	return &ArtifactStore{root: root}
}

// Dir returns (and creates) the per-job product directory.
func (a *ArtifactStore) Dir(jobID string) (string, error) {
	dir := filepath.Join(a.root, jobID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Write atomically-enough writes one product file (write-temp-rename so a
// crash mid-write never leaves a half file under the canonical name — the
// file-then-done ordering in the step contract relies on this).
func (a *ArtifactStore) Write(jobID, name string, data []byte) error {
	dir, err := a.Dir(jobID)
	if err != nil {
		return err
	}
	final := filepath.Join(dir, name)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// Read returns one product file's bytes (os.ErrNotExist propagates — the
// caller decides whether absence is a retry precondition or an error).
func (a *ArtifactStore) Read(jobID, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(a.root, jobID, name))
}

// PurgeArtifactsByDoc removes the product directories of jobs whose
// job-row source doc matches docID. v1 keeps it simple: the caller (the
// 53-03 DELETE handler) passes the job IDs it already knows; a doc that
// never ingested has no directory and removing nothing is success.
func (a *ArtifactStore) PurgeArtifacts(jobIDs []string) {
	for _, id := range jobIDs {
		_ = os.RemoveAll(filepath.Join(a.root, id))
	}
}
