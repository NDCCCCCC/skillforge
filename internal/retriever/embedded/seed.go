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

// seed.go 是 embedded 后端的语料装载器。
//
// # 为什么自写一份而不是复用 internal/knowledge.Index
//
// internal/knowledge/indexer.go:168 的 Index 签名耦合 milvus.Config +
// client.Client，不可复用。internal/retriever/mongo/seed.go:38-48 的
// "WHY THIS EXISTS" 注释逐字记录了这件事：本该做但至今没做的 backend-agnostic
// indexing 重构（提交 dadd63a 留了 milvus 耦合签名）。
//
// **backend-agnostic indexing 重构在本 phase 显式推迟**：动它会牵连 milvus
// 生产路径（Index 是 milvus 的唯一装载入口），风险与收益不成比例；本 phase
// 的目标是新增一条并行实现，不是重构既有的。后续另立 plan，不在 49 范围。
//
// 所以本文件照 mongo/seed.go 的先例自写一份解耦 seed：只依赖
// knowledge.WalkRegisteredDirs + knowledge.WalkDocs 这一条共享 walker
// （KBM-06 单一遍历 —— 第三份目录遍历是 49-RESEARCH Pitfall 3），不 import
// 任何 milvus 符号。
//
// # 零兜底（D-01）
//
// Embed 失败 ⇒ 整体中止并向上返回错误。**不跳过该文件、不用零向量占位、
// 不生成 hash 向量**。internal/embedder/bge/bge.go:22-26 把"端点挂了就本地
// 算"定性为用语义无意义的向量掩盖真实故障 —— 一个坏掉的检索后端比一个
// 明确报错的检索后端危险得多。

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// seedAuditHook is the audit callback handed to the shared walker for the
// unregistered-directory decision (KBM-06). Deliberately a callback seam and
// NOT a store parameter: the seed must stay usable from cmd/eval and the spike
// binaries without dragging the sqlite store in (same shape as
// mongo/seed.go:165-180).
//
// Production binds an INSERT-only wrapper over store.PutAudit at boot via
// SetSeedAuditHook; nil (the default) means "do not audit" — the walker's
// WARN still fires.
var seedAuditHook func(action, detail string)

// SetSeedAuditHook binds the production audit sink for the unregistered-
// directory decision on the embedded seed path. The wrapper the caller hands
// in MUST be INSERT-only. Called once at boot, before any seed call, so no
// synchronization is needed afterwards.
func SetSeedAuditHook(fn func(action, detail string)) { seedAuditHook = fn }

// SeedFromKnowledgeDir walks the ACTIVE knowledge/<slug>/ directories under
// knowledgeDir (through the SHARED walker), strips YAML frontmatter, embeds
// each body via emb, and installs the resulting rows.
//
// 返回实际装载的条数。参数显式传入（与 mongo/seed.go:68 同签名形状），不引入
// 任何 milvus 类型。
//
// Behavior parity with mongo.SeedFromKnowledgeDir / knowledge.Index:
//   - active slug dir        → seeded (tier = the slug)
//   - deactivated slug dir   → skipped with a WARN (the walker decides)
//   - unregistered dir       → WARN + INSERT-only audit row via seedAuditHook
//   - 空语料树 / 缺失的语料根 ⇒ 合法降级状态：返回 (0, nil)，不报错
//   - frontmatter 在 embedding 前剥离（同一算法）
//   - ID 1-based、跨 slug 单调递增（与 milvus/mongo 同一约定，保证 citation
//     的 chunk_id 可跨后端移植）
//
// 先全量构建、再一次性提交：装载中途失败时 docs 保持原状，不留半份语料。
func (c *Client) SeedFromKnowledgeDir(ctx context.Context, knowledgeDir string, emb embedder.Embedder, reg kb.Registry) (int, error) {
	// 幂等短路（第一条语句）：Ensure 可能已经从缓存装入了语料，此时不需要
	// 任何网络调用。这一步是"缓存命中 ⇒ 零 embedding"的全部实现。
	c.mu.RLock()
	alreadyLoaded := len(c.docs)
	c.mu.RUnlock()
	if alreadyLoaded > 0 {
		return alreadyLoaded, nil
	}

	if knowledgeDir == "" {
		return 0, errors.New("embedded.SeedFromKnowledgeDir: knowledgeDir is required")
	}
	if emb == nil {
		return 0, errors.New("embedded.SeedFromKnowledgeDir: embedder is required (D-01: no in-process or hash fallback)")
	}
	if reg == nil {
		return 0, errors.New("embedded.SeedFromKnowledgeDir: registry is required (fail-closed; a nil registry would silently seed nothing)")
	}

	// 补齐缓存要素（幂等；Ensure 通常已经算过）。零网络：只做语料 hash。
	if err := c.prepareCachePaths(); err != nil {
		return 0, err
	}

	// 抢缓存锁：并发首次启动时只有一个进程做 218 次 embedding，其余进程等它
	// 写完缓存后直接读。抢不到（超时）时**先尝试一次 readCache**（持锁进程
	// 可能刚好写完），读到就装入；读不到就返回超时错误 —— 不降级为假向量、
	// 不返回部分语料（D-01）。
	var release func() error
	if c.cachePath != "" {
		r, err := acquireCacheLock(cacheLockPath(c.cachePath), c.cfg.CacheWaitTimeout, c.cfg.Logf)
		if err != nil {
			if rows, rerr := readCache(c.cachePath, cacheSpec{
				Model:    c.cfg.Model,
				Dim:      c.cfg.Dimensions,
				Corpus:   c.corpusHash,
				Endpoint: c.endpointHash,
			}); rerr == nil {
				// 与 Ensure 的命中路径同处理：缓存不存正文，必须回填；
				// 回填失败则不作废（这里已经在抢锁失败的分支上），直接报超时。
				if herr := c.hydrateContent(rows); herr == nil {
					c.install(rows)
					return len(rows), nil
				}
			}
			return 0, err
		}
		release = r
		defer func() {
			if release != nil {
				if rerr := release(); rerr != nil {
					c.logf("[WARN] embedded: %v", rerr)
				}
			}
		}()
	}

	slugs, err := knowledge.WalkRegisteredDirs(knowledgeDir, reg, seedAuditHook)
	if err != nil {
		return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: walk %s: %w", knowledgeDir, err)
	}

	docs := make([]retriever.Document, 0, 256)
	nextID := int64(1)

	for _, slug := range slugs {
		tierAbs := filepath.Join(knowledgeDir, slug)
		entries, err := knowledge.WalkDocs(tierAbs)
		if err != nil {
			return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: walk %s: %w", tierAbs, err)
		}
		for _, entry := range entries {
			path := filepath.Join(tierAbs, filepath.FromSlash(entry.RelPath))
			raw, err := os.ReadFile(path)
			if err != nil {
				// 不跳过：一个读不出来的语料文件意味着索引不完整，而
				// "少了几篇"在检索侧表现为莫名其妙的无法回答。
				return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: read %s: %w", path, err)
			}
			body := stripFrontmatter(raw)
			if len(body) == 0 {
				continue
			}

			// 生产 embedding，绝无确定性兜底（D-01 / Pitfall 8）。
			vec, err := emb.Embed(ctx, string(body))
			if err != nil {
				return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: embed %s: %w", path, err)
			}
			if len(vec) != c.cfg.Dimensions {
				return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: %s embedding dim %d != configured dim %d", path, len(vec), c.cfg.Dimensions)
			}

			rel, err := filepath.Rel(knowledgeDir, path)
			if err != nil {
				return 0, fmt.Errorf("embedded.SeedFromKnowledgeDir: rel %s: %w", path, err)
			}
			docs = append(docs, retriever.Document{
				ID:        nextID,
				Tier:      slug,
				Content:   string(body),
				Source:    filepath.ToSlash(rel),
				Embedding: vec,
			})
			nextID++
		}
	}

	if err := validateDocs(docs, reg); err != nil {
		return 0, err
	}

	out := make([]row, 0, len(docs))
	for _, d := range docs {
		out = append(out, row{
			ID:            d.ID,
			Tier:          d.Tier,
			Content:       d.Content,
			Source:        d.Source,
			Embedding:     d.Embedding,
			EffectiveFrom: d.EffectiveFrom,
			EffectiveTo:   d.EffectiveTo,
		})
	}

	// 全部校验通过后才一次性提交（先全量构建、再提交）。
	c.install(out)

	if len(out) == 0 {
		// 空语料树：与 milvus indexer / mongo seed 同姿态 —— 返回 nil，检索侧
		// 对任何查询都给出 no_authorized_source。不写缓存（空缓存没有意义，
		// 且会让"删掉语料后仍能启动"变成一次静默的缓存命中）。
		return 0, nil
	}

	if c.cachePath != "" {
		if err := writeCache(c.cachePath, cacheSpec{
			Model:    c.cfg.Model,
			Dim:      c.cfg.Dimensions,
			Corpus:   c.corpusHash,
			Endpoint: c.endpointHash,
		}, out); err != nil {
			// docs 已经在内存里可用，所以这是一个"可用但没缓存"的状态：
			// 必须暴露（缓存写不成是配置/权限问题），但错误串要说清楚。
			return len(out), fmt.Errorf("embedded.SeedFromKnowledgeDir: write cache %s: %v (vectors are loaded in memory; only the on-disk cache failed)", c.cachePath, err)
		}
	}
	return len(out), nil
}

// install replaces the in-memory corpus in one write-locked step.
func (c *Client) install(rows []row) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docs = rows
	c.byID = make(map[int64]int, len(rows))
	for i, r := range rows {
		c.byID[r.ID] = i
	}
}

// validateDocs enforces the two invariants the corpus must satisfy before it
// becomes searchable:
//
//  1. 每个 Tier 必须属于 reg.ActiveSlugs()（fail-closed）。不在白名单的 slug
//     进了索引，就等于把未授权的目录变成可检索内容 —— 而唯一的授权真源是
//     registry。
//  2. effective 窗口不得倒置、不得为负（KNOWN-02 的种子侧对应检查）。
//
// 空文档集返回 nil（空语料是合法降级状态，不是错误）。
func validateDocs(docs []retriever.Document, reg kb.Registry) error {
	if len(docs) == 0 {
		return nil
	}
	if reg == nil {
		return errors.New("embedded.validateDocs: registry is required (fail-closed)")
	}
	active := make(map[string]bool)
	for _, s := range reg.ActiveSlugs() {
		active[s] = true
	}
	for _, d := range docs {
		if !active[d.Tier] {
			return fmt.Errorf("embedded.validateDocs: doc id %d carries tier %q which is not an active knowledge base (fail-closed; register it before seeding)", d.ID, d.Tier)
		}
		if d.EffectiveTo < 0 {
			return fmt.Errorf("embedded.validateDocs: doc id %d has negative effective_to %d", d.ID, d.EffectiveTo)
		}
		if d.EffectiveFrom > 0 && d.EffectiveTo > 0 && d.EffectiveTo < d.EffectiveFrom {
			return fmt.Errorf("embedded.validateDocs: doc id %d has inverted effective window (from=%d to=%d)", d.ID, d.EffectiveFrom, d.EffectiveTo)
		}
	}
	return nil
}

// stripFrontmatter 逐字照搬 internal/retriever/mongo/seed.go:186-200 的算法
// （真源在 mongo/seed.go 的同名函数；它未导出，跨包调用会迫使改 mongo 的
// 导出面 —— Pitfall 5）。
//
// 复制而非共享是刻意的取舍：算法只有 14 行且已稳定多年，两个后端各自持有
// 一份的漂移风险，远小于把 mongo 的私有 helper 提升为公共 API 带来的接口面
// 扩散。**真源位置已在此注明**，两份实现若需修改必须同步。
func stripFrontmatter(raw []byte) []byte {
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	const open = "---\n"
	if !bytes.HasPrefix(raw, []byte(open)) {
		return raw
	}
	body := raw[len(open):]
	const close = "\n---"
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		return raw
	}
	after := body[end+len(close):]
	return bytes.TrimLeft(after, "\n")
}
