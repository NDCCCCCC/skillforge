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
//
// Index is the canonical Pattern 7 (02-RESEARCH.md §lines 576-607)
// implementation: walk the knowledge/<slug>/ directories the kb.Registry
// reports active (WalkRegisteredDirs), strip YAML frontmatter, embed the
// body via the Embedder interface (production — NOT the deterministic
// spike-only fallback per Pitfall 8), and insert into the backend with the
// directory's slug as the tier value. The registry is the single source of
// truth for which directories exist and what they are called, so the
// loader cannot drift from the retriever: both consume the same slug set.
//
// One-doc-one-chunk is the Phase 2 minimal model — each .md file becomes
// exactly one Document row. The corpus is hand-authored, each clause is
// 300-800 bytes, well under BGE-M3's 8192-token context window, and
// further splitting would fragment a clause and hurt retrieval precision.
// A hierarchical markdown splitter has never been scheduled by any
// phase (it was silently dropped from planning); whether to build one
// is an open backlog decision. CMPL-06 is unrelated — it is the Phase 5
// counter-evidence highlighting feature, completed in Plan 05-06.
package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"gopkg.in/yaml.v3"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
)

// maxDocBytes is a defensive cap on the body size we will embed and insert.
// BGE-M3's embedding quality degrades past roughly 4K characters on CJK
// text (the BGE-M3 8K-token context window is a ceiling, not a sweet spot).
// The Phase 2 hand-authored corpus is well under 1 KB per file — this cap
// is a guardrail against an operator dropping a 100 KB policy PDF into
// knowledge/authority/ and silently degrading retrieval for everyone else.
// When a file exceeds the cap we emit a warning and skip it; the operator
// can split the doc manually. (No automatic splitter exists; whether to
// build one is an open backlog decision.)
const maxDocBytes = 4000

// Index walks the ACTIVE knowledge/<slug>/ directories under knowledgeDir
// (via WalkRegisteredDirs), embeds the body of each .md file via the
// Embedder interface, and seeds the resulting Documents into Milvus with
// the slug as the tier metadata.
//
// Behavior:
//   - The directory walk is registry-driven (KBM-06). A deactivated base's
//     directory is skipped with a WARN and NO audit row; an unregistered
//     directory is skipped with a WARN plus an INSERT-only audit row
//     delivered through indexAuditHook; neither fails the walk (SC-5).
//     Registered directories that do not exist on disk are skipped
//     silently. Empty dirs ARE valid: knowledge/personal/ being empty is
//     the SC-4 refusal mechanism — a Skill whose reads=[authority/**] can
//     never query personal/, AND a contact-info query against an empty
//     personal/ returns 0 chunks → no_authorized_source refusal.
//   - .md files without YAML frontmatter are still indexed (the body is
//     the whole file). Production Skill files have frontmatter; corpus
//     policy files typically don't, but the loader is permissive.
//   - Files exceeding maxDocBytes are logged via fmt.Fprintf(os.Stderr, ...)
//     and SKIPPED (not silently truncated — the operator needs to see it).
//   - Documents are assigned numeric IDs starting at 1, monotonically
//     increasing across slug dirs. Re-running on the same collection
//     inserts duplicates (Milvus INT64 PK is not auto-deduped); the
//     startup hook may want to drop+recreate for clean re-seeds.
//   - KNOW-02 (Phase 3): docs with empty or unknown Tier are rejected at
//     insert time before any embedder / backend call (see validateTiers).
//     The valid tier values are the registry's ACTIVE slugs
//     (reg.ActiveSlugs()) — a deactivated slug is rejected exactly like an
//     unknown one. A rejection here means an operator can trust a
//     downstream no_authorized_source refusal really means "no authorized
//     source for this query", never "we forgot to tag the docs"
//     (refusal-masking moat, T-3-MASK).
//
// Returns:
//   - nil on success (collection seeded with N>=0 documents).
//   - wrapped error on Milvus or embed failure. Embed failure is a HARD
//     error in production — we never fall back to a deterministic offline
//     vector (Pitfall 8 / T-2-25): the deterministic fallback exists for
//     scripts/spiked regression only and would silently mask a BGE-M3 outage
//     with semantically meaningless vectors.
//
// knowledgeDir is expected to be the project-relative "knowledge/" path
// but any directory whose first-level subdirectories are registered slugs
// works. emb is the embedder.Embedder used to vectorize each document; mc
// is the Milvus client used to seed the resulting documents; reg is the
// knowledge-base registry — REQUIRED, because a nil registry would make
// every directory look unregistered and silently index nothing.
func Index(ctx context.Context, knowledgeDir string, emb embedder.Embedder, mc client.Client, cfg milvus.Config, reg kb.Registry) error {
	if knowledgeDir == "" {
		return errors.New("knowledge.Index: knowledgeDir is empty")
	}
	if emb == nil {
		return errors.New("knowledge.Index: embedder is nil")
	}
	// The registry is a hard requirement (fail-closed): without it the walk
	// would classify every directory as unregistered and produce an empty
	// corpus — indistinguishable from a healthy one. It is checked before
	// the client because it is required regardless of backend.
	if reg == nil {
		return errors.New("knowledge.Index: registry is required (fail-closed; pass kb.Open or kb.LoadSeedRegistry)")
	}
	// mc==nil is rejected only when the test hook is not active; in test
	// mode the hook intercepts before any Milvus SDK call, so a nil
	// client is acceptable.
	if mc == nil && indexSeedHook == nil {
		return errors.New("knowledge.Index: milvus client is nil")
	}

	var docs []retriever.Document
	nextID := int64(1)

	// Registry-driven directory walk (KBM-06). The walker owns the
	// three-state decision table — active/deactivated/unregistered — and
	// returns the ascending active slug set, which is what keeps chunk IDs
	// deterministic across re-seeds. Audit rows for unregistered
	// directories ride the indexAuditHook callback (Pitfall 7: callback
	// injection, never a store parameter on this pure index function).
	dirs, err := WalkRegisteredDirs(knowledgeDir, reg, indexAuditHook)
	if err != nil {
		return fmt.Errorf("knowledge.Index: walk %s: %w", knowledgeDir, err)
	}

	for _, slug := range dirs {
		tierAbs := filepath.Join(knowledgeDir, slug)

		// Glob *.md in this slug dir. Files are processed in lexical order
		// for deterministic nextID assignment across re-runs.
		pattern := filepath.Join(tierAbs, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("knowledge.Index: glob %s: %w", pattern, err)
		}
		sort.Strings(matches)

		for _, path := range matches {
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("knowledge.Index: read %s: %w", path, err)
			}

			body, fm := splitFrontmatter(raw)
			if len(body) == 0 {
				// Empty body (frontmatter-only file or 0-byte file). Skip
				// silently — the operator probably wrote a placeholder.
				continue
			}
			if len(body) > maxDocBytes {
				fmt.Fprintf(os.Stderr,
					"[WARN] knowledge.Index: %s body=%d bytes exceeds cap=%d; skipping. "+
						"Split the doc manually (no automatic splitter is scheduled).\n",
					path, len(body), maxDocBytes)
				continue
			}

			// PRODUCTION embed via the Embedder interface. We never call
			// a deterministic fallback here — that would silently hide a
			// BGE-M3 outage (Pitfall 8).
			vec, err := emb.Embed(ctx, string(body))
			if err != nil {
				return fmt.Errorf("knowledge.Index: embed %s: %w", path, err)
			}

			// Source path is the slash-normalized relative path (no
			// knowledgeDir prefix) — keeps the Citation title short and
			// portable. e.g. "authority/travel_expense.md".
			rel, err := filepath.Rel(knowledgeDir, path)
			if err != nil {
				return fmt.Errorf("knowledge.Index: relative path for %s: %w", path, err)
			}
			relSlash := filepath.ToSlash(rel)

			// Plan 05-02 D5-15: parse the optional effective_from /
			// effective_to fields from the YAML frontmatter. Missing or
			// unparseable fields default to the sentinel 0 (perpetual).
			// validateEffective (called below) rejects inverted or
			// negative windows; legacy docs without frontmatter get 0,0
			// and pass.
			ef, et := parseEffectiveWindow(fm)

			docs = append(docs, retriever.Document{
				ID:            nextID,
				Tier:          slug,
				Content:       string(body),
				Source:        relSlash,
				Embedding:     vec,
				EffectiveFrom: ef,
				EffectiveTo:   et,
			})
			nextID++
		}
	}

	// KNOW-02: tier-mandatory enforcement at the indexer layer
	// (backend-agnostic; the hard path around D7 per CONTEXT.md
	// discretion). validateTiers runs BEFORE seedDocs so a rejected doc
	// can never reach the backend — Milvus, Mongo, or any future backend
	// (Pitfall 6: enforcing only at the backend would let a future
	// contributor add RETRIEVER_BACKEND=foo and silently bypass tier
	// enforcement). An empty docs slice trivially passes (loop is a
	// no-op), so the empty-corpus path below still fires unchanged.
	if err := validateTiers(docs, reg); err != nil {
		return err
	}

	// KNOW-02 extension (Plan 05-02 D5-15): policy-version window
	// validation. validateEffective rejects docs with inverted or
	// negative effective_to windows before they reach the backend.
	// The check runs AFTER validateTiers (same backend-agnostic
	// rationale: never let a known-bad doc reach the store).
	if err := validateEffective(docs); err != nil {
		return err
	}

	// Seed in one batched Insert. An empty knowledge/ tree (all tier dirs
	// empty) is valid — SeedDocuments rejects empty input so we skip
	// the call entirely in that case (returning nil = "no corpus to index
	// right now, but startup continues"; the retrieval path will still
	// produce no_authorized_source refusals).
	if len(docs) == 0 {
		fmt.Fprintln(os.Stderr, "[WARN] knowledge.Index: no documents found; Milvus collection left empty (every query → no_authorized_source refusal)")
		return nil
	}

	if err := seedDocs(ctx, mc, cfg, docs); err != nil {
		return fmt.Errorf("knowledge.Index: seed %d docs into %s: %w", len(docs), cfg.CollectionName, err)
	}
	return nil
}

// validateTiers enforces the KNOW-02 tier-mandatory invariant on a slice
// of docs built by Index, BEFORE the docs reach seedDocs / any backend
// write. A doc with an empty, unknown, or DEACTIVATED Tier is rejected
// with a wrapped error that names the offending doc index (0-based, e.g.
// "doc[2]"), the source path, the failing rule, and the canonical set of
// valid tier values (the registry's ACTIVE slugs) — so the operator can
// jump straight to the bad file and see what to fix.
//
// This is the backend-agnostic hard path around D7 ("tier is metadata,
// filter PRE-similarity"). Backend-specific schema validators (Milvus,
// Mongo) are belt-and-suspenders on top of this Go-level check — the
// indexer layer is the PRIMARY path per CONTEXT.md discretion
// (03-RESEARCH.md §Pitfall 6: the wrong-layer trap; enforcing ONLY at the
// backend would let a future contributor add RETRIEVER_BACKEND=foo and
// silently bypass tier enforcement, and a tier-less doc would mask as an
// SC-4 no_authorized_source refusal — eroding the refusal-by-construction
// moat, T-3-MASK).
//
// The tier check resolves through kb.Registry.Lookup, so the valid set is
// the registry's ACTIVE slug set: an unknown slug and a deactivated one
// are the same failure (a deactivated base must disappear from the index
// side too, not just from retrieval). The function is pure (no I/O, no
// globals beyond the registry argument) so the rejection branches are
// unit-testable via direct calls with hand-constructed doc slices. The
// normal scan path always assigns Tier = the walked directory's slug and
// can therefore never produce an empty / invalid Tier; the rejection
// branches exist for defense-in-depth (a future code path that constructs
// docs without going through the registry walk, e.g. a batch-ingest CLI,
// would still be gated by this check).
func validateTiers(docs []retriever.Document, reg kb.Registry) error {
	if reg == nil {
		return errors.New("knowledge.Index: registry is required (KNOW-02 gate cannot resolve tier values without it)")
	}
	active := reg.ActiveSlugs()
	for i, d := range docs {
		if d.Tier == "" {
			return fmt.Errorf("knowledge.Index: doc[%d] source=%s has empty tier (KNOW-02: tier is mandatory metadata; active knowledge bases: %v)",
				i, d.Source, active)
		}
		if _, ok := reg.Lookup(d.Tier); !ok {
			return fmt.Errorf("knowledge.Index: doc[%d] source=%s has unknown tier=%q (KNOW-02: unknown or deactivated knowledge base; active slugs: %v)",
				i, d.Source, d.Tier, active)
		}
	}
	return nil
}

// validateEffective enforces the Plan 05-02 D5-15 policy-version window
// invariants on a slice of docs built by Index, BEFORE the docs reach
// seedDocs / any backend write. Sentinel-0 design (NOT nullable): zero
// means "perpetually valid". A rejection here means a corrupted or
// hand-edited frontmatter can never poison the effective_to PRE-filter
// at the database layer.
//
// Rejected cases:
//
//   - effective_to < 0                              — negative epoch nonsense.
//   - both effective_from > 0 AND effective_to < effective_from
//     — inverted window.
//
// Permitted cases (each documented inline in retriever.Document.EffectiveTo):
//
//   - {from=0, to=0}        — perpetual (legacy / versionless).
//   - {from=0, to>0}        — open-ended from, ends at <to>.
//   - {from>0, to=0}        — starts at <from>, perpetual.
//   - {from>0, to>=from}    — bounded window.
//
// Pure function (no I/O, no globals). An empty docs slice trivially
// passes (the loop is a no-op). The static-guard test
// TestIndexer_ProductionWiresValidateEffective asserts the production
// Index path actually calls this helper.
func validateEffective(docs []retriever.Document) error {
	for i, d := range docs {
		if d.EffectiveTo < 0 {
			return fmt.Errorf("knowledge.Index: doc[%d] source=%s has negative effective_to=%d (D5-15: epoch seconds must be >= 0)",
				i, d.Source, d.EffectiveTo)
		}
		if d.EffectiveFrom > 0 && d.EffectiveTo > 0 && d.EffectiveTo < d.EffectiveFrom {
			return fmt.Errorf("knowledge.Index: doc[%d] source=%s has inverted window effective_to=%d < effective_from=%d (D5-15)",
				i, d.Source, d.EffectiveTo, d.EffectiveFrom)
		}
	}
	return nil
}

// seedDocs is the package-internal indirection point that tests use to
// intercept the Milvus write without standing up a real Milvus server.
//
// Production behavior (indexSeedHook == nil): delegate to
// milvus.SeedDocuments verbatim.
//
// Test behavior (indexSeedHook != nil): invoke the hook with the docs;
// the hook returns the error that production would have surfaced. This
// keeps the production code path identical to the test path (no mock
// branch inside Index) while letting unit tests assert on the docs
// that would have been inserted.
var indexSeedHook func(ctx context.Context, mc client.Client, cfg milvus.Config, docs []retriever.Document) error

// indexAuditHook is the audit callback Index hands to WalkRegisteredDirs
// for the unregistered-directory decision (KBM-06: WARN + skip + audit).
// It is deliberately a callback seam and NOT a store.Store parameter
// (Pitfall 7): this is a pure index function and must not drag the sqlite
// store — or any store — into cmd/eval, the spike binaries, or any test
// path just to record one audit row.
//
// Production binds an INSERT-only wrapper over store.PutAudit at boot via
// SetIndexAuditHook (cmd/server main.go); hermetic tests bind an in-memory
// collector; nil (the default) means "do not audit" — the WARN still
// fires, only the audit row is skipped.
var indexAuditHook func(action, detail string)

// SetIndexAuditHook binds the production audit sink for the
// unregistered-directory decision. The wrapper the caller hands in MUST be
// INSERT-only (it records an anomaly; it never updates or deletes audit
// rows). Called once at boot, before any Index call, so no synchronization
// is needed afterwards.
func SetIndexAuditHook(fn func(action, detail string)) { indexAuditHook = fn }

func seedDocs(ctx context.Context, mc client.Client, cfg milvus.Config, docs []retriever.Document) error {
	if indexSeedHook != nil {
		return indexSeedHook(ctx, mc, cfg, docs)
	}
	return milvus.SeedDocuments(ctx, mc, cfg, docs)
}

// splitFrontmatter extracts the markdown body and the raw YAML
// frontmatter block from raw. Returns (raw, nil) when no frontmatter
// is present, (raw, nil) on unterminated frontmatter (treat as no
// frontmatter; the file is still indexable). When frontmatter is
// present and well-formed, returns (body, frontmatterBytes) where
// frontmatterBytes is the YAML content between the --- markers (without
// the --- lines themselves).
//
// Pattern adapted from internal/skill/loader.go:parseFrontmatter — the
// skill loader is strict (opening --- must be on line 1, missing opener
// is an error). The knowledge indexer is permissive: most corpus files
// do NOT have frontmatter; rejecting them would break every Phase 2
// corpus file. The frontmatter IS stripped when present so a future
// indexer can add metadata fields (effective_date, owner, etc.) without
// poisoning the embedded body.
//
// Plan 05-02 D5-15: the frontmatter is returned so parseEffectiveWindow
// can read the effective_from / effective_to fields. Legacy files
// without frontmatter get (raw, nil) — parseEffectiveWindow returns
// (0, 0) on nil input (the perpetual sentinel).
func splitFrontmatter(raw []byte) (body []byte, frontmatter []byte) {
	// Normalize CRLF → LF for marker matching across Windows-committed
	// files (mirrors the skill loader behavior).
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	const open = "---\n"
	if !bytes.HasPrefix(raw, []byte(open)) {
		return raw, nil
	}
	body = raw[len(open):]
	// Closing marker is "\n---" — the body continues after the next newline.
	const close = "\n---"
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		// Unterminated frontmatter — treat as no frontmatter (don't fail;
		// the file is still indexable, just the frontmatter will be
		// embedded as part of the body. Better than refusing to index).
		return raw, nil
	}
	// Skip the close line itself ("\n---") + the trailing newline.
	fm := body[:end]
	after := body[end+len(close):]
	return bytes.TrimLeft(after, "\n"), fm
}

// parseEffectiveWindow reads effective_from / effective_to (epoch
// seconds) from a YAML frontmatter block. Returns (0, 0) — the
// perpetual sentinel — on nil input, malformed YAML, or missing fields.
// Unknown / unparseable values fall back to 0 silently; inverted or
// negative windows are caught downstream by validateEffective.
//
// Plan 05-02 D5-15: the indexer is permissive here because every Phase
// 2 corpus file is unversioned (effective_* defaults to 0). Future
// versions that DO use effective windows will populate the frontmatter
// here; this helper is the structural seam that pulls the values
// without coupling the rest of Index to YAML.
func parseEffectiveWindow(fm []byte) (effectiveFrom, effectiveTo int64) {
	if len(fm) == 0 {
		return 0, 0
	}
	var doc struct {
		EffectiveFrom int64 `yaml:"effective_from"`
		EffectiveTo   int64 `yaml:"effective_to"`
	}
	// Tolerate malformed YAML — we don't want a stray frontmatter bug
	// to brick the whole indexer; the missing fields default to 0.
	if err := yamlUnmarshal(fm, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] knowledge.parseEffectiveWindow: malformed frontmatter (%v); treating as perpetual\n", err)
		return 0, 0
	}
	return doc.EffectiveFrom, doc.EffectiveTo
}

// yamlUnmarshal is a package-internal indirection so tests can swap the
// YAML decoder (mirroring indexSeedHook). Production delegates to
// gopkg.in/yaml.v3 directly.
var yamlUnmarshal func(data []byte, v any) error = func(data []byte, v any) error {
	return yamlUnmarshalDefault(data, v)
}

// yamlUnmarshalDefault is the production YAML decoder. Tests may
// override yamlUnmarshal to assert specific frontmatter shapes.
func yamlUnmarshalDefault(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
