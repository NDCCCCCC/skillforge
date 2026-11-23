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

// Phase 54 / Plan 54-01 — upload persistence, migrated verbatim from
// internal/server/upload.go (where 53-01 authored it). The ingest package
// cannot import internal/server (a server→ingest→server cycle), so the
// durable-corpus writer now lives beside the corpus loader it feeds.
// Signature and behavior are unchanged; the only code-level differences
// are (a) the roots are package-local vars here with exported
// getters/setters replacing the server package's, and (b) the raw-bytes
// archive path derivation is single-homed in ArchivePath so the ingest
// parse step records the same source_path PersistUploadedDoc actually
// wrote (54-03 retry's staging source — a second derivation could drift
// from the file on disk).
package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/kb"
)

// Upload persistence constants (D-53-01). knowledge/ is the single source
// of truth: every uploaded document is written there before the vector
// insert, so the boot-time seedDocs tier purge re-indexes uploads instead
// of silently evaporating them (Phase 53 breakpoint A).
const (
	// mimeMarkdown is the only MIME type whose canonical file body is the
	// frontmatter-stripped RAW markdown (see persistBodyFor — load-bearing
	// for restart chunk equivalence).
	mimeMarkdown = "text/markdown"

	// persistExtMarkdown is the canonical file extension: even PDF/DOCX
	// uploads land as .md because the persisted body is extracted TEXT.
	persistExtMarkdown = ".md"

	// persistSHA8Len is the sha256-hex prefix used in canonical file names.
	// 8 hex chars (32 bits) make accidental same-name collisions negligible
	// and full-content collisions are already excluded by SHA-256 dedup.
	persistSHA8Len = 8

	// archiveExtFallback names extension-less uploads in the raw-bytes
	// archive so the file still carries a conventional suffix.
	archiveExtFallback = ".bin"

	// defaultPersistUploadsDir is the raw-bytes archive dir, following the
	// runtime data-tree convention (cf. SKILLFORGE_DATASETS_DIR →
	// data/datasets, ontology → data/ontology).
	defaultPersistUploadsDir = "data/uploads"

	// persistDirMode / persistFileMode are the on-disk modes for the two
	// persist targets. 0o755 dirs keep tier dirs browsable by operators;
	// 0o644 files are group/world readable like the seeded corpus files.
	persistDirMode  = 0o755
	persistFileMode = 0o644
)

// The two persist roots. persistKnowledgeRoot is the corpus root
// PersistUploadedDoc writes canonical knowledge files under (default
// matches the seed root the boot Index walks); persistUploadsDir is the
// raw-bytes archive root. Package vars (not consts) following the
// server package's posture they replace: bound at boot, and the upload
// pipeline only READS them (the per-slug mutex already serializes
// same-slug persist writes).
var (
	persistKnowledgeRoot = "knowledge/"
	persistUploadsDir    = defaultPersistUploadsDir
)

// SetPersistKnowledgeRoot binds the corpus root PersistUploadedDoc writes
// canonical knowledge files under. Production never calls this (the
// default matches the seed root); tests use it with t.Cleanup restore.
func SetPersistKnowledgeRoot(root string) { persistKnowledgeRoot = root }

// SetPersistUploadsDir binds the raw-bytes archive dir. Same posture as
// SetPersistKnowledgeRoot.
func SetPersistUploadsDir(dir string) { persistUploadsDir = dir }

// PersistUploadsDir reports the raw-bytes archive root. The ingest parse
// step reads it to record the job row's source_path — the SAME
// derivation PersistUploadedDoc writes to (via ArchivePath), never a
// second copy of the rule.
func PersistUploadsDir() string { return persistUploadsDir }

// PersistKnowledgeRoot reports the canonical corpus root (the setter's
// counterpart). The server package's read side (docs list / chunk source
// view) and delete side (docDeleteHandler file removal) read through this
// instead of keeping their own root var — the 53-03 review unified the
// roots once already; 54-01's move of the persist path into this package
// would have re-created the parallel-root bug, so this getter is the
// single source of truth for every consumer.
func PersistKnowledgeRoot() string { return persistKnowledgeRoot }

// ArchivePath returns the raw-bytes archive path for one upload:
// <persistUploadsDir>/<sha256><ext> (extension-less uploads get the
// .bin fallback). Single home for the rule: PersistUploadedDoc writes
// through it and the ingest parse step records the job row's source_path
// through it, so the pointer can never drift from the file on disk.
func ArchivePath(sha256Hex, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = archiveExtFallback
	}
	return filepath.Join(persistUploadsDir, sha256Hex+ext)
}

// PersistedDocName derives the canonical knowledge-file name for one
// upload: "<sha8>-<sanitized-stem>.md" (persistSHA8Len hex prefix of the
// file's sha256 + the kb-whitelisted stem). Extracted from
// PersistUploadedDoc in Phase 53-03 so the doc_id the UPLOAD pipeline
// feeds the upsert (before persist even runs) and the doc_id written into
// the canonical file's frontmatter derive from ONE naming home — a second
// literal copy of the rule would let the upsert's delete key drift from
// the file the persist step writes, resurrecting the doc under a new
// name at the next reseed.
func PersistedDocName(sha256Hex, filename string) string {
	return sha256Hex[:persistSHA8Len] + "-" + kb.SanitizeFileStem(filename) + persistExtMarkdown
}

// PersistedDocID returns the corpus-relative doc_id of one upload:
// "<slug>/<PersistedDocName>" — the lifecycle key the upsert and the
// DELETE endpoint address the doc by, identical to the returned value of
// PersistUploadedDoc (53-CONTEXT: 落盘后两条路径的 doc_id 语义归一).
func PersistedDocID(slug, sha256Hex, filename string) string {
	return slug + "/" + PersistedDocName(sha256Hex, filename)
}

// PersistUploadedDoc writes an uploaded document into the durable corpus
// so knowledge/ stays the single source of truth (Phase 53-01 / D-53-01).
// Without this the upload's chunks live only in the vector store, and the
// boot-time seedDocs tier purge (indexer.go: DeleteTierRows → reseed
// directory docs only) silently evaporates them — Phase 53 breakpoint A.
//
// Two writes, in this order:
//
//  1. Archive — the ORIGINAL bytes at <persistUploadsDir>/<sha256><ext>
//     (raw provenance for forensics/re-import; never indexed by the walk,
//     which only reads knowledge/<slug>/*.md).
//  2. Canonical file — <persistKnowledgeRoot>/<slug>/<sha8>-<stem>.md:
//     provenance frontmatter (source_sha256 / source_mime / uploaded_at /
//     doc_id) + the exact body the chunker consumed (persistBodyFor), so
//     a boot reseed re-chunks it into IDENTICAL chunks.
//
// Ordering is load-bearing in BOTH directions:
//   - archive BEFORE canonical file: if the archive write fails we return
//     before touching knowledge/, so a failed upload can never leave a
//     canonical file on disk that the next reseed would index even though
//     the client saw a rejection.
//   - the caller invokes this BEFORE any vector insert (Phase 53 STEP 7d
//     before STEP 8; in the 54-01 step machine, inside the parse step,
//     three steps before vector_insert): "insert succeeded but no file on
//     disk" is the one unrecoverable direction (CONTEXT 坑 4 fail-closed).
//     The converse — canonical file present, a later step failed — is
//     self-healing: the next reseed indexes the canonical file like any
//     directory doc, and 54-03's step retry resumes from it.
//
// Any write error is returned wrapped with a "persist failed" prefix; the
// ingest parse step maps it to step=failed + job=failed. Re-uploading the
// same bytes overwrites both targets (sha-named → idempotent; the sha8
// truncation collision window is ignored per plan). The returned path is
// the slash-normalized corpus-relative doc_id, which after this write
// gives uploaded docs the SAME doc_id semantics as directory docs
// (53-CONTEXT: 落盘后两条路径的 doc_id 语义归一).
func PersistUploadedDoc(ctx context.Context, slug, filename, mimeType string, raw []byte) (string, error) {
	hash := sha256.Sum256(raw)
	sha256Hex := hex.EncodeToString(hash[:])

	body, fm, err := persistBodyFor(ctx, mimeType, filename, raw)
	if err != nil {
		return "", fmt.Errorf("persist failed: derive body: %w", err)
	}
	// Effective-window passthrough (53-01 review fix): the canonical
	// file must carry the SOURCE window so reseed reads back what
	// upload-time docs carried (equivalence covers the window too).
	ef, et := ParseEffectiveWindow(fm)
	if len(body) == 0 {
		// Unreachable via the upload path (the parse step already rejects
		// an empty body before persisting) — guarded here so direct
		// callers of this function can never persist an empty canonical
		// file.
		return "", errors.New("persist failed: derived body is empty")
	}

	name := PersistedDocName(sha256Hex, filename)
	docID := slug + "/" + name

	// Write 1: raw-bytes archive (ordering rationale above).
	archivePath := ArchivePath(sha256Hex, filename)
	if err := os.MkdirAll(persistUploadsDir, persistDirMode); err != nil {
		return "", fmt.Errorf("persist failed: archive dir: %w", err)
	}
	if err := os.WriteFile(archivePath, raw, persistFileMode); err != nil {
		return "", fmt.Errorf("persist failed: archive %s: %w", archivePath, err)
	}

	// Write 2: canonical knowledge file.
	dir := filepath.Join(persistKnowledgeRoot, slug)
	if err := os.MkdirAll(dir, persistDirMode); err != nil {
		return "", fmt.Errorf("persist failed: knowledge dir %s: %w", dir, err)
	}
	file := buildPersistedMarkdown(sha256Hex, mimeType, docID, time.Now().UTC().Format(time.RFC3339), ef, et, body)
	if err := os.WriteFile(filepath.Join(dir, name), file, persistFileMode); err != nil {
		return "", fmt.Errorf("persist failed: knowledge file %s: %w", filepath.Join(dir, name), err)
	}
	return docID, nil
}

// persistBodyFor selects the body bytes written into the canonical
// knowledge file. The two branches are load-bearing for restart chunk
// equivalence (D-53-01):
//
//   - text/markdown → the frontmatter-stripped RAW body. The upload
//     pipeline chunks exactly this byte span (its md-parser output is
//     discarded); the parser's goldmark-rendered plain text strips
//     markdown syntax and would re-chunk differently at the next reseed,
//     silently diverging the corpus from the upload-time chunk set.
//
//   - every other registered type → the parser's plain-text Content.
//     splitFrontmatter on raw PDF/DOCX bytes yields the binary itself,
//     and a knowledge/*.md containing those bytes is garbage on reseed.
//     (The upload path's raw-binary chunking for non-md types is a
//     pre-existing defect out of 53-01 scope; persisting extracted text
//     means the DISK copy — the D-53-01 source of truth — is the correct
//     one, and the next reseed converges the vector store onto it.)
func persistBodyFor(ctx context.Context, mimeType, filename string, raw []byte) (body, frontmatter []byte, err error) {
	if mimeType == mimeMarkdown {
		body, fm := SplitFrontmatterRaw(raw)
		return body, fm, nil
	}
	pr, err := document.NewParserRegistry().Parse(ctx, mimeType, bytes.NewReader(raw), filename)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s for persist: %w", filename, err)
	}
	// Non-md sources carry no frontmatter concept: nil fm -> (0,0) window,
	// which is exactly how the directory path treats them.
	return pr.Content, nil, nil
}

// buildPersistedMarkdown renders the canonical file: provenance YAML
// frontmatter + the chunker's source body. Frontmatter values are all
// whitelist-constrained — hex digest, a ParserRegistry-allowlisted MIME
// (enforced upstream by the magic-bytes gate), a sanitized stem +
// validated slug in doc_id, an RFC3339 timestamp — so plain-scalar YAML
// emission is injection-safe without quoting. The trailing newline keeps
// the body byte-exact across the write/re-read round trip without gluing
// the last line to whatever follows.
func buildPersistedMarkdown(sha256Hex, mimeType, docID, uploadedAt string, ef, et int64, body []byte) []byte {
	// NOTE: format-verb order MUST match the argument order below —
	// 53-01 review caught docID/uploadedAt swapped into each other's
	// fields (uploaded_at rendered the corpus path, doc_id the timestamp).
	fm := fmt.Sprintf("source_sha256: %s\nsource_mime: %s\nuploaded_at: %s\ndoc_id: %s\neffective_from: %d\neffective_to: %d\n",

		sha256Hex, mimeType, uploadedAt, docID, ef, et)

	out := make([]byte, 0, len(fm)+len(body)+8)
	out = append(out, "---\n"...)
	out = append(out, fm...)
	out = append(out, "---\n"...)
	out = append(out, body...)
	if body[len(body)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}
