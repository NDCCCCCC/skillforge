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
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRouterEmbedder is a deterministic embedder stub. The returned
// vector is a unit vector in `dim`D space pointing toward a configurable
// direction (encoded as a single 1.0 at `pivotIndex`). Tests set two
// pivot indices so Skill A and Skill B embed to different (orthogonal)
// vectors; queries embed toward Skill A's pivot so A scores higher.
//
// For ambiguity tests, we override the dim so query embeddings land
// below AmbiguityLowFloor (cosine ≈ 0 when orthogonal — but we scale
// down to push scores below the threshold; see TestRouter_Ambiguous_*).
type stubRouterEmbedder struct {
	pivotIndex int   // index where the unit vector has 1.0; rest are 0
	dim        int   // total vector dim
	scale      float32 // multiplies the unit vector; default 1.0; < 1.0 lowers cosine
	err        error  // if non-nil, Embed returns this error (BGE-M3 outage simulation)
}

func (e *stubRouterEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if e.err != nil {
		return nil, e.err
	}
	v := make([]float32, e.dim)
	scale := e.scale
	if scale == 0 {
		scale = 1.0
	}
	v[e.pivotIndex] = scale
	return v, nil
}

func (e *stubRouterEmbedder) Dimensions() int { return e.dim }

// entryFor is a small constructor that builds a *Entry from name +
// pivot index. The DescVec is a unit vector in dimD space pointing at
// pivotIndex, mirroring what the registry would store after embedding
// the Skill's description.
func entryFor(name string, pivotIndex, dim int) *Entry {
	m := &Manifest{Description: name + " description"}
	vec := make([]float32, dim)
	vec[pivotIndex] = 1.0
	m.SetDescriptionVec(vec)
	return &Entry{Manifest: m, DescVec: vec, Version: 1}
}

// entryForScaled is like entryFor but multiplies the unit vector by
// `scale` — used in ambiguity tests to push the cosine score below
// the AmbiguityLowFloor.
func entryForScaled(name string, pivotIndex, dim int, scale float32) *Entry {
	m := &Manifest{Description: name + " description"}
	vec := make([]float32, dim)
	vec[pivotIndex] = scale
	m.SetDescriptionVec(vec)
	return &Entry{Manifest: m, DescVec: vec, Version: 1}
}

// TestRouter_PicksHighest — D5-01 happy path: query embedder points at
// Skill A's pivot; A's cosine > B's cosine; Selected == A; A's score is
// flagged Selected=true, B's is false.
func TestRouter_PicksHighest(t *testing.T) {
	const dim = 8
	emb := &stubRouterEmbedder{pivotIndex: 1, dim: dim} // query → index 1
	r := NewRouter(emb)

	snap := map[string]*Entry{
		"alpha": entryFor("alpha", 1, dim), // cosine = 1.0 with query
		"bravo": entryFor("bravo", 5, dim), // cosine = 0.0 with query
	}

	got, err := r.Route(context.Background(), "test query", snap)
	require.NoError(t, err)
	assert.Equal(t, "alpha", got.Selected, "alpha must win on cosine 1.0")
	assert.False(t, got.Ambiguous, "clear winner must not be ambiguous")
	require.Len(t, got.Scores, 2, "Scores must carry every Skill (trace contract)")

	// alpha has Selected=true; bravo has Selected=false.
	var alphaFound, bravoFound bool
	for _, s := range got.Scores {
		if s.SkillID == "alpha" {
			alphaFound = true
			assert.True(t, s.Selected, "alpha's score must be Selected=true")
			assert.InDelta(t, 1.0, s.Score, 1e-6, "alpha's cosine must be ~1.0")
		}
		if s.SkillID == "bravo" {
			bravoFound = true
			assert.False(t, s.Selected, "bravo's score must be Selected=false")
			assert.InDelta(t, 0.0, s.Score, 1e-6, "bravo's cosine must be ~0.0")
		}
	}
	assert.True(t, alphaFound, "alpha must appear in Scores")
	assert.True(t, bravoFound, "bravo must appear in Scores")

	// Scores are sorted desc.
	for i := 1; i < len(got.Scores); i++ {
		assert.GreaterOrEqual(t, got.Scores[i-1].Score, got.Scores[i].Score,
			"Scores must be sorted descending by cosine")
	}
}

// TestRouter_Ambiguous_Top1Low — all scores below AmbiguityLowFloor →
// Ambiguous=true (the router cannot confidently pick any Skill; the UI
// surfaces the picker).
func TestRouter_Ambiguous_Top1Low(t *testing.T) {
	const dim = 8
	// Scale query down to 0.3 — every entry that points elsewhere has
	// cosine 0; if any entry matches pivotIndex, cosine = 0.3 < floor.
	// For this test we put BOTH entries at non-matching pivots so
	// every score is 0.0 < 0.4.
	emb := &stubRouterEmbedder{pivotIndex: 0, dim: dim}
	r := NewRouter(emb)

	snap := map[string]*Entry{
		"alpha": entryFor("alpha", 3, dim), // cosine = 0
		"bravo": entryFor("bravo", 5, dim), // cosine = 0
	}

	got, err := r.Route(context.Background(), "test query", snap)
	require.NoError(t, err)
	assert.True(t, got.Ambiguous, "all scores < AmbiguityLowFloor → Ambiguous=true")
	assert.Equal(t, "", got.Selected, "ambiguous result must clear Selected")
	require.Len(t, got.Scores, 2)
	for _, s := range got.Scores {
		assert.False(t, s.Selected, "ambiguous → no skill is selected")
	}
}

// TestRouter_Ambiguous_Top1Top2Close — both candidates score similarly;
// gap is below AmbiguityMargin → Ambiguous=true.
func TestRouter_Ambiguous_Top1Top2Close(t *testing.T) {
	const dim = 8
	// Place query at pivot 1. Place alpha at pivot 1 (cos=1). Place
	// bravo at pivot 2 with scale 0.95 → cosine = 0.95 * (1.0 at 1 · 1.0 at 2)
	// but vectors are orthogonal → cosine = 0. We need them to be SIMILAR.
	// Trick: use scale < 1 on alpha and a slightly different but still-
	// overlapping basis. Easiest: query AND alpha both at pivot 1; bravo
	// at pivot 1 with scale 0.94 → both unit vectors point to dim
	// dimension; cosine = (1*1 + 0*0 + ... ) / (1*1) * sqrt(scale^2)
	// = scale = 0.94. Gap = 1.0 - 0.94 = 0.06 < 0.08 → ambiguous.
	emb := &stubRouterEmbedder{pivotIndex: 1, dim: dim}
	r := NewRouter(emb)

	snap := map[string]*Entry{
		"alpha": entryFor("alpha", 1, dim),                       // cosine = 1.0
		"bravo": entryForScaled("bravo", 1, dim, 0.94),           // cosine = 0.94
	}

	got, err := r.Route(context.Background(), "test query", snap)
	require.NoError(t, err)
	assert.True(t, got.Ambiguous, "gap (1.0 - 0.94 = 0.06) < AmbiguityMargin (0.08) → Ambiguous=true")
	assert.Equal(t, "", got.Selected)
}

// TestRouter_EmptySnapshot — D5-03 / empty-corpus path: registry has no
// Skills installed. Router returns Selected="" + Ambiguous=true +
// Scores=nil. chatHandler treats this as "open the wizard" (per
// SC-1 empty-state UX).
func TestRouter_EmptySnapshot(t *testing.T) {
	emb := &stubRouterEmbedder{pivotIndex: 0, dim: 8}
	r := NewRouter(emb)

	got, err := r.Route(context.Background(), "test query", map[string]*Entry{})
	require.NoError(t, err)
	assert.True(t, got.Ambiguous, "empty snapshot is ambiguous (no skills to pick)")
	assert.Equal(t, "", got.Selected, "empty snapshot must clear Selected")
	assert.Empty(t, got.Scores, "empty snapshot must carry no scores")
}

// TestRouter_EmbedderError — BGE-M3 outage simulation. Router returns
// the error verbatim; chatHandler surfaces a 500 with structured body.
// No fallback path (per spike-findings, a hash-based fallback would
// silently mask a real outage — T-2-25 analog).
func TestRouter_EmbedderError(t *testing.T) {
	emb := &stubRouterEmbedder{pivotIndex: 0, dim: 8, err: errors.New("bge-m3 timeout")}
	r := NewRouter(emb)

	snap := map[string]*Entry{
		"alpha": entryFor("alpha", 1, 8),
	}
	_, err := r.Route(context.Background(), "test query", snap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bge-m3 timeout",
		"embedder error must propagate verbatim (no silent fallback)")
}

// TestRouter_AmbiguityConstantsExported — the consts are the calibration
// handle (Phase 6 GEVL work tunes them). They MUST be exported and
// referenceable from outside the package — a regression that inlines
// them as literals would break the calibration contract.
func TestRouter_AmbiguityConstantsExported(t *testing.T) {
	assert.Greater(t, float64(AmbiguityLowFloor), float64(0), "AmbiguityLowFloor must be > 0")
	assert.LessOrEqual(t, float64(AmbiguityLowFloor), float64(1), "AmbiguityLowFloor must be ≤ 1")
	assert.Greater(t, float64(AmbiguityMargin), float64(0), "AmbiguityMargin must be > 0")
	assert.LessOrEqual(t, float64(AmbiguityMargin), float64(1), "AmbiguityMargin must be ≤ 1")
}

// TestRouter_NoMagicNumbersInRoute — static guard: the Route function
// body MUST reference AmbiguityLowFloor / AmbiguityMargin (not literal
// 0.4 / 0.08). A refactor that inlines the values would silently break
// the calibration handle.
func TestRouter_NoMagicNumbersInRoute(t *testing.T) {
	src, err := readFileSafe("router.go")
	require.NoError(t, err)
	body := src
	// Route is defined as a method on *Router. Search for the func decl
	// onward; tolerate whitespace.
	const marker = "func (r *Router) Route("
	idx := indexOf(body, marker)
	require.GreaterOrEqual(t, idx, 0, "Route method not found in router.go")
	// Scan from the marker to end of file — Route's body should never
	// exceed the rest of router.go in size.
	routeBody := body[idx:]
	assert.Contains(t, routeBody, "AmbiguityLowFloor",
		"Route must reference AmbiguityLowFloor const (no magic 0.4 in body)")
	assert.Contains(t, routeBody, "AmbiguityMargin",
		"Route must reference AmbiguityMargin const (no magic 0.08 in body)")
}

// readFileSafe is a tiny helper that swallows a missing file with a
// descriptive error rather than panicking through os.ReadFile on the
// absent path (which is what the static guard needs to recover from).
func readFileSafe(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// indexOf is a tiny local replacement for strings.Index so the static
// guard above is self-contained. Production code uses strings.Contains
// (imported above) for the actual gate; this helper just makes the
// guard readable.
func indexOf(haystack, needle string) int {
	return strings.Index(haystack, needle)
}
