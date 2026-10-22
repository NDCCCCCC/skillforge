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

// rank.go 是 embedded 的纯函数排序层。它**只收候选集**（tier_isolation.go
// 的 candidates 的输出），并且不认识 tiers / effective_to —— 那是过滤层的
// 职责。两者物理分离，使"禁止全量打分后过滤"（49-RESEARCH Pitfall 11）成为
// 结构约束而不是纪律要求。

import (
	"sort"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/vecmath"
)

// row is the in-memory storage record.
//
// Content 只在内存里（缓存不存正文：正文可以随时从语料重读，缓存只负责
// 省掉 218 次 embedding 网络调用）。所以缓存命中的行 Content 为空，正文由
// 上层按 Source 重新读取 —— 这一取舍让缓存文件从 ~1MB 降到 ~0.9MB（1024 维
// float32 × 218 ≈ 893KB）且永远不会与语料正文不一致。
type row struct {
	ID            int64
	Tier          string
	Content       string
	Source        string
	Embedding     []float32
	EffectiveFrom int64
	EffectiveTo   int64
}

// ranked is one scored candidate.
type ranked struct {
	row   row
	score float32
}

// rank scores the candidate set against queryVec and returns the top k hits.
//
// 逐字照搬 internal/retriever/mongo/rank.go:94-132 的算法（真源在那里）：
//   - 三个前置守卫：k <= 0 / 空候选集 / 空 queryVec → nil（不是错误）
//   - 跳过维度不符与空 embedding 的行（vecmath 的 Cosine 在这些输入上
//     **panic** 而不是返回错误 —— 跳过是唯一安全选择）
//   - sort.SliceStable + (score desc, id asc) 的 tie-break
//   - 最后按 k 截断并映射成 retriever.Hit
//
// 为什么 tie-break 必须有 id asc：全量同分（同一向量、或 zero-norm 全 0）
// 时，没有二级键的顺序取决于输入顺序，而输入顺序在并发 seed 下不稳定 ⇒
// eval 结果不可复现。mongo 已经踩过这个坑，这里照抄它的修法。
//
// 相似度**必须**调用 vecmath 包的 Cosine：第二份 cosine 实现会带来"两个
// 后端的相似度在小数第几位上不一致"的隐性分裂（49-PATTERNS §1 明确禁止）。
func rank(rows []row, queryVec []float32, k int) []retriever.Hit {
	if k <= 0 || len(rows) == 0 || len(queryVec) == 0 {
		return nil
	}
	out := make([]ranked, 0, len(rows))
	for _, r := range rows {
		if len(r.Embedding) != len(queryVec) || len(r.Embedding) == 0 {
			continue
		}
		out = append(out, ranked{row: r, score: vecmath.Cosine(r.Embedding, queryVec)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].row.ID < out[j].row.ID
	})
	if len(out) > k {
		out = out[:k]
	}
	hits := make([]retriever.Hit, 0, len(out))
	for _, r := range out {
		hits = append(hits, retriever.Hit{
			ID:            r.row.ID,
			Score:         r.score,
			Tier:          r.row.Tier,
			Content:       r.row.Content,
			Source:        r.row.Source,
			EffectiveFrom: r.row.EffectiveFrom,
			EffectiveTo:   r.row.EffectiveTo,
		})
	}
	return hits
}
