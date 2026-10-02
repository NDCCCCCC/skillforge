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

// Package skill — manifest_builder.go is the Phase 13 / Plan 13-01 fluent
// builder that constructs a sub-Manifest with structural tier-closure
// inheritance (P13-L2 — the sub-agent's reads are the INTERSECTION of the
// parent's ResolvedTiers() and the sub-skill's declared Reads, baked in
// AT CONSTRUCTION so the sub-agent's retrieve Tool closes over the
// intersection — not over a runtime-supplied or ctx-value-supplied slice).
//
// Why a builder (instead of direct struct literal mutation)?
//
//   - **Atomic validation**: the builder's Build() is the single
//     fail-closed point. Empty Reads → Build() error. Invalid tier
//     names → Validate() error. The caller never sees a half-built
//     Manifest.
//   - **Immutability of the base**: NewManifestBuilder clones the
//     input Manifest's slices (Reads, Writes, Guardrails.Prohibitions,
//     Guardrails.RequiredRefusalPhrases) so WithReads(intersection) does
//     NOT mutate the caller's input. Verified by
//     TestManifestBuilderCloneSemantics.
//   - **Fluent extension surface**: future tiers of per-Run
//     construction (e.g. P14 budget tightening, P15 audit-row seeding)
//     add With* methods without breaking callers — the Build() return
//     signature is stable.
//
// FAN-02 SC-2 invariant: empty intersection is a LOAD-TIME error, not a
// runtime silence. The runtime.BuildSubAgent caller catches this and
// fails the orchestrator's pre-fanout step BEFORE any LLM call costs
// are incurred.
package skill

import (
	"errors"
	"fmt"

	"github.com/skillforge/skillforge/internal/kb"
)

// ManifestBuilder is the fluent builder seeded with a clone of an
// existing Manifest. Chained WithReads().Build() produces a NEW
// Manifest whose Reads slice is the per-Run intersection — the
// sub-agent's Tool closure reads ONLY from this slice (P13-L2 +
// FAN-02).
//
// NOT goroutine-safe: each orchestrator Run constructs its own builder.
// Sharing one builder across concurrent Runs is forbidden by the
// per-Run construction envelope (Phase 12 D11-B4 invariant — Agent is
// per-session singleton, sub-agent construction is per-Run).
type ManifestBuilder struct {
	base *Manifest // working copy — Clone()'d from the input base at construction
}

// NewManifestBuilder returns a fluent builder seeded with a CLONE of
// the supplied base Manifest. nil base is rejected with a panic (the
// caller MUST pass a valid Manifest — the orchestrator pattern never
// passes nil).
//
// The clone protects the caller's input from accidental mutation
// (TestManifestBuilderCloneSemantics). Future WithReads / WithWrites
// methods mutate ONLY the builder's working copy.
func NewManifestBuilder(base *Manifest) *ManifestBuilder {
	if base == nil {
		panic("skill.NewManifestBuilder: base manifest required (P13-L2 invariant)")
	}
	return &ManifestBuilder{base: cloneManifest(base)}
}

// cloneManifest returns a deep-enough copy of m for the builder's
// working copy. We clone the slices the builder is expected to mutate
// (Reads is the primary case; Writes + Guardrails slices are cloned
// defensively in case future With* methods touch them). DescriptionVec
// is intentionally NOT cloned — embedding vectors are read-only after
// the loader populates them, and the clone would double memory for no
// benefit.
//
// CRITICAL: nil slices stay nil (append([]T(nil), ...) returns nil when
// the source has zero elements). The Manifest schema distinguishes nil
// from empty ([]string{}) — Validate rejects nil Writes with a
// "field is absent" error but accepts []string{} as the valid read-only
// form (SKILL_SCHEMA field 4). Using append-of-nil would silently turn
// valid empty slices into invalid nil ones.
func cloneManifest(m *Manifest) *Manifest {
	cp := *m // value copy — copies scalars, slices share backing array
	cp.Reads = cloneStringSlice(m.Reads)
	cp.Writes = cloneStringSlice(m.Writes)
	cp.Guardrails.Prohibitions = cloneStringSlice(m.Guardrails.Prohibitions)
	cp.Guardrails.RequiredRefusalPhrases = cloneStringSlice(m.Guardrails.RequiredRefusalPhrases)
	if m.Guardrails.TierLimits != nil {
		limits := make(map[string][]string, len(m.Guardrails.TierLimits))
		for k, v := range m.Guardrails.TierLimits {
			limits[k] = cloneStringSlice(v)
		}
		cp.Guardrails.TierLimits = limits
	}
	cp.resolvedTiers = cloneStringSlice(m.resolvedTiers)
	// Reset name — the builder's output is a NEW manifest, not the base.
	cp.Name = ""
	return &cp
}

// cloneStringSlice copies s so the builder's mutations are isolated
// from the caller's input. Preserves nil → nil and empty → empty so
// Validate's nil-vs-empty distinction is not broken.
func cloneStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// WithReads replaces the working copy's Reads slice with intersection.
// The validation that intersection is non-empty happens at Build() —
// fluent chains (WithReads().WithWrites()) flow naturally.
//
// intersection MAY be any subset of base.ResolvedTiers() (including the
// full set). Empty intersection is rejected at Build() with a
// distinctive error so runtime.BuildSubAgent can surface it as the
// FAN-02 SC-2 load-time failure.
func (b *ManifestBuilder) WithReads(intersection []string) *ManifestBuilder {
	b.base.Reads = intersection
	return b
}

// Build returns the constructed Manifest after validation. The two
// fail-closed checks are:
//  1. len(Reads) == 0 → empty-intersection error (FAN-02 SC-2).
//  2. Validate(b.base, reg) error → 7-field schema violation.
//
// reg is the knowledge-base registry the sub-manifest's reads contract
// resolves against (Phase 26 KBM-04). It is a Build argument (not a
// builder field) so the fluent chain stays per-Run stateless and the
// orchestrator passes the SAME registry its parent manifest was
// validated against. A nil registry fails closed for tier-channel
// sub-skills — a sub-agent must never be constructed against a reads
// contract nobody could verify.
//
// On error, the caller MUST treat the returned Manifest as undefined
// (the working copy may be in a partially-mutated state).
func (b *ManifestBuilder) Build(reg kb.Registry) (*Manifest, error) {
	if len(b.base.Reads) == 0 {
		return nil, errors.New("skill.ManifestBuilder: empty Reads (FAN-02 tier inheritance requires non-empty intersection)")
	}
	if err := Validate(b.base, reg); err != nil {
		return nil, fmt.Errorf("skill.ManifestBuilder: Validate: %w", err)
	}
	return b.base, nil
}
