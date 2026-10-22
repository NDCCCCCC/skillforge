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

package embedded

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vec builds a dim-length vector whose first component carries `head` and the
// rest are zero-filled. Tests use it instead of random vectors so every
// expected cosine is hand-computable and reproducible.
func vec(dim int, head float32) []float32 {
	v := make([]float32, dim)
	if dim > 0 {
		v[0] = head
	}
	return v
}

// TestRankOrdersByScoreDescThenIDAsc locks the exact tie-break contract
// copied from internal/retriever/mongo/rank.go:109-114: score desc, then
// id asc. Without the id tie-break a re-run could reorder equal scores and
// break eval reproducibility.
func TestRankOrdersByScoreDescThenIDAsc(t *testing.T) {
	query := []float32{1, 0, 0}

	// 三行同分（同一向量）不同 ID：输出必须是 ID 升序。
	rows := []row{
		{ID: 30, Tier: "authority", Embedding: []float32{1, 0, 0}},
		{ID: 10, Tier: "authority", Embedding: []float32{1, 0, 0}},
		{ID: 20, Tier: "authority", Embedding: []float32{1, 0, 0}},
	}
	hits := rank(rows, query, 3)
	require.Len(t, hits, 3)
	assert.Equal(t, int64(10), hits[0].ID)
	assert.Equal(t, int64(20), hits[1].ID)
	assert.Equal(t, int64(30), hits[2].ID)

	// 两个不同分：高分在前，与插入序无关。
	rows = []row{
		{ID: 1, Tier: "authority", Embedding: []float32{0, 1, 0}},  // cos = 0
		{ID: 2, Tier: "authority", Embedding: []float32{1, 0, 0}},  // cos = 1
	}
	hits = rank(rows, query, 2)
	require.Len(t, hits, 2)
	assert.Equal(t, int64(2), hits[0].ID)
	assert.InDelta(t, float32(1), hits[0].Score, 1e-6)
	assert.Equal(t, int64(1), hits[1].ID)
	assert.InDelta(t, float32(0), hits[1].Score, 1e-6)
}

// TestRankSkipsDimMismatchAndEmptyEmbedding: 维度不符与空 embedding 的行必须
// 被跳过而不是让 vecmath.Cosine panic（它是 panic 而不是返回错误）。
func TestRankSkipsDimMismatchAndEmptyEmbedding(t *testing.T) {
	query := []float32{1, 0, 0}
	rows := []row{
		{ID: 1, Embedding: []float32{1, 0}},      // 长度不符
		{ID: 2, Embedding: nil},                  // 空
		{ID: 3, Embedding: []float32{}},          // 空（len==0）
		{ID: 4, Embedding: []float32{1, 0, 0}},   // 合法
	}
	assert.NotPanics(t, func() {
		got := rank(rows, query, 10)
		require.Len(t, got, 1)
		assert.Equal(t, int64(4), got[0].ID)
	})
}

// TestRankDeterministicAcrossRuns: 同输入连跑 20 次结果必须逐字段全等。
func TestRankDeterministicAcrossRuns(t *testing.T) {
	query := []float32{0.6, 0.8, 0}
	rows := []row{
		{ID: 7, Tier: "authority", Content: "a", Source: "authority/a.md", Embedding: []float32{0.6, 0.8, 0}},
		{ID: 3, Tier: "authority", Content: "b", Source: "authority/b.md", Embedding: []float32{0.6, 0.8, 0}},
		{ID: 9, Tier: "sql-guides", Content: "c", Source: "sql-guides/c.md", Embedding: []float32{0, 0, 1}},
	}
	first := rank(rows, query, 3)
	require.NotEmpty(t, first)
	for i := 0; i < 20; i++ {
		assert.True(t, reflect.DeepEqual(first, rank(rows, query, 3)),
			"run %d produced a different ordering", i)
	}
}

// TestRankTopKGuards: k <= 0 返回 nil；k > len(rows) 返回全部；空输入返回 nil。
func TestRankTopKGuards(t *testing.T) {
	query := []float32{1, 0, 0}
	rows := []row{
		{ID: 1, Embedding: []float32{1, 0, 0}},
		{ID: 2, Embedding: []float32{0, 1, 0}},
	}
	assert.Nil(t, rank(rows, query, 0))
	assert.Nil(t, rank(rows, query, -1))
	assert.Nil(t, rank(nil, query, 5))
	assert.Nil(t, rank(rows, nil, 5))
	assert.Len(t, rank(rows, query, 99), 2)
	assert.Len(t, rank(rows, query, 1), 1)
}
