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

// Package vecmath holds vector-math primitives shared across packages
// that have no shared ancestor (e.g. internal/retriever/mongo and
// internal/skill). Extracted from internal/retriever/mongo/rank.go
// in Plan 05-02 so the Skill Router can reuse cosine without dragging
// in the Mongo driver + the skill.TierEnum dependency that seed.go
// imports (which would create an import cycle: skill -> mongo -> skill).
//
// Why a separate package (not inlined): the plan's "Don't Hand-Roll"
// rule says the existing cosine is the single source of truth for
// vector similarity in this codebase — bit-exact and unit-tested. A
// second copy would silently drift (different zero-norm handling, etc.).
//
// Why not internal/retriever/ranker or similar: that would still be a
// leaf of internal/retriever and could only be imported by packages
// that already depend on retriever. skill/ currently has no such
// dependency and adding one would invert the import direction
// (skill -> retriever -> ...). A leaf package with no retriever
// dependency keeps both call sites symmetric.
package vecmath

import "math"

// Cosine returns the cosine similarity between two equal-length
// vectors. Panics on length mismatch — callers MUST validate length
// before calling (Search enforces this on the query vector and again
// per-row via the rank helper). A zero-norm vector (no magnitude)
// returns 0 to avoid div-by-zero; this is the standard convention and
// matches what Milvus returns for the same input.
//
// Originally lived in internal/retriever/mongo/rank.go; promoted to a
// leaf package in Plan 05-02 so internal/skill/router.go can import it
// without a cycle through internal/retriever/mongo/seed.go (which
// itself imports internal/skill for TierEnum).
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("vecmath.Cosine: length mismatch")
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	// float32 precision is acceptable for ranking; BGE-M3 embeddings are
	// L2-normalized in practice so the norms cluster near 1.0. math.Sqrt
	// operates in float64 (hardware sqrt on amd64); the round-trip is
	// negligible against the dot-product loop and keeps test assertions
	// bit-exact across platforms.
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
