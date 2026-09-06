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

package skill

import (
	"context"
	"sort"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/vecmath"
	"github.com/skillforge/skillforge/internal/wire"
)

// AmbiguityLowFloor is the cosine score below which the router
// considers the top-1 candidate insufficiently confident to auto-route.
// Below this floor the chat handler emits ambiguous=true and the FE
// surfaces the Skill picker (D5-03).
//
// RESEARCH §Open Question 3 establishes 0.4 as the planner-tunable
// starting value; Phase 6 GEVL work calibrates against the golden set
// (D2-15 / D3-08 pattern). The const is exported so calibration can
// bump it without rewriting the routing code.
const AmbiguityLowFloor = 0.4

// AmbiguityMargin is the minimum gap (top-1 minus top-2) below which
// the router considers the top-2 candidates indistinguishable and
// flags the decision as ambiguous. Even with a strong top-1 score,
// two Skills within `AmbiguityMargin` of each other are too close to
// trust — the user must pick (D5-03).
//
// RESEARCH §Open Question 3 establishes 0.08 as the planner-tunable
// starting value; Phase 6 GEVL work calibrates against the golden set.
const AmbiguityMargin = 0.08

// Router selects the best-matching Skill for an incoming query by
// embedding the query (via the same BGE-M3 path the registry uses to
// embed Skill descriptions) and computing cosine similarity against
// every Entry.DescVec in the registry snapshot. The highest-cosine
// Skill wins, subject to ambiguity thresholds (AmbiguityLowFloor +
// AmbiguityMargin — exported consts so calibration can tune them).
//
// The router is the SC-1 substrate (D5-01..05). chatHandler invokes it
// BEFORE VerdictPipeline; on ambiguous / empty / embed-error paths
// the chatHandler surfaces the Skill picker (or an error envelope)
// instead of running the verdict.
//
// Reuse contract (RESEARCH §Don't Hand-Roll): cosine math is
// delegated to internal/vecmath.Cosine — the single source of truth
// for vector similarity in this codebase. Originally lived in
// internal/retriever/mongo/rank.go; promoted to a leaf package in
// Plan 05-02 so this Router (in internal/skill) can reuse it
// without a cycle through mongo/seed.go (which imports internal/skill
// for the reads resolution). Future contributors must NOT hand-roll a
// second cosine — the existing implementation is bit-exact and
// unit-tested.
type Router struct {
	embedder embedder.Embedder
}

// NewRouter constructs a Router that uses emb to embed incoming
// queries. emb may not be nil — Route returns an error if the
// embedder is nil or its Embed call fails (no hash-based fallback:
// a hash fallback would silently mask a real BGE-M3 outage, per
// spike-findings Pitfall 8 / T-2-25).
//
// Named NewRouter (not New) to avoid colliding with Registry's New in
// the same package.
func NewRouter(emb embedder.Embedder) *Router {
	return &Router{embedder: emb}
}

// DecisionResult is the router's output. The chatHandler emits a
// data-decision SSE chunk carrying this structure (per D5-04 — the
// routing decision extends the existing data-decision wire type, no
// new wire type).
//
// Field semantics:
//   - Selected:    the chosen Skill's directory name (e.g. "compliance").
//     Empty when Ambiguous is true (including the empty-
//     snapshot case — "no Skills installed" routes to the
//     picker UX with the wizard empty state).
//   - Scores:      ranked list of every candidate Skill in the snapshot,
//     sorted by cosine descending. Exactly one entry has
//     Selected=true when Ambiguous is false; no entry has
//     Selected=true when Ambiguous is true (the user picks).
//     Used by the React SkillForgeInspector (SC-1 trace) to
//     render the routing confidence breakdown.
//   - Ambiguous:   true when (top-1 score < AmbiguityLowFloor) OR
//     (top-1 - top-2 < AmbiguityMargin). The chatHandler
//     emits the data-decision chunk and returns WITHOUT
//     invoking VerdictPipeline; the FE surfaces the picker.
type DecisionResult struct {
	Selected  string
	Scores    []wire.SkillScore
	Ambiguous bool
}

// Route selects the Skill best matching query from the registry
// snapshot. See type Router for the algorithm summary.
//
// Edge cases:
//   - Empty snapshot: returns Selected="" + Ambiguous=true + nil Scores.
//     The chatHandler renders the "no Skills installed — open the
//     wizard" empty state (per SC-1 UX, no new wire type).
//   - Single-Skill install: no ambiguity check possible (no top-2).
//     Returns the lone Skill as Selected with Ambiguous=false (the
//     trivial install is by definition unambiguous — there is no
//     alternative to compare against).
//   - Embedder error: returns the error verbatim. chatHandler maps
//     this to a 500 with structured envelope. No fallback path
//     (silently masking a BGE-M3 outage is the wrong posture).
func (r *Router) Route(ctx context.Context, query string, snap map[string]*Entry) (DecisionResult, error) {
	// Empty snapshot short-circuit (D5-03 — picker empty state).
	if len(snap) == 0 {
		return DecisionResult{Selected: "", Scores: nil, Ambiguous: true}, nil
	}

	// Embed the query via the same embedder path the registry used for
	// Skill descriptions. Embedder failures are surfaced verbatim — the
	// spike-findings / T-2-25 disposition is "never silently mask an
	// outage with a deterministic fallback".
	qvec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return DecisionResult{}, err
	}

	// Score every Skill. DescVec is owned by the registry (in-memory,
	// json:"-") and immutable for the snapshot's lifetime (atomic.Pointer
	// copy-on-write per D5-06) — concurrent reads are safe.
	type scored struct {
		entry *Entry
		score float32
	}
	out := make([]scored, 0, len(snap))
	for _, e := range snap {
		if e == nil || len(e.DescVec) == 0 {
			// A Skill without a description vector cannot be routed.
			// The registry logs a warn when embedding fails at reload
			// time (registry.go:reloadOne) but the entry still becomes
			// routable on its previous vec or nil — we skip nil-vec
			// entries here rather than panic on cosine.
			continue
		}
		out = append(out, scored{entry: e, score: vecmath.Cosine(qvec, e.DescVec)})
	}

	// Defensive: if every entry was nil-vec (e.g. embedder outage at
	// reload time left all DescVecs empty), we still have an empty
	// scored list. Treat as ambiguous — no Skill is selectable.
	if len(out) == 0 {
		return DecisionResult{Selected: "", Scores: nil, Ambiguous: true}, nil
	}

	// Sort desc by cosine; tie-break by SkillID asc so re-runs produce
	// identical ordering (eval reproducibility — D2-15 / D3-08 pattern).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].entry.Manifest.Description < out[j].entry.Manifest.Description
	})

	// Build the SkillScore slice. The Selected flag is filled in below
	// after we compute Ambiguous — we want every score to know whether
	// it won, so the FE can highlight it.
	scores := make([]wire.SkillScore, len(out))
	// Map SkillID -> index so we can flip Selected on the chosen entry.
	idOrder := make([]string, len(out))
	for i, s := range out {
		// SkillID is the registry map key — recovered via a second loop
		// below (snap is the source of truth for SkillID). For now we
		// build the scores with empty SkillID, then patch.
		scores[i] = wire.SkillScore{
			SkillID:   "",
			SkillName: s.entry.Manifest.Description,
			Score:     float64(s.score),
			Selected:  false,
		}
		idOrder[i] = ""
	}
	// Second loop: patch SkillID from the snap map. snap is a
	// map[string]*Entry; we look up each entry's pointer to find its key.
	for id, e := range snap {
		for i := range scores {
			if scores[i].SkillID == "" && out[i].entry == e {
				scores[i].SkillID = id
				idOrder[i] = id
				break
			}
		}
	}

	// Ambiguity gate (D5-03). Two cases flag ambiguous:
	//   1. top-1 < AmbiguityLowFloor — no candidate is clearly relevant.
	//   2. (top-1 - top-2) < AmbiguityMargin — top-2 is too close.
	// Case 2 requires ≥2 candidates; with 1 Skill installed, top-1
	// wins unambiguously (no alternative to compare against).
	top1 := scores[0].Score
	var ambiguous bool
	if top1 < float64(AmbiguityLowFloor) {
		ambiguous = true
	} else if len(scores) >= 2 {
		gap := top1 - scores[1].Score
		if gap < float64(AmbiguityMargin) {
			ambiguous = true
		}
	}

	if ambiguous {
		// Clear Selected on every entry — the picker lets the user
		// choose, and the FE renders all candidates as unselected.
		for i := range scores {
			scores[i].Selected = false
		}
		return DecisionResult{Selected: "", Scores: scores, Ambiguous: true}, nil
	}

	// Clear winner: flag the top entry as Selected.
	scores[0].Selected = true
	return DecisionResult{Selected: idOrder[0], Scores: scores, Ambiguous: false}, nil
}
