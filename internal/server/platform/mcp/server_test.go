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

// MCP server 的 tier 信任隔离回归测试（D-TRUST-01 / D-NZK-DEF-01）。
//
// 背景：MCP server 的 resolveSkillTiers 曾经把 hasReadAccess 的对象直接
// TrimPrefix 当 tier 名，而真实 KG 的连线形状是
//
//	skf:skill_X skf:hasReadAccess skf:cat_<tier>   (KBCategory 个体)
//	skf:cat_<tier> skf:belongsToTier skf:<tier>     (Tier 个体)
//
// 于是 tier 名变成 "cat_sql-guides"，与 store.QueryScoped 拿来比对
// belongsToTier 对象的 tier 名（"sql-guides"）永远对不上 —— skf_query 在
// 真实 KG 上恒返回空。skf_chunk_for 还额外拼了一次 "cat_" 前缀（双重
// 前缀），恒空；skf_neighbors 则直接用裸 kg.Query，完全绕过分层过滤，
// 与 cmd/server/main.go 公开声明的 "All tool calls route through
// QueryScoped — external agents cannot bypass tier filtering (D-TRUST-01)"
// 直接矛盾。
//
// 这些测试全部离线运行：store.New() + 手工 Insert 三元组，不依赖任何
// 真实 KG 文件或外部服务。

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// fixtureLiveCatKG 构造"真实 KG 形状"的最小图：
//
//	skf:skill_sql a skf:Skill
//	skf:skill_sql skf:hasReadAccess skf:cat_sql-guides
//	skf:cat_sql-guides a skf:KBCategory
//	skf:cat_sql-guides skf:belongsToTier skf:sql-guides
//	skf:sql-guides a skf:Tier
//
// 外加两个分属不同 tier 的 ABox 实体（note_a / note_b），每个都带一条
// referencesTable 边，用来验证 tier 隔离。
func fixtureLiveCatKG() *store.Store {
	s := store.New()

	// Skill 个体 + 读权限（指向 KBCategory）。
	s.Insert(store.Triple{S: ns + "skill_sql", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_sql", P: skfHasReadAccess, O: ns + "cat_sql-guides"})

	// KBCategory 个体 → Tier 个体。
	s.Insert(store.Triple{S: ns + "cat_sql-guides", P: rdfType, O: ns + "KBCategory"})
	s.Insert(store.Triple{S: ns + "cat_sql-guides", P: skfBelongsToTier, O: ns + "sql-guides"})

	// Tier 个体。
	s.Insert(store.Triple{S: ns + "sql-guides", P: rdfType, O: skfTier})

	// ABox：两个不同 tier 的实体，各带一条 referencesTable 边。
	s.Insert(store.Triple{S: ns + "note_a", P: skfBelongsToTier, O: ns + "tier-a"})
	s.Insert(store.Triple{S: ns + "note_a", P: ns + "referencesTable", O: ns + "tbl_a"})
	s.Insert(store.Triple{S: ns + "note_b", P: skfBelongsToTier, O: ns + "tier-b"})
	s.Insert(store.Triple{S: ns + "note_b", P: ns + "referencesTable", O: ns + "tbl_b"})
	// 反向边：不可见 tier 的 note_b 指向可见的 note_a。这条边只有在
	// 裸 Query 下才会漏出来 —— QueryScoped 按 subject 的可见性过滤，
	// note_b 不可见 → 它的边一条都不返回。
	s.Insert(store.Triple{S: ns + "note_b", P: ns + "referencesTable", O: ns + "note_a"})

	// 让 skill_sql 只能读 tier-a（cat_tier-a → tier-a），note_a 属于 tier-a。
	s.Insert(store.Triple{S: ns + "cat_tier-a", P: rdfType, O: ns + "KBCategory"})
	s.Insert(store.Triple{S: ns + "cat_tier-a", P: skfBelongsToTier, O: ns + "tier-a"})
	s.Insert(store.Triple{S: ns + "skill_sql", P: skfHasReadAccess, O: ns + "cat_tier-a"})

	return s
}

// newTestServer 用给定 KG 造一个 MCP server（service skill = "sql"）。
func newTestServer(kg *store.Store) *Server {
	return NewServer(kg, "sql")
}

// ---------------------------------------------------------------------------
// resolveSkillTiers —— 三态归一（与兄弟 resolver 一致）
// ---------------------------------------------------------------------------

// TestResolveSkillTiers_LiveCategoryWiring 是核心回归：
// hasReadAccess 指向 KBCategory 时，必须沿 belongsToTier 展开成 tier 名
// "sql-guides"，而不是把本地名 "cat_sql-guides" 当 tier 名。
func TestResolveSkillTiers_LiveCategoryWiring(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	tiers := srv.resolveSkillTiers("sql")
	require.Contains(t, tiers, "sql-guides",
		"KBCategory 形状的 hasReadAccess 对象必须展开成 belongsToTier 的 tier 名")
	assert.NotContains(t, tiers, "cat_sql-guides",
		"本地名 cat_<tier> 绝不能作为 tier 名返回")
}

// TestResolveSkillTiers_DirectTierObjectBackwardCompat 固定 case 1：
// 直连 Tier 个体的旧写法仍然解析成 tier 名。
func TestResolveSkillTiers_DirectTierObjectBackwardCompat(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: ns + "skill_direct", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_direct", P: skfHasReadAccess, O: ns + "sql-guides"})
	s.Insert(store.Triple{S: ns + "sql-guides", P: rdfType, O: skfTier})

	tiers := newTestServer(s).resolveSkillTiers("direct")
	assert.Equal(t, []string{"sql-guides"}, tiers)
}

// TestResolveSkillTiers_UnknownSkillNil 固定 nil 语义：未知 skill 解析为
// nil（调用方据此 fail-closed）。
func TestResolveSkillTiers_UnknownSkillNil(t *testing.T) {
	assert.Nil(t, newTestServer(fixtureLiveCatKG()).resolveSkillTiers("nope"))
}

// TestResolveSkillTiers_UnknownAccessObjectFailsClosed 固定 case 3：
// 既不是 Tier 个体、又没有 belongsToTier 的访问对象不贡献任何 tier；
// 全部未知时解析为 nil（而不是 ["cat_unknown"]）。
func TestResolveSkillTiers_UnknownAccessObjectFailsClosed(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: ns + "skill_orphan", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_orphan", P: skfHasReadAccess, O: ns + "cat_unknown"})

	tiers := newTestServer(s).resolveSkillTiers("orphan")
	assert.Nil(t, tiers,
		"全部未知的访问对象必须解析为 nil（fail-closed），绝不返回 [\"cat_unknown\"]")
}

// TestResolveSkillTiers_DedupesPreservesOrder 固定去重语义：多条路径到达
// 同一个 tier 时只出现一次，且保持首次出现顺序。
func TestResolveSkillTiers_DedupesPreservesOrder(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: ns + "tierA", P: rdfType, O: skfTier})
	s.Insert(store.Triple{S: ns + "tierB", P: rdfType, O: skfTier})
	s.Insert(store.Triple{S: ns + "skill_multi", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_multi", P: skfHasReadAccess, O: ns + "tierA"})
	s.Insert(store.Triple{S: ns + "skill_multi", P: skfHasReadAccess, O: ns + "cat_x"})
	s.Insert(store.Triple{S: ns + "skill_multi", P: skfHasReadAccess, O: ns + "cat_y"})
	s.Insert(store.Triple{S: ns + "cat_x", P: skfBelongsToTier, O: ns + "tierB"})
	s.Insert(store.Triple{S: ns + "cat_y", P: skfBelongsToTier, O: ns + "tierB"})

	tiers := newTestServer(s).resolveSkillTiers("multi")
	assert.Equal(t, []string{"tierA", "tierB"}, tiers,
		"重复可达的 tier 只出现一次，且保持首次出现顺序")
}

// ---------------------------------------------------------------------------
// skf_query —— 归一修复后 ABox 行必须可见
// ---------------------------------------------------------------------------

// TestSKFQuery_VisibleABoxAfterNormalization 是端到端回归：
// 归一之后 resolver 输出的 tier 名能真正驱动 QueryScoped，ABox 行可见。
func TestSKFQuery_VisibleABoxAfterNormalization(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	res, err := srv.CallTool(context.Background(), "skf_query", map[string]any{
		"skill": "sql",
	})
	require.NoError(t, err)

	results, _ := res["results"].([]map[string]any)
	var sawNoteA bool
	for _, r := range results {
		if r["s"] == ns+"note_a" {
			sawNoteA = true
		}
	}
	assert.True(t, sawNoteA,
		"resolver 归一后 QueryScoped 必须能看到可见 tier 的 ABox 行（此前恒为空）")
}

// TestSKFQuery_UnknownSkillFailsClosed 固定 fail-closed：未知 skill 不
// 返回任何行（绝不退化成未过滤的全量）。
func TestSKFQuery_UnknownSkillFailsClosed(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	res, err := srv.CallTool(context.Background(), "skf_query", map[string]any{
		"skill": "does-not-exist",
	})
	require.NoError(t, err)

	results, _ := res["results"].([]map[string]any)
	assert.Empty(t, results, "未知 skill 必须返回空结果（fail-closed）")
}

// ---------------------------------------------------------------------------
// skf_neighbors —— tier 隔离
// ---------------------------------------------------------------------------

// neighborIDs 从 skf_neighbors 的返回里取出节点 id 集合。
func neighborIDs(res map[string]any) []string {
	nodes, _ := res["nodes"].([]map[string]any)
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		id, _ := n["id"].(string)
		out = append(out, id)
	}
	return out
}

// neighborEdges 从 skf_neighbors 的返回里取出边。
func neighborEdges(res map[string]any) []map[string]string {
	edges, _ := res["edges"].([]map[string]string)
	return edges
}

// TestSKFNeighbors_TierIsolation 是核心隔离回归：A（tier-a）的邻居查询
// 不能返回 B（tier-b）的三元组。
func TestSKFNeighbors_TierIsolation(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	res, err := srv.CallTool(context.Background(), "skf_neighbors", map[string]any{
		"uri":   ns + "note_a",
		"hops":  1,
		"skill": "sql",
	})
	require.NoError(t, err)

	ids := neighborIDs(res)
	assert.Contains(t, ids, ns+"tbl_a", "可见 tier 的邻居必须返回")
	assert.NotContains(t, ids, ns+"note_b", "其它 tier 的实体绝不能出现在邻居里")
	assert.NotContains(t, ids, ns+"tbl_b", "其它 tier 的实体绝不能出现在邻居里")

	for _, e := range neighborEdges(res) {
		assert.NotEqual(t, ns+"note_b", e["source"], "其它 tier 的三元组绝不能出现")
		assert.NotEqual(t, ns+"tbl_b", e["source"], "其它 tier 的三元组绝不能出现")
		assert.NotEqual(t, ns+"tbl_b", e["target"], "其它 tier 的三元组绝不能出现")
	}
}

// TestSKFNeighbors_InvisibleEntityReturnsNoEdges 是"裸 kg.Query 会失败"
//的对照：查一个**不可见 tier** 的实体时，隔离过滤必须把它的边全部
//挡住。裸 Query 会把 note_b 的边原样返回（等于把 tier-b 的内容交给
//只被授权 tier-a 的外部 agent），QueryScoped 则一条都不返回。
func TestSKFNeighbors_InvisibleEntityReturnsNoEdges(t *testing.T) {
	kg := fixtureLiveCatKG()

	// 对照：裸 Query 确实能看到 tier-b 的边。
	raw := kg.Query(ns+"note_b", "", "")
	require.NotEmpty(t, raw, "对照：裸 Query 能看到不可见 tier 的边")

	srv := newTestServer(kg)
	res, err := srv.CallTool(context.Background(), "skf_neighbors", map[string]any{
		"uri":   ns + "note_b",
		"hops":  1,
		"skill": "sql",
	})
	require.NoError(t, err)

	assert.Empty(t, neighborEdges(res),
		"不可见 tier 的实体不得返回任何边（否则 tier 过滤被绕过）")
	ids := neighborIDs(res)
	assert.NotContains(t, ids, ns+"tbl_b", "不可见 tier 的邻居不得出现")
	assert.NotContains(t, ids, ns+"note_a",
		"不可见实体的反向边也不得泄露可见 tier 的实体")
}

// TestSKFNeighbors_NoUnfilteredQueryGuard 是"旧实现会失败"的对照：
// 用裸 kg.Query 一定能拿到 note_b 的三元组 —— 证明隔离不是数据本身
// 不可见，而是必须靠 QueryScoped 才能挡住。
func TestSKFNeighbors_NoUnfilteredQueryGuard(t *testing.T) {
	kg := fixtureLiveCatKG()
	raw := kg.Query(ns+"note_b", "", "")
	assert.NotEmpty(t, raw,
		"对照：裸 Query 确实能看到别的 tier 的数据 —— 所以隔离必须由 QueryScoped 保证")
}

// TestSKFNeighbors_UnknownSkillFailsClosed 固定 fail-closed：解析不出任何
// tier 时返回空邻居，而不是全量。
func TestSKFNeighbors_UnknownSkillFailsClosed(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	res, err := srv.CallTool(context.Background(), "skf_neighbors", map[string]any{
		"uri":   ns + "note_a",
		"hops":  1,
		"skill": "does-not-exist",
	})
	require.NoError(t, err)

	assert.Empty(t, neighborIDs(res), "未知 skill 必须返回空邻居（fail-closed）")
	assert.Empty(t, neighborEdges(res), "未知 skill 必须返回空边（fail-closed）")
}

// TestSKFNeighbors_UnresolvableAccessObjectFailsClosed 固定"空
// allowedTiers"场景：skill 存在，但它的访问对象无法解析出任何 tier。
// 此时必须 fail-closed 返回空 —— store.QueryScoped 把空 allowedTiers 当
// 作"不过滤"的 admin 基线，所以这里绝不能把空切片传下去。
func TestSKFNeighbors_UnresolvableAccessObjectFailsClosed(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: ns + "skill_orphan", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_orphan", P: skfHasReadAccess, O: ns + "cat_unknown"})
	s.Insert(store.Triple{S: ns + "note_x", P: skfBelongsToTier, O: ns + "tier-x"})
	s.Insert(store.Triple{S: ns + "note_x", P: ns + "referencesTable", O: ns + "tbl_x"})

	srv := newTestServer(s)

	res, err := srv.CallTool(context.Background(), "skf_neighbors", map[string]any{
		"uri":   ns + "note_x",
		"hops":  1,
		"skill": "orphan",
	})
	require.NoError(t, err)

	assert.Empty(t, neighborIDs(res), "解析不出 tier 时必须返回空邻居（fail-closed）")
	assert.Empty(t, neighborEdges(res), "解析不出 tier 时必须返回空边（fail-closed）")
}

// ---------------------------------------------------------------------------
// skf_chunk_for —— 双重 "cat_" 前缀修复
// ---------------------------------------------------------------------------

// TestSKFChunkFor_NoDoubleCatPrefix 固定 tier IRI 的形状：
// belongsToTier 的对象是 Tier 个体（ns+"sql-guides"），不是 KBCategory
// 个体（ns+"cat_sql-guides"）。拼错前缀会让 chunk_for 恒空。
func TestSKFChunkFor_NoDoubleCatPrefix(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: ns + "skill_sql", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: ns + "skill_sql", P: skfHasReadAccess, O: ns + "cat_sql-guides"})
	s.Insert(store.Triple{S: ns + "cat_sql-guides", P: rdfType, O: ns + "KBCategory"})
	s.Insert(store.Triple{S: ns + "cat_sql-guides", P: skfBelongsToTier, O: ns + "sql-guides"})
	s.Insert(store.Triple{S: ns + "sql-guides", P: rdfType, O: skfTier})

	prov := &store.Provenance{
		SourceKind: "file",
		SourcePath: "sql-guides/note.md",
		LineStart:  3,
		LineEnd:    9,
	}
	s.Insert(store.Triple{
		S: ns + "note_001", P: skfBelongsToTier, O: ns + "sql-guides", Provenance: prov,
	})
	s.Insert(store.Triple{S: ns + "tbl_etl", P: skfBelongsToTier, O: ns + "sql-guides"})
	s.Insert(store.Triple{S: ns + "note_001", P: ns + "referencesTable", O: ns + "tbl_etl"})
	s.Insert(store.Triple{S: ns + "entity_x", P: skfBelongsToTier, O: ns + "sql-guides"})
	s.Insert(store.Triple{S: ns + "entity_x", P: ns + "referencesTable", O: ns + "tbl_etl"})

	srv := newTestServer(s)
	res, err := srv.CallTool(context.Background(), "skf_chunk_for", map[string]any{
		"entity_iri": ns + "entity_x",
		"skill":      "sql",
	})
	require.NoError(t, err)

	chunks, _ := res["chunks"].([]map[string]any)
	require.NotEmpty(t, chunks, "可见 tier 的 chunk 必须返回（此前双重 cat_ 前缀导致恒空）")

	var found bool
	for _, c := range chunks {
		if c["source_path"] == "sql-guides/note.md" {
			found = true
		}
		assert.NotContains(t, c["text_snippet"], "cat_sql-guides",
			"chunk 列表不得混入 schema 个体（QueryScoped 的 schema 直通分支会绕过模式过滤）")
	}
	assert.True(t, found,
		"可见 tier 的 note chunk 必须带 provenance 出现（此前双重 cat_ 前缀导致恒空）")
}

// ---------------------------------------------------------------------------
// skf_skills —— reads_tiers 与过滤用的 tier 名一致
// ---------------------------------------------------------------------------

// TestSKFSkills_ReadsTiersNormalized 固定 handleSkills 暴露的
// reads_tiers 必须与 resolveSkillTiers 的输出同形（否则外部 agent 拿到
// 的 tier 名无法用来判断可见性）。
func TestSKFSkills_ReadsTiersNormalized(t *testing.T) {
	srv := newTestServer(fixtureLiveCatKG())

	res, err := srv.CallTool(context.Background(), "skf_skills", map[string]any{})
	require.NoError(t, err)

	skills, _ := res["skills"].([]map[string]any)
	require.NotEmpty(t, skills)

	var found bool
	for _, sk := range skills {
		name, _ := sk["name"].(string)
		if name != "sql" {
			continue
		}
		found = true
		reads, _ := sk["reads_tiers"].([]string)
		assert.Contains(t, reads, "sql-guides")
		assert.NotContains(t, reads, "cat_sql-guides",
			"reads_tiers 必须与过滤用的 tier 名同形")
	}
	assert.True(t, found, "fixture 里的 skill 必须出现在列表中")
}
