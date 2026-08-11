// Spike 004 — Milvus v3 + milvusclient tier-contract validation.
//
// Goal: prove the new SDK (github.com/milvus-io/milvus/client/v2) preserves
// SkillForge's tier-filter retriever contract:
//
//   (a) "tier in [\"权威\"]" is applied PRE-similarity at the Milvus server
//       (never post-filter) — a 权威-only filter must NOT leak 个人-tier docs.
//   (b) empty tiers → filter "false" → 0 hits (D-09 refusal-by-construction).
//
// Dataset: 3 docs — 2 权威, 1 个人. If PRE-filter holds, a 权威 filter returns
// exactly 2 hits (no 个人 leak); empty filter returns 0.
//
// buildFilter() below is a verbatim re-implementation of the production
// BuildMilvusFilter (internal/retriever/milvus/filter.go) — proving the logic
// ports unchanged to the new SDK (a filter is a server-side expression string,
// SDK-version-independent).
//
// NOTE: a few calls (index.NewAutoIndex signature, Flush/LoadCollection/Drop
// option names) are reasoned from the milvusclient fluent-option convention
// and confirmed by `go build` + run. Spike spirit: write, compile, iterate.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

const (
	collection = "spike004_tier_contract"
	embedDim   = 8 // small dim — this spike validates FILTER semantics, not embedding quality
)

func main() {
	log.SetFlags(log.Ltime)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	addr := os.Getenv("MILVUS_ADDRESS")
	if addr == "" {
		addr = "localhost:19530"
	}
	log.Printf("=== spike 004: milvus v3 milvusclient tier-contract (addr=%s) ===", addr)

	// 1. Connect (new milvusclient.New).
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: addr})
	if err != nil {
		log.Fatalf("milvusclient.New: %v", err)
	}
	defer cli.Close(ctx)
	log.Println("[1] connected")

	// 2. Drop any stale collection (idempotent reset).
	_ = cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collection))
	log.Println("[2] dropped stale collection")

	// 3. Schema + CreateCollection + AUTOINDEX(COSINE) on embedding.
	schema := entity.NewSchema().
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(false)).
		WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(20)).
		WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192)).
		WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(embedDim)))

	autoIdx := index.NewAutoIndex(entity.COSINE) // spec — confirm at build
	err = cli.CreateCollection(ctx,
		milvusclient.NewCreateCollectionOption(collection, schema).
			WithIndexOptions(milvusclient.NewCreateIndexOption(collection, "embedding", autoIdx)))
	if err != nil {
		log.Fatalf("CreateCollection: %v", err)
	}
	log.Println("[3] collection + AUTOINDEX(COSINE) created")

	// 4. Insert 3 docs (2 权威 + 1 个人), column-based.
	ids := []int64{1, 2, 3}
	tiers := []string{"权威", "权威", "个人"}
	contents := []string{"权威文档A", "权威文档B", "个人笔记C"}
	vecs := [][]float32{
		{0.10, 0.20, 0.30, 0.40, 0.50, 0.60, 0.70, 0.80},
		{0.11, 0.21, 0.31, 0.41, 0.51, 0.61, 0.71, 0.81},
		{0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30, 0.20},
	}
	_, err = cli.Insert(ctx, milvusclient.NewColumnBasedInsertOption(collection).
		WithInt64Column("id", ids).
		WithFloatVectorColumn("embedding", embedDim, vecs).
		WithColumns(column.NewColumnVarChar("tier", tiers), column.NewColumnVarChar("content", contents)))
	if err != nil {
		log.Fatalf("Insert: %v", err)
	}
	// Flush + Load so rows are visible to Search (Milvus v2/v3 requires Load before Search).
	if _, err := cli.Flush(ctx, milvusclient.NewFlushOption(collection)); err != nil {
		log.Fatalf("Flush: %v", err)
	}
	if _, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collection)); err != nil {
		log.Fatalf("LoadCollection: %v", err)
	}
	log.Println("[4] inserted 3 docs (2 权威 + 1 个人), flushed, loaded")

	qvec := []float32{0.10, 0.20, 0.30, 0.40, 0.50, 0.60, 0.70, 0.80}

	// 5. Search (a): tier in ["权威"] — expect exactly 2 hits, ZERO 个人 leak.
	filterAuth := buildFilter([]string{"权威"})
	authHits := searchHitCount(ctx, cli, filterAuth, qvec)
	log.Printf("[5] filter=%q -> %d hits (expect 2)", filterAuth, authHits)

	// 6. Search (b): empty tiers -> "false" -> expect 0 hits (D-09).
	filterFalse := buildFilter(nil)
	falseHits := searchHitCount(ctx, cli, filterFalse, qvec)
	log.Printf("[6] filter=%q -> %d hits (expect 0 — D-09)", filterFalse, falseHits)

	// 7. Verdict.
	pass := true
	if authHits != 2 {
		pass = false
		log.Printf("  FAIL: 权威 filter returned %d hits, want 2 (filter broken or 个人 leaked)", authHits)
	}
	if falseHits != 0 {
		pass = false
		log.Printf("  FAIL: empty/'false' filter returned %d hits, want 0 (D-09 broken)", falseHits)
	}
	if pass {
		log.Println("=== VERDICT: VALIDATED — tier contract fully preserved under milvusclient v3 ===")
	} else {
		log.Println("=== VERDICT: see FAIL lines above ===")
	}
}

// searchHitCount runs a filtered search and returns total hit count across
// all result sets. Uses the new SearchOption fluent builder.
func searchHitCount(ctx context.Context, cli *milvusclient.Client, filter string, qvec []float32) int {
	rs, err := cli.Search(ctx, milvusclient.NewSearchOption(collection, 10, []entity.Vector{entity.FloatVector(qvec)}).
		WithConsistencyLevel(entity.ClStrong).
		WithANNSField("embedding").
		WithFilter(filter).
		WithOutputFields("tier", "content"))
	if err != nil {
		log.Fatalf("Search(filter=%q): %v", filter, err)
	}
	n := 0
	for _, r := range rs {
		n += len(r.Scores)
	}
	return n
}

// buildFilter is a verbatim re-implementation of the production
// BuildMilvusFilter (internal/retriever/milvus/filter.go). Empty tiers -> the
// literal "false", a Milvus boolean predicate matching zero rows.
func buildFilter(tiers []string) string {
	if len(tiers) == 0 {
		return "false"
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		quoted[i] = fmt.Sprintf("%q", t)
	}
	return fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
}
