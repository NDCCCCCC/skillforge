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
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/retriever"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Network resilience — mirrors the milvus dial timeouts so a typo in
// MONGO_URI fails fast (not after the default 30s server-selection
// timeout) while still tolerating a transient LAN blip.
const (
	dialTimeout = 5 * time.Second
	pingTimeout = 5 * time.Second
)

// Default Database / Collection names. The operator can override via
// Config; cmd/server always sets these explicitly so the defaults are
// only used by ad-hoc tooling / tests that construct Config{} directly.
const (
	DefaultDatabase   = "skillforge"
	DefaultCollection = "knowledge"
)

// Config configures the MongoDB client. URI is required; Database and
// Collection fall back to Default* when empty (mirrors the milvus.Config
// withDefaults pattern).
type Config struct {
	URI        string // e.g. "mongodb://your-milvus-host:27017"
	Database   string // defaults to DefaultDatabase
	Collection string // defaults to DefaultCollection
}

func (cfg Config) withDefaults() Config {
	if cfg.Database == "" {
		cfg.Database = DefaultDatabase
	}
	if cfg.Collection == "" {
		cfg.Collection = DefaultCollection
	}
	return cfg
}

// rowFinder is the testable seam for the row-fetch step of Search.
// Production wires this to mongoCollectionFinder (a thin wrapper around
// *mongo.Collection.Find); tests inject a fake that returns canned rows
// so the Search ranking logic can be exercised without a live MongoDB.
//
// The filter argument is whatever Search builds (a bson.D in production)
// and the finder is expected to apply it unchanged — this is where the
// PRE-similarity tier gate lives (SYSK-07 / P-RAG-2).
type rowFinder interface {
	FindRows(ctx context.Context, filter any) ([]docRow, error)
}

// aggregator is the testable seam for the aggregation step of GetMaxID.
// Production wires this to mongoCollectionAggregator; tests inject a fake
// that returns canned cursor results without a live MongoDB.
type aggregator interface {
	Aggregate(ctx context.Context, pipeline any) (aggregatorCursor, error)
}

// aggregatorCursor is the subset of *mongo.Cursor that GetMaxID needs.
// It is the return type of aggregator.Aggregate.
type aggregatorCursor interface {
	All(ctx context.Context, out interface{}) error
	Close(ctx context.Context) error
}

// mongoCollectionAggregator adapts *mongo.Collection to the aggregator
// interface.
type mongoCollectionAggregator struct {
	coll *mongo.Collection
}

func (a mongoCollectionAggregator) Aggregate(ctx context.Context, pipeline any) (aggregatorCursor, error) {
	return a.coll.Aggregate(ctx, pipeline)
}

// mongoCollectionFinder adapts *mongo.Collection to the rowFinder
// interface. Find → cursor → cursor.All decodes the BSON rows into the
// docRow value type with no per-row allocation beyond the slice grow.
type mongoCollectionFinder struct {
	coll *mongo.Collection
}

func (f mongoCollectionFinder) FindRows(ctx context.Context, filter any) ([]docRow, error) {
	cur, err := f.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("mongo.Find: %w", err)
	}
	defer func() {
		// Use a fresh context for Close so a cancelled query does not
		// leak the cursor. The driver tolerates a nil/ctx-cancelled Close.
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cur.Close(closeCtx)
	}()
	var rows []docRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("mongo.Cursor.All: %w", err)
	}
	return rows, nil
}

// Client implements retriever.Retriever against a MongoDB 4.4+ server.
// It owns the underlying mongo-driver *Client + the resolved collection
// handle. The tier-filter PRE-cosine contract (SYSK-07, P-RAG-2) is
// enforced inside Search via the {tier: {$in: tiers}} filter.
type Client struct {
	cfg    Config
	conn   *mongo.Client
	coll   *mongo.Collection
	finder rowFinder  // injectable; defaults to mongoCollectionFinder{coll}
	agg    aggregator // injectable; defaults to mongoCollectionAggregator{coll}
}

// Compile-time check that *Client satisfies retriever.Retriever.
// Drift in the interface will fail the build here, not at runtime.
var _ retriever.Retriever = (*Client)(nil)

// New dials MongoDB, pings it (Connect itself does not validate
// reachability), resolves the collection handle, and returns a connected
// Client. cfg.URI is required; cfg.Database / cfg.Collection default.
//
// Bounded by dialTimeout for the connect+ping so a typo in MONGO_URI
// surfaces in seconds rather than after the driver's 30s server-selection
// timeout. After New the collection may not yet exist — call Ensure to
// create it (idempotent).
func New(ctx context.Context, cfg Config) (*Client, error) {
	cfg = cfg.withDefaults()
	if cfg.URI == "" {
		return nil, errors.New("mongo.New: Config.URI is empty (set MONGO_URI)")
	}

	conn, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, fmt.Errorf("mongo.Connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := conn.Ping(pingCtx, readpref.Primary()); err != nil {
		// Best-effort disconnect so we don't leak the connection pool when
		// the ping fails. Errors are ignored — the ping error is the one
		// we want to surface.
		_ = conn.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo.Ping %s: %w", cfg.URI, err)
	}

	coll := conn.Database(cfg.Database).Collection(cfg.Collection)
	c := &Client{
		cfg:    cfg,
		conn:   conn,
		coll:   coll,
		finder: mongoCollectionFinder{coll: coll},
		agg:    mongoCollectionAggregator{coll: coll},
	}
	return c, nil
}

// Health probes the live connection (Ping). Used by cmd/server startup
// to fail fast on a misconfigured MONGO_URI.
func (c *Client) Health(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return c.conn.Ping(pingCtx, readpref.Primary())
}

// Disconnect closes the underlying client. The instance is unusable
// after this call.
func (c *Client) Disconnect() error {
	if c.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	return c.conn.Disconnect(ctx)
}

// GetMaxID returns the highest chunk ID currently stored for the given
// tier via aggregation pipeline [{$match: {tier}}, {$sort: {id: -1}},
// {$limit: 1}]. Returns 0 (no error) when no chunks exist for the tier.
// Implements the retriever.Retriever contract added by Phase 27/Plan 02.
func (c *Client) GetMaxID(ctx context.Context, tier string) (int64, error) {
	if c == nil || c.agg == nil {
		return 0, nil
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"tier": tier}}},
		{{Key: "$sort", Value: bson.M{"id": -1}}},
		{{Key: "$limit", Value: 1}},
	}
	cur, err := c.agg.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("mongo.GetMaxID: Aggregate: %w", err)
	}
	var rows []struct{ ID int64 `bson:"id"` }
	if err := cur.All(ctx, &rows); err != nil {
		return 0, fmt.Errorf("mongo.GetMaxID: cursor.All: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].ID, nil
}

// Ensure creates the collection + a {tier:1} index if absent. Idempotent
// — safe to call on every startup. The {tier:1} index makes the PRE-
// similarity tier filter an indexed lookup; for the Phase 2 ~30-row
// corpus this is overkill, but it future-proofs against a 100k-row
// corpus where a collection scan on tier would dominate the latency.
//
// We use CreateCollection explicitly so a brand-new MongoDB instance
// gets the collection created with our defaults (no validator needed —
// the indexer writes well-formed docs and we trust the write path).
func (c *Client) Ensure(ctx context.Context) error {
	db := c.conn.Database(c.cfg.Database)

	// CreateCollection errors if the collection already exists; that case
	// is expected and tolerated. Any other error is surfaced.
	if err := db.CreateCollection(ctx, c.cfg.Collection); err != nil {
		// v2 driver returns a mongo.CommandError with a "collection already
		// exists" NamespaceExists code (48) on re-run. Tolerate it.
		var ce mongo.CommandError
		if !errors.As(err, &ce) || !isNamespaceExists(ce) {
			// Fall back to a name-based sniff so we tolerate drivers/versions
			// that phrase the error differently. Lowercase substring match
			// is intentionally permissive.
			if !containsAlreadyExists(err.Error()) {
				return fmt.Errorf("mongo.CreateCollection(%q): %w", c.cfg.Collection, err)
			}
		}
	}

	// Create the {tier:1} index. CreateOne tolerates a re-run by treating
	// "IndexOptionsConflict" / already-exists as success.
	idxName, err := c.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "tier", Value: 1}},
	})
	if err != nil && !containsAlreadyExists(err.Error()) {
		return fmt.Errorf("mongo.CreateIndex(tier): %w (name=%q)", err, idxName)
	}
	return nil
}

// Reset deletes every document in the collection. It makes the seed
// path idempotent: SeedFromKnowledgeDir calls Reset before Insert so
// repeated server starts (which re-seed the whole corpus) do not
// accumulate duplicate rows. Without this, every restart appends
// another copy of each chunk and top-K retrieval fills with duplicates
// of whatever chunk happens to score highest — observed in live verify
// as 4× the same id in the top-5.
func (c *Client) Reset(ctx context.Context) error {
	if _, err := c.coll.DeleteMany(ctx, bson.D{}); err != nil {
		return fmt.Errorf("mongo.Reset: DeleteMany: %w", err)
	}
	return nil
}

// isNamespaceExists returns true for the v2 "NamespaceExists" CommandError
// code (48). Kept as a helper so the Ensure path reads cleanly.
func isNamespaceExists(ce mongo.CommandError) bool {
	// The v2 CommandError exposes Code via an embedded field; check both
	// the named code and the message as a defensive fallback.
	return ce.Code == 48 || containsAlreadyExists(ce.Message)
}

// containsAlreadyExists matches the substring "already exists" in any
// case. The driver's exact wording has varied across versions; a
// case-insensitive substring is the most stable predicate.
func containsAlreadyExists(s string) bool {
	const needle = "already exists"
	// ASCII-lowercase to avoid pulling unicode/norm tables.
	for i := 0; i+len(needle) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

// equalFoldASCII is a tiny ASCII-only fold comparator used by
// containsAlreadyExists. Pulling in strings.ToLower would add a unicode
// table dependency for a single substring check.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Insert adds docs to the collection via InsertMany. Caller pre-assigns
// numeric IDs; we do NOT auto-dedup (mirrors the milvus MilvusClient.Insert
// contract — re-running inserts duplicates).
//
// Each retriever.Document is wrapped in a bson.M so the field names on
// the wire match the docRow bson tags (id/tier/content/source/embedding)
// exactly. We do not insert an _id: the collection uses MongoDB's default
// ObjectId for _id and stores our 1-based numeric ID in the `id` field,
// matching the milvus collection layout (where id IS the primary key).
//
// Plan 05-02 D5-15: EffectiveFrom + EffectiveTo are written as INT64
// columns (sentinel-0 design, NOT nullable). The default 0 value is the
// explicit "perpetual" state.
func (c *Client) Insert(ctx context.Context, docs []retriever.Document) error {
	if len(docs) == 0 {
		return errors.New("mongo.Insert: no docs to insert")
	}
	docsAny := make([]any, len(docs))
	for i, d := range docs {
		docsAny[i] = bson.M{
			"id":             d.ID,
			"tier":           d.Tier,
			"content":        d.Content,
			"source":         d.Source,
			"embedding":      d.Embedding,
			"effective_from": d.EffectiveFrom,
			"effective_to":   d.EffectiveTo,
		}
	}
	if _, err := c.coll.InsertMany(ctx, docsAny); err != nil {
		return fmt.Errorf("mongo.InsertMany: %w", err)
	}
	return nil
}

// Search applies the tier filter as a real MongoDB query BEFORE computing
// cosine similarity in Go. The PRE-filter semantics (SYSK-07, P-RAG-2)
// are enforced by the database query itself — we never load all docs and
// then filter.
//
// The tier values are knowledge-base registry slugs (what kb.Registry's
// ActiveSlugs / ResolveReads produce). Phase 26 moved the VALUE source from
// the retired 4-value CN-char tier enum to the registry; the filter shape,
// its PRE-similarity position, and the default-deny semantics below are
// unchanged.
//
// Empty tiers → {tier: {$in: []}} matches no documents → 0 hits. This
// is the structural refusal-by-construction (D-09): the database itself
// refuses, independent of any agent policy. We do not special-case the
// empty-tiers branch in Go — the same code path runs and produces zero
// rows, which is the precise guarantee D-09 requires.
//
// Plan 05-02 D5-15: Search gains a `now int64` parameter. The filter
// gains an $or clause on effective_to, co-layered at the same DB layer
// as the tier filter (NEVER a post-filter fallback). The clause has two
// arms:
//
//	{effective_to: 0}               — perpetual (sentinel)
//	{effective_to: {$gte: <now>}}   — still in the validity window
//
// Sentinel-0 design (NOT nullable) sidesteps MongoDB's
// NULL-evaluates-FALSE pitfall: legacy rows with no effective_*
// metadata get the default 0 (perpetual) and pass the predicate.
//
// The queryVec length is validated against the first row's embedding
// length (defensive — production inserts a fixed BGE-M3 dim=1024). The
// per-row length check happens inside rank() which skips mismatched rows
// rather than panicking.
func (c *Client) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	if topK <= 0 {
		return nil, errors.New("mongo.Search: topK must be > 0")
	}
	if len(queryVec) == 0 {
		return nil, errors.New("mongo.Search: queryVec is empty")
	}

	// {tier: {$in: tiers}}. Empty tiers → $in:[] → no matches → 0 hits
	// (D-09). This is THE trust boundary: building the filter any other
	// way (e.g. skipping the find on empty tiers) would still produce
	// 0 rows but would not be the DATABASE refusing — the structural
	// guarantee would be in Go, not in the storage layer. Always build
	// the filter, always run the find.
	//
	// D5-15: the effective_to $or clause is co-layered at the same DB
	// query level as the tier $in. bson.D is ordered, so the tier
	// clause runs first (the selectivity win — a default-deny Skill
	// never evaluates the effective clause).
	filter := bson.D{
		{Key: "tier", Value: bson.D{{Key: "$in", Value: tiers}}},
		{
			Key: "$or",
			Value: bson.A{
				bson.D{{Key: "effective_to", Value: 0}},
				bson.D{{Key: "effective_to", Value: bson.D{{Key: "$gte", Value: now}}}},
			},
		},
	}

	rows, err := c.finder.FindRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rank(rows, queryVec, topK), nil
}

// CollectionName exposes the resolved collection name (used by cmd/server
// logging + future tooling). Read-only — callers must not mutate.
func (c *Client) CollectionName() string { return c.cfg.Collection }

// DatabaseName exposes the resolved database name (mirrors CollectionName).
func (c *Client) DatabaseName() string { return c.cfg.Database }

// ListMetadata returns the metadata projection of every document in the
// collection. Plan 05-05 / SC-5 / KNOW-06 — the graph browser data
// source. No tier filter, no effective_to PRE-filter (the graph
// deliberately includes superseded chunks so the operator can browse
// them with the 已失效 badge per UI-SPEC §SC-8).
//
// The fields decoded are id / tier / source / effective_from /
// effective_to only — embedding + content are not loaded. This keeps
// the network round-trip lean and avoids loading chunk text into the
// browser (the graph endpoint contract; threat T-5-22 mitigation).
//
// Implementation note: we reuse the existing mongoCollectionFinder with
// a permissive bson.D{{}} filter (matches everything). For very large
// corpora (>10k docs) this would warrant a cursor + pagination seam;
// the Phase 2 corpus is hand-authored and ~30 docs total.
func (c *Client) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	rows, err := c.finder.FindRows(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("mongo.ListMetadata: FindRows: %w", err)
	}
	out := make([]retriever.DocumentMeta, 0, len(rows))
	for _, r := range rows {
		out = append(out, retriever.DocumentMeta{
			ID:            r.ID,
			Tier:          r.Tier,
			Source:        r.Source,
			EffectiveFrom: r.EffectiveFrom,
			EffectiveTo:   r.EffectiveTo,
		})
	}
	return out, nil
}

// GetChunk looks up a single chunk by numeric primary key and returns
// its verbatim content + metadata. Plan 260813-dno / UAT #4 — the
// on-demand chunk-content fetch backing GET /api/knowledge/chunk/:id.
// The graph payload stays metadata-only (T-5-22); this is the separate
// fetch path the FE fires when the operator clicks a graph chunk.
//
// Implementation mirrors ListMetadata's rowFinder seam: a FindRows
// call with a {id: <id>} filter. len(rows)==0 → ErrChunkNotFound (soft
// miss, → 404 at the server); any finder error is wrapped (→ 500). The
// D-09 trust boundary is preserved because the id was already
// enumerated by ListMetadata (the graph only lists existing chunks).
//
// docRow (mongo/rank.go) carries every field incl Content, so the
// mapping is a straight 1:1 field copy. No tier filter is applied
// here — this is a by-PK read of a specific chunk the caller already
// chose to view, not a similarity search.
func (c *Client) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	rows, err := c.finder.FindRows(ctx, bson.D{{Key: "id", Value: id}})
	if err != nil {
		return retriever.ChunkContent{}, fmt.Errorf("mongo.GetChunk: FindRows: %w", err)
	}
	if len(rows) == 0 {
		return retriever.ChunkContent{}, retriever.ErrChunkNotFound
	}
	r := rows[0]
	return retriever.ChunkContent{
		ID:            r.ID,
		Tier:          r.Tier,
		Source:        r.Source,
		Content:       r.Content,
		EffectiveFrom: r.EffectiveFrom,
		EffectiveTo:   r.EffectiveTo,
	}, nil
}
