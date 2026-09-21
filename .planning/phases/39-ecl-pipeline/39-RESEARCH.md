# Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction - Research

**Researched:** 2026-09-21
**Domain:** Go code productionization — corpus re-chunking pipeline (walk/merge), LLM entity extraction → RDF ABox, Milvus schema migration, admin polling surface
**Confidence:** HIGH (all canonical refs verified in-repo on 2026-09-21; no new external libraries)

## Summary

Phase 39 is almost entirely **productionization of verified in-repo assets**, not greenfield library integration. The 005c spike (`VALIDATED`) already proves the 185→26 section-aware merge with 100% boundary detection and carries a ready `ChunkMeta` contract; Phase 38 delivered the ontology stack (emitter / memory-map store / induction / admin handlers) whose APIs are the integration surface; the LLM call pattern (temperature-0 + `ExtractJSON` + `response_format:json_object`) exists twice in-repo. Zero new Go or npm dependencies are needed — the only "new" third-party import is `eino-ext` `splitter/recursive`, already present in `go.mod` (indirect), for sub-splitting oversized merged sections.

Research surfaced **four load-bearing design problems the planner must resolve explicitly** (none is a blocker; all have in-repo precedent):

1. **Milvus purge-scope conflict (highest risk).** The replace-by-tier purge (`milvus.DeleteTierRows`, tier-level expr) is incompatible with a *mixed* tier: when ECL is on, the `sql-guides` tier holds BOTH legacy-path rows (17 non-`__secNN` files) AND ECL rows (merged-section chunks). Boot's tier-level purge would wipe ECL rows; ECL's tier-level purge would wipe legacy rows. Recommendation: ownership-marker purge (`section_count > 0` = ECL-owned) via a narrow `DeleteByExpr` helper.
2. **Oversized merged sections (silent data loss).** `chunk_metadata.json` proves merged section bodies reach **93,993 bytes (note 039 SQL section ≈ 88K chars)** — 14 of 69 sections exceed 8192 bytes. The `content` VARCHAR column caps at 8192 **runes** (`truncateRunes` silently truncates) and BGE-M3's context is 8192 tokens. Direct section-level seeding would truncate SQL and violate SC-1's SQL-completeness claim. Recommendation: keep ChunkMeta per section, but sub-split any section body over a ~3000–4000-rune threshold with the recursive splitter; all parts share the section metadata.
3. **ABox durability: boot does NOT restore the KG snapshot.** `cmd/server/main.go:894-906` builds the KG from the TBox TTL pointer only and **overwrites** `skillforge_kg.json` at boot. Phase 38 got away with this because KG content == TBox content. With ECL ABox triples (which only exist after a manual Run, per D-Run-01), every server restart would silently wipe them. The plan must implement the D-KG-02 restore order (LoadSnapshot → TBox merge with dedupe → snapshot) — and note `store.Insert` does **not** dedupe (`store.go:67-68`).
4. **TBox pointer blocks the 6→8 class extension.** Boot only emits the TBox when the pointer file is **missing** (`main.go:877`). An environment that already booted Phase 38 keeps its 6-class v1 TTL forever, so `skf:NoteChunk` / `skf:TableSchema` would be undeclared. The plan needs a one-time upgrade trigger (recommend: pointer lacks `NoteChunk` → emit v2 + bump `tbox_version.txt` + replay any approved induction triples).

The admin/API/UI surface is low-risk: the Phase 38 `admin_ontology.go` gate (403-before-503), route table, and the approved `39-UI-SPEC.md` (9th tab, polling 2s/10s, verbatim copy contract) leave almost nothing open.

**Primary recommendation:** Build `internal/knowledge/ecl/` as a self-contained orchestrator (merge → extract → kg-load → milvus-load) with a narrow `Extractor` interface (LLM + deterministic stub), wire it into `Index` as a routing branch plus marker-scoped purge, fix the boot KG-restore order, and let the planner resolve the four problems above as explicit plan tasks.

## User Constraints (from CONTEXT.md)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-ECL-01 (LOCKED):** 新建独立 `internal/knowledge/ecl/` 包 (walk + merge + 抽取编排)；`indexer.go` 只加路由分支——文件名含 `__secNN` 且 frontmatter 有 `section` 字段走 ECL 路径，否则走现有 Phase 2 逻辑 (authority 等其他 KB **零变化**)。env 开关 `SKILLFORGE_ECL_ENABLED` (默认 on) 可整体回退老路径。
- **D-ECL-02:** 005c 算法原样生产化: filename regex `__sec(\d{2})_(.+)\.md$` + frontmatter `section` 分组 + first-seen 顺序; `ChunkMeta` 契约 (source_idx / title / section / section_idx / section_count / source_paths / body_bytes) 与 spike `chunk_metadata.json` 一一对应。
- **D-Milvus-01 (LOCKED):** `internal/retriever/milvus/seed.go` schema 加 4 个**原生字段**: `source_idx` (VarChar) / `section` (VarChar) / `section_idx` (Int64) / `section_count` (Int64)。部署时 `SKILLFORGE_MILVUS_RECREATE=1` 一次性重建 + 全量 re-embed (202 文件量级; reseed 幂等已由 replace-by-tier purge 保证, daf98d1)。**不用** source 字段塞 JSON (破坏 filter 能力，Phase 41 hybrid 检索需要原生字段)。
- **D-Milvus-02:** 老 KB (无 section 元数据) 的字段填零值 (`source_idx=""` / `section_idx=0` / `section_count=0`)，检索 filter 语义不变。
- **D-Run-01 (LOCKED):** ECL 由 **admin 手动触发** `POST /api/admin/pipeline/run` (token 门控) + FE「Run ECL」按钮。boot **不**自动跑 (内网 LLM 间歇断——自动跑会在断线时把 boot 炸进重试风暴)。重复 run 幂等: KG 按 note 级 replace (先删该 note 旧 triples 再插新)。
- **D-GT-01 (LOCKED):** ground truth = **regex 起草 + 人工 review**: 确定性 regex 抽全部 26 notes 的表引用 + note 间关系，人工 review 产出 `eval/ecl-golden/sql-guides.jsonl`。**LLM 永不参与标注集构造**。verify-phase-39.sh 用它算 F1。
- **D-TBox2-01 (LOCKED):** ABox schema 走 **emitter 硬编码扩展 6→8 类**: + `skf:NoteChunk` + `skf:TableSchema` + 对应 ObjectProperty (`skf:belongsToNote` / `skf:referencesTable`)。boot 可重现、确定性; induction approve 闭环留给 Phase 40+。
- **D-Ext-01:** LLM 抽取 prompt schema 固定 JSON: `{tables: [{name, kind: "update|select|insert|create"}], related_notes: [{source_idx, relation}]}` per merged note; temperature 0; 解析走 ExtractJSON 防御路径。抽出的 triples 形态: `skf:note_{source_idx} a skf:NoteChunk` / `skf:note_{source_idx} skf:referencesTable skf:tbl_{name}` / TableSchema individuals 去重全局共享。
- **D-DL-01 (LOCKED):** dead-letter 存 **SQLite 新表 `ecl_failures`** (note_id / stage / attempt_count / last_error / status `pending|dead|resolved`)，复用现有 store migration + INSERT-only 模式; 重启后 dead 记录仍在。手动重触发: `POST /api/admin/pipeline/retry/:note_id`。
- **D-RT-01:** 每段 (extract / kg-load / milvus-load) 失败自动 retry 3 次 (指数退避)，3 次后置 `status=dead` 并继续处理其余 notes (单 note 失败不阻塞整批)。
- **D-Prog-01 (LOCKED):** 进度走**轮询** `GET /api/admin/pipeline/status` → `{stage, current_file, done, total, queue_len, failures[]}`；FE 2s 轮询 (run 进行中) / 10s (空闲)。
- **D-Route-01:** 新增 API: `POST /api/admin/pipeline/run` / `GET /api/admin/pipeline/status` / `POST /api/admin/pipeline/retry/:note_id`。全部 admin token 门控 (403 before 503)。
- **D-Boot-01:** ABox KG 写入复用 Phase 38 `store.Insert` + debounce snapshot (SetOnChange 同款 hook 模式)；note 级 replace 需要给 store 加 `DeleteWhere(pred, obj)` 辅助 (或等价删除原语) — planner 定具体 API。

### Claude's Discretion
- ECL stage 编排的并发度 (26 notes 串行或小并发; LLM 调用建议 ≤3 并发)
- ecl_failures 表 DDL 细节 + migration 挂载点 (跟随现有 store 模式)
- run 取消机制 (cancel API) 是否纳入本 phase
- retry 退避的具体间隔 (e.g. 1s/4s/16s)
- KG 删除原语的具体签名
- 标注集 review 流程的操作方式 (一次性 checklist 文档 vs 分批)
- UI-SPEC 后的 `/admin/pipeline` 视觉细节 (交给 ui-phase — **已交付**: 39-UI-SPEC.md approved 2026-09-21)

### Deferred Ideas (OUT OF SCOPE)
- Phase 40: SHACL 校验 + Provenance + TBox 版本演进日志 + rollback UX; induction approve 闭环实战使用
- Phase 41: MCP server + SPARQL endpoint + Hybrid Retrieval; 按 section filter 的 Milvus 检索
- 本 phase 明确 out-of-scope: SQL 语句级 boundary 检测; sec01 section==title 病理修复; 非 sql-guides KB 的 ECL 化; ECL run 的 cancel API
</user_constraints>

## Phase Requirements

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ONT-10 | Replace byte-cap chunker with section-aware markdown chunker for sql-guides; chunks grouped by `source_idx` + `section` frontmatter | 005c spike algorithm (main.go:57-199) verified; routing branch design (Pitfall 1, Pattern 2); ChunkMeta contract (spike main.go:264-284); corpus verified: 185 `__secNN` fragments + 17 non-sec files in 7 subdirs |
| ONT-11 | 100% boundary detection 185 fragments → 26 logical notes; preserves complete SQL across sec boundaries | Spike VALIDATED (README: 185 fragment-contributions, 26 notes, note 001 3/3 etl_* tables); hermetic Go test against in-repo corpus is the verification vehicle; oversized-section sub-split must NOT break the merge-level boundary count (Pitfall 2) |
| ONT-12 | LLM-driven entity extraction on merged notes (etl_* table refs + relationships → ABox triples) | Extraction harness pattern verified (induction.Generate L128-188 + llm.ExtractJSON L83 + WrapImplSpecificOptFn response-format L185); D-Ext-01 prompt schema; stub-Extractor seam; golden-set F1 harness (eval/golden/*.jsonl convention) |
| ONT-13 | ECL writes ABox triples to memory-map KG + indexes chunks to Milvus with consistent metadata (tier / source_idx / section / section_idx) | store.Insert + SnapshotDebounced verified (store.go:69/199); D-KG-03 debounce already anticipates ECL; Milvus schema extension points verified (seed.go:70-88, retriever.go:70-78); **boot snapshot-restore gap found and must be fixed** (Pitfall 3); marker-scoped purge design (Pattern 3) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Tech stack (frozen):** Go + Eino + Eino ADK + Hertz backend; React 19 + Vite + TS + Vercel AI SDK + shadcn/Tailwind frontend. No new frameworks; this phase needs zero new deps.
- **License:** Apache-2.0 — every new file carries the SPDX header block (all in-repo files follow this; see any file in `internal/retriever/milvus/`).
- **GSD workflow:** repo edits only through GSD commands (this research is part of that flow).
- **Env runbooks (project memory):** backend builds/runs ONLY under WSL Go (`go1.25.0` verified); Go does not read `.env`; `.env` CRLF trap. Milvus/BGE/GLM are intranet services with intermittent outages — probe before live runs, never let LLM outage crash boot (D-Run-01 encodes this). bigmodel rejects `[1M]`-suffixed model codes — do not introduce model-name changes.
- **Zero-hardcode standard (v1.2, referenced in milvus/config.go:46-51):** schema widths derive from a single home constant (e.g. `kb.MaxSlugLen`); new VarChar widths should follow the same single-source pattern.
- **INSERT-only discipline** applies to *audit* tables; `ecl_failures` is a mutable state table (status transitions) — CONTEXT explicitly rules this is not an audit table, so UPDATE-on-status is sanctioned.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Section-aware walk + merge | Backend (Go, `internal/knowledge/ecl`) | — | Filesystem corpus transform; pure Go, hermetically testable |
| Routing branch (`__secNN` → ECL) | Backend (Go, `internal/knowledge/indexer.go`) | — | Index owns per-file disposition; other KBs must be untouched |
| LLM entity extraction | Backend (Go, `internal/knowledge/ecl` via chat model) | — | Same `cm` ChatModel instance as induction/redline (main.go:907); server-side only |
| ABox triples → memory-map KG | Backend (Go, `internal/ontology/store`) | — | Phase 38 in-memory store + debounced JSON snapshot owns KG state |
| Merged chunks → Milvus (+4 fields) | Database/Storage (Milvus) | Backend (`internal/retriever/milvus/seed.go`) | Schema + column writers live in the milvus package; ECL only produces `[]retriever.Document` |
| Failure ledger (`ecl_failures`) | Database/Storage (SQLite) | Backend (`internal/store/sqlite/schema.go`) | Follows the store-owned CREATE TABLE IF NOT EXISTS migration pattern |
| Run/status/retry admin API | API/Backend (Hertz, `internal/server/admin_pipeline.go`) | — | Phase 38 admin handler gate pattern (403 → 503, 409 for double-run) |
| Pipeline status polling UI | Browser/Client (React, `web/src/components/admin/pipeline/`) | API/Backend (status endpoint) | D-Prog-01 polling; UI-SPEC locks every visual/interaction detail |
| TBox 6→8 classes | Backend (Go, `internal/ontology/emitter`) | — | Hardcoded emitter lists are the single schema source (D-TBox2-01) |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib (`regexp`, `os/filepath`, `sort`, `encoding/json`) | go1.25.0 (WSL) | walk/merge/serialize | 005c spike uses exactly this; zero-dep merge [VERIFIED: repo spike + wsl go version] |
| `gopkg.in/yaml.v3` | in go.mod | frontmatter parse | Spike-proven on CJK section names (005c gotcha #3) [VERIFIED: go.mod + spike README] |
| `milvus-sdk-go/v2` | v2.4.2 (go.mod:14) | schema + purge + seed | Existing collection owner; `entity.NewField().WithMaxLength`, `NewColumnVarChar`, `mc.Delete(expr)` all proven in-repo [VERIFIED: seed.go/filter.go/config.go] |
| `cloudwego/eino` `model` options | v0.9.14 (go.mod:6) | temperature-0 + `WrapImplSpecificOptFn` response_format | Locked eino release; the option surface is documented at llm/extract.go:173-192 [VERIFIED: go.mod + extract.go] |
| `eino-ext .../splitter/recursive` | v0.0.0-20260904 (go.mod:41, indirect) | Sub-split oversized merged sections | Already in dep tree; the CLAUDE.md blessed chunker for non-hierarchical text [VERIFIED: go.mod] |
| `stretchr/testify` | v1.11.1 (go.mod:16) | Go test assertions | Project test standard [VERIFIED: go.mod] |
| vitest + @testing-library | vitest ^2.0.0 (web/package.json) | PipelineTab tests | Existing admin FE test stack (37 admin tests pass in Phase 38) [VERIFIED: package.json + 38-VERIFICATION.md] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/llm.ExtractJSON` | in-repo | Defensive LLM response parsing | Every extraction call (D-Ext-01) [VERIFIED: extract.go:83] |
| `internal/ontology/store` (Insert/Query/DeleteWhere-new/LoadSnapshot/SnapshotDebounced) | in-repo | KG state | D-Boot-01; delete primitive is new this phase [VERIFIED: store.go] |
| `MilvusClient.GetMaxID(ctx, tier)` | in-repo (client.go:88-122) | Chunk ID allocation for ECL inserts | Avoids PK-generation collision with boot-seeded rows — but see Pitfall 8 (per-tier max is not collection-wide-safe) [VERIFIED: client.go] |
| `scripts/verify-phase-38.sh` | in-repo | Template for verify-phase-39.sh (hermetic SC-stage structure, WSL header, ok/fail counters) | [VERIFIED: scripts/verify-phase-38.sh] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Marker-scoped purge (`section_count > 0` expr via new `DeleteByExpr`) | Change `DeleteTierRows` signature to take an extra condition | New helper keeps the 4 existing DeleteTierRows call sites + tests untouched; signature change ripples through indexer tests. **Recommend the helper.** |
| Recursive splitter for oversized sections | Hand-rolled SQL-statement splitter | SQL-boundary detection is explicitly out of scope (CONTEXT §deferred); recursive splitter is already in the tree. **Recommend splitter.** |
| Extend big `store.Store` interface with ecl_failures methods | Narrow `ecl.FailureStore` interface (sqlite implements; tests fake it) | Matches the `retriever.IndexerBackend` narrow-interface precedent (backend.go:23-33) and avoids breaking server test fakes. **Recommend narrow interface** with DDL still in schema.go. |
| `eval/ecl-golden/sql-guides.jsonl` (CONTEXT path) | `eval/golden/ecl-sql-guides.jsonl` (existing convention — `eval/golden/` holds compliance.jsonl, hitl.jsonl, kpi.jsonl…) | D-GT-01 names the path in a locked decision; flag the convention mismatch to the user in discuss/plan rather than silently deviating. Default: keep the CONTEXT path. |

**Installation:** None. `go.mod` / `package.json` unchanged (splitter/recursive already an indirect require; promoting it to a direct import adds no new module).

## Package Legitimacy Audit

No new external packages are installed by this phase (backend: stdlib + existing go.mod modules; frontend: zero new npm deps per 39-UI-SPEC §Design System). Slopcheck gate not applicable — nothing enters the registry supply chain. The one new *direct import* (`eino-ext splitter/recursive`) is an existing `go.mod` entry published under the CloudWeGo/eino-ext org, already resolved into the module graph by current builds.

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                        admin (browser /admin/pipeline, 9th tab)
                              │  POST /api/admin/pipeline/run      (X-Admin-Token)
                              │  GET  /api/admin/pipeline/status   (poll 2s/10s)
                              │  POST /api/admin/pipeline/retry/:note_id
                              ▼
                    ┌──────────────────────────┐
                    │  admin_pipeline.go gate  │  requireAdmin → 403
                    │  (Deps.ECLManager nil →  │  nil/ECL-off → 503
                    │   503; running → 409)    │
                    └───────────┬──────────────┘
                                ▼
                    ┌──────────────────────────┐        in-memory status
                    │  ecl.Manager (singleton) │────── {stage,current_file,done,total,queue_len}
                    │  stage orchestrator      │
                    └───┬───────┬───────┬──────┘
        merge           │       │       │      milvus-load
   ┌────────────────────┘       │       └───────────────────────┐
   ▼                            ▼                               ▼
┌──────────────┐      ┌──────────────────┐           ┌─────────────────────────┐
│ walk+merge   │      │ extract (per     │           │ purge ECL-owned rows     │
│ 185 __secNN  │      │ note, ≤3 conc.)  │           │ (section_count>0 marker) │
│ → 26 notes   │      │ LLM or stub      │           │ + SeedDocuments          │
│ 69 sections  │      │ ExtractJSON      │           │ (+4 new columns)         │
└──────┬───────┘      └───────┬──────────┘           └───────────┬─────────────┘
       │ per-note retry ×3 → ecl_failures (SQLite)               │ re-embed via
       │                      │ ABox triples                     │ embedder.Embedder (BGE-M3)
       ▼                      ▼                                  ▼
  knowledge/sql-guides  ┌──────────────────┐            Milvus skillforge collection
  (read-only corpus)    │ ontology store   │            (schema: RECREATE=1 one-shot)
                        │ Insert +         │
                        │ DeleteWhere(note)│←── note-level replace (D-Run-01 idempotency)
                        │ → debounce 5s    │
                        │   snapshot       │
                        └──────────────────┘
   Boot (separate path): TBox emitter (6→8 classes, pointer-upgrade check)
                         → KG LoadSnapshot-first restore fix → knowledge.Index
                         (routing branch: __secNN skipped from legacy seed when ECL on;
                          marker-scoped purge for sql-guides)
```

### Recommended Project Structure
```
internal/knowledge/ecl/
├── merge.go            # walk (secRegex) + frontmatter parse + group + sort + merge (005c port)
├── merge_test.go       # 185/185 boundary + 26 notes + note 001 3/3 tables (real corpus, hermetic)
├── meta.go             # ChunkMeta contract (spike main.go:264-284 verbatim fields)
├── chunk.go            # section → []retriever.Document with sub-split of oversized sections
├── extract.go          # Extractor interface + LLM impl (prompt D-Ext-01, temp 0, ExtractJSON) + regex stub
├── extract_test.go     # stub-driven F1 harness vs golden jsonl
├── f1.go               # set-based precision/recall/F1 vs golden jsonl (pure function)
├── manager.go          # stage orchestrator: run/status/retry, retry ×3 backoff, dead-letter
├── manager_test.go     # dead-letter flow, idempotent re-run, single-note isolation
├── abox.go             # triples build (note_N / tbl_NAME), note-level DeleteWhere+Insert
└── failures.go         # narrow FailureStore interface (sqlite impl binds existing store)
internal/server/
└── admin_pipeline.go   # 3 handlers + requireECL gate (admin_ontology.go pattern)
internal/store/sqlite/
└── schema.go           # + ecl_failures DDL (schemaSQL, idempotent)
internal/retriever/milvus/
├── seed.go             # schema +4 fields; SeedDocuments +4 columns; retriever.Document +4 fields
└── delete_expr.go      # DeleteByExpr helper (marker-scoped purge)
web/src/components/admin/pipeline/
├── PipelineTab.tsx     # per 39-UI-SPEC §3
├── pipelineApi.ts      # 3 calls, X-Admin-Token from localStorage sf_admin_token
└── types.ts            # PipelineStatus / PipelineFailure (UI-SPEC §2 shape)
```

### Pattern 1: Routing branch inside Index (D-ECL-01)
**What:** One decision point per file, BEFORE the `maxDocBytes` guard (ECL fragments/sections routinely exceed 4000 bytes — ordering matters).
**When to use:** The per-file loop in `Index` (indexer.go:211).
**Example:**
```go
// Inside Index's per-file loop, after splitFrontmatter:
if eclEnabled && ecl.IsSecFragment(filepath.Base(path), fm) {
    continue // ECL owns this file; excluded from the legacy doc set AND
             // from the legacy purge scope (see Pattern 3)
}
// existing Phase 2 path: maxDocBytes check → embed → append (unchanged)
```
`IsSecFragment` = `__sec(\d{2})_(.+)\.md$` on the base name AND a non-empty `section` key in frontmatter (verified: non-sec file `030_承保清单.md` has `title`+`source_idx` but no `section`; sec files carry all three) [VERIFIED: file samples read 2026-09-21].

### Pattern 2: ECL Manager state machine + admin gate
**What:** A singleton manager with a CAS "running" flag; status struct read by the poll endpoint; per-note failures persisted to `ecl_failures`.
**When to use:** `POST run` (409 if running), `GET status`, `POST retry/:note_id`.
**Wire shape:** `server.Deps` gains a field like `ECL *ecl.Manager` (mirrors `Deps.Ontology *OntologyDeps`, server.go:467); routes registered next to the ontology block (server.go:655-662); gate mirrors `requireOntology` (admin_ontology.go:68-77): `requireAdmin` → 403 first, then nil/ECL-off → 503, then running → 409 (409 only on `run`).
**Stage names are FE contract:** `idle / merge / extract / kg-load / milvus-load / done` — the UI-SPEC §Stage Mapping hardcodes these sentinels (`isRunning = stage !== 'idle' && stage !== 'done'`). Do not rename.

### Pattern 3: Ownership-marker purge (resolves the mixed-tier conflict)
**What:** ECL-owned Milvus rows are identifiable by `section_count > 0` (D-Milvus-02 zero-fills everything else). Purges become marker-scoped:
- ECL milvus-load: `tier in ["sql-guides"] and section_count > 0` → then insert merged chunks.
- Boot Index (ECL enabled): `tier in [<seeded slugs>] and section_count == 0` for the legacy rows.
- Boot Index (ECL disabled): plain tier purge (legacy contract) — which also wipes stale ECL rows on rollback. Coherent both directions.
**Why:** `DeleteTierRows` is tier-level only (seed.go:216-252) and has 4+ call sites; a new narrow `milvus.DeleteByExpr(ctx, mc, cfg, expr)` (LoadCollection → Delete → Flush, copying DeleteTierRows mechanics) avoids touching them. `%q`-escape every interpolated value (filter.go:55-59 precedent).

### Pattern 4: Narrow Extractor interface with deterministic stub
**What:** The F1 harness and dead-letter tests need a deterministic extractor; production needs the LLM. Structural interface, induction.Generator-style (induction.go:51).
```go
type Extractor interface {
    ExtractNote(ctx context.Context, note Note) (Extraction, error)
}
type Extraction struct {
    Tables       []ExtractedTable `json:"tables"`        // {name, kind}
    RelatedNotes []RelatedNote    `json:"related_notes"` // {source_idx, relation}
}
```
LLM impl: `cm.Generate(ctx, [SystemMessage("You output only valid JSON."), UserMessage(prompt)], model.WithTemperature(0.0))` + `llm.ExtractJSON` + `json.Unmarshal` (exact induction.Generate shape, induction.go:135-152). Note `withJSONObjectResponseFormat` is **unexported** in internal/llm — either export a helper from `internal/llm` or inline the `model.WrapImplSpecificOptFn` block (extract.go:185-192) in the ecl package; planner's call, prefer exporting to avoid a second copy.
Stub impl: regex (`update|insert into|create or replace|delete from|select` over `etl_\w+`, 005c regexes extended to the D-Ext-01 kind enum) — this same stub is the golden-set drafting tool (D-GT-01) and the verify-script harness subject.

### Pattern 5: F1 computation vs golden jsonl
**What:** Per-note set comparison, macro-averaged. Golden row: `{"source_idx":"001","tables":[{"name":"etl_payment_due_t","kind":"update"}],"related":["006"]}` (CONTEXT §specifics).
```go
// precision = |P ∩ G| / |P|; recall = |P ∩ G| / |G|; F1 = 2PR/(P+R)
// computed on table-NAME sets per note (kind as secondary check),
// macro-averaged over the 26 notes; related_notes scored the same way
// as a second metric. Pure function — hermetically testable.
```
SC-2's "≥80% precision@recall" should be pinned in the plan as **macro-F1 ≥ 0.8 over table-name sets** (26 notes). The verify script asserts the *harness* by running the regex stub against the regex-drafted golden (expect ≈1.0); the live-LLM F1 remains a manual operator gate (same disposition as Phase 38's live induction gate, 38-VERIFICATION deviation #4).

### Anti-Patterns to Avoid
- **Purging by tier inside the ECL path** — wipes the cohabiting legacy rows (Pitfall 1).
- **Seeding merged sections >8192 runes** — `truncateRunes` truncates silently; SQL completeness breaks without any error (Pitfall 2).
- **Re-emitting the TBox unconditionally at boot** — drops induction-approved triples and rewrites versioned history; the pointer-exists short-circuit (main.go:877) exists for a reason. Upgrade = one triggered version bump, not a re-emit-every-boot policy.
- **KG writes without snapshot trigger** — ECL must invoke the same debounced snapshot fn wired via SetOnChange (main.go:911-912); otherwise ABox survives only until process exit (even with the boot-restore fix).
- **Placing the routing check after the maxDocBytes skip** — silently starves ECL of exactly the files it exists to fix.
- **Hardcoding 185/26 in verify without a dynamic cross-check** — corpus drift turns a real regression into a confusing number mismatch; assert contribution-completeness AND the pinned SC numbers, with distinct failure messages.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Defensive LLM JSON parsing | Custom fence/brace stripping | `internal/llm.ExtractJSON` (extract.go:83) | Handles OOS-1 fence-eating failure mode; three call sites already standardize on it |
| response_format injection | New option plumbing | `model.WrapImplSpecificOptFn` block (extract.go:185) | Documented DEVIATION with eino v0.9.14 option surface; replicating the analysis is wasted risk |
| Oversized-text splitting | Custom SQL-aware splitter | eino-ext `splitter/recursive` | SQL-boundary detection explicitly out of scope; splitter already in module graph |
| Milvus delete mechanics | Ad-hoc Delete calls in ecl pkg | `milvus.DeleteByExpr` helper copying DeleteTierRows mechanics (Load→Delete→Flush) | The Load-before-DML and flush-after-tombstone invariants are documented at seed.go:210-215 |
| SQLite table + migration | Separate DB/file for failures | `schemaSQL` DDL in `internal/store/sqlite/schema.go` | CREATE TABLE IF NOT EXISTS idempotent migrate-on-New is the established pattern (schema.go:25-31) |
| Admin gating/envelopes | New auth/403/503/409 shapes | `requireOntology`-style gate + `graphResponseError` envelope | 403-before-503 is a locked contract (D-Route-01) with a working template |
| FE polling loop | Raw setInterval in component | `useEvalReports` pattern (web/src/hooks/useEvalReports.ts) with injected interval | UI-SPEC §4 mandates it incl. `vi.useFakeTimers` testability |

**Key insight:** Every hard problem in this phase already has a validated in-repo implementation one paste away — the risk is not invention, it is *integration seams* (purge scope, boot order, ID allocation) where the existing components' contracts silently conflict.

## Runtime State Inventory

Migration-adjacent phase (Milvus schema + TBox + boot sequence), so the inventory is answered explicitly:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Milvus collection `skillforge_*`: 202-file corpus rows, schema WITHOUT the 4 new fields → inserts with new columns fail until recreate. `data/ontology/` **does not exist yet** in this working copy (first Phase-39 boot emits the 8-class TBox v1 directly — upgrade path only matters for environments that already booted Phase 38). `skillforge.db`: no ecl_failures table yet. KG snapshot `skillforge_kg.json` (TBox-only content) gets overwritten at every boot today. | `SKILLFORGE_MILVUS_RECREATE=1` boot + unset (existing one-shot door, main.go:1134-1139); schema DDL is auto-migrate; boot restore fix (Pitfall 3) |
| Live service config | Milvus/BGE/GLM are intranet services (addresses via env). No dashboards/ACLs carry ECL-related names. | Probe before live verification runs (memory: env-intranet-service-flakiness) |
| OS-registered state | None — no scheduler/task registrations involved (ECL is manual-trigger by D-Run-01) | None — verified: no ECL/Schedule references outside internal/server/admin_schedules.go's unrelated feature |
| Secrets/env vars | New env: `SKILLFORGE_ECL_ENABLED` (default on). Existing: `SKILLFORGE_MILVUS_RECREATE` (literal "1" only), `SKILLFORGE_ADMIN_TOKEN`, `SKILLFORGE_ONTOLOGY_*`. No secret keys renamed. | Add `.env.example` row for SKILLFORGE_ECL_ENABLED (RECREATE row precedent, main.go:1133) |
| Build artifacts | `go` / `server` / `server.test` binaries in repo root are untracked build droppings (git status); WSL-built. No egg-info-style staleness. | None for this phase; standard clean-worktree verification per project memory (feedback-verify-commits-in-clean-worktree) |

## Common Pitfalls

### Pitfall 1: Tier-level purge vs mixed-tier rows (CRITICAL)
**What goes wrong:** With ECL on, `sql-guides` contains ECL rows (185 fragments → merged chunks) AND legacy rows (17 non-sec files). Boot `Index` purges `tier in ["sql-guides"]` (DeleteTierRows) → deletes the ECL chunks after every boot; conversely an ECL run's tier-level purge deletes the 17 legacy rows.
**Why it happens:** `DeleteTierRows` predates the concept of two producers sharing one tier.
**How to avoid:** Pattern 3 marker-scoped purge (`section_count` as ownership bit), conditional on `SKILLFORGE_ECL_ENABLED`.
**Warning signs:** After reboot, sql-guides retrieval returns only ~17 docs; after ECL run, the non-sec docs vanish from the graph.

### Pitfall 2: Oversized merged sections silently truncated (CRITICAL)
**What goes wrong:** 14/69 merged sections exceed 8192 bytes; note 039's SQL section is 93,993 bytes (~31K runes CJK-heavy). `SeedDocuments` runs `truncateRunes(content, 8192)` — silent truncation mid-SQL; embedding quality collapses long before that (BGE-M3 8192-token window).
**Why it happens:** The spike's merge step was validated on completeness, never on chunk-size ceilings (its output went to files, not Milvus).
**How to avoid:** Sub-split sections over a ~3000–4000-rune threshold with the recursive splitter; all parts carry identical section metadata (multiple rows may share source_idx/section/section_idx/section_count — the PK is `id`, uniqueness is preserved).
**Warning signs:** verify shows note 039/029 merged body ≠ sum of seeded row content; or `update etl_*` counts drop after seeding.

### Pitfall 3: ABox wiped on restart (HIGH)
**What goes wrong:** Boot builds the KG from the TBox TTL pointer only (`onstore.New()` → `LoadFromTurtle`, main.go:894-902) and then **overwrites** the snapshot (main.go:904). ECL ABox triples (which only exist post-Run) vanish on every restart.
**Why it happens:** Phase 38 deviation — D-KG-02 specified LoadSnapshot-first, but the implementation skipped it because KG content == TBox content at the time (38-VERIFICATION deviations list doesn't call it out; it was harmless then).
**How to avoid:** Implement D-KG-02 now: if snapshot exists → `LoadSnapshot` → merge TBox triples with **dedupe** (`Insert` does not dedupe, store.go:67-68 — add an `InsertUnique`/set-guard or dedupe during merge) → write snapshot. ECL calls the shared debounce fn after KG writes.
**Warning signs:** `admin/ontology/kg/stats` triple_count drops to the TBox baseline (~176) after restart.

### Pitfall 4: TBox pointer short-circuit blocks the 6→8 extension (HIGH)
**What goes wrong:** Boot emits the TBox only when `skillforge_tbox.ttl` is missing (main.go:877). Any environment that booted Phase 38 code keeps a 6-class TBox; `skf:NoteChunk`/`skf:TableSchema` stay undeclared while ABox instances reference them.
**How to avoid:** One-time upgrade trigger in the boot emit step: if pointer exists but lacks `skf:NoteChunk a owl:Class` → emit fresh output as `skillforge_tbox.v{N+1}.ttl`, bump `tbox_version.txt`, swap pointer; preserve any induction-approved triples from the current version (replay approved proposals from the induction queue, or verify none exist in the target env — planner pins the approach).
**Warning signs:** `GET /api/admin/ontology/tbox` shows 6 classes after deploying Phase 39.

### Pitfall 5: Chunk ID collisions between boot rows and ECL rows (MEDIUM-HIGH)
**What goes wrong:** Boot assigns IDs 1..N monotonically across all tiers. ECL inserts later; restarting IDs at 1 reuses PKs — Milvus doesn't dedupe inserts, so both generations become searchable (the exact 2026-09-15 duplicate-citations incident, seed.go:183-191).
**How to avoid:** Allocate ECL IDs above a safe watermark. `GetMaxID(ctx, tier)` is **per-tier** (client.go:88-122) while the PK is collection-wide — compute the max across all active tiers (or add a collection-wide variant) before assigning.
**Warning signs:** Graph shows duplicate contents across generations after a corpus change + ECL re-run.

### Pitfall 6: Extraction prompt size on the largest notes (MEDIUM)
**What goes wrong:** Note 039's merged body is 88K chars (~40-60K tokens CJK+SQL). GLM-5.2's context is large enough [ASSUMED], but latency/cost spike and truncation-on-endpoint risk are real; a mid-body truncation would silently lose table refs and tank recall.
**How to avoid:** Size guardrail in the extract stage: whole-note prompt up to a cap (e.g. ~40K chars), fall back to per-section extraction with result union above it. Planner's discretion area (concurrency ≤3 for LLM calls per CONTEXT).
**Warning signs:** Extraction stage duration outliers on notes 039/029; F1 recall holes on the largest notes only.

### Pitfall 7: Windows/WSL + CRLF + console encoding (MEDIUM, known project terrain)
**What goes wrong:** `.env` CRLF breaks parsing (project memory); Windows console mangles CJK in the spike outputs (the garbled `chunk_metadata.json` print in this session's terminal is display-side only). `go build ./...` fails on Windows host for unrelated cgo reasons (38-VERIFICATION deviation #3).
**How to avoid:** All build/test via WSL (`/usr/local/go/bin/go`, go1.25.0 verified); CRLF→LF normalize on frontmatter parse (reuse `splitFrontmatter`, indexer.go:494-497, which already does `ReplaceAll("\r\n", "\n")`).
**Warning signs:** Frontmatter parse misses on files edited on Windows.

### Pitfall 8: Golden-set self-confirmation (process pitfall)
**What goes wrong:** Using the LLM (or the ECL extractor itself) to build the golden set makes F1 a tautology.
**How to avoid:** D-GT-01 is locked: regex draft → human review → jsonl. The verify script's regex-stub-vs-golden ≈1.0 assertion is a *harness* check, not a quality claim about the LLM.

## Code Examples

### Section-aware merge core (port from spike, verified at .planning/spikes/005c-ontology-aware-chunker/main.go:55-199)
```go
// Source: .planning/spikes/005c-ontology-aware-chunker/main.go (VERDICT: VALIDATED)
var secRegex = regexp.MustCompile(`__sec(\d{2})_(.+)\.md$`)
// walk → parse frontmatter (title/source_idx/section) → group by source_idx
// → sort parts by secNN → merge same-section bodies (first-seen order)
// → ChunkMeta{source_idx,title,section,section_idx,section_count,source_paths,body_bytes}
// Verified numbers: 185 fragments → 26 notes → 69 section records;
// note 001: 3 sections, 3/3 corpus SQL targets
// (etl_payment_due_t / etl_policy_request_issue_t / etl_NONAUTO_POLICY_T)
```

### Milvus schema extension (seed.go:70-88 extension point, verified)
```go
// Source: internal/retriever/milvus/seed.go:70-88 (current 7-field schema)
schema := entity.NewSchema().WithName(cfg.CollectionName).WithAutoID(false).
    WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(false)).
    WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultTierMaxLen))).
    // ... content / source / embedding / effective_from / effective_to ...
    // Phase 39 (D-Milvus-01): +4 native fields —
    WithField(entity.NewField().WithName("source_idx").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
    WithField(entity.NewField().WithName("section").WithDataType(entity.FieldTypeVarChar).WithMaxLength(256)).
    WithField(entity.NewField().WithName("section_idx").WithDataType(entity.FieldTypeInt64)).
    WithField(entity.NewField().WithName("section_count").WithDataType(entity.FieldTypeInt64))
// SeedDocuments gains 4 parallel column slices (entity.NewColumnVarChar / NewColumnInt64),
// retriever.Document gains 4 zero-value-defaulted fields (D-Milvus-02).
// NOTE: VarChar columns use NewColumnVarChar, never NewColumnString
// (skill spike-findings: DataType String=20 vs FieldTypeVarChar=21 mismatch).
```

### Marker-scoped purge helper (new, mirrors DeleteTierRows mechanics seed.go:216-252)
```go
// Source: mechanics from internal/retriever/milvus/seed.go DeleteTierRows
func DeleteByExpr(ctx context.Context, mc client.Client, cfg Config, expr string) error {
    // HasCollection → no-op on fresh deploy; LoadCollection (DML needs loaded);
    // mc.Delete(ctx, cfg.CollectionName, "", expr); Flush (durable tombstones).
    // Caller interpolates values with %q (filter.go:55-59 injection-safe precedent).
}
// ECL:  DeleteByExpr(ctx, mc, cfg, `tier in ["sql-guides"] and section_count > 0`)
// Boot: DeleteByExpr(ctx, mc, cfg, `tier in [...] and section_count == 0`)  // ECL on
```

### LLM extraction call shape (induction.go:128-152 + extract.go:185, verified)
```go
// Source: internal/ontology/induction/induction.go Generate + internal/llm/extract.go
resp, err := cm.Generate(ctx, []*schema.Message{
    schema.SystemMessage("You output only valid JSON."),
    schema.UserMessage(buildExtractionPrompt(note)), // D-Ext-01 fixed JSON schema
}, model.WithTemperature(0.0), llm.JSONObjectResponseFormat()) // export the helper
raw := llm.ExtractJSON(resp.Content)
var ex Extraction
if err := json.Unmarshal([]byte(raw), &ex); err != nil { /* retry → dead-letter */ }
```

### Admin gate (admin_ontology.go:68-77 shape, verified)
```go
// Source: internal/server/admin_ontology.go requireOntology (Phase 38 newest sample)
func (s *Deps) requireECL(c *app.RequestContext) (*ecl.Manager, bool) {
    if !requireAdmin(c) { return nil, false }              // 403 first (D-Route-01)
    if s.ECL == nil || !s.ECL.Enabled() {
        c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ECL pipeline not wired"})
        return nil, false                                   // 503 after 403
    }
    return s.ECL, true
}
// run handler: gate → if !mgr.CASRunning() → 409 {error:"pipeline already running"}
// (UI-SPEC §Interaction: 409 copy 「管线正在运行中，请等待完成」)
```

### ecl_failures DDL (schema.go pattern, import_batches precedent at schema.go:232)
```sql
-- Source: internal/store/sqlite/schema.go schemaSQL pattern (CREATE TABLE IF NOT EXISTS;
-- migrate() idempotent on every New). Mutable STATUS column is sanctioned here
-- (NOT an audit table — CONTEXT D-DL-01 note).
CREATE TABLE IF NOT EXISTS ecl_failures (
    note_id       TEXT PRIMARY KEY,      -- e.g. "007"; retry re-runs one note
    stage         TEXT NOT NULL,         -- extract | kg-load | milvus-load
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending | dead | resolved
    updated_at    TEXT NOT NULL DEFAULT ''
);
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Byte-cap chunking (`maxDocBytes=4000`, skips 35 oversized sql-guides fragments today) | Section-aware merge (this phase) for `__secNN` files only | Phase 39 | 35 currently-skipped files become retrievable; authority/historical/personal/draft untouched |
| Boot = TBox-only KG rebuild + snapshot overwrite | Boot = snapshot-first restore + TBox dedupe-merge (this phase fixes D-KG-02 deviation) | Phase 39 | ABox becomes durable across restarts |
| Tier-level Milvus purge as the only replace primitive | Tier purge + ownership-marker expr purge coexisting | Phase 39 | Two producers can share one tier safely |
| `walkTier` flat-glob corpus view | Recursive walk (Phase 999.2, already in indexer.go:76-79) | pre-Phase 39 | sql-guides subdirs already indexed recursively; ECL walk must also descend subdirs (spike source_paths confirm subdirs existed at spike time) |

**Deprecated/outdated:** None of the phase's building blocks are deprecated; eino v0.9.14 is the locked release (v0.10.0-alpha explicitly not used), milvus-sdk-go v2.4.2 remains the production SDK (the spike-004 `milvus/client/v2` migration was validated but NOT applied to go.mod — do not migrate SDKs in this phase).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GLM-5.2 endpoint context window accommodates whole-note extraction prompts up to ~40-60K tokens (note 039 = 88K chars) | Pitfall 6 | Extraction falls back to per-section mode; mitigation already designed in, so impact is bounded |
| A2 | Corpus counts are stable at 185 `__secNN` fragments / 26 notes / 3 corpus SQL tables until verify runs | Pitfalls, SC-1 | Verify script should assert both pinned numbers and dynamic 100%-contribution so drift produces a precise failure, not a mystery |
| A3 | `SKILLFORGE_ECL_ENABLED` parse semantics: disabled only on explicit `off`/`false` (default on) | Pattern 1 | A differently-parsed gate could flip the default path; trivial to pin in plan review |
| A4 | `skf:belongsToNote` has no ABox triple instances this phase (D-Ext-01's triple list only instantiates `referencesTable` + type triples) — declared in TBox, reserved for section-level ABox later | Open Questions | If the user intends section-level ABox now, extraction/kg-load scope grows; needs plan-level confirmation |
| A5 | Golden file path stays `eval/ecl-golden/sql-guides.jsonl` (D-GT-01 verbatim) despite the existing `eval/golden/*.jsonl` convention | Standard Stack / Alternatives | Cosmetic inconsistency only; flag to user, don't silently deviate |
| A6 | Note-level KG replace semantics: "note" = merged logical note (`skf:note_{source_idx}` individual); DeleteWhere targets all triples with that subject (+ TableSchema cleanup only when orphaned) | Architecture / D-Boot-01 | A different note-granularity reading would change the delete primitive's signature; planner pins the exact predicate set |
| A7 | RETRIEVER_BACKEND=mongo deployments: ECL run answers 503 for the milvus-load stage (ECL is Milvus-schema-bound per D-Milvus-01) | Open Questions | If mongo+KG-only ECL is desired, the milvus-load stage needs a backend abstraction — out of scope by default |

## Open Questions (RESOLVED)

1. **belongsToNote domain/range (A4)**
   - What we know: D-TBox2-01 adds the property; D-Ext-01's triple shapes never instantiate it.
   - What's unclear: whether any Phase 39 ABox uses it (e.g. section individuals → note) or it is TBox-only this phase.
   - Recommendation: declare it in the emitter (2 classes + 2 props exactly as locked), emit zero belongsToNote triples this phase; note in UI-SPEC-visible docs. Confirm in plan review.
   - **RESOLVED** (open-question resolution #1): pinned in 39-01-PLAN.md Task 2 (TBox declares `skf:belongsToNote` with domain/range as emitted; zero ABox instances this phase per D-TBox2-01's locked triple list) and 39-04-PLAN.md Task 3 (BuildTriples emits NO belongsToNote triples). Domain/range choice (NoteChunk→NoteChunk, no Note class exists) is noted for the Phase 40 section-level ABox in the 39-01 summary.
2. **ECL-first-run coverage gap**
   - What we know: with ECL on, boot skips the 185 sec-fragments from legacy seeding; ECL runs only on manual trigger. Until the first Run, sql-guides retrieval covers only the 17 non-sec files.
   - What's unclear: is that acceptable as a documented operator step (RECREATE boot → admin clicks Run ECL), or should the status endpoint surface "never run" prominently (UI-SPEC already renders idle state 「尚未运行」)?
   - Recommendation: document the deploy sequence (RECREATE=1 boot → Run ECL → unset) in `.env.example` comment + verify runbook; the idle UI state already covers discoverability.
   - **RESOLVED** (open-question resolution #2): pinned in 39-06-PLAN.md Task 3 — `.env.example` gains the SKILLFORGE_ECL_ENABLED row plus a 3-line comment block documenting the deploy sequence (SKILLFORGE_MILVUS_RECREATE=1 first boot after upgrade → admin Run ECL from /admin → unset RECREATE); the idle 「尚未运行」 UI state covers discoverability.
3. **Where the golden-set review checklist lives**
   - What we know: D-GT-01 requires human review; discretion says "一次性 checklist 文档 vs 分批" is open.
   - Recommendation: single checklist doc under `.planning/phases/39-ecl-pipeline/` (one 26-row review pass), referenced by the plan's golden-set task.
   - **RESOLVED** (open-question resolution #3): pinned in 39-09-PLAN.md Task 1 — single checklist doc at `.planning/phases/39-ecl-pipeline/golden-review-checklist.md` (one 26-row review pass; becomes the committed human-review record after Task 2's checkpoint).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go (WSL) | all backend build/test | ✓ | go1.25.0 linux/amd64 (`wsl bash -lc go version`) | — (hard requirement per runbook) |
| node | web build/vitest | ✓ | v24.19.0 | — |
| pnpm | web package manager | ✓ | 11.22.0 | — |
| python | golden-set drafting helper (optional; Go helper equally viable) | ✓ | 3.13.2 | Go-based drafter in ecl package |
| Milvus (intranet) | milvus-load live stage + RECREATE migration | ⚠ intermittent | — (probe before live) | retry ×3 → dead-letter (D-RT-01/D-DL-01); stub tests hermetic |
| GLM (intranet, via `cm`) | live extraction (manual operator gate) | ⚠ intermittent | — | stub extractor for all automated tests; live F1 is a manual gate |
| SQLite (skillforge.db) | ecl_failures | ✓ | file DB via mattn driver already in deps | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** Milvus/GLM intranet flakiness — handled by dead-letter design + hermetic stub tests (verify-phase-39.sh must stay hermetic like verify-phase-38.sh).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + testify v1.11.1 (backend); vitest ^2.0.0 + @testing-library (web) |
| Config file | none for Go (stdlib); `web/package.json` scripts for vitest |
| Quick run command | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./internal/knowledge/ecl/... ./internal/ontology/... ./internal/server/ -count=1"` |
| Full suite command | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./... -count=1"` + `pnpm -C web test` + `bash scripts/verify-phase-39.sh` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ONT-10 | Routing branch: `__secNN`+section → ECL; 17 non-sec + other KBs unchanged; ordering before maxDocBytes | unit | `go test ./internal/knowledge/ -run TestIndexRouting -count=1` | ❌ Wave 0 |
| ONT-10/11 | Merge: 185 fragments → 26 notes, 185/185 contributions, ChunkMeta contract (69-section shape) | unit (real corpus, hermetic file reads) | `go test ./internal/knowledge/ecl/ -run TestMergeBoundary -count=1` | ❌ Wave 0 |
| ONT-11 | Note 001 merged body contains 3/3 corpus SQL targets | unit | `go test ./internal/knowledge/ecl/ -run TestNote001SQL -count=1` | ❌ Wave 0 |
| ONT-12 | F1 harness: regex stub vs golden jsonl macro-F1 ≥ 0.8 (expect ≈1.0 — harness check); golden parsing + set math | unit | `go test ./internal/knowledge/ecl/ -run TestF1 -count=1` | ❌ Wave 0 (needs `eval/ecl-golden/sql-guides.jsonl`) |
| ONT-12 | Extraction schema conformance (stub + LLM-shape JSON through ExtractJSON), kind enum validation | unit | `go test ./internal/knowledge/ecl/ -run TestExtract -count=1` | ❌ Wave 0 |
| ONT-13 | ABox build + note-level replace via DeleteWhere + debounce trigger; TBox upgrade trigger fires once | unit | `go test ./internal/ontology/... ./internal/knowledge/ecl/ -run TestABox -count=1` | ❌ Wave 0 |
| ONT-13 | Milvus doc build: +4 fields populated, oversized-section sub-split keeps metadata, zero-value defaults for legacy docs | unit (mock client, delete_tier_rows_test.go pattern) | `go test ./internal/retriever/milvus/ -count=1` | ❌ Wave 0 (extend existing suite) |
| SC-5 / D-RT-01/D-DL-01 | Retry ×3 backoff → dead; single-note failure isolation; retry endpoint re-enqueues; failures persist across "restart" (new store from DSN) | unit | `go test ./internal/knowledge/ecl/ -run TestManager -count=1` + `go test ./internal/server/ -run TestAdminPipeline -count=1` | ❌ Wave 0 |
| SC-4 / D-Route-01 | Admin gate: 403 before 503; 409 double-run; status DTO shape `{stage,current_file,done,total,queue_len,failures[]}` | unit | `go test ./internal/server/ -run TestAdminPipeline -count=1` | ❌ Wave 0 |
| SC-4 (UI) | PipelineTab: copy contract, cadence 2s/10s re-arm, retry enabled only on dead, 403/503/409 rendering | component (vitest, vi.useFakeTimers) | `pnpm -C web test -- PipelineTab` | ❌ Wave 0 |
| SC-6 | verify-phase-39.sh aggregate hermetic gate (modeled on verify-phase-38.sh: SC stages, ok/fail counters, WSL header) | script | `bash scripts/verify-phase-39.sh` | ❌ Wave 0 |
| — | Regression: existing knowledge/milvus/server/ontology suites stay green | regression | `go test ./... -count=1` | ✅ exists |

### Sampling Rate
- **Per task commit:** quick run command (ecl + touched package, `-count=1`)
- **Per wave merge:** `go test ./... -count=1` + `pnpm -C web test`
- **Phase gate:** `bash scripts/verify-phase-39.sh` all-green + clean-worktree build/test (project memory: feedback-verify-commits-in-clean-worktree); live LLM F1 + live Milvus E2E = manual operator gate

### Wave 0 Gaps
- [ ] `internal/knowledge/ecl/` package + test files (merge/extract/f1/manager/abox) — covers ONT-10..13
- [ ] `eval/ecl-golden/sql-guides.jsonl` — 26-row golden set (regex draft + human review task, D-GT-01)
- [ ] `scripts/verify-phase-39.sh` — hermetic SC gate
- [ ] `internal/store/sqlite/schema.go` ecl_failures DDL + narrow FailureStore interface (test fake)
- [ ] milvus package: extend seed/delete tests for the 4 new fields + DeleteByExpr (existing `delete_tier_rows_test.go` / mock-client pattern)
- [ ] `web/src/components/admin/pipeline/PipelineTab.test.tsx`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | No new identities; admin surface reuses `X-Admin-Token` (`SKILLFORGE_ADMIN_TOKEN`) |
| V3 Session Management | no | Stateless admin token per request; no sessions introduced |
| V4 Access Control | yes | `requireAdmin` gate first (403), then nil/ECL-off (503) — 403-before-503 is locked (D-Route-01); `ecl_failures` reads/writes only via server-side store |
| V5 Input Validation | yes | `:note_id` path param validated against the known note set (`^(\d{3}|\w+)$` + ecl_failures lookup) before retry; admin JSON bodies size-capped by existing body limits; LLM output parsed through ExtractJSON + strict struct unmarshal (schema conformance, kind enum) |
| V6 Cryptography | no | No new crypto |
| V7 Error/Logging | yes | `last_error` stored raw but rendered verbatim only in the authed admin UI; no secrets in errors (LLM/Milvus errors are endpoint-shaped, not key-bearing) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Milvus filter expression injection via interpolated tier/source_idx values | Tampering | `%q` escaping precedent (filter.go:55-59); values originate from registry slugs / golden jsonl, never raw user input |
| Prompt injection via corpus content (note bodies → extraction prompt) | Tampering | Same posture as induction: output constrained to a fixed JSON schema, parsed defensively, admin-triggered only; extracted names sanitized for IRI local names (`emitter.sanitizeLocal` precedent, emitter.go:327) |
| Path traversal via note_id / source paths in retry | Elevation | note_id resolved against in-store failure rows / merged-note set, never used to build filesystem paths; corpus paths stay server-derived (knowledge.go `validRelDocPath` precedent for the pattern) |
| SQL injection in ecl_failures queries | Tampering | Parameterized placeholders via the existing sqlite store helpers (T-3-SQLI discipline in Store interface docs) |
| Admin surface enumeration (403 vs 404 oracles) | Information Disclosure | Established uniform 403-before-503 envelope; no new divergence |

## Sources

### Primary (HIGH confidence)
- `.planning/spikes/005c-ontology-aware-chunker/` — main.go (secRegex L57, merge L155-199, ChunkMeta L264-284), README.md (185→26, 69 records, 4 boundary rules, gotchas), chunk_metadata.json (69 records; size distribution), merged_notes/ (26 files; note 039 = 88,306 chars)
- `internal/knowledge/indexer.go` — Index signature L166, maxDocBytes L69, walkTier recursion L76-107, seedDocs purge seam L443-451, indexSeedHook L421, splitFrontmatter L494
- `internal/retriever/milvus/` — seed.go schema L70-88 / SeedDocuments L129-180 / DeleteTierRows L216-252; config.go constants L43-62, truncateRunes L132, RecreateEnvVar L153; filter.go BuildMilvusFilter L64-85; client.go GetMaxID L88-122; backend.go IndexerBackend L23-33
- `internal/retriever/retriever.go` — Document L70-78, DocumentMeta L89-95
- `internal/ontology/` — store/store.go (Insert-no-dedupe L67-77, Query L88, LoadSnapshot L181, SnapshotDebounced L199, LoadFromTurtle L231); emitter/emitter.go (classes L202-209, objProps L215-225, sanitizeLocal L327); induction/induction.go (Generator L51, Generate L128-188)
- `internal/llm/extract.go` — ExtractJSON L83-120, withJSONObjectResponseFormat L185-192
- `cmd/server/main.go` — ontology boot L864-914 (pointer short-circuit L876-893, KG rebuild L894-906, debounce wiring L911-912), RECREATE gate L1134-1139, boot Index L1144, chat model `cm` L490-507
- `internal/server/` — server.go Deps L235+ / Ontology L467 / admin routes L637-662; admin_ontology.go gate L68-77; audit.go requireAdmin L131
- `internal/store/sqlite/schema.go` — schemaSQL pattern L25-31, import_batches DDL L232, kpi_calls L286
- `.planning/phases/39-ecl-pipeline/39-UI-SPEC.md` — approved 2026-09-21 (6/6 checker dimensions); `.planning/phases/38-foundation-tbox-kg/38-CONTEXT.md` (D-TBox/KG/Induct/Lib/Route) + `38-VERIFICATION.md` (deviations incl. Windows cgo, live-gate disposition)
- Corpus ground truth (2026-09-21): `knowledge/sql-guides/` — 202 .md total, 185 `__secNN`, 17 non-sec (sampled frontmatter: non-sec lacks `section` field), 7 subdirs; `eval/golden/` convention (6 existing jsonl files)

### Secondary (MEDIUM confidence)
- Project memory: env-wsl-backend-runbook, env-intranet-service-flakiness, fix-subagent-modelcode-1214-bigmodel, fix-milvus-reseed-duplicate-generations, feedback-verify-commits-in-clean-worktree
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — NewColumnVarChar vs NewColumnString gotcha; byte-determinism impossibility → semantic-determinism philosophy

### Tertiary (LOW confidence)
- GLM-5.2 exact context window size (A1) — training-data knowledge, bounded by the per-section fallback design

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; every API surface read in-repo this session
- Architecture: HIGH — all integration seams (purge/boot/TBox/IDs) traced to exact lines; the four design problems carry concrete recommendations with in-repo precedent
- Pitfalls: HIGH — each pitfall is anchored to verified code state or measured corpus data (93,993-byte section, 202/185/17 counts, 69 records); only A1/A2 carry environment-level uncertainty

**Research date:** 2026-09-21
**Valid until:** 2026-10-21 (stable: in-repo codebase targets; re-verify corpus counts if knowledge/ changes)
