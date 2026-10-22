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

// Package embedded is the in-process vector store backend of
// retriever.Retriever (RETRIEVER_BACKEND=embedded, Phase 49 / D-03 / D-04 /
// D-06).
//
// # 什么'embedded'，什么不是（D-03）
//
// embedded 专指「向量存储进程内」，**不指**「向量化进程内」。向量仍然由
// 外部 BGE-M3 端点经 internal/embedder/bge 计算：本包不认识任何 embedding
// 模型，只接收已经算好的 []float32。所以这个后端不是离线 embedder，也不
// 提供任何替代向量化的能力。
//
// 零兜底（D-01）：端点不可达就是失败。本包不生成随机向量、不生成 hash
// 向量、不读预计算向量文件。internal/embedder/bge/bge.go:17-26 把
// "端点挂了就本地算" 定性为用语义无意义的向量掩盖真实故障 —— 本包不重复
// 那个错误，SeedFromKnowledgeDir 与 Insert 的 Embed 错误一律向上返回。
//
// # 与 milvus / mongo / none 的并列关系
//
//   - internal/retriever/milvus — 生产实现（Milvus gRPC）。
//   - internal/retriever/mongo  — 存储侧过滤 + Go 侧 cosine。
//   - internal/retriever/embedded（本包）— 全进程内：row 切片 + cosine 排序。
//   - RETRIEVER_BACKEND=none 的 degraded default-deny 语义**不在本包**：
//     那是 cmd/server 的 noopRetriever（每次查询 0 命中、Health 恒报错），
//     与本包的"等价可用模式"是两件事。本包是真实实现，不是降级模式。
//
// # 信任边界
//
// Search 的两段式过滤在 tier_isolation.go（candidates）与 rank.go（rank）
// 里物理分离：候选集构造阶段按 tiers + effective_to 过滤，rank 只对候选集
// 打分。空 tiers ⇒ 候选集为空 ⇒ 0 命中且 err 为 nil。
//
// ListMetadata 是图谱浏览面，**不过滤 tier、不做 effective_to 过滤**（把
// 被取代的 chunk 也暴露给"已失效"徽章），与 mongo.go:444-458 同语义。
package embedded

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// ErrDisconnected is returned by every method that touches state after
// Disconnect. It is a sentinel (not a formatted error) so callers can match
// it with errors.Is.
var ErrDisconnected = errors.New("embedded: client disconnected")

// defaultCacheWaitTimeout is the zero-value replacement for
// Config.CacheWaitTimeout. 5 minutes covers a cold first boot on a slow
// BGE-M3 endpoint (218 docs) without letting a stuck lock wedge startup
// forever.
const defaultCacheWaitTimeout = 5 * time.Minute

// Config is the construction surface of the embedded backend.
//
// 九个字段一次给全：构造函数做完全部校验，之后运行时只读 cfg（Client 的
// 方法与 seed 都从 c.cfg 取值），所以新增字段只需改这里与 New。
type Config struct {
	// Embedder 用于 seed 与 Insert 路径的向量化。可以 nil —— nil 时只允许
	// "纯缓存读"路径（Ensure 从磁盘缓存装入，SeedFromKnowledgeDir 会明确
	// 报错而不是悄悄跳过）。
	Embedder embedder.Embedder

	// Model 是缓存 key 的要素之一（BGE_M3_MODEL，默认 bge-m3）。换模型必须
	// 让缓存作废，否则不同模型的向量会被混用。
	Model string

	// Dimensions 必须与 Embedder.Dimensions() 一致。不一致时 New 直接报错：
	// 维度不符会让 vecmath.Cosine 在查询时 panic（不是返回错误），启动期拒绝
	// 比请求期崩溃好（49-RESEARCH Pitfall 7）。
	Dimensions int

	// CacheDir 是本地向量缓存的目录（默认 .skillforge-cache/vectors）。
	// 空字符串 = 禁用缓存（禁用不等于假向量：禁用时每次启动重新 embedding，
	// 失败照样失败）。
	CacheDir string

	// EndpointURL 只用于算 sha256（缓存 key 的要素）。原始串**绝不**落盘：
	// URL 可能带 token，写进文件名会进 git status / 日志（49-RESEARCH
	// Pitfall 9）。本包任何持久化字节里都不出现它。
	EndpointURL string

	// Logf 是日志注入点（nil → 标准 log）。缓存锁等待等可观测事件走它。
	Logf func(format string, args ...any)

	// KnowledgeDir 是语料根（默认 knowledge/）。Ensure 用它算语料 hash，
	// SeedFromKnowledgeDir 用它枚举语料。
	KnowledgeDir string

	// Registry 是 tier 白名单的唯一真源（kb.Registry 的 ActiveSlugs /
	// Lookup）。seed 用它做 fail-closed 校验，Ensure 用它走共享 walker。
	Registry kb.Registry

	// CacheWaitTimeout 是抢缓存锁的最长等待。零值 → 5 分钟。
	CacheWaitTimeout time.Duration
}

// Client is the in-process vector store. It satisfies retriever.Retriever
// (the 8-method read/write surface) and retriever.IndexerBackend (Insert).
//
// 并发模型：mu 保护 docs / byID / closed / 缓存字段。Search 走读锁（打分在
// 锁内完成，因为 row 的 Embedding 切片是共享的，拷贝整份候选集更贵）；
// 装载 / Insert / Disconnect 走写锁。没有后台 goroutine，没有连接生命周期。
type Client struct {
	cfg Config

	mu     sync.RWMutex
	docs   []row
	byID   map[int64]int // ID → docs 下标
	closed bool

	// 缓存要素（Ensure 填充，SeedFromKnowledgeDir 消费）。
	corpusHash   string
	endpointHash string
	cachePath    string
	cacheKey     string
}

// compile-time proofs: the two interfaces cmd/server wires.
var (
	_ retriever.Retriever      = (*Client)(nil)
	_ retriever.IndexerBackend = (*Client)(nil)
)

// New validates the configuration and returns a ready (but empty) client.
// It performs NO network I/O and NO corpus loading — Ensure / seed do that,
// in that order, at boot.
func New(cfg Config) (*Client, error) {
	if cfg.Dimensions <= 0 {
		return nil, fmt.Errorf("embedded.New: Dimensions must be > 0 (got %d)", cfg.Dimensions)
	}
	// 启动期维度自检（Pitfall 7）：维度不符会让 vecmath.Cosine 在查询期
	// panic("vecmath.Cosine: length mismatch")，那是一个没有任何上下文的
	// 崩溃。在这里拒绝，错误串直接说明原因。
	if cfg.Embedder != nil && cfg.Embedder.Dimensions() != cfg.Dimensions {
		return nil, fmt.Errorf("embedded.New: embedder dimensions %d != configured dimensions %d (a mismatch would make similarity computation panic at query time, so it is refused at startup)",
			cfg.Embedder.Dimensions(), cfg.Dimensions)
	}
	if cfg.Logf == nil {
		cfg.Logf = log.Printf
	}
	if cfg.CacheWaitTimeout <= 0 {
		cfg.CacheWaitTimeout = defaultCacheWaitTimeout
	}
	if cfg.CacheDir != "" {
		// 目录建不出来就是配置/权限问题 —— 报错，而不是静默禁用缓存
		// （静默禁用会让"为什么每次启动都重新 embedding"变成一个谜）。
		if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
			return nil, fmt.Errorf("embedded.New: create cache dir %s: %w", cfg.CacheDir, err)
		}
	}
	return &Client{cfg: cfg, byID: make(map[int64]int)}, nil
}

// Health reports whether the in-process store is in a consistent state.
//
// 真检查，不恒 true：closed / 维度非法 / docs 与 byID 失配都是错误。
// 刻意**不做网络 I/O**：BGE-M3 的可用性由 seed 的 Embed 错误暴露（Pitfall 8
// 的既有立场：BGE 故障不在启动期致命），让 Health 每次调用都打网络会把
// 启动健康检查变成外部依赖。
func (c *Client) Health(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrDisconnected
	}
	if c.cfg.Dimensions <= 0 {
		return fmt.Errorf("embedded.Health: configured dimensions %d is not positive", c.cfg.Dimensions)
	}
	if len(c.docs) != len(c.byID) {
		return fmt.Errorf("embedded.Health: internal index inconsistency (docs=%d byID=%d)", len(c.docs), len(c.byID))
	}
	return nil
}

// Search is the two-stage read path: build the candidate set (tiers +
// effective_to, PRE-similarity), then rank ONLY that candidate set.
//
// 函数体里不出现任何 tier 判定 —— 那是 candidates 的职责。空候选集直接
// 返回 (nil, nil)（0 命中 + 无 error，D-09），rank 根本不被调用。
func (c *Client) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	if topK <= 0 {
		return nil, errors.New("embedded.Search: topK must be > 0")
	}
	if len(queryVec) == 0 {
		return nil, errors.New("embedded.Search: queryVec is empty")
	}
	// 查询期维度守卫（Pitfall 7 / T-49-06）：维度不符的向量不得进入相似度
	// 函数，否则 vecmath.Cosine 会 panic。这里返回错误，让调用方看到 500
	// 而不是进程崩溃。
	if len(queryVec) != c.cfg.Dimensions {
		return nil, fmt.Errorf("embedded.Search: query vector dim %d != configured dim %d", len(queryVec), c.cfg.Dimensions)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, ErrDisconnected
	}
	cands := candidates(c.docs, tiers, now)
	if len(cands) == 0 {
		return nil, nil
	}
	return rank(cands, queryVec, topK), nil
}

// Ensure prepares the store for Search: it computes the corpus hash, derives
// the cache key and tries to load the on-disk vector cache into memory.
// Idempotent — safe on every startup.
//
// 关键约束：**Ensure 先于 SeedFromKnowledgeDir 被调用**，所以它不能依赖
// "seed 已装载"才能算出缓存 key。语料 hash 只依赖语料**字节**、不依赖向量，
// 因此这里可以自己算：Ensure 全程零网络（不调 Embed）。
//
// 三条分支：
//   - CacheDir == "" → 只做内存侧准备，返回 nil（缓存可禁用，但禁用不等于
//     假向量：禁用时每次启动重新 embedding，失败照样失败）。
//   - 缓存命中 → 装入 docs / byID，返回 nil（随后的 seed 会短路返回）。
//   - 未命中（不存在或损坏）→ 返回 nil，docs 保持为空（由 seed 装载并写缓存）。
//     损坏按未命中处理是刻意的：一份 header 对不上的缓存毫无价值，重建它
//     才是正确动作；日志里留一条可见记录。
//   - 其他错误（权限、IO）→ 返回 wrapped 错误（不静默当作未命中：读不了
//     缓存是配置问题，静默重建会让"为什么每次启动都重新 embedding"变成谜）。
//
// Ensure **不抢锁、不轮询、不写缓存** —— 锁与写缓存全部归
// SeedFromKnowledgeDir。这样 Ensure 无阻塞、重复调用幂等，且不会与 seed 的
// 锁路径互相等待。
func (c *Client) Ensure(ctx context.Context) error {
	c.mu.RLock()
	closed := c.closed
	loaded := len(c.docs)
	c.mu.RUnlock()
	if closed {
		return ErrDisconnected
	}
	if loaded > 0 {
		// 已经装入（Ensure 被重复调用，或 seed 已跑过）—— 幂等返回。
		return nil
	}

	if c.cfg.CacheDir != "" {
		if err := os.MkdirAll(c.cfg.CacheDir, 0o700); err != nil {
			return fmt.Errorf("embedded.Ensure: create cache dir %s: %w", c.cfg.CacheDir, err)
		}
	}

	c.mu.Lock()
	c.byID = make(map[int64]int, len(c.docs))
	for i, d := range c.docs {
		c.byID[d.ID] = i
	}
	c.mu.Unlock()

	if c.cfg.CacheDir == "" {
		return nil
	}

	// 语料 hash（零网络）→ cacheKey → readCache。
	if err := c.prepareCachePaths(); err != nil {
		return err
	}
	if c.cachePath == "" {
		return nil
	}

	rows, err := readCache(c.cachePath, cacheSpec{
		Model:    c.cfg.Model,
		Dim:      c.cfg.Dimensions,
		Corpus:   c.corpusHash,
		Endpoint: c.endpointHash,
	})
	switch {
	case err == nil:
		// 缓存不存正文（见 cache.go 的格式注释），命中后按 Source 零网络回填。
		// 回填失败 ⇒ 整份缓存作废（正文与向量必须来自同一代语料；混代的
		// 结果会让 citation 指向一段和向量不匹配的文字）。
		if herr := c.hydrateContent(rows); herr != nil {
			c.logf("[WARN] embedded: cached vectors in %s cannot be re-hydrated (%v); rebuilding from the corpus", c.cachePath, herr)
			return nil
		}
		c.install(rows)
		c.logf("[INFO] embedded: loaded %d cached vector(s) from %s (key=%s)", len(rows), c.cachePath, shortKey(c.cacheKey))
		return nil
	case errors.Is(err, errCacheInvalid), os.IsNotExist(err):
		if errors.Is(err, errCacheInvalid) {
			c.logf("[WARN] embedded: vector cache %s is stale/corrupt (%v); rebuilding from the corpus", c.cachePath, err)
		}
		return nil
	default:
		return fmt.Errorf("embedded.Ensure: read cache %s: %w", c.cachePath, err)
	}
}

// hydrateContent re-reads each cached row's body from the corpus and fills in
// Content. 零网络：只读本地文件。
//
// 语料 hash 已经证明"文件内容与写缓存时一致"，所以这里读到的正文必然与
// 缓存里的向量同代。任一文件读不出 ⇒ 返回错误，让调用方把整份缓存作废
// （不跳过、不填空串：空正文的 hit 在 UI 上表现为"命中但没有内容"，比报错
// 更难排查）。
func (c *Client) hydrateContent(rows []row) error {
	if c.cfg.KnowledgeDir == "" {
		return errors.New("embedded.Ensure: Config.KnowledgeDir is required to re-hydrate cached content")
	}
	for i := range rows {
		if rows[i].Content != "" {
			continue
		}
		// Source 是相对 knowledgeDir 的 slash 路径（seed 用 ToSlash 写入）。
		path := filepath.Join(c.cfg.KnowledgeDir, filepath.FromSlash(rows[i].Source))
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rows[i].Content = string(stripFrontmatter(raw))
	}
	return nil
}

// logf is the nil-safe logging seam (Config.Logf is optional).
func (c *Client) logf(format string, args ...any) {
	if c.cfg.Logf != nil {
		c.cfg.Logf(format, args...)
	}
}

// prepareCachePaths fills cachePath / corpusHash / endpointHash / cacheKey.
// 零网络；CacheDir 为空时 cachePath 保持空（缓存禁用）。
func (c *Client) prepareCachePaths() error {
	if c.cfg.CacheDir == "" {
		return nil
	}
	// 调用方可能没走 New（测试直接构造 Config）——这里补齐默认值，
	// 让 Ensure / seed 在两条构造路径上行为一致。
	if c.cfg.Logf == nil {
		c.cfg.Logf = log.Printf
	}
	if c.cfg.CacheWaitTimeout <= 0 {
		c.cfg.CacheWaitTimeout = defaultCacheWaitTimeout
	}
	if c.cachePath == "" {
		c.cachePath = cacheFileNameFor(c.cfg.CacheDir)
	}
	if c.corpusHash == "" {
		h, err := c.computeCorpusHash()
		if err != nil {
			return err
		}
		c.corpusHash = h
	}
	if c.endpointHash == "" {
		c.endpointHash = sha256Hex(c.cfg.EndpointURL)
	}
	c.cacheKey = cacheKey(c.cfg.Model, c.cfg.Dimensions, c.corpusHash, c.endpointHash)
	return nil
}

// computeCorpusHash walks the corpus through the SHARED walker and hashes
// (relative path + bytes). 零网络：不调 Embed，因此可以在 seed 之前完成。
func (c *Client) computeCorpusHash() (string, error) {
	if c.cfg.KnowledgeDir == "" {
		return "", errors.New("embedded.Ensure: Config.KnowledgeDir is required when CacheDir is set (the cache key covers the corpus content)")
	}
	if c.cfg.Registry == nil {
		return "", errors.New("embedded.Ensure: Config.Registry is required when CacheDir is set (fail-closed: a nil registry would hash an empty corpus and accept any cache)")
	}
	slugs, err := knowledge.WalkRegisteredDirs(c.cfg.KnowledgeDir, c.cfg.Registry, seedAuditHook)
	if err != nil {
		return "", fmt.Errorf("embedded.Ensure: walk %s: %w", c.cfg.KnowledgeDir, err)
	}
	var entries []knowledge.DocEntry
	// rel → abs 的映射，让 corpusHash 的 read 回调不用再拼一遍路径。
	abs := make(map[string]string)
	for _, slug := range slugs {
		tierAbs := filepath.Join(c.cfg.KnowledgeDir, slug)
		es, err := knowledge.WalkDocs(tierAbs)
		if err != nil {
			return "", fmt.Errorf("embedded.Ensure: walk %s: %w", tierAbs, err)
		}
		for _, e := range es {
			rel := filepath.ToSlash(filepath.Join(slug, e.RelPath))
			entries = append(entries, knowledge.DocEntry{RelPath: rel, Bytes: e.Bytes})
			abs[rel] = filepath.Join(tierAbs, filepath.FromSlash(e.RelPath))
		}
	}
	// WalkRegisteredDirs 已按 slug 排序、WalkDocs 已按相对路径排序，所以
	// entries 的顺序是确定的；再显式排一次，让"按路径排序"不依赖上层实现。
	sort.Slice(entries, func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath })
	return corpusHash(entries, func(rel string) ([]byte, error) {
		p, ok := abs[rel]
		if !ok {
			return nil, fmt.Errorf("embedded.Ensure: no path recorded for %s", rel)
		}
		return os.ReadFile(p)
	})
}

// shortKey trims a hex cache key for log lines (full hashes are noise).
func shortKey(k string) string {
	if len(k) <= 12 {
		return k
	}
	return k[:12]
}

// Insert appends documents to the store. Caller pre-assigns numeric IDs;
// this implementation does NOT auto-dedup (retriever.go:184-188 contract) —
// a duplicate id is an error, not a silent overwrite.
func (c *Client) Insert(ctx context.Context, docs []retriever.Document) error {
	if len(docs) == 0 {
		return errors.New("embedded.Insert: no docs to insert")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrDisconnected
	}
	// 先全量校验、再一次性 append：校验中途失败不留半份数据。
	for _, d := range docs {
		if len(d.Embedding) != c.cfg.Dimensions {
			// 不静默跳过：损坏语料必须在启动期暴露，而不是让某些 chunk
			// 永远检索不到却没人知道。
			return fmt.Errorf("embedded.Insert: doc id %d embedding dim %d != configured dim %d", d.ID, len(d.Embedding), c.cfg.Dimensions)
		}
		if _, ok := c.byID[d.ID]; ok {
			return fmt.Errorf("embedded.Insert: duplicate id %d (callers pre-assign IDs; this backend does not auto-dedup)", d.ID)
		}
	}
	for _, d := range docs {
		c.byID[d.ID] = len(c.docs)
		c.docs = append(c.docs, row{
			ID:            d.ID,
			Tier:          d.Tier,
			Content:       d.Content,
			Source:        d.Source,
			Embedding:     d.Embedding,
			EffectiveFrom: d.EffectiveFrom,
			EffectiveTo:   d.EffectiveTo,
		})
	}
	return nil
}

// ListMetadata returns the metadata projection of every document, sorted by
// ID ascending.
//
// **不过滤 tier、不做 effective_to 过滤**（retriever.go:190-195 与
// mongo.go:444-458 的图谱契约）：知识图谱是浏览面，故意暴露被取代的 chunk
// （前端渲染"已失效"徽章）。在这里加过滤会让图谱静默丢节点。
func (c *Client) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, ErrDisconnected
	}
	out := make([]retriever.DocumentMeta, 0, len(c.docs))
	for _, d := range c.docs {
		out = append(out, retriever.DocumentMeta{
			ID:            d.ID,
			Tier:          d.Tier,
			Source:        d.Source,
			EffectiveFrom: d.EffectiveFrom,
			EffectiveTo:   d.EffectiveTo,
		})
	}
	// 确定性输出：docs 是插入序，插入序可能来自并发 seed 之外的路径，
	// 按 ID 升序让图谱端点稳定（与 mongo 的隐式顺序不同，这里显式保证）。
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// GetChunk looks up one chunk by numeric primary key.
//
// 未命中必须返回 retriever.ErrChunkNotFound **同一个变量**（不是新建错误）。
// internal/server 用 errors.Is 把它映射成 404；新建一个同文案的错误会让
// 404 变成 500，前端从"片段不存在"变成"服务器错误"。
func (c *Client) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return retriever.ChunkContent{}, ErrDisconnected
	}
	idx, ok := c.byID[id]
	if !ok || idx < 0 || idx >= len(c.docs) {
		return retriever.ChunkContent{}, retriever.ErrChunkNotFound
	}
	d := c.docs[idx]
	return retriever.ChunkContent{
		ID:            d.ID,
		Tier:          d.Tier,
		Source:        d.Source,
		Content:       d.Content,
		EffectiveFrom: d.EffectiveFrom,
		EffectiveTo:   d.EffectiveTo,
	}, nil
}

// GetMaxID returns the highest chunk ID stored for the given tier (slug).
// Empty tier or an unknown tier → 0, nil (retriever.go:209-212 contract).
func (c *Client) GetMaxID(ctx context.Context, tier string) (int64, error) {
	if tier == "" {
		return 0, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return 0, ErrDisconnected
	}
	var max int64
	for _, d := range c.docs {
		if d.Tier == tier && d.ID > max {
			max = d.ID
		}
	}
	return max, nil
}

// Disconnect marks the client unusable and drops its rows. Idempotent.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docs = nil
	c.byID = nil
	c.closed = true
	return nil
}

// sha256Hex is the shared endpoint-hash helper (cache.go consumes it).
// 单独成函数是为了让"URL 只以 sha256 形式出现"这件事只有一个落点。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
