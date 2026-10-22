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

// cache.go 是本地向量缓存（D-14）：让第二次启动跳过 BGE-M3 的 218 次网络
// 调用，同时保证缓存不可能把"错误向量"喂进检索。
//
// 三条不变量，任何一条破了缓存就必须整份作废：
//
//  1. key 四要素 = model + dimension + 语料内容 hash + endpoint 的 sha256。
//     换模型、换维度、改语料、换 BGE 端点，都必须让旧缓存失效。
//  2. 语料 hash 基于**内容**（相对路径 + 字节，按路径排序），**绝不用 mtime**：
//     git clone / git checkout 会重写 mtime，用 mtime 做 key 会让"语料没变"
//     被判成"变了"（无谓重建）或更糟的相反情形。
//  3. endpoint 只以 sha256 形式出现。原始 BGE_M3_URL 绝不进 key、绝不进
//     文件名、绝不进缓存载荷 —— URL 可能带 token，而文件名会进 git status
//     与日志（49-RESEARCH Pitfall 9）。
//
// 文件格式（紧凑二进制头 + 原始 float32 小端，JSON 存 218×1024 float 体积
// 约 10×）：
//
//	magic        [4]byte  "SFVC"
//	version      uint32   1
//	modelLen     uint16 + model bytes
//	dim          uint32
//	docCount     uint32
//	corpusHash   [32]byte  sha256(语料内容，见 corpusHash())
//	endpointHash [32]byte  sha256(BGE_M3_URL)
//	payloadHash  [32]byte  sha256(全部向量字节)
//	then per doc: id int64 | tierLen uint16 + tier | sourceLen uint16 + source
//	              | effectiveFrom int64 | effectiveTo int64 | vec(dim × float32 LE)
//
// corpusHash / endpointHash 进头部（而不只进 cacheKey 的文件名）是有意的：
// 缓存文件名固定（vectors.bin），key 必须靠自校验兜住 —— 否则"换了语料但
// 文件名没变"会让旧向量被静默复用，这是本 cache 最危险的一种失效。
//
// 读取时逐项校验八个头部字段，任一不符 ⇒ errCacheInvalid ⇒ 整份重建。
// **绝不部分可用**：一个 header 对得上但载荷被截断的文件，返回的 rows 会是
// 一份"看起来能用"的残缺语料，那比没有缓存危险得多。
//
// 正文（Content）**不入缓存**：缓存只负责省掉 218 次 embedding 网络调用，
// 正文随时可从语料重读且永远不会与语料漂移。缓存命中后由 Ensure 按 Source
// 回填正文（零网络，只读文件）。

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"time"

	"github.com/skillforge/skillforge/internal/knowledge"
)

const (
	cacheMagic      = "SFVC"
	cacheVersion    = uint32(1)
	cacheFileName   = "vectors.bin"
	cacheLockSuffix = ".lock"
)

// errCacheInvalid is the "整份作废" sentinel. Callers treat it identically to
// a cache miss (rebuild) but keep it distinguishable for logging and tests.
var errCacheInvalid = errors.New("embedded: cache invalid")

// cacheKey is the content-addressed identity of one cache file.
//
// 四个输入全部参与（缺一个都会让某类变化无法失效缓存）。endpointHash 是
// caller 算好的 sha256 —— 本函数**不接收 URL**，接口层面就断了"URL 原串
// 进 key"的可能。
func cacheKey(model string, dim int, corpusHash, endpointHash string) string {
	h := sha256.New()
	io.WriteString(h, "v1")
	h.Write([]byte{0})
	io.WriteString(h, model)
	h.Write([]byte{0})
	io.WriteString(h, fmt.Sprintf("%d", dim))
	h.Write([]byte{0})
	io.WriteString(h, corpusHash)
	h.Write([]byte{0})
	io.WriteString(h, endpointHash)
	return hex.EncodeToString(h.Sum(nil))
}

// corpusHash hashes the corpus by CONTENT: for each entry (in WalkDocs order —
// that function guarantees slash-relative-path ordering) it writes
//
//	<relPath> \x00 <byteLen decimal> \x00 <bytes>
//
// 不用 entry.Bytes 之外的任何时间元数据，不用 mtime，不用文件计数：
// 真正进 hash 的是**实际读到的字节**，entry.Bytes 只作为长度前缀存在。
// 文件被 touch 但内容不变 ⇒ hash 不变（缓存命中）；内容改一个字节 ⇒ hash
// 变（缓存作废）。
func corpusHash(entries []knowledge.DocEntry, read func(rel string) ([]byte, error)) (string, error) {
	h := sha256.New()
	for _, e := range entries {
		b, err := read(e.RelPath)
		if err != nil {
			return "", fmt.Errorf("embedded.corpusHash: read %s: %w", e.RelPath, err)
		}
		io.WriteString(h, e.RelPath)
		h.Write([]byte{0})
		io.WriteString(h, fmt.Sprintf("%d", len(b)))
		h.Write([]byte{0})
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cacheSpec carries every input the cache file must self-verify against.
//
// 单独成结构体而不是散落参数：read/write 需要的是同一组要素，漏传一个就会
// 让某类失效静默通过（编译器会盯住结构体字段）。
type cacheSpec struct {
	Model    string
	Dim      int
	Corpus   string // corpusHash: 语料内容 sha256
	Endpoint string // sha256(BGE_M3_URL) —— 原始 URL 绝不出现
}

// readCache decodes the cache file and validates every header field before
// returning any row.
//
// 八项独立校验（magic / version / model / dim / docCount / corpusHash /
// endpointHash / payloadHash）各自有独立分支与独立原因串，违反任意一项都
// 返回 errCacheInvalid。
// 文件不存在时返回原始 os 错误 —— caller 用 os.IsNotExist 区分"未命中"
// 与"损坏"（前者安静重建，后者应当可见）。
func readCache(path string, spec cacheSpec) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)

	invalid := func(format string, args ...any) ([]row, error) {
		return nil, fmt.Errorf("%w: %s", errCacheInvalid, fmt.Sprintf(format, args...))
	}

	magic := make([]byte, len(cacheMagic))
	if _, err := io.ReadFull(br, magic); err != nil {
		return invalid("header truncated (magic): %v", err)
	}
	if string(magic) != cacheMagic {
		return invalid("magic mismatch (got %q want %q)", string(magic), cacheMagic)
	}

	var version uint32
	if err := binary.Read(br, binary.LittleEndian, &version); err != nil {
		return invalid("header truncated (version): %v", err)
	}
	if version != cacheVersion {
		return invalid("version mismatch (got %d want %d)", version, cacheVersion)
	}

	var modelLen uint16
	if err := binary.Read(br, binary.LittleEndian, &modelLen); err != nil {
		return invalid("header truncated (modelLen): %v", err)
	}
	modelBytes := make([]byte, modelLen)
	if _, err := io.ReadFull(br, modelBytes); err != nil {
		return invalid("header truncated (model): %v", err)
	}
	if string(modelBytes) != spec.Model {
		return invalid("model mismatch (cache %q want %q)", string(modelBytes), spec.Model)
	}

	var fileDim uint32
	if err := binary.Read(br, binary.LittleEndian, &fileDim); err != nil {
		return invalid("header truncated (dim): %v", err)
	}
	if int(fileDim) != spec.Dim {
		return invalid("dim mismatch (cache %d want %d)", fileDim, spec.Dim)
	}

	var docCount uint32
	if err := binary.Read(br, binary.LittleEndian, &docCount); err != nil {
		return invalid("header truncated (docCount): %v", err)
	}

	var corpus [32]byte
	if _, err := io.ReadFull(br, corpus[:]); err != nil {
		return invalid("header truncated (corpusHash): %v", err)
	}
	if hex.EncodeToString(corpus[:]) != spec.Corpus {
		return invalid("corpusHash mismatch (cache %s want %s); the corpus changed since this cache was written",
			hex.EncodeToString(corpus[:]), spec.Corpus)
	}

	var endpoint [32]byte
	if _, err := io.ReadFull(br, endpoint[:]); err != nil {
		return invalid("header truncated (endpointHash): %v", err)
	}
	if hex.EncodeToString(endpoint[:]) != spec.Endpoint {
		return invalid("endpointHash mismatch (cache %s want %s); BGE_M3_URL changed since this cache was written",
			hex.EncodeToString(endpoint[:]), spec.Endpoint)
	}

	var payloadHash [32]byte
	if _, err := io.ReadFull(br, payloadHash[:]); err != nil {
		return invalid("header truncated (payloadHash): %v", err)
	}

	// 逐条解码，同时把向量字节喂给一个 sha256：读完与头部 payloadHash 比对。
	// 这一步是"载荷损坏"的唯一检测手段（长度检查只能抓到截断）。
	payload := sha256.New()
	rows := make([]row, 0, docCount)
	for i := uint32(0); i < docCount; i++ {
		r, err := readCacheRow(br, int(fileDim), payload)
		if err != nil {
			return invalid("row %d: %v", i, err)
		}
		rows = append(rows, r)
	}
	// 尾部不允许有额外字节：多出来的内容只有两种来源 —— 格式版本不一致，
	// 或文件被追加写坏。两种都当作整份无效。
	if _, err := br.ReadByte(); err == nil {
		return invalid("trailing bytes after docCount=%d rows", docCount)
	} else if !errors.Is(err, io.EOF) {
		return invalid("read trailing byte: %v", err)
	}

	got := hex.EncodeToString(payload.Sum(nil))
	if got != hex.EncodeToString(payloadHash[:]) {
		return invalid("payloadHash mismatch (cache %s want %s)", hex.EncodeToString(payloadHash[:]), got)
	}
	return rows, nil
}

// readCacheRow decodes one row and writes the vector bytes into payload.
func readCacheRow(br *bufio.Reader, dim int, payload hash.Hash) (row, error) {
	var r row
	if err := binary.Read(br, binary.LittleEndian, &r.ID); err != nil {
		return r, fmt.Errorf("id: %w", err)
	}
	tier, err := readLenPrefixedString(br)
	if err != nil {
		return r, fmt.Errorf("tier: %w", err)
	}
	r.Tier = tier
	source, err := readLenPrefixedString(br)
	if err != nil {
		return r, fmt.Errorf("source: %w", err)
	}
	r.Source = source
	if err := binary.Read(br, binary.LittleEndian, &r.EffectiveFrom); err != nil {
		return r, fmt.Errorf("effectiveFrom: %w", err)
	}
	if err := binary.Read(br, binary.LittleEndian, &r.EffectiveTo); err != nil {
		return r, fmt.Errorf("effectiveTo: %w", err)
	}
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		var bits uint32
		if err := binary.Read(br, binary.LittleEndian, &bits); err != nil {
			return r, fmt.Errorf("vec[%d]: %w", i, err)
		}
		v := math.Float32frombits(bits)
		vec[i] = v
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], bits)
		payload.Write(buf[:])
	}
	r.Embedding = vec
	return r, nil
}

func readLenPrefixedString(br *bufio.Reader) (string, error) {
	var n uint16
	if err := binary.Read(br, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(br, b); err != nil {
		return "", err
	}
	return string(b), nil
}

// writeCache serializes rows to path via tmp + rename.
//
// tmp+rename 是 load-bearing 的：直接写目标文件时，进程被杀会留下一个半截
// 文件，下一个进程的 readCache 可能把它当成合法缓存（如果头部已经写完）。
// rename 是原子的，因此"存在即完整"成立。
func writeCache(path string, spec cacheSpec, rows []row) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("embedded.writeCache: create %s: %w", tmp, err)
	}
	// 任何提前返回都必须清掉 tmp：残留的 .tmp 会让"只有 vectors.bin"的
	// 断言失败，也会在磁盘上留下半截文件。
	fail := func(format string, args ...any) error {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf(format, args...)
	}
	bw := bufio.NewWriterSize(f, 1<<20)

	// 先算 payloadHash（覆盖全部向量字节），再写头 —— 头部需要它。
	payload := sha256.New()
	for _, r := range rows {
		if len(r.Embedding) != spec.Dim {
			return fail("embedded.writeCache: row id %d embedding dim %d != %d", r.ID, len(r.Embedding), spec.Dim)
		}
		for _, v := range r.Embedding {
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
			payload.Write(buf[:])
		}
	}

	if _, err := bw.WriteString(cacheMagic); err != nil {
		return fail("embedded.writeCache: write magic: %w", err)
	}
	write := func(v any) error { return binary.Write(bw, binary.LittleEndian, v) }
	modelBytes := []byte(spec.Model)
	if len(modelBytes) > math.MaxUint16 {
		return fail("embedded.writeCache: model name too long (%d bytes)", len(modelBytes))
	}
	if err := errors.Join(
		write(cacheVersion),
		write(uint16(len(modelBytes))),
	); err != nil {
		return fail("embedded.writeCache: write header: %w", err)
	}
	if _, err := bw.Write(modelBytes); err != nil {
		return fail("embedded.writeCache: write model: %w", err)
	}
	if err := errors.Join(
		write(uint32(spec.Dim)),
		write(uint32(len(rows))),
	); err != nil {
		return fail("embedded.writeCache: write header: %w", err)
	}
	// 语料 hash 与 endpoint hash 必须能解码回 32 字节原始摘要；调用方传的是
	// hex 串（64 字符），长度不符说明调用方出 bug，早失败。
	corpusRaw, err := hex.DecodeString(spec.Corpus)
	if err != nil || len(corpusRaw) != sha256.Size {
		return fail("embedded.writeCache: corpus hash must be a %d-byte hex sha256 (got %q)", sha256.Size, spec.Corpus)
	}
	endpointRaw, err := hex.DecodeString(spec.Endpoint)
	if err != nil || len(endpointRaw) != sha256.Size {
		return fail("embedded.writeCache: endpoint hash must be a %d-byte hex sha256 (got %q)", sha256.Size, spec.Endpoint)
	}
	for _, blob := range [][]byte{corpusRaw, endpointRaw, payload.Sum(nil)} {
		if _, err := bw.Write(blob); err != nil {
			return fail("embedded.writeCache: write header hash: %w", err)
		}
	}

	for _, r := range rows {
		if err := write(r.ID); err != nil {
			return fail("embedded.writeCache: write id: %w", err)
		}
		if err := writeLenPrefixedString(bw, r.Tier); err != nil {
			return fail("embedded.writeCache: write tier: %w", err)
		}
		if err := writeLenPrefixedString(bw, r.Source); err != nil {
			return fail("embedded.writeCache: write source: %w", err)
		}
		if err := errors.Join(write(r.EffectiveFrom), write(r.EffectiveTo)); err != nil {
			return fail("embedded.writeCache: write window: %w", err)
		}
		for _, v := range r.Embedding {
			if err := write(math.Float32bits(v)); err != nil {
				return fail("embedded.writeCache: write vec: %w", err)
			}
		}
	}

	if err := bw.Flush(); err != nil {
		return fail("embedded.writeCache: flush: %w", err)
	}
	// Sync 后再 rename：否则崩溃可能让目录项指向一个还没落盘的数据块
	// （rename 是原子的，但它不承诺 rename 之前的数据已经持久）。
	if err := f.Sync(); err != nil {
		return fail("embedded.writeCache: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("embedded.writeCache: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("embedded.writeCache: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func writeLenPrefixedString(bw *bufio.Writer, s string) error {
	b := []byte(s)
	if len(b) > math.MaxUint16 {
		return fmt.Errorf("string too long (%d bytes)", len(b))
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(len(b))); err != nil {
		return err
	}
	_, err := bw.Write(b)
	return err
}

// acquireCacheLock takes the inter-process first-boot mutex.
//
// 用 os.O_CREATE|os.O_EXCL 原子创建锁文件：内核保证"只有一个进程能创建成功"，
// 不需要 flock（那会引入依赖）、不需要"建目录当锁"（不可靠）、更不需要
// sleep 猜时长。抢不到锁的进程按 200ms 轮询等待，最多等 wait；超时**明确
// 报错**，绝不降级为假向量（D-01）。
//
// 循环必须有 deadline —— 无界 sleep 会让一次异常退出（进程被 kill 没删锁）
// 永久卡住后续所有启动。
func acquireCacheLock(lockPath string, wait time.Duration, logf func(string, ...any)) (func() error, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	deadline := time.Now().Add(wait)
	release := func() error {
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("embedded: release cache lock %s: %w", lockPath, err)
		}
		return nil
	}

	// 一次尝试；成功返回 release。
	tryOnce := func() (bool, error) {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// 写一行持有者的 pid/时间，方便人工排查残留锁（不影响语义）。
			fmt.Fprintf(f, "pid=%d at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			f.Close()
			return true, nil
		}
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("embedded: create cache lock %s: %w", lockPath, err)
	}

	if ok, err := tryOnce(); err != nil {
		return nil, err
	} else if ok {
		return release, nil
	}

	if wait <= 0 {
		return nil, fmt.Errorf("embedded: timed out waiting for the vector cache after %s; another process holds %s (no fake-vector fallback: D-01)", wait, lockPath)
	}

	logf("[INFO] embedded: waiting for the vector cache lock %s (another process is seeding)", lockPath)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("embedded: timed out waiting for the vector cache after %s; another process holds %s (no fake-vector fallback: D-01)", wait, lockPath)
			}
			ok, err := tryOnce()
			if err != nil {
				return nil, err
			}
			if ok {
				return release, nil
			}
		}
	}
}

// cacheLockPath is the lock file that guards one cache file.
func cacheLockPath(cachePath string) string { return cachePath + cacheLockSuffix }

// cacheFileNameFor returns the cache file name inside a cache dir.
func cacheFileNameFor(dir string) string { return dir + string(os.PathSeparator) + cacheFileName }
