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

// cache_test.go 锁定 D-14 的五条不变量：
//  1. key 四要素（任一变化都换 key）
//  2. 语料 hash 基于内容、mtime 免疫
//  3. 六项头部校验、整份作废（绝不部分可用）
//  4. tmp + rename 原子写 + 并发首启互斥（带超时）
//  5. seed 的 Embed 失败整体中止、不留半份语料
//
// 全部零网络：embedder 是 embedded_test.go 里的 fakeEmbedder（只存在于
// _test.go，生产代码零替身分支 —— D-01）。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

const testModel = "bge-m3"

// makeTestRows builds n synthetic rows with deterministic embeddings.
func makeTestRows(n, dim int) []row {
	out := make([]row, 0, n)
	for i := 0; i < n; i++ {
		v := make([]float32, dim)
		if dim > 0 {
			v[0] = float32(i+1) / 10
		}
		out = append(out, row{
			ID:            int64(i + 1),
			Tier:          "authority",
			Content:       "正文 " + strconv.Itoa(i),
			Source:        "authority/doc" + strconv.Itoa(i) + ".md",
			Embedding:     v,
			EffectiveFrom: int64(i),
			EffectiveTo:   0,
		})
	}
	return out
}

// TestCacheKeyIncludesModelDimAndCorpusHash: any of the four inputs changing
// must change the key; the same inputs must be stable; the output is a 64-char
// hex sha256.
func TestCacheKeyIncludesModelDimAndCorpusHash(t *testing.T) {
	base := cacheKey("bge-m3", 1024, "corpusA", "endpointA")
	require.Len(t, base, 64)
	assert.Equal(t, base, cacheKey("bge-m3", 1024, "corpusA", "endpointA"),
		"identical inputs must produce an identical key")

	assert.NotEqual(t, base, cacheKey("bge-large", 1024, "corpusA", "endpointA"),
		"model must participate in the key")
	assert.NotEqual(t, base, cacheKey("bge-m3", 768, "corpusA", "endpointA"),
		"dimension must participate in the key")
	assert.NotEqual(t, base, cacheKey("bge-m3", 1024, "corpusB", "endpointA"),
		"corpus hash must participate in the key")
	assert.NotEqual(t, base, cacheKey("bge-m3", 1024, "corpusA", "endpointB"),
		"endpoint hash must participate in the key")

	// 空值仍产生确定性 key（不 panic）。
	empty := cacheKey("", 0, "", "")
	require.Len(t, empty, 64)
	assert.Equal(t, empty, cacheKey("", 0, "", ""))
}

// TestCorpusHashIgnoresMtime is the mechanical lock for 49-RESEARCH Pitfall 8:
// git clone / git checkout rewrite mtime, so mtime must not influence the
// corpus hash. Content changes must, though.
func TestCorpusHashIgnoresMtime(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	write("a.md", "# A\n某个制度的正文\n")
	write("b.md", "# B\n另一段正文\n")

	entries, err := knowledge.WalkDocs(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	read := func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	}

	first, err := corpusHash(entries, read)
	require.NoError(t, err)

	// 改 mtime（内容不变）：hash 必须完全不变。
	later := time.Now().Add(48 * time.Hour)
	for _, name := range []string{"a.md", "b.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(dir, name), later, later))
	}
	second, err := corpusHash(entries, read)
	require.NoError(t, err)
	assert.Equal(t, first, second, "mtime must not participate in the corpus hash")

	// 改一个字节：hash 必须变。
	write("b.md", "# B\n另一段正文（已修订）\n")
	third, err := corpusHash(entries, read)
	require.NoError(t, err)
	assert.NotEqual(t, first, third, "content change must change the corpus hash")
}

// TestCorpusHashOverRealKnowledgeTree freezes the corpus census from
// 49-PATTERNS.md § 9 as a regression gate: 218 real .md files =
// authority 16 + sql-guides 202. (220 is the directory-entry count and 221 is
// the file count including the three .gitkeep files — both are wrong.)
func TestCorpusHashOverRealKnowledgeTree(t *testing.T) {
	root := testRepoRoot(t)
	knowledgeRoot := filepath.Join(root, "knowledge")

	count := func(dir string) int {
		es, err := knowledge.WalkDocs(filepath.Join(knowledgeRoot, dir))
		require.NoError(t, err)
		return len(es)
	}
	authority := count("authority")
	sqlGuides := count("sql-guides")
	assert.Equal(t, 16, authority, "knowledge/authority must hold 16 .md files")
	assert.Equal(t, 202, sqlGuides, "knowledge/sql-guides must hold 202 .md files")
	assert.Equal(t, 218, authority+sqlGuides, "the indexable corpus is exactly 218 .md files")
	assert.Equal(t, 218, authority+sqlGuides+count("draft")+count("historical")+count("personal"),
		"draft/historical/personal carry no .md content (only .gitkeep)")

	// 内容哈希的确定性：同一语料两次调用结果相同。
	entries := collectEntries(t, knowledgeRoot, testRegistry())
	require.Len(t, entries, 218)

	read := func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(knowledgeRoot, filepath.FromSlash(rel)))
	}
	h1, err := corpusHash(entries, read)
	require.NoError(t, err)
	h2, err := corpusHash(entries, read)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

// TestCacheCorruptFileTriggersRebuild covers five corruption shapes plus a
// model mismatch. Every one must yield errCacheInvalid (never a partial row
// set).
func TestCacheCorruptFileTriggersRebuild(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cacheFileName)
	const dim = 3
	rows := makeTestRows(2, dim)
	spec := cacheSpec{Model: testModel, Dim: dim, Corpus: hex32("corpus"), Endpoint: hex32("endpoint")}
	require.NoError(t, writeCache(path, spec, rows))

	// 先证明写出来的文件是合法的（写/读严格互逆）。
	got, err := readCache(path, spec)
	require.NoError(t, err)
	require.Len(t, got, len(rows))
	assert.Equal(t, rows[1].Source, got[1].Source)
	assert.Equal(t, rows[1].Embedding, got[1].Embedding)
	assert.Equal(t, rows[1].EffectiveFrom, got[1].EffectiveFrom)
	// 正文**不入缓存**（cache.go 格式注释）：缓存只省 embedding 网络调用，
	// 正文由 Ensure 按 Source 从语料回填。所以读回的空 Content 是契约行为。
	assert.Empty(t, got[1].Content, "the cache must not store chunk bodies")

	// 头部偏移由格式推导（magic[4] | version u32 | modelLen u16 | model |
	// dim u32 | docCount u32 | corpusHash[32] | endpointHash[32] | payload[32]）。
	const modelLen = len(testModel)
	versionOff := len(cacheMagic)
	dimOff := versionOff + 4 + 2 + modelLen
	docCountOff := dimOff + 4
	corpusOff := docCountOff + 4
	endpointOff := corpusOff + 32
	payloadHashOff := endpointOff + 32

	cases := []struct {
		name   string
		mutate func(b []byte) []byte
	}{
		{"magic flipped", func(b []byte) []byte { b[0] ^= 0xFF; return b }},
		{"version bumped", func(b []byte) []byte { b[versionOff] = 2; return b }},
		{"dim changed", func(b []byte) []byte {
			// 768 = 0x00000300 → LE 字节序：[0]=0x00, [1]=0x03
			b[dimOff] = 0
			b[dimOff+1] = 3
			b[dimOff+2] = 0
			b[dimOff+3] = 0
			return b
		}},
		{"docCount changed", func(b []byte) []byte { b[docCountOff] = 9; return b }},
		{"corpusHash flipped", func(b []byte) []byte { b[corpusOff+31] ^= 0xFF; return b }},
		{"endpointHash flipped", func(b []byte) []byte { b[endpointOff+31] ^= 0xFF; return b }},
		{"payloadHash flipped", func(b []byte) []byte { b[payloadHashOff+31] ^= 0xFF; return b }},
		{"tail truncated", func(b []byte) []byte { return b[:len(b)-10] }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, rerr := os.ReadFile(path)
			require.NoError(t, rerr)
			corrupt := tc.mutate(append([]byte(nil), raw...))
			require.NoError(t, os.WriteFile(path, corrupt, 0o600))

			out, rerr := readCache(path, spec)
			require.Error(t, rerr)
			assert.True(t, errors.Is(rerr, errCacheInvalid),
				"%s must be reported as errCacheInvalid, got %v", tc.name, rerr)
			assert.Nil(t, out, "%s must not yield a partial row set", tc.name)
		})
	}

	// 另一个模型写下的缓存不能拿来用。
	require.NoError(t, writeCache(path, spec, rows))
	_, err = readCache(path, cacheSpec{Model: "bge-large", Dim: dim, Corpus: spec.Corpus, Endpoint: spec.Endpoint})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errCacheInvalid))
	assert.Contains(t, err.Error(), "model mismatch")
}

// TestReadCacheMissingFileIsNotInvalid: a missing file must surface the raw
// os error so callers can distinguish "miss" (silent rebuild) from "corrupt"
// (logged rebuild).
func TestReadCacheMissingFileIsNotInvalid(t *testing.T) {
	spec := cacheSpec{Model: testModel, Dim: 3, Corpus: hex32("c"), Endpoint: hex32("e")}
	_, err := readCache(filepath.Join(t.TempDir(), "nope.bin"), spec)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	assert.False(t, errors.Is(err, errCacheInvalid))
}

// hex32 returns a deterministic 64-char hex digest for a label, standing in
// for a real corpus / endpoint hash in cache tests.
func hex32(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

// TestCacheConcurrentFirstStart: four concurrent first-boot writers serialize
// on the O_EXCL lock, leave exactly one vectors.bin (no .tmp residue), and all
// observe the same row count. The overdue-holder sub-case proves the wait is
// bounded and the timeout error names the cache.
func TestCacheConcurrentFirstStart(t *testing.T) {
	const dim = 3
	rows := makeTestRows(3, dim)

	t.Run("four writers", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, cacheFileName)

		const n = 4
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				release, err := acquireCacheLock(cacheLockPath(path), 10*time.Second, nil)
				if err != nil {
					errs[i] = err
					return
				}
				defer func() {
					if rerr := release(); rerr != nil {
						errs[i] = rerr
					}
				}()
				errs[i] = writeCache(path, cacheSpec{Model: testModel, Dim: dim, Corpus: hex32("corpus"), Endpoint: hex32("endpoint")}, rows)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			require.NoError(t, err, "writer %d", i)
		}

		ents, err := os.ReadDir(dir)
		require.NoError(t, err)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		assert.Equal(t, []string{cacheFileName}, names,
			"only the final cache file may remain — a .tmp or .lock leftover means an unlock/rename bug")

		got, err := readCache(path, cacheSpec{Model: testModel, Dim: dim, Corpus: hex32("corpus"), Endpoint: hex32("endpoint")})
		require.NoError(t, err)
		assert.Len(t, got, len(rows), "no partial cache may be readable")
	})

	t.Run("timeout while another process holds the lock", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, cacheFileName)

		held := make(chan struct{})
		stop := make(chan struct{})
		go func() {
			release, err := acquireCacheLock(cacheLockPath(path), 2*time.Second, nil)
			if err != nil {
				close(held)
				return
			}
			close(held)
			<-stop
			_ = release()
		}()
		<-held
		defer close(stop)

		start := time.Now()
		_, err := acquireCacheLock(cacheLockPath(path), 50*time.Millisecond, nil)
		elapsed := time.Since(start)

		require.Error(t, err, "a held lock must time out, never be ignored")
		assert.Contains(t, err.Error(), "timed out waiting for the vector cache")
		assert.Contains(t, err.Error(), "no fake-vector fallback")
		assert.Less(t, elapsed, 10*time.Second, "the wait must be bounded by the configured timeout")
	})
}

// TestSeedEmbedFailureIsFatalAndLeavesNoPartialCorpus: an embedding failure
// mid-corpus aborts the WHOLE load and leaves the store empty (never a partial
// corpus, never a zero/hash vector — D-01).
func TestSeedEmbedFailureIsFatalAndLeavesNoPartialCorpus(t *testing.T) {
	root := testRepoRoot(t)
	knowledgeRoot := filepath.Join(root, "knowledge")
	reg := testRegistry()

	emb := &fakeEmbedder{dim: 1024, failAfter: 2}
	c, err := New(Config{
		Embedder:     emb,
		Model:        testModel,
		Dimensions:   1024,
		CacheDir:     "", // 缓存禁用：本测试只关心装载语义，不写仓库内路径
		KnowledgeDir: knowledgeRoot,
		Registry:     reg,
	})
	require.NoError(t, err)

	n, err := c.SeedFromKnowledgeDir(context.Background(), knowledgeRoot, emb, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded.SeedFromKnowledgeDir")
	assert.Contains(t, err.Error(), "endpoint unavailable")
	assert.Equal(t, 0, n, "a failed seed must not report a partial count")

	metas, merr := c.ListMetadata(context.Background())
	require.NoError(t, merr)
	assert.Len(t, metas, 0, "a failed seed must leave no partial corpus (build fully, commit once)")
}

// TestSeedEmptyCorpusIsNotAnError: an absent/empty corpus is the legal
// degraded state (SC-4) — (0, nil), never an error.
func TestSeedEmptyCorpusIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	emb := &fakeEmbedder{dim: 4}
	c, err := New(Config{Embedder: emb, Dimensions: 4})
	require.NoError(t, err)

	n, err := c.SeedFromKnowledgeDir(context.Background(), dir, emb, testRegistry())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// TestSeedRequiresEmbedderAndRegistry: both are fail-closed prerequisites —
// a nil embedder must never silently produce zero vectors (D-01), and a nil
// registry must never silently seed nothing.
func TestSeedRequiresEmbedderAndRegistry(t *testing.T) {
	c, err := New(Config{Dimensions: 4})
	require.NoError(t, err)
	ctx := context.Background()

	_, err = c.SeedFromKnowledgeDir(ctx, "", nil, testRegistry())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledgeDir is required")

	_, err = c.SeedFromKnowledgeDir(ctx, t.TempDir(), nil, testRegistry())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no in-process or hash fallback")

	_, err = c.SeedFromKnowledgeDir(ctx, t.TempDir(), &fakeEmbedder{dim: 4}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail-closed")
}

// TestValidateDocsFailClosed is the direct unit test for the two corpus
// invariants (active slug + effective window).
func TestValidateDocsFailClosed(t *testing.T) {
	reg := testRegistry()

	// 不在 ActiveSlugs 的 tier → error（fail-closed）。
	bad := []retriever.Document{{ID: 1, Tier: "rogue"}}
	err := validateDocs(bad, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an active knowledge base")

	// 倒置窗口 → error。
	inverted := []retriever.Document{{ID: 2, Tier: "authority", EffectiveFrom: 100, EffectiveTo: 50}}
	err = validateDocs(inverted, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inverted effective window")

	// 负 effective_to → error。
	negative := []retriever.Document{{ID: 3, Tier: "authority", EffectiveTo: -1}}
	require.Error(t, validateDocs(negative, reg))

	// nil registry + 非空 docs → error。
	require.Error(t, validateDocs([]retriever.Document{{ID: 4, Tier: "authority"}}, nil))

	// 合法 → nil；空集 → nil（空语料是合法降级状态）。
	ok := []retriever.Document{{ID: 5, Tier: "authority", EffectiveFrom: 10, EffectiveTo: 20}}
	require.NoError(t, validateDocs(ok, reg))
	require.NoError(t, validateDocs(nil, reg))
}

// TestStripFrontmatterMatchesMongoAlgorithm: the local copy must behave
// identically to internal/retriever/mongo/seed.go:186-200 (the source of
// truth this copy names in its comment).
func TestStripFrontmatterMatchesMongoAlgorithm(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"no frontmatter", "正文", "正文"},
		{"plain frontmatter", "---\ntitle: x\n---\n正文", "正文"},
		{"crlf frontmatter", "---\r\ntitle: x\r\n---\r\n正文", "正文"},
		{"unterminated frontmatter", "---\ntitle: x\n正文", "---\ntitle: x\n正文"},
		{"frontmatter only", "---\ntitle: x\n---\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, string(stripFrontmatter([]byte(tc.in))))
		})
	}
}

// TestEnsureLoadsValidCacheAndSkipsSeed: with a valid cache present, Ensure
// installs the rows and a subsequent seed short-circuits with zero embedding
// calls (the whole point of the cache).
func TestEnsureLoadsValidCacheAndSkipsSeed(t *testing.T) {
	root := testRepoRoot(t)
	knowledgeRoot := filepath.Join(root, "knowledge")
	reg := testRegistry()
	cacheDir := t.TempDir()
	const dim = 8

	c, err := New(Config{
		Model:        testModel,
		Dimensions:   dim,
		CacheDir:     cacheDir,
		KnowledgeDir: knowledgeRoot,
		Registry:     reg,
	})
	require.NoError(t, err)

	// 先跑一次 Ensure 让 cachePath / corpusHash 就位，然后手工写一份缓存。
	// rows 的 Source 必须指向**真实语料文件**：缓存不存正文，Ensure 命中后
	// 要按 Source 回填，路径不存在会让整份缓存被判为不可用（这是刻意行为）。
	require.NoError(t, c.Ensure(context.Background()))
	require.NotEmpty(t, c.cachePath)
	entries := collectEntries(t, knowledgeRoot, reg)
	require.GreaterOrEqual(t, len(entries), 3)
	rows := make([]row, 0, 3)
	for i, e := range entries[:3] {
		v := make([]float32, dim)
		v[0] = float32(i+1) / 10
		rows = append(rows, row{
			ID:        int64(i + 1),
			Tier:      strings.SplitN(e.RelPath, "/", 2)[0],
			Source:    e.RelPath,
			Embedding: v,
		})
	}
	require.NoError(t, writeCache(c.cachePath, cacheSpec{Model: testModel, Dim: dim, Corpus: c.corpusHash, Endpoint: c.endpointHash}, rows))

	c2, err := New(Config{
		Model:        testModel,
		Dimensions:   dim,
		CacheDir:     cacheDir,
		KnowledgeDir: knowledgeRoot,
		Registry:     reg,
	})
	require.NoError(t, err)
	require.NoError(t, c2.Ensure(context.Background()))

	metas, err := c2.ListMetadata(context.Background())
	require.NoError(t, err)
	assert.Len(t, metas, len(rows), "a valid cache must be loaded by Ensure")

	// seed 必须短路：embedder 为 nil 也不会被用到（它连报错的机会都没有）。
	emb := &fakeEmbedder{dim: dim}
	n, err := c2.SeedFromKnowledgeDir(context.Background(), knowledgeRoot, emb, reg)
	require.NoError(t, err)
	assert.Equal(t, len(rows), n)
	assert.Equal(t, 0, emb.calls, "a cache hit must not call the embedder at all")
}

// TestEnsureRebuildsOnStaleCache: a cache written for a different corpus must
// not be loaded — Ensure treats it as a miss and the seed rebuilds.
func TestEnsureRebuildsOnStaleCache(t *testing.T) {
	root := testRepoRoot(t)
	knowledgeRoot := filepath.Join(root, "knowledge")
	reg := testRegistry()
	cacheDir := t.TempDir()
	const dim = 8

	c, err := New(Config{
		Model:        testModel,
		Dimensions:   dim,
		CacheDir:     cacheDir,
		KnowledgeDir: knowledgeRoot,
		Registry:     reg,
	})
	require.NoError(t, err)
	require.NoError(t, c.Ensure(context.Background()))

	// 写一份 dim 不符的缓存 ⇒ readCache 整份作废 ⇒ Ensure 当作未命中。
	require.NoError(t, writeCache(c.cachePath, cacheSpec{Model: testModel, Dim: dim + 1, Corpus: c.corpusHash, Endpoint: c.endpointHash}, makeTestRows(2, dim+1)))

	c2, err := New(Config{
		Model:        testModel,
		Dimensions:   dim,
		CacheDir:     cacheDir,
		KnowledgeDir: knowledgeRoot,
		Registry:     reg,
	})
	require.NoError(t, err)
	require.NoError(t, c2.Ensure(context.Background()))

	metas, err := c2.ListMetadata(context.Background())
	require.NoError(t, err)
	assert.Len(t, metas, 0, "a stale cache must not be partially usable")
}

// --- helpers -------------------------------------------------------------

// testRepoRoot locates the repository root from the package directory.
func testRepoRoot(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("SKILLFORGE_TEST_REPO_ROOT"); v != "" {
		return v
	}
	abs, err := filepath.Abs("../../..")
	require.NoError(t, err)
	// 确认定位成功（go test 的工作目录就是包目录）。
	_, err = os.Stat(filepath.Join(abs, "knowledge"))
	require.NoError(t, err, "expected the repo root at %s (set SKILLFORGE_TEST_REPO_ROOT to override)", abs)
	return abs
}

// testRegistry builds a read-only registry with the two content-bearing bases.
func testRegistry() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "sql-guides", DisplayName: "SQL", Role: kb.RolePersonal, RetrievalEnabled: true},
	})
}

// collectEntries walks the registry-active corpus dirs into one slice, the same
// shape Ensure hashes.
func collectEntries(t *testing.T, knowledgeRoot string, reg kb.Registry) []knowledge.DocEntry {
	t.Helper()
	slugs, err := knowledge.WalkRegisteredDirs(knowledgeRoot, reg, nil)
	require.NoError(t, err)
	var out []knowledge.DocEntry
	for _, slug := range slugs {
		es, err := knowledge.WalkDocs(filepath.Join(knowledgeRoot, slug))
		require.NoError(t, err)
		for _, e := range es {
			out = append(out, knowledge.DocEntry{
				RelPath: filepath.ToSlash(filepath.Join(slug, e.RelPath)),
				Bytes:   e.Bytes,
			})
		}
	}
	return out
}
