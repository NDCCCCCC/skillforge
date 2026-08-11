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

package retriever

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Document represents one seeded markdown fixture row. DocID is the
// human-readable fixture identifier (e.g. "auth_001"); it is NOT persisted
// to Milvus — only the numeric ID is — but is kept on the struct for
// trace logging during seed/search. Source holds the fixture relative path
// and IS persisted so search results can be traced back to a fixture file.
type Document struct {
	DocID     string   // fixture frontmatter id (logging only, not persisted)
	ID        int64    // numeric Milvus primary key (1..N)
	Tier      string   // one of 权威/历史/个人/草稿
	Content   string   // markdown body (post-frontmatter)
	Source    string   // fixture relative path, e.g. "testdata/docs/authority/auth_001.md"
	Embedding []float32
}

// TierSearchResult is a simplified search hit. The raw Milvus SearchResult
// is deliberately not leaked past this package so callers (cmd/spiked,
// future 02-03 pipeline) do not import the Milvus SDK directly.
type TierSearchResult struct {
	ID      int64
	Tier    string
	Content string
	Source  string
	Score   float32
}

// SearchWithTierFilter runs a vector search with the given filter expression
// (produced by BuildMilvusFilter) applied PRE-similarity at the SDK layer
// (Pitfall 3: never post-filter). topK bounds the number of hits. The query
// vector must be cfg.EmbedDim-dimensional.
//
// The filter is a parameter (not derived inside this function) so callers
// can assert on the exact filter string they built — making the trust
// boundary auditable end-to-end.
func SearchWithTierFilter(ctx context.Context, mc client.Client, cfg MilvusConfig, queryVec []float32, filter string, topK int) ([]TierSearchResult, error) {
	cfg = cfg.withDefaults()
	if len(queryVec) != cfg.EmbedDim {
		return nil, fmt.Errorf("queryVec dim=%d, want=%d", len(queryVec), cfg.EmbedDim)
	}

	sp, err := entity.NewIndexIvfFlatSearchParam(DefaultIVFNProbe)
	if err != nil {
		return nil, fmt.Errorf("NewIndexIvfFlatSearchParam: %w", err)
	}

	// SDK v2.4.2 positional Search API (verified against module source).
	// expr (the filter) is the 4th positional argument and is applied at
	// index-scan time, BEFORE vector similarity ranking.
	results, err := mc.Search(
		ctx,
		cfg.CollectionName,
		nil, // partitions (nil = default)
		filter,
		[]string{"id", "tier", "content", "source"},
		[]entity.Vector{entity.FloatVector(queryVec)},
		"embedding",
		entity.COSINE,
		topK,
		sp,
		client.WithSearchQueryConsistencyLevel(entity.ClStrong),
	)
	if err != nil {
		return nil, fmt.Errorf("Search: %w", err)
	}

	out := make([]TierSearchResult, 0)
	for _, sr := range results {
		if sr.Err != nil {
			return nil, fmt.Errorf("SearchResult error: %w", sr.Err)
		}
		tierCol := sr.Fields.GetColumn("tier")
		contentCol := sr.Fields.GetColumn("content")
		sourceCol := sr.Fields.GetColumn("source")
		for i := 0; i < sr.ResultCount; i++ {
			row := TierSearchResult{Score: -1}
			if i < len(sr.Scores) {
				row.Score = sr.Scores[i]
			}
			if sr.IDs != nil {
				if v, e := sr.IDs.GetAsInt64(i); e == nil {
					row.ID = v
				}
			}
			if tierCol != nil {
				if v, e := tierCol.GetAsString(i); e == nil {
					row.Tier = v
				}
			}
			if contentCol != nil {
				if v, e := contentCol.GetAsString(i); e == nil {
					row.Content = v
				}
			}
			if sourceCol != nil {
				if v, e := sourceCol.GetAsString(i); e == nil {
					row.Source = v
				}
			}
			out = append(out, row)
		}
	}
	return out, nil
}
