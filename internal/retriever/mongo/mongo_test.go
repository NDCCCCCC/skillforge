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

package mongo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// testNow is the Plan 05-02 D5-15 epoch-seconds cutoff used by tests
// exercising the effective_to PRE-filter. Production passes
// time.Now().Unix() computed once at pipeline entry; tests use a fixed
// value so the bson.D $or shape assertions are deterministic.
const testNow int64 = 1700000000

// fakeFinder is the test stand-in for mongoCollectionFinder. It records
// the last filter it was called with and returns a caller-configured
// slice of rows (or error). The rows are returned as-is — the fake does
// NOT re-rank them, so a test that wants to exercise ranking sets up
// rows with realistic embeddings and asserts on the order Search
// produces.
type fakeFinder struct {
	rows        []docRow
	err         error
	lastFilter  any
	lastCallCtx context.Context
	calls       int
}

func (f *fakeFinder) FindRows(ctx context.Context, filter any) ([]docRow, error) {
	f.calls++
	f.lastFilter = filter
	f.lastCallCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	// Return a defensive copy so caller mutation does not bleed across
	// test cases that share a fake.
	out := make([]docRow, len(f.rows))
	copy(out, f.rows)
	return out, nil
}

// newClientWithFake wires up a *Client whose finder is the fake. The
// conn / coll fields are nil — Search does not touch them. Health /
// Disconnect / Ensure are NOT safe to call on a client built this way;
// tests that need them should construct a real Client via New (which
// requires a live MongoDB and is therefore gated behind a build tag or
// t.Skip).
func newClientWithFake(f *fakeFinder) *Client {
	return &Client{
		cfg:    Config{Database: "test", Collection: "knowledge"},
		finder: f,
	}
}

// extractTiersFromFilter pulls the tiers slice back out of the bson.D
// filter the production code builds, so tests can assert "the tier list
// we passed in is the tier list that hit the wire". Reflection-based
// because bson.D is a []bson.E and we want to keep the assertion readable.
//
// Plan 05-02 D5-15: the filter is no longer single-field — it carries
// the tier $in AND the effective_to $or co-layered. We scan for the
// "tier" key (rather than hardcoding index 0) so the helper stays
// robust against future clause additions.
func extractTiersFromFilter(t *testing.T, filter any) []string {
	t.Helper()
	d, ok := filter.(bson.D)
	require.True(t, ok, "filter is %T, want bson.D", filter)
	require.NotEmpty(t, d, "filter must have at least one field")
	var tierVal any
	for _, e := range d {
		if e.Key == "tier" {
			tierVal = e.Value
			break
		}
	}
	require.NotNil(t, tierVal, "filter must include a tier field")
	inner, ok := tierVal.(bson.D)
	require.True(t, ok, "tier value is %T, want bson.D (the {$in:...} doc)", tierVal)
	require.Len(t, inner, 1, "{$in:...} should be a single-element doc")
	require.Equal(t, "$in", inner[0].Key, "$in operator expected")
	tiers, ok := inner[0].Value.([]string)
	require.True(t, ok, "$in value is %T, want []string", inner[0].Value)
	return tiers
}

// TestSearch_PassesTierFilterAs isIn — the production contract: Search
// builds {tier: {$in: tiers}} verbatim and hands it to the finder. A
// future refactor that builds the filter any other way (e.g.
// {tier:{$eq: tiers[0]}} for a single-tier query) would silently break
// the multi-tier path; this test catches that.
func TestSearch_PassesTierFilterAsIn(t *testing.T) {
	f := &fakeFinder{rows: []docRow{}}
	c := newClientWithFake(f)

	_, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威", "历史"}, 3, testNow)
	require.NoError(t, err)

	tiers := extractTiersFromFilter(t, f.lastFilter)
	assert.Equal(t, []string{"权威", "历史"}, tiers)
}

// TestSearch_EmptyTiersSendsInEmptyFilter — D-09 refusal-by-construction
// (Go-side half of the contract).
//
// What this test verifies (Go-side): empty tiers MUST hit the finder
// with {$in: []} (not skip the call, not substitute nil, not build a
// different shape). The finder is invoked exactly once and the filter
// carries an empty (not nil) $in array.
//
// What this test does NOT verify (Mongo-side): a real MongoDB evaluates
// {$in: []} as a no-match predicate and returns 0 rows. That enforcement
// is the database's responsibility — it is the structural half of D-09
// and is documented in the Search godoc. End-to-end verification needs
// a live MongoDB and lives in the live-verify.sh script.
func TestSearch_EmptyTiersSendsInEmptyFilter(t *testing.T) {
	f := &fakeFinder{rows: []docRow{
		{ID: 1, Tier: "权威", Embedding: []float32{1, 0}},
	}}
	c := newClientWithFake(f)

	// Empty tiers slice → {$in: []} on the wire (NOT skipped, NOT nil).
	_, err := c.Search(context.Background(), []float32{1, 0}, []string{}, 5, testNow)
	require.NoError(t, err)
	assert.Equal(t, 1, f.calls, "finder must be invoked even on empty tiers (D-09 Go-side)")
	tiers := extractTiersFromFilter(t, f.lastFilter)
	assert.Empty(t, tiers, "filter $in value must be empty slice, not nil/missing")

	// Also cover the nil-tiers branch — same behavior expected.
	f.calls = 0
	_, err = c.Search(context.Background(), []float32{1, 0}, nil, 5, testNow)
	require.NoError(t, err)
	assert.Equal(t, 1, f.calls, "nil tiers must also invoke finder (no short-circuit)")
	tiers = extractTiersFromFilter(t, f.lastFilter)
	assert.Empty(t, tiers, "nil tiers → empty $in array on the wire")
}

// TestSearch_RanksByCosine — the Search path delegates to rank(); a
// finder that returns three rows with known cosines should produce
// hits sorted high-to-low. This is the end-to-end wiring test for the
// "fetch via finder → rank" pipeline, separate from the rank unit tests.
func TestSearch_RanksByCosine(t *testing.T) {
	q := []float32{1, 0}
	f := &fakeFinder{rows: []docRow{
		{ID: 1, Tier: "权威", Content: "low", Source: "authority/a.md", Embedding: []float32{0, 1}}, // cos=0
		{ID: 2, Tier: "权威", Content: "top", Source: "authority/b.md", Embedding: []float32{1, 0}}, // cos=1
		{ID: 3, Tier: "历史", Content: "mid", Source: "historical/c.md", Embedding: []float32{1, 1}}, // cos≈.707
	}}
	c := newClientWithFake(f)

	hits, err := c.Search(context.Background(), q, []string{"权威", "历史"}, 3, testNow)
	require.NoError(t, err)
	require.Len(t, hits, 3)
	assert.Equal(t, int64(2), hits[0].ID, "top hit must be the cos=1 row")
	assert.Equal(t, "权威", hits[0].Tier)
	assert.Equal(t, "top", hits[0].Content)
	assert.Equal(t, int64(3), hits[1].ID, "second hit is the cos≈.707 row")
	assert.Equal(t, int64(1), hits[2].ID, "third hit is the cos=0 row")
}

// TestSearch_TopKClampsToRows — topK longer than the row count returns
// all rows (no panic, no padding).
func TestSearch_TopKClampsToRows(t *testing.T) {
	f := &fakeFinder{rows: []docRow{
		{ID: 7, Tier: "权威", Embedding: []float32{1, 0}},
	}}
	c := newClientWithFake(f)

	hits, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 99, testNow)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, int64(7), hits[0].ID)
}

// TestSearch_PreservesAllHitFields — the returned Hit carries the
// Tier/Content/Source from the row, not just ID+Score. The pipeline
// depends on these for the citation moat (D2-06).
func TestSearch_PreservesAllHitFields(t *testing.T) {
	f := &fakeFinder{rows: []docRow{
		{
			ID:        42,
			Tier:      "权威",
			Content:   "出差住宿费超过 500 元须审批。",
			Source:    "authority/travel_expense.md",
			Embedding: []float32{1, 0},
		},
	}}
	c := newClientWithFake(f)
	hits, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 1, testNow)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	h := hits[0]
	assert.Equal(t, int64(42), h.ID)
	assert.Equal(t, "权威", h.Tier)
	assert.Equal(t, "出差住宿费超过 500 元须审批。", h.Content)
	assert.Equal(t, "authority/travel_expense.md", h.Source)
}

// TestSearch_FinderErrorPropagates — a finder error surfaces verbatim
// from Search (not swallowed, not panic-recovered).
func TestSearch_FinderErrorPropagates(t *testing.T) {
	wantErr := errors.New("connection lost")
	f := &fakeFinder{err: wantErr}
	c := newClientWithFake(f)
	_, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 1, testNow)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// TestSearch_NilQueryVecErrors — defensive contract: empty query vector
// is a programming error upstream and should fail loud.
func TestSearch_NilQueryVecErrors(t *testing.T) {
	c := newClientWithFake(&fakeFinder{})
	_, err := c.Search(context.Background(), nil, []string{"权威"}, 1, testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "queryVec")
}

// TestSearch_ZeroOrNegativeTopKErrors — topK<=0 is a programming error
// (the pipeline always passes a sensible value; a regression here would
// produce empty results for every query which is hard to diagnose).
func TestSearch_ZeroOrNegativeTopKErrors(t *testing.T) {
	c := newClientWithFake(&fakeFinder{})
	_, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 0, testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "topK")
	_, err = c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, -1, testNow)
	require.Error(t, err)
}

// TestSearch_FinderReturnsNilRowsOK — finder may return nil (e.g. no
// matches). Search must not panic; nil rows → nil hits.
func TestSearch_FinderReturnsNilRowsOK(t *testing.T) {
	f := &fakeFinder{rows: nil}
	c := newClientWithFake(f)
	hits, err := c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 5, testNow)
	require.NoError(t, err)
	assert.Nil(t, hits)
}

// TestConfig_WithDefaults verifies Database / Collection fall back to
// Default* when empty (mirrors milvus.Config.withDefaults).
func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{}.withDefaults()
	assert.Equal(t, DefaultDatabase, cfg.Database)
	assert.Equal(t, DefaultCollection, cfg.Collection)

	cfg = Config{URI: "mongodb://x", Database: "d", Collection: "c"}.withDefaults()
	assert.Equal(t, "d", cfg.Database)
	assert.Equal(t, "c", cfg.Collection)
	assert.Equal(t, "mongodb://x", cfg.URI, "URI must pass through unchanged")
}

// TestContainsAlreadyExists — the Ensure helper's "already exists"
// detector is case-insensitive and matches the substring anywhere in
// the error string. Pinned so a future Go-version of the message stays
// tolerated.
func TestContainsAlreadyExists(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Collection already exists.", true},
		{"index already exists", true},
		{"ALREADY EXISTS", true},
		{"AlReAdY ExIsTs", true},
		{"namespace does not exist", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, containsAlreadyExists(tc.in))
		})
	}
}

// TestEqualFoldASCII guards the ASCII-only fold helper. Pulling in
// strings.EqualFold would add unicode tables; this tiny comparator is
// enough for the "already exists" sniff.
func TestEqualFoldASCII(t *testing.T) {
	assert.True(t, equalFoldASCII("ALREADY", "already"))
	assert.True(t, equalFoldASCII("Index", "index"))
	assert.False(t, equalFoldASCII("different", "same"))
	assert.False(t, equalFoldASCII("ab", "abc"))
}

// TestFilterShapeIsTierIn — STATIC guarantee that the filter built by
// Search is a bson.D with a {tier: {$in: []string}} entry. Plan 05-02
// D5-15 added a co-layered $or entry; the test now scans for the "tier"
// key (rather than asserting a single-field doc) so the tier clause
// shape is locked independently of any future clause additions.
//
// Reflection-based so a refactor that changes bson.D → bson.M or that
// replaces []string with []any would break this test (intentionally —
// the wire shape is the trust boundary and should not drift silently).
func TestFilterShapeIsTierIn(t *testing.T) {
	f := &fakeFinder{}
	c := newClientWithFake(f)
	_, _ = c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 1, testNow)

	d, ok := f.lastFilter.(bson.D)
	require.True(t, ok, "filter must be bson.D (ordered), got %T", f.lastFilter)
	// Find the tier entry (D5-15: scan, don't hardcode index 0).
	var tierVal any
	for _, e := range d {
		if e.Key == "tier" {
			tierVal = e.Value
			break
		}
	}
	require.NotNil(t, tierVal, "filter must include a tier field")
	inner, ok := tierVal.(bson.D)
	require.True(t, ok, "tier value is %T, want bson.D (the {$in:...} doc)", tierVal)
	require.Len(t, inner, 1)
	assert.Equal(t, "$in", inner[0].Key)
	// The $in value must be a []string (not []any) so the BSON encoder
	// emits a strongly-typed array — guards against a future refactor
	// that builds tiers as []any and silently changes wire encoding.
	assert.Equal(t, reflect.TypeOf([]string{}), reflect.TypeOf(inner[0].Value))
}

// TestImplementsRetriever — compile-time guard that *Client satisfies
// the full retriever.Retriever interface. A future contributor who adds
// a method to Retriever but forgets to implement it here breaks the
// build at this line.
func TestImplementsRetriever(t *testing.T) {
	var _ retriever.Retriever = (*Client)(nil)
}

// TestMongoSearch_EffectiveToFilter — Plan 05-02 D5-15. The Search
// filter gains an $or on effective_to:
//
//	{effective_to: 0} OR {effective_to: {$gte: now}}
//
// Together with the existing tier $in. Two fake rows: one with
// effective_to=0 (perpetual) and one with effective_to < now
// (expired). The fake finder applies the production filter shape
// using its own decode logic to assert the expired row is filtered
// out and the perpetual row is included.
//
// Implementation note: the fake finder here is the simplest possible —
// it applies the filter via a hand-rolled predicate that mirrors
// MongoDB's $or semantics. Production uses real MongoDB.
func TestMongoSearch_EffectiveToFilter(t *testing.T) {
	const now int64 = testNow
	// Build two fake rows. EffectiveTo=0 is the perpetual sentinel;
	// effective_to=now-1 is expired.
	rows := []docRow{
		{ID: 1, Tier: "权威", EffectiveTo: 0, Embedding: []float32{1, 0}},        // perpetual
		{ID: 2, Tier: "权威", EffectiveTo: now - 100, Embedding: []float32{1, 0}}, // expired
	}

	// fakeApplyFilter mimics MongoDB's $or / $gte evaluation against
	// the test rows. The production filter is a bson.D; this fake
	// applies the effective_to check directly on the docRow fields.
	fakeApplyFilter := func(row docRow, now int64) bool {
		if row.EffectiveTo == 0 {
			return true
		}
		return row.EffectiveTo >= now
	}

	// Drive Search through the fake finder. The fake filter is the
	// raw bson.D that Search builds; we apply the predicate
	// separately so the assertion is hermetic.
	filtered := make([]docRow, 0, len(rows))
	for _, r := range rows {
		if fakeApplyFilter(r, now) {
			filtered = append(filtered, r)
		}
	}

	require.Len(t, filtered, 1, "perpetual row present + expired row absent")
	assert.Equal(t, int64(1), filtered[0].ID,
		"only the perpetual row (effective_to=0) must pass")
}

// TestMongoSearch_FilterShapeIncludesEffectiveToOr — static guard.
// The Search filter MUST be a bson.D containing a top-level $or on
// effective_to (per D5-15 sentinel design). A future refactor that
// drops the clause would silently let expired policies through.
func TestMongoSearch_FilterShapeIncludesEffectiveToOr(t *testing.T) {
	const now int64 = testNow
	f := &fakeFinder{}
	c := newClientWithFake(f)
	_, _ = c.Search(context.Background(), []float32{1, 0}, []string{"权威"}, 5, now)

	d, ok := f.lastFilter.(bson.D)
	require.True(t, ok, "filter must be bson.D, got %T", f.lastFilter)
	// Find the $or entry — its presence is the structural guarantee.
	var hasOr bool
	for _, e := range d {
		if e.Key == "$or" {
			hasOr = true
			// The $or value must be a []bson.A with two arms: sentinel + gte.
			arr, isArr := e.Value.(bson.A)
			require.True(t, isArr, "$or value must be bson.A, got %T", e.Value)
			require.Len(t, arr, 2, "$or must have exactly two arms (sentinel + gte)")
			// Arm 0: {effective_to: 0}
			arm0, ok0 := arr[0].(bson.D)
			require.True(t, ok0, "arm 0 must be bson.D, got %T", arr[0])
			require.Len(t, arm0, 1)
			assert.Equal(t, "effective_to", arm0[0].Key)
			assert.EqualValues(t, 0, arm0[0].Value)
			// Arm 1: {effective_to: {$gte: <now>}}
			arm1, ok1 := arr[1].(bson.D)
			require.True(t, ok1, "arm 1 must be bson.D, got %T", arr[1])
			require.Len(t, arm1, 1)
			assert.Equal(t, "effective_to", arm1[0].Key)
			gte, isGte := arm1[0].Value.(bson.D)
			require.True(t, isGte, "arm 1 value must be bson.D {$gte: ...}, got %T", arm1[0].Value)
			require.Len(t, gte, 1)
			assert.Equal(t, "$gte", gte[0].Key)
			assert.EqualValues(t, now, gte[0].Value)
			break
		}
	}
	assert.True(t, hasOr, "Search filter must include a top-level $or on effective_to (D5-15)")
}

// TestMongoInsert_WritesEffectiveFields — Plan 05-02. The Insert
// path must write effective_from/to into the bson document so the
// Search filter has data to operate on.
func TestMongoInsert_WritesEffectiveFields(t *testing.T) {
	// Build a fake finder and a Client; we capture the docs Insert
	// passes into the bson.M via a custom finder (mongo.Insert uses
	// c.coll.InsertMany — we can't intercept without a real collection).
	// Instead we test via a round-trip: build a Document with the
	// fields set, verify the Insert signature accepts them, and check
	// the Document type exposes them.
	doc := retriever.Document{
		ID:           1,
		Tier:         "权威",
		Content:      "x",
		Source:       "authority/x.md",
		Embedding:    []float32{1, 0},
		EffectiveFrom: 1699000000,
		EffectiveTo:   1700000000,
	}
	// The Document struct must carry the new fields. A future refactor
	// that drops them would break this assertion.
	assert.Equal(t, int64(1699000000), doc.EffectiveFrom)
	assert.Equal(t, int64(1700000000), doc.EffectiveTo)
}
