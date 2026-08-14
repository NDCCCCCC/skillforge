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

package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
)

// Search runs a tier-filtered vector search against Milvus. The
// filter is built INSIDE Search via BuildMilvusFilter(tiers, now) and
// applied PRE-similarity at the SDK layer (never post-filter).
// Empty/zero tiers → "false" default-deny → 0 hits (D-09).
//
// Plan 05-02 / D5-15: the filter co-layers the effective_to PRE-filter
// at the same layer as the tier filter (NOT post-filter). `now` is the
// epoch-seconds cutoff computed once at pipeline entry.
func (m *MilvusClient) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	cfg := m.cfg.withDefaults()
	if len(queryVec) != cfg.EmbedDim {
		return nil, fmt.Errorf("queryVec dim=%d, want=%d", len(queryVec), cfg.EmbedDim)
	}
	filter := BuildMilvusFilter(tiers, now)

	sp, err := entity.NewIndexIvfFlatSearchParam(DefaultIVFNProbe)
	if err != nil {
		return nil, fmt.Errorf("NewIndexIvfFlatSearchParam: %w", err)
	}

	// SDK v2.4.2 positional Search API (verified against module source).
	// expr (the filter) is the 4th positional argument and is applied at
	// index-scan time, BEFORE vector similarity ranking.
	results, err := m.mc.Search(
		ctx,
		cfg.CollectionName,
		nil, // partitions (nil = default)
		filter,
		// OutputFields include effective_from + effective_to (Plan 05-02 D5-15)
		// so the returned Hit carries the policy-version window for audit.
		[]string{"id", "tier", "content", "source", "effective_from", "effective_to"},
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

	out := make([]retriever.Hit, 0)
	for _, sr := range results {
		if sr.Err != nil {
			return nil, fmt.Errorf("SearchResult error: %w", sr.Err)
		}
		tierCol := sr.Fields.GetColumn("tier")
		contentCol := sr.Fields.GetColumn("content")
		sourceCol := sr.Fields.GetColumn("source")
		effectiveFromCol := sr.Fields.GetColumn("effective_from")
		effectiveToCol := sr.Fields.GetColumn("effective_to")
		for i := 0; i < sr.ResultCount; i++ {
			row := retriever.Hit{Score: -1}
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
			if effectiveFromCol != nil {
				if v, e := effectiveFromCol.GetAsInt64(i); e == nil {
					row.EffectiveFrom = v
				}
			}
			if effectiveToCol != nil {
				if v, e := effectiveToCol.GetAsInt64(i); e == nil {
					row.EffectiveTo = v
				}
			}
			out = append(out, row)
		}
	}
	return out, nil
}
