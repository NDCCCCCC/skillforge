# Phase 27: 文档上传入库 (Document Upload Pipeline) - Research

**Researched:** 2026-09-07
**Domain:** Go/Eino document parsing + HTTP upload pipeline + dual-backend indexing
**Confidence:** MEDIUM-HIGH (code-pattern grounded; eino-ext parser availability at v0.1.13 unverified — see Risk R1)

## User Constraints (from ROADMAP Phase 27 + PROJECT.md v1.2 standards)

### Locked Decisions
- **DOC-01**: multipart 上传端点 (admin token 门) + `/admin` 知识库管理视图; 格式支持 md/pdf/docx/xlsx/pptx; 解析器注册表驱动 (扩展名+MIME 白名单, 零硬编码)
- **DOC-02**: 解析管线 — 格式按内容探测 (不仅扩展名); 解析 → 既有分块 → 向量化 → 索引入指定知识库; 解析失败 fail-closed
- **DOC-03**: 原始文件即弃 — 上传件仅存于内存/临时区, 处理完成即删; 持久化的只有解析文本与 chunk
- **DOC-04**: 上传防护配置化 — 单文件大小上限/并发上限/内容哈希去重策略全部配置驱动
- **DOC-05**: 入库即审计 — 每次上传审计行 (INSERT-only): 操作者/知识库/文件名/大小/hash/chunk 数/结果
- **DOC-06**: 上传即可见 — 成功上传后文档立即出现在知识图谱与检索 (无需重启); 失败错误明确回报 UI
- **Phase 26 耦合**: 上传目标 = 注册表中的知识库; Phase 26 `kb.Registry` 是本 phase 的前置依赖
- **Phase 28 耦合**: 上传文档须在图谱中可见 (GRF-01 节点 + GRF-02 关系边); Phase 27 落地后图谱须包含上传文档
- **路由分离**: Phase 15 `POST /api/import/*` 是 CSV 数据导入, 文档上传是 `POST /api/admin/kb/:slug/upload` (不同路径族, 不同 Content-Type 族, 不同 body-size 策略)
- **不落盘原则**: 原始上传字节永远不写磁盘 (D15-B1 content-hash bridge 精神 — `io.TeeReader` + `sha256` 在内存中完成; `import_api.go:16-29` multipart  streaming 模式是本 phase 的直接先例)
- **双后端兼容**: 索引入 Milvus **和** MongoDB; `RETRIEVER_BACKEND=mongo` 时 `knowledge.Index` 无法运行 (main.go:860 comment: "the milvus-coupled knowledge.Index cannot run here"); 新管线必须两条后端都能达

### Claude's Discretion
- 路由路径: `POST /api/admin/kb/:slug/upload` vs `POST /api/admin/upload/:slug` — 两种 REST 风格均可
- 分块策略: PDF per-page 切片 + markdown header splitter + recursive splitter 的具体参数 (chunk size/overlap)
- 内容哈希策略: SHA-256 全文哈希; 去重逻辑 (拒绝 vs 替换为新版)
- 并发策略: 单 slug 串行 vs 并发上传
- nextID 协调: audit 行携带 range + boot 时校准 vs retriever 层原子 MAX 查询

### Deferred Ideas (OUT OF SCOPE)
- 图谱动态化 + 关系边 → Phase 28
- 反馈落库/工单系统 → Phase 29
- 知识库级 RBAC/多租户 → v2
- LLM 实体关系抽取 → 明确 out of scope

---

## 1. eino-ext parser landscape (the locked v0.1.13 set)

> **CONFIRMED in go.mod:** `github.com/cloudwego/eino-ext/components/model/openai v0.1.13` 是唯一 `eino-ext` 间接依赖; `eino-ext` 本身 **不在 go.mod** (`go list -m github.com/cloudwego/eino-ext` 返回 "not a known dependency"). eino-ext 的 parser/splitter 子模块 **尚未加入项目依赖**.

**关键风险:** ROADMAP Phase 27 Notes 明确警告 "eino-ext document parsers 尚不在 go.mod 且内网网络间歇断 — plan-phase 必须把 parser 依赖获取当显式风险处理并预备纯 Go fallback 决策".

`go list -m -versions` 对 `eino-ext/components/document/parser` 和 `eino-ext/components/document/transformer/splitter/markdown` 和 `eino-ext/components/document/transformer/splitter/recursive` 的响应是 module path 本身 (无版本列表输出), 这表明这些是 **module path 而非可直接版本化的子模块** — eino-ext v0.1.13 是带 `eino-ext` 前缀的 monorepo tag, 子模块版本跟随主版本 tag.

| Package | Import Path (assumed) | Status | File Types | Limitations | Decision |
|---------|----------------------|--------|-------------|-------------|----------|
| PDF parser | `eino-ext/components/document/parser/pdf` | **NOT in go.mod** | PDF | Per-page text extraction; password-protected PDFs need separate handling | plan-phase must add `go get github.com/cloudwego/eino-ext/components/document/parser/pdf` — if CGO dep surfaces, document WSL-only constraint |
| DOCX parser | `eino-ext/components/document/parser/docx` | **NOT in go.mod** | DOCX | Sections/headers/footers/tables; no embedded images | same — plan-phase add + CGO check |
| HTML parser | `eino-ext/components/document/parser/html` | **NOT in go.mod** | HTML | CSS selector; may need content-only mode | same — plan-phase add + CGO check |
| Markdown splitter | `eino-ext/components/document/transformer/splitter/markdown` | **NOT in go.mod** | md | Hierarchical header splitting (H1/H2/H3) | same — plan-phase add; this is the natural partner for md file parsing |
| Recursive splitter | `eino-ext/components/document/transformer/splitter/recursive` | **NOT in go.mod** | Any (binary docs) | Configurable chunk size + overlap; for PDF/DOCX/XLSX/PPTX | same — plan-phase add |

**Critical verification required:** `go get github.com/cloudwego/eino-ext/components/document/parser/pdf@v0.1.13` + `go mod tidy` must succeed in the project network environment. If the intranet BGE/Milvus flakiness (memory: env-intranet-service-flakiness) also affects the Go proxy, this phase needs a `GOPROXY` fallback or vendor prep.

**CGO dependency check:** Run `go get github.com/cloudwego/eino-ext/components/document/parser/pdf@v0.1.13` on a clean machine and inspect `go list -m -f '{{.BuildConstraints}}'` output. If `cgo` appears, the parser is CGO-dependent and this phase is **WSL-only** for PDF/DOCX processing.

**Assumption tag:** All eino-ext parser/splitter availability claims in this section are `[ASSUMED]` — verified by `go list` module path resolution but not by actual `go get` + import in this session.

---

## 2. Format policy (the 5 user-named formats)

### md (Markdown)
- **Parser:** `eino-ext/components/document/parser/md` (assumed; markdown parser may be goldmark-based — `github.com/yuin/goldmark` already in go.mod:17)
- **Alternative (pure-Go fallback):** Use `goldmark` directly if eino-ext md parser is unavailable — goldmark is already a dependency
- **Chunker:** `markdown` splitter with hierarchical header splitting (`internal/knowledge/indexer.go:169 splitFrontmatter` strips YAML frontmatter; the body goes to embedding. For uploaded md, frontmatter may be absent — use the same `splitFrontmatter` pattern)
- **Edge cases:** Empty body (0-byte file) → skip silently; files exceeding `maxDocBytes` (4000 chars) → warn-and-skip (same as `knowledge.Index`, indexer.go:175-180)

### pdf (PDF)
- **Parser:** `eino-ext/components/document/parser/pdf` (assumed; if CGO or unavailable, use pure-Go fallback below)
- **Pure-Go fallback:** `github.com/unidoc/unipdf/v3` (community; CGO-free) or `github.com/ledongthuc/pdf` — requires verification at plan-phase
- **Parse strategy:** Per-page extraction → recursive splitter with page-level overlap. PDF pages are already chunk boundaries; recursive splitter adds sub-page chunking if pages are too large
- **Chunker:** `recursive` splitter (chunk size: 4000 chars, overlap: 200 chars — mirrors `maxDocBytes` cap)
- **Edge cases:** Password-protected PDFs → 400 with "password-protected PDFs are not supported"; embedded images → extract alt-text if available, otherwise skip with WARN; multi-column PDFs → per-page text extraction may interleave columns (known limitation, document in output)

### docx (Word)
- **Parser:** `eino-ext/components/document/parser/docx` (assumed; if unavailable, `github.com/nguyenthenguyen/docx` or `github.com/lukasjarvisch/docx` — pure-Go)
- **Parse strategy:** Section-based extraction (headings become chunk boundaries). Tables extracted as markdown tables in body.
- **Chunker:** `recursive` splitter (chunk size: 4000 chars, overlap: 200 chars)
- **Edge cases:** Track changes/comments → strip; password-protected → 400; embedded images → skip with WARN; footnotes/endnotes → include in body if extractable

### xlsx (Excel)
- **Parser:** `eino-ext` has no explicit xlsx parser in available docs; pure-Go fallback: `github.com/xuri/excelize/v2` (most popular pure-Go xlsx; 20k+ stars)
- **Parse strategy:** Sheet-by-sheet; each row becomes a text line; headers + row data concatenated as `"col1: val1 | col2: val2 | ..."`. Multi-sheet documents produce multiple chunks.
- **Chunker:** `recursive` splitter (chunk size: 4000 chars, overlap: 200 chars) applied to concatenated sheet text
- **Edge cases:** Password-protected → 400; macro-enabled `.xlsm` → reject (not `.xlsx`); empty sheets → skip; merged cells → expand to last-write-wins

### pptx (PowerPoint)
- **Parser:** `eino-ext` has no explicit pptx parser; pure-Go fallback: `github.com/unirust/golang pptx` or `github.com/nguyenthenguyen/docx` (docx library may handle pptx too); most reliable pure-Go: `github.com/partykedia-io/pptx` or `github.com/ledongthuc/pdf` is PDF-only
- **Parse strategy:** Slide-by-slide; each slide's title + content block becomes a text unit. Speaker notes extracted as separate text block (tagged with `[notes]` prefix).
- **Chunker:** `recursive` splitter (chunk size: 4000 chars, overlap: 200 chars)
- **Edge cases:** Password-protected → 400; embedded OLE objects → skip with WARN; media (images/video) → skip, log; animations → text only (logical content, not animated overlays)

---

## 3. Upload HTTP contract

### Streaming multipart (mirror Phase 15 import surface)

**Directly reuse `parseImportForm` + `multipart.NewReader` pattern from `internal/server/import_api.go:325-398`.** The Phase 15 import handlers (`importPreviewHandler`, `importCommitHandler`) use:
- `c.Request.BodyStream()` + `multipart.NewReader(c.Request.BodyStream(), boundary)` — never `c.FormFile` or `c.Request.MultipartForm` (T-15-02: those paths materialize parts on disk)
- `WithStreamBody(true)` + `WithDisablePreParseMultipartForm(true)` on the Hertz server (`import_api.go:21-29`)

**For Phase 27:** Apply the same pattern. The `parseImportForm` function is already a general-purpose streaming multipart reader; it should be extracted from `import_api.go` into a shared helper or re-implemented in `kb_admin.go`.

### Required form fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | multipart file part | **Yes** | The uploaded document (streaming, never disk) |
| `kb_slug` | text | **Yes** | Target knowledge base slug; must pass `kb.ValidateSlug` + `KBRegistry.Lookup` |
| `metadata` | JSON text | No | Optional key-value metadata: `{ "effective_from": 0, "effective_to": 0, "description": "..." }` |

**Slug validation chain:**
1. `kb.ValidateSlug(kb_slug)` → 400 with rule text on failure
2. `KBRegistry.Lookup(kb_slug)` → 404 if not found or deactivated

### Body-size policy

Mirror Phase 15 per-route split (`internal/server/server.go:322-329`):

| Route | Limit | Default | Via |
|-------|-------|---------|-----|
| `POST /api/admin/kb/:slug/upload` | `UploadMaxBodyBytes` | **100 MB** (planner decision) | `ImportMaxBodyBytes`-style field on `Deps` |
| All other routes | `NonImportMaxBodyBytes` | 8 MiB (already `NonImportMaxBodyDefault`) | existing body-size middleware |

**100 MB is the recommended default** — large enough for 100-page PDFs or multi-sheet Excel files, small enough to prevent trivial DoS. This is a **planner decision**: verify with `go.mod` ecosystem (100 MB is standard for document processing).

### Content-Type allowlist

**Enumerated, never inferred from extension:**

| Extension | MIME Type | Notes |
|-----------|-----------|-------|
| `.md` | `text/markdown` | [ASSUMED: eino-ext md parser accepts text/markdown] |
| `.pdf` | `application/pdf` | |
| `.docx` | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | |
| `.xlsx` | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | |
| `.pptx` | `application/vnd.openxmlformats-officedocument.presentationml.presentation` | |

**Detection logic:** Accept the `Content-Type` header; verify the file's magic bytes (first 4 bytes) match the expected signature for the declared type. If magic bytes mismatch → 400 with "content type does not match file signature". Never trust the filename extension.

**GBK/GB18030 rejection (T-15-12 lesson):** UTF-8 is enforced throughout. If a text-based upload (md) contains non-UTF-8 byte sequences, the YAML frontmatter parser (`yaml.Unmarshal` with `gopkg.in/yaml.v3`) will fail on non-UTF-8. This is acceptable — the document is malformed and should fail.

---

## 4. Indexing pipeline integration

### Reuse patterns from `internal/knowledge/indexer.go`

The new `UploadIndex` function (to be created in `internal/knowledge/upload.go`) shares the **exact same** three helpers already in `indexer.go`:

| Helper | Used by `Index` | To be used by `UploadIndex` |
|--------|----------------|---------------------------|
| `splitFrontmatter(raw)` | `indexer.go:169` | Yes — same YAML frontmatter strip for md uploads |
| `validateTiers(docs, reg)` | `indexer.go:229` | Yes — KNOW-02 fail-closed gate |
| `parseEffectiveWindow(fm)` | `indexer.go:206` | Yes — frontmatter `effective_from`/`effective_to` for policy windows |

### `UploadIndex(target_slug, file_bytes, emb, reg)` entry point

```
UploadIndex(ctx, slug string, fileBytes []byte, mimeType string, emb embedder.Embedder, reg kb.Registry, uploadAuditHook func(...)) error
```

**Steps:**
1. **Format detection** — `mimeType` argument (from Content-Type header, verified by magic bytes) drives parser selection. Unknown MIME → 400.
2. **Parse** — Call `eino-ext` parser or pure-Go fallback to extract text chunks. For md: call `splitFrontmatter` + embed body. For binary formats: per-page/per-section extraction.
3. **Frontmatter** — If present in parsed output, extract via `splitFrontmatter`; apply `parseEffectiveWindow`.
4. **Validate tiers** — Call `validateTiers(docs, reg)` BEFORE any embedder call (same backend-agnostic gate as `Index`, indexer.go:229).
5. **Embed** — For each chunk, call `emb.Embed(ctx, chunkText)`. Embed failure → fail-closed (never fall back to deterministic zero vector).
6. **nextID coordination** — See below.
7. **Insert** — Dispatch to Milvus `Insert` **or** Mongo `Insert` based on `RETRIEVER_BACKEND`. **Must NOT couple to a single backend** (ROADMAP Phase 27 risk: "上传管线不得继承 `knowledge.Index` 的 milvus-coupling").
8. **Audit** — Call `uploadAuditHook` with `action_doc_uploaded` + frozen args (file size, sha256, target slug, parser version, nextID range, chunk count).

### Backend dispatch without coupling to `RETRIEVER_BACKEND`

**Pattern:** Inject both `milvusRetriever` and `mongoRetriever` as `*retriever.Retriever` interface values (or a narrow `IndexerBackend` interface):

```go
// narrow interface — only what UploadIndex needs
type IndexerBackend interface {
    Insert(ctx context.Context, docs []retriever.Document) error
}
```

`cmd/server/main.go` assembles both `milvus.Client` and `mongo.Client` at boot; the `UploadIndex` call site picks the right one based on `RETRIEVER_BACKEND` env (read once at boot, passed in). This is already how `buildRetriever` works (main.go:386-390).

**Anti-pattern to avoid:** `if backend == "milvus" { milvus.Insert(...) } else { mongo.Insert(...) }` scattered through handler code. Use a narrow interface.

### nextID determinism

**Problem:** `Index` at boot walks `knowledge/<slug>/` directories and assigns chunk IDs 1..N in sorted order (indexer.go:138, `nextID := int64(1)`). `UploadIndex` runs at upload time, potentially before or after a full reindex. If both paths use sequential ID generation from 1, ID collisions are **guaranteed** on a re-seed after upload.

**Solution — audit row carries the range:**

Each `action_doc_uploaded` audit row records the assigned nextID range:

```go
type DocUploadAudit struct {
    Action        string   `json:"action"`
    Slug          string   `json:"slug"`
    FileName      string   `json:"file_name"`
    FileSize      int64    `json:"file_size"`
    SHA256        string   `json:"sha256"`
    ParserVersion string   `json:"parser_version"`
    ChunkCount    int      `json:"chunk_count"`
    IDRangeStart  int64    `json:"id_range_start"`  // first assigned ID
    IDRangeEnd    int64    `json:"id_range_end"`     // last assigned ID (inclusive)
}
```

**Re-seed coordination:** `knowledge.Index` (at boot) queries the **maximum ID** in the audit table for the target slug, then starts `nextID = maxID + 1`. This requires a `store.GetMaxChunkID(ctx, slug)` method on the audit store. If no prior upload audit row exists for the slug, `nextID = 1`.

**Concurrency:** If two uploads for the same slug race, the audit row's `id_range_*` is the authoritative record. Re-seed uses `MAX(id_range_end)` across all audit rows for the slug.

**Race condition (acceptable):** If the server crashes between `Insert` and `PutAudit`, the audit row is missing but the chunk IDs are in the backend. On re-seed, `MAX(audit)` will be lower than actual backend IDs → re-seed IDs will collide. Mitigation: `Insert` → audit write is synchronous in the same handler (fail the upload if audit write fails), so the gap cannot happen in normal operation. Server crash during `Insert` is a corrupted state requiring manual intervention (not solvable without distributed transactions).

---

## 5. Trust-contract surface

### KNOW-02: uploaded chunks carry target slug as Tier

`validateTiers(docs, reg)` is called BEFORE the embedder runs (indexer.go:291-303 pattern). For upload: `Tier = target_slug` from the URL parameter. If `target_slug` is unknown/deactivated → `validateTiers` returns error → 400, no embedder call, no chunk inserted. This is identical to the boot-seed path — the same fail-closed gate protects both.

**Citation whitelist unchanged:** `ChunkAccumulator.Add` (trust_floor.go:113) checks `kb.Role(slug) == authority` — uploaded chunks from non-authority slugs are already excluded from citation by the same gate. No new code path needed.

### D-09 refusal-by-construction: empty slug set = refusal preserved

If `retriever.Search` receives an empty `tiers` slice (should not happen from any valid code path since `kb.Registry.ActiveSlugs()` always returns at least the seed slugs), it returns `false` (milvus filter.go:29) or `$in:[]` (mongo.go:346). This is unchanged by uploads — no regression risk.

### Doc-view audit row

`action_doc_uploaded` is written with frozen args carrying the `DocUploadAudit` struct (above). This row is **INSERT-only** — never mutated, never deleted. It follows the same pattern as `action_kb_created` / `action_kb_deactivated` (kb_admin.go:230, 282).

**Frozen args example:**
```json
{
  "slug": "authority",
  "file_name": "travel_expense_policy.pdf",
  "file_size": 1048576,
  "sha256": "a3f5...c9d",
  "parser_version": "eino-ext/pdf@v0.1.13",
  "chunk_count": 12,
  "id_range_start": 47,
  "id_range_end": 58
}
```

### File-on-disk: NEVER persist original bytes

**Pattern from `internal/server/import_api.go:21-29`:** The Phase 15 import surface uses `multipart.NewReader` over `c.Request.BodyStream()` — bytes flow directly from the network socket through the parser without ever touching disk.

For Phase 27: Same pattern. In the `onFile` callback, read `io.Reader` directly into a `bytes.Buffer` (in-memory, bounded by `UploadMaxBodyBytes`). Compute `sha256` via `hash.SHA256()` on the buffer. Then parse the buffer. **Never `os.WriteFile` the original bytes.**

The `sha256` hash is computed in-memory and written into the audit row. The hash serves as:
1. **Content-identity deduplication key** (DOC-04: same hash = same content = reject or replace)
2. **Audit evidence** (what file produced which chunks)

---

## 6. FE upload surface

### Extend `KnowledgeBaseList.tsx` vs new `KnowledgeUploadPanel.tsx`

**Recommendation: New `KnowledgeUploadPanel.tsx` as a sub-panel within the `/admin` KB management tab** — not a separate tab, but a "Upload Document" expandable section at the bottom of the KB list, mirroring the "Register New Knowledge Base" form in `KnowledgeBaseList.tsx:291-360`.

**Rationale:** The KB admin view already has the slug list + create form. Adding an upload section keeps all KB management in one view. Separating into a new tab would require a new route + nav entry for a single sub-operation.

**Form layout:**
```tsx
// State: idle | uploading | parsing | indexing | done | error
// Fields: target KB dropdown (active slugs from GET /api/knowledge/bases), file picker (accept=".md,.pdf,.docx,.xlsx,.pptx"), optional metadata accordion
// Submit: POST /api/admin/kb/:slug/upload (streaming XHR for progress)
// Post-success: refresh admin KB list (to show updated chunk count for the slug)
```

**States:**
| State | UI | Behavior |
|-------|----|----------|
| idle | "上传文档" button | |
| uploading | progress bar (XHR `upload.onprogress`) | streaming multipart POST |
| parsing | "正在解析文档..." spinner | handler processing |
| indexing | "正在索引入库..." spinner | embed + insert |
| done | green toast: "N 个片段已入库" | auto-refresh KB list |
| error | red toast with backend error message | stays on form |

**`GET /api/knowledge/bases` (public projection):** The upload dropdown lists active KB slugs via the **public** `GET /api/knowledge/bases` endpoint (no token required — slug list is non-sensitive). This is the same endpoint the chat FE uses for tier badges.

**Post-success refresh:** After a successful upload, the admin KB list should re-fetch via `GET /api/admin/kb` to update `chunk_count` (if that field is added) or simply confirm the upload by showing the updated table state.

---

## 7. Risks & Unknowns

### Risk R1: eino-ext v0.1.13 parser availability
**Severity: HIGH** — All five formats depend on `eino-ext` submodules not currently in `go.mod`. `go list -m -versions` returned the module path without version tags, suggesting these are subpaths of the `eino-ext` monorepo tagged at the repo root. If `go get github.com/cloudwego/eino-ext/components/document/parser/pdf@v0.1.13` fails (intranet proxy, corporate firewall, or CGO dependency), this phase needs a fallback plan: pure-Go parsers (`excelize` for xlsx, `goldmark` for md, `unidoc` for PDF if available) must be evaluated **before plan-phase commits to the eino-ext path**.

**Mitigation:** Reserve a `planner` task to run `go get` for each parser submodule in a clean GOPROXY environment. If any fail, switch to pure-Go fallback and document the library choice.

### Risk R2: CGO parser dependencies
**Severity: MEDIUM** — If `eino-ext/components/document/parser/pdf` or `docx` requires CGO (native bindings), the entire upload pipeline becomes WSL-only on Windows. The `go.mod` already has `mattn/go-sqlite3` (CGO) as a required dependency for the store, so the server build is already WSL-dependent. If the PDF/DOCX parsers are also CGO, the constraint is consistent. **Document this clearly in the plan so the WSL-only constraint is explicit, not a surprise.**

### Risk R3: Body-size limit 100 MB
**Severity: LOW** — `server.go:70-71` shows `ImportMaxBodyDefault = 2 GiB` for the import pipeline. The document upload pipeline is different (binary docs, not CSV streams). 100 MB is a reasonable guess for PDFs/Excel files. If enterprise users need to upload >100 MB documents (e.g., 500-page compliance PDFs), this limit should be configurable. **Planner decision required.**

### Risk R4: nextID race between concurrent uploads
**Severity: MEDIUM** — If two admins upload to the same slug simultaneously, the audit rows' `id_range_*` could be written in inconsistent order if the handler doesn't serialize. Recommended: a per-slug mutex or single-flight pattern (only for the same slug — different slugs can be concurrent). This adds a `sync.Map` of per-slug mutexes to the upload handler.

**Simpler alternative (acceptable):** Accept that concurrent uploads to the same slug interleave their chunk IDs. The audit row records the actual range; re-seed recomputes MAX from audit. Chunk ID interleaving is cosmetic (citation labels may be non-contiguous), not a functional bug.

### Risk R5: `retrieval_enabled=false` slug upload
**Severity: LOW** — If an admin uploads to a deactivated KB slug (or one with `retrieval_enabled=false`), the chunks will be inserted into the vector backend. The deactivated KB will not appear in `ActiveSlugs()`, so `validateTiers` will **fail-closed** and reject the upload (the slug is not in `ActiveSlugs()` → `validateTiers` error). This is correct behavior — upload to a deactivated KB should be blocked.

**Edge case:** A deactivated KB is re-activated later — the previously rejected uploads (from when it was deactivated) are not stored; the admin must re-upload after re-activation. This is correct.

### Risk R6: Auto-reindex side effect on active Skills
**Severity: LOW** — The Phase 26 `skill.Validate` checks `reads:` declarations against `kb.Registry`. Adding new chunks to a KB slug does NOT change the Skill's `reads:` contract. No Skill reload is triggered by an upload. The new chunks are immediately visible to any Skill that declares the slug in `reads:` — this is the intended behavior (DOC-06: "立即可见").

### Unknown U1: Chunk count display in admin KB list
**The current `GET /api/admin/kb` response (kb_admin.go:136-157) does not include chunk count per KB.** DOC-06 requires "上传后文档立即出现在知识图谱与检索 (无需重启)". The graph (Phase 28) will show per-slug chunk counts dynamically. The admin KB list should also show chunk count, but this requires either:
1. A new `GET /api/admin/kb/chunk-counts` endpoint that queries Milvus/Mongo for per-slug counts
2. A `ListMetadata` call on the retriever

**Recommendation:** Add `chunk_count` to the admin KB list as a Phase 28 enhancement (the graph query will be the primary visibility mechanism). For Phase 27, a post-upload toast confirmation ("N chunks indexed") is sufficient for the success visibility requirement.

---

## 8. Open Questions for the Planner

1. **File size limit:** 100 MB default for `UploadMaxBodyBytes` — confirm or adjust? Should it be configurable via env var (`SKILLFORGE_UPLOAD_MAX_BYTES`)?

2. **Upload to `retrieval_enabled=false` slug:** Should upload to a deactivated KB be rejected (fail-closed, current recommendation) or should chunks be written but hidden from retrieval (deferred visibility on re-activation)?

3. **Temporary buffer strategy:** `bytes.Buffer` in memory for the uploaded file (max 100 MB). Is this acceptable vs streaming directly to the parser without a full buffer? The sha256 hash requires either (a) a full buffer or (b) a `io.TeeReader` that hashes while streaming to the parser. Option (b) is more memory-efficient but requires the parser to support streaming from an `io.Reader`. **Recommendation: bytes.Buffer for simplicity; 100 MB max is bounded.**

4. **FE: extension `KnowledgeBaseList.tsx` or new `KnowledgeUploadPanel.tsx`?** Both approaches work. The decision affects the scope of Phase 27's FE work. New panel keeps file size smaller per commit; extension keeps related logic together.

5. **Concurrent uploads per slug:** Should the same slug accept simultaneous uploads (simpler, ID ranges may interleave) or serialize them (cleaner IDs, more complex synchronization)? Interleaved IDs are a cosmetic issue only; serialized is safer for operator understanding of "how many chunks are mine".

---

## Sources

### Primary (HIGH confidence — codebase-grounded)
- `internal/knowledge/indexer.go` — `splitFrontmatter`, `validateTiers`, `parseEffectiveWindow`, `seedDocs`, `indexSeedHook` (all patterns to be reused)
- `internal/knowledge/walk.go` — `WalkRegisteredDirs` three-state decision table, `DirAuditFn` callback (26-RESEARCH Pattern R6)
- `internal/server/kb_admin.go` — admin token gate (`isAdminRequest`), `kbAdminEntry` wire shape, `putKBAdminAuditRow` pattern (26-RESEARCH Pattern R4)
- `internal/server/import_api.go:16-29` — streaming multipart pattern (multipart.NewReader over BodyStream; WithStreamBody + WithDisablePreParseMultipartForm)
- `internal/server/import_api_test.go:190-235` — `TestImportStreaming` proves no temp-file materialization for large uploads
- `internal/store/sqlite/audit.go` — `PutAudit` INSERT-only pattern; `ActionBudgetTriggered`/`ActionFanoutDispatched` enum expansion pattern for new action strings
- `cmd/server/main.go:386-870` — `RETRIEVER_BACKEND` switch; `buildRetriever` function; Mongo path comment "the milvus-coupled knowledge.Index cannot run here" (860)
- `cmd/server/main.go:70-73` — `ImportMaxBodyDefault = 2 GiB`; `NonImportMaxBodyDefault = 8 MiB`
- `web/src/components/admin/KnowledgeBaseList.tsx` — FE form pattern, wire interface, `X-Admin-Token` header pattern, KB slug regex (26-08)
- `internal/retriever/retriever.go:70-78` — `Document` struct (Tier/Content/Source/Embedding/EffectiveFrom/EffectiveTo)
- `go.mod:7` — `github.com/cloudwego/eino-ext/components/model/openai v0.1.13` (the locked eino-ext version); no other eino-ext document parser in `go.mod`

### Secondary (MEDIUM confidence — ecosystem documentation)
- `go list -m -versions` output for eino-ext submodules (module path resolution confirmed; version availability unverified due to intranet proxy)
- Phase 15 `parseImportForm` multipart streaming pattern (import_api.go — fully verified in tests)
- Phase 26 `kb.Registry` interface + `validateTiers` + `validateEffective` patterns (26-RESEARCH verified)

### Tertiary (LOW confidence — architectural reasoning)
- eino-ext parser import paths (assumed from standard CloudWeGo eino-ext package naming convention; not verified by `go get` in this session)
- CGO dependency status of PDF/DOCX parsers (unverified — requires actual `go get` + `go list -f '{{.BuildConstraints}}'` on a clean machine)
- Pure-Go fallback library choices for xlsx/pptx (researched via ecosystem knowledge; not verified in project context)

---

## Metadata

**Confidence breakdown:**
- Parser landscape: MEDIUM — eino-ext submodules confirmed via `go list` module paths; actual availability at v0.1.13 requires `go get` verification
- Format policy: MEDIUM — pure-Go fallback choices are ecosystem-reasonable but unverified in project
- HTTP contract: HIGH — Phase 15 pattern fully verified; body-size constants are sound
- Indexing pipeline: HIGH — reuse patterns are exact code-level matches
- Trust contract: HIGH — all gates are existing code paths with known behavior
- FE surface: MEDIUM — form pattern is exact; XHR streaming for progress is a design decision

**Research date:** 2026-09-07
**Valid until:** 2026-10-07 (stable — eino-ext v0.1.13 is locked; parser API surface unlikely to change)
