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

// tier_isolation.go 是 embedded 后端的唯一过滤点，也是本包最重要的文件。
//
// 三件事必须在读代码之前知道：
//
//  1. tier 过滤是 PRE-similarity（internal/retriever/retriever.go:142-153 的
//     契约：`The filter is applied PRE-similarity at the SDK layer`）。被禁
//     tier 的行不得出现在任何打分路径上 —— 不是"打完分不返回"，而是
//     "根本进不了打分函数"。
//
//  2. 空 tiers 返回 0 命中是 refusal-by-construction（D-09），不是"全返回"。
//     对齐两条既有实现：internal/retriever/milvus/filter.go:64-84（空 tier
//     直接产出字面量 `false`）、internal/retriever/mongo/mongo.go:416-425
//     （`{tier: {$in: []}}` 匹配不到任何行）。这条语义是"未声明 reads 的
//     Skill 什么都读不到"的结构性保证，一旦改成"空 = 全部"就是信任边界塌陷。
//
//  3. 禁止「全量打分后再 filter」。那种写法的信任边界交给打分逻辑：只要有人
//     把过滤写错、写晚、或写成 `if hit.Tier != ...` 的软过滤，被禁内容就会
//     在内存里被算过、在日志/跟踪里出现过（49-RESEARCH Pitfall 11）。
//     本文件在物理上与 rank.go 分开，且不引用任何打分符号。

// tierAllowed reports whether tier is in the tiers whitelist.
//
// 空 tiers（nil 或 len==0）返回 false —— default-deny，调用方（candidates）
// 依赖这一点，但 candidates 自身也有独立的提前返回，不把正确性压在一条
// 隐含路径上。
//
// 精确匹配、大小写敏感：tier 值是 registry slug（knowledge_bases.yaml 的
// slug），slug 本身就是大小写敏感的标识符，模糊匹配会把 `Authority` 之类
// 的伪 slug 放进信任集。
//
// 线性查找而不是 map：218 篇规模 + 每次 Search 一次，map 的构造开销大于
// 收益，且 map 迭代序会给"确定性"叙事添一个不必要的变量。
func tierAllowed(tiers []string, tier string) bool {
	if len(tiers) == 0 {
		return false
	}
	for _, t := range tiers {
		if t == tier {
			return true
		}
	}
	return false
}

// effectiveAllowed reports whether a row with the given effective_to is still
// in its validity window at now.
//
// 哨兵 0 = 永久有效（internal/retriever/mongo/mongo.go:364-396 的 sentinel
// 设计，不是 nullable）。边界相等必须保留：effective_to == now 表示
// "此刻仍然有效"（mongo 侧用 `$gte: now`，milvus 侧同义），把它排除掉会
// 让一个刚到期的策略提前一秒消失。
//
// 对齐 mongo 的 `$or: [{effective_to: 0}, {effective_to: {$gte: now}}]`。
func effectiveAllowed(effectiveTo, now int64) bool {
	return effectiveTo == 0 || effectiveTo >= now
}

// candidates 构造候选集：先按 tiers，再按 effective_to。
//
// 这是本包唯一的过滤点，也是 Search 唯一允许产生候选集的地方。rank 只收到
// 本函数的返回值，因此"被禁 tier 的内容绝不会被 cosine 读到"是结构性事实，
// 不依赖任何调用约定。
//
// 空 tiers 的提前返回必须是函数体的第一条语句（在任何循环之前）：这样
// "空 tier 不产生任何候选" 不依赖循环内某个分支的正确性，空集合就是函数
// 的直接值。空候选集 ⇒ Search 直接 return nil, nil，rank 根本不被调用。
func candidates(docs []row, tiers []string, now int64) []row {
	if len(tiers) == 0 {
		return nil
	}
	out := make([]row, 0, len(docs))
	for _, d := range docs {
		if !tierAllowed(tiers, d.Tier) {
			continue
		}
		if !effectiveAllowed(d.EffectiveTo, now) {
			continue
		}
		out = append(out, d)
	}
	return out
}
