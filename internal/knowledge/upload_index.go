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

// UploadIndex parses a document, chunks it, embeds the chunks, and returns
// the resulting []retriever.Document ready for backend Insert.
//
// UploadIndex does NOT call Insert — the caller (the upload handler, Plan 27-03)
// decides when to Insert and handles backend errors. This keeps UploadIndex
// testable without a real Milvus/Mongo backend.
//
// Behavior summary:
//   - Slug is validated against the registry before any I/O (fail-closed).
//   - validateTiers runs BEFORE the embedder (KNOW-02 fail-closed gate).
//   - Embed failure returns an error — never falls back to a zero vector.
//   - nextID starts at GetMaxID(slug)+1, or 1 if no chunks exist.
//   - No temp files: parsing operates on in-memory bytes via io.Reader.
func UploadIndex(
	ctx context.Context,
	slug string,
	fileBytes []byte,
	mimeType string,
	filename string,
	emb embedder.Embedder,
	reg kb.Registry,
	r retriever.Retriever,
) ([]retriever.Document, error) {
	// Step 1: slug validation (fail-closed gate).
	if err := validateSlug(slug, reg); err != nil {
		return nil, err
	}

	// Step 2: parse via the document parser registry.
	parserReg := document.NewParserRegistry()
	pr, err := parserReg.Parse(ctx, mimeType, bytes.NewReader(fileBytes), filename)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	// Step 3: extract body and frontmatter.
	// If frontmatter is present, strip it from the body (the frontmatter
	// metadata is consumed separately below; it must not be embedded).
	var body []byte
	if pr.Frontmatter != nil {
		raw := append(pr.Frontmatter, []byte("\n---\n")...)
		raw = append(raw, pr.Content...)
		body, _ = splitFrontmatter(raw)
	} else {
		body = pr.Content
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("parse failed: document body is empty")
	}

	// Step 4: chunk if body exceeds maxDocBytes.
	chunks := chunkBody(body, maxDocBytes)

	// Step 5: parse effective window from frontmatter (0,0 if absent).
	ef, et := parseEffectiveWindow(pr.Frontmatter)

	// Step 6: validate tiers BEFORE embedder (KNOW-02 fail-closed gate).
	// Build the doc slice with Tier set so validateTiers can check each
	// doc's tier against the registry.
	docs := make([]retriever.Document, 0, len(chunks))
	for i, chunk := range chunks {
		docs = append(docs, retriever.Document{
			ID:            int64(i), // placeholder; will be overwritten below
			Tier:          slug,
			Content:       string(chunk),
			Source:        filename,
			Embedding:     nil, // filled in embed step
			EffectiveFrom: ef,
			EffectiveTo:   et,
		})
	}
	if err := validateTiers(docs, reg); err != nil {
		return nil, err
	}

	// Step 7: embed each chunk. Embed failure is a hard error — never
	// fall back to a zero vector.
	for i, chunk := range chunks {
		vec, err := emb.Embed(ctx, string(chunk))
		if err != nil {
			return nil, fmt.Errorf("embed failed: %w", err)
		}
		docs[i].Embedding = vec
	}

	// Step 8: coordinate nextID via the backend's GetMaxID.
	startID, err := getUploadNextID(ctx, slug, r)
	if err != nil {
		return nil, err
	}
	for i := range docs {
		docs[i].ID = startID + int64(i)
	}

	return docs, nil
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
	if _, ok := reg.Lookup(slug); !ok {
		// Distinguish unknown vs deactivated by checking the full List().
		// Lookup returns false for both, so we probe the active set.
		active := reg.ActiveSlugs()
		for _, s := range active {
			if s == slug {
				// Lookup would have returned true for an active slug.
				// This branch is defensive; treat as deactivated.
				return fmt.Errorf("kb %q is deactivated", slug)
			}
		}
		return fmt.Errorf("kb %q is not registered", slug)
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

// chunkBody splits body into chunks of at most maxBytes characters, with
// 200-character overlap between consecutive chunks. Returns a single-element
// slice if body fits within maxBytes. The overlap prevents clause splitting
// at chunk boundaries from losing context.
func chunkBody(body []byte, maxBytes int) [][]byte {
	if len(body) <= maxBytes {
		return [][]byte{body}
	}
	var chunks [][]byte
	pos := 0
	for pos < len(body) {
		end := pos + maxBytes
		if end >= len(body) {
			chunks = append(chunks, body[pos:])
			break
		}
		// Back off to avoid splitting in the middle of a UTF-8 rune.
		// Scan back up to 20 bytes for a safe cut point near maxBytes.
		cut := end
		for i := 0; i < 20 && cut > pos; i++ {
			b := body[cut-1]
			if b&0x80 == 0 || (b&0xC0) == 0xC0 {
				break
			}
			cut--
		}
		chunks = append(chunks, body[pos:cut])
		overlap := 200
		if cut-overlap > pos {
			pos = cut - overlap
		} else {
			pos = cut
		}
	}
	return chunks
}
