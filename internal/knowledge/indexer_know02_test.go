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

// KNOW-02 regression lock: the tier-mandatory enforcement at the indexer
// layer is covered by these focused tests. The validation is split out of
// Index into a pure helper (validateTiers) so the rejection branches can
// be exercised directly with hand-constructed doc slices — the normal
// scan path always assigns Tier = the registry slug of the walked
// directory and can therefore never produce an empty / invalid Tier, so
// the rejection branches are unreachable through Index end-to-end and
// MUST be tested at the helper boundary. The end-to-end happy path (all
// baseline slugs) IS exercised through Index + indexSeedHook to
// regression-lock the existing seeding behavior.
//
// Phase 26 note: the gate resolves through kb.Registry.Lookup, so the
// canonical valid set is the registry's ACTIVE slug set — a deactivated
// slug is rejected exactly like an unknown one — and the error message
// lists the active slugs so a Skill/corpus author can self-correct.

package knowledge

import (
	"os"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndexerRejectsMissingTier (KNOW-02): a doc with an empty Tier field
// is rejected by validateTiers BEFORE any backend write. The error names
// the rule (KNOW-02), the failure ("empty tier"), the offending doc index,
// and the source path so the operator can locate the bad file.
//
// "Hook NOT called" is asserted structurally: validateTiers is a pure
// function that never touches indexSeedHook, and the code-order guarantee
// (locked by TestIndexer_ValidateTiersRunsBeforeSeedDocs) ensures
// validateTiers runs before seedDocs inside Index — so a rejection here
// means seedDocs is never reached on any path, Milvus OR Mongo OR any
// future backend.
func TestIndexerRejectsMissingTier(t *testing.T) {
	cap := withSeedHook(t) // would intercept seedDocs if it were ever called
	docs := []retriever.Document{
		{ID: 1, Tier: "authority", Source: "authority/valid.md", Content: "valid"},
		{ID: 2, Tier: "", Source: "authority/missing.md", Content: "no tier"},
	}

	err := validateTiers(docs, baselineRegistry())
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "empty tier", "error must name the failing rule")
	assert.Contains(t, msg, "KNOW-02", "error must cite the requirement ID")
	assert.Contains(t, msg, "authority/missing.md", "error must name the source path so the operator can locate the file")
	assert.Contains(t, msg, "doc[1]", "error must name the offending doc index (0-based)")
	assert.Empty(t, cap.docs, "seedDocs hook must NOT be called on rejection (validateTiers is upstream of seedDocs)")
}

// TestIndexerRejectsInvalidTier (KNOW-02): a doc with a Tier value outside
// the registry's active slug set is rejected, and the error lists ALL of
// the active slugs so a Skill author sees the canonical set instead of
// guessing. Asserts every baseline slug appears in the error message.
func TestIndexerRejectsInvalidTier(t *testing.T) {
	cap := withSeedHook(t)
	docs := []retriever.Document{
		{ID: 1, Tier: "classified", Source: "authority/bad.md", Content: "bad tier"},
	}

	err := validateTiers(docs, baselineRegistry())
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "unknown tier", "error must name the failing rule")
	assert.Contains(t, msg, "classified", "error must echo the offending tier value")
	assert.Contains(t, msg, "KNOW-02")
	// Every active slug must appear so authors see the valid set.
	// ActiveSlugs() returns them ascending; only presence is asserted, not
	// ordering, to stay robust to future registry reshaping.
	for _, v := range testBaselineSlugs {
		assert.Containsf(t, msg, v, "error must list valid tier %q (KNOW-02: enumerate the canonical set)", v)
	}
	assert.Empty(t, cap.docs, "seedDocs hook must NOT be called on rejection")
}

// TestIndexerAcceptsAllValidTiers (happy-path regression): a corpus
// covering every baseline slug indexes end-to-end through Index. This
// regression-locks the existing seeding behavior AND proves the KNOW-02
// validation loop is a no-op on a valid corpus (no valid doc is rejected,
// no doc is mutated, all docs reach the seedDocs hook in order).
func TestIndexerAcceptsAllValidTiers(t *testing.T) {
	dir := makeKnowledgeDir(t)
	writeDoc(t, dir, "authority", "a.md", "authority tier doc")
	writeDoc(t, dir, "historical", "h.md", "historical tier doc")
	writeDoc(t, dir, "personal", "p.md", "personal tier doc")
	writeDoc(t, dir, "draft", "d.md", "draft tier doc")

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder(), baselineRegistry()))

	// All 4 docs pass validateTiers (no error returned by Index) and
	// reach the seedDocs hook unchanged.
	require.Lenf(t, cap.docs, 4, "all 4 baseline-slug docs must be seeded; got %d", len(cap.docs))
	seen := make(map[string]bool, 4)
	for _, d := range cap.docs {
		seen[d.Tier] = true
	}
	for _, v := range testBaselineSlugs {
		assert.Truef(t, seen[v], "doc with tier %q must be present in the seeded set", v)
	}

	// Direct unit-level confirmation: validateTiers on the captured docs
	// returns nil. The hook-fired docs already passed it implicitly, but
	// assert directly to lock the contract on the pure function itself.
	assert.NoError(t, validateTiers(cap.docs, baselineRegistry()),
		"validateTiers must accept every active registry slug")
}

// TestIndexerErrorMessageNamesDocIndex (operator ergonomics): when a
// corpus has multiple docs and the bad one is at index 2 (0-based), the
// error message contains "doc[2]" so the operator can locate the
// offending file without scanning the whole corpus. This is the
// belt-and-suspenders companion to the source-path assertion — together
// they let the operator jump straight to the bad file.
func TestIndexerErrorMessageNamesDocIndex(t *testing.T) {
	docs := []retriever.Document{
		{ID: 1, Tier: "authority", Source: "authority/a.md", Content: "ok"},
		{ID: 2, Tier: "historical", Source: "historical/b.md", Content: "ok"},
		{ID: 3, Tier: "bad", Source: "authority/x.md", Content: "bad tier"},
	}

	err := validateTiers(docs, baselineRegistry())
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "doc[2]", "error must name the offending doc index (0-based)")
	assert.Contains(t, msg, "authority/x.md", "error must name the source path")
}

// TestIndexer_ValidateTiersRunsBeforeSeedDocs is the static-analysis
// guarantee that the KNOW-02 validation loop fires BEFORE the seedDocs
// call inside Index — so a rejected doc can NEVER reach the backend
// (Milvus, Mongo, or any future backend). If a future contributor
// reorders the two calls, this test breaks the build. Mirrors the
// TestIndexer_ProductionUsesEmbedderInterface pattern (source-level
// invariant guard).
func TestIndexer_ValidateTiersRunsBeforeSeedDocs(t *testing.T) {
	data, err := os.ReadFile("indexer.go")
	require.NoError(t, err)
	src := string(data)

	validatePos := strings.Index(src, "validateTiers(docs)")
	seedPos := strings.Index(src, "seedDocs(ctx, mc, cfg, docs)")
	require.GreaterOrEqualf(t, validatePos, 0, "Index must call validateTiers(docs) — internal/knowledge/indexer.go")
	require.GreaterOrEqualf(t, seedPos, 0, "Index must call seedDocs(ctx, mc, cfg, docs) — internal/knowledge/indexer.go")
	assert.Less(t, validatePos, seedPos,
		"validateTiers(docs) must run BEFORE seedDocs(ctx, mc, cfg, docs) in Index "+
			"so KNOW-02-rejected docs never reach the backend (Pitfall 6: backend-agnostic enforcement)")
}
