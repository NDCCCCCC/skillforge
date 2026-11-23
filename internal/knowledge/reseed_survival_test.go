// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

package knowledge

import (
	"context"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUploadPersistReseedSurvival pins the 53-04 core acceptance (ING-01):
// the 53-01 canonical file shape — provenance frontmatter, effective-window
// passthrough, frontmatter-stripped body — must survive the boot-time reseed
// (Index tier walk) as retrievable chunks carrying the SAME content and the
// SAME window the upload-time docs carried.
//
// Before the persist step existed (the 断点 A defect), an upload's chunks
// lived only inside the vector store, and seedDocs's tier purge silently
// erased them at the next boot while the audit row kept saying "uploaded".
// This test walks the FULL reseed path (driveIndex → WalkRegisteredDirs →
// splitFrontmatter → splitBody → seedDocs hook) against the exact bytes a
// persisted upload leaves on disk, so any future divergence between the
// upload pipeline and the reseed pipeline (frontmatter shape, window
// parsing, chunk budget) fails here before it silently eats knowledge.
func TestUploadPersistReseedSurvival(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	me := &mockEmbedder{}
	mr := &mockRetriever{
		GetMaxIDFunc: func(ctx context.Context, tier string) (int64, error) {
			return 0, nil
		},
	}

	// The canonical file PersistUploadedDoc writes: provenance frontmatter
	// (4 fields) + effective-window passthrough (53-01 review fix) + the
	// frontmatter-stripped body. The body is deliberately long enough to
	// force the recursive splitter (> chunkSizeRunes) with CJK punctuation
	// so the chunk-boundary contract is exercised, not just the 1-chunk
	// happy path.
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("source_sha256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n")
	b.WriteString("source_mime: text/markdown\n")
	b.WriteString("uploaded_at: 2026-11-23T00:00:00Z\n")
	b.WriteString("doc_id: authority/abc12345-policy.md\n")
	b.WriteString("effective_from: 1700000000\n")
	b.WriteString("effective_to: 0\n")
	b.WriteString("---\n")
	for i := 0; i < 40; i++ {
		b.WriteString("第")
		b.WriteString(string(rune('A' + i%26)))
		b.WriteString("条制度条款：出差住宿费超过标准额度时必须逐级审批，未经批准的超出部分由申请人自行承担，且不得报销。\n\n")
	}
	canonical := b.String()

	// Path 1 — what the upload pipeline produced at upload time.
	upDocs, err := UploadIndex(context.Background(), "authority", []byte(canonical),
		"text/markdown", "abc12345-policy.md", "authority/abc12345-policy.md", me, reg, mr)
	require.NoError(t, err)
	require.NotEmpty(t, upDocs, "upload must produce chunks")

	// Path 2 — the fresh boot after the upload: the persisted file is the
	// ONLY thing in the corpus, and the reseed walk re-derives chunks from
	// it (the seedDocs hook intercepts before any real backend).
	cap := withSeedHook(t)
	dir := makeKnowledgeDir(t)
	writeDoc(t, dir, "authority", "abc12345-policy.md", canonical)
	require.NoError(t, driveIndex(t, dir, me, reg))
	require.NotEmpty(t, cap.docs, "reseed must pick up the persisted file")

	// Survival assertions — byte-identical content, identical window.
	require.Len(t, cap.docs, len(upDocs),
		"reseed must produce the same chunk count the upload did: %d vs %d", len(cap.docs), len(upDocs))
	for i := range upDocs {
		assert.Equal(t, upDocs[i].Content, cap.docs[i].Content,
			"chunk %d diverged between upload-time and reseed-time", i)
		assert.Equal(t, upDocs[i].EffectiveFrom, cap.docs[i].EffectiveFrom,
			"chunk %d effective_from degraded across reseed (window passthrough broken)", i)
		assert.Equal(t, upDocs[i].EffectiveTo, cap.docs[i].EffectiveTo,
			"chunk %d effective_to degraded across reseed", i)
		assert.Equal(t, "authority/abc12345-policy.md", cap.docs[i].DocID,
			"reseeded chunk %d carries the corpus-relative doc_id", i)
	}
}
