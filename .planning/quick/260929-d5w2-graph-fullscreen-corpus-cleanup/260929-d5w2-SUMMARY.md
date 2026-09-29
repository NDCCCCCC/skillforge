# Quick 260929-d5w2 Summary — graph fullscreen + pretty source paths + test-KB cleanup + corpus rename & full rebuild

## One-liner
Detail-panel knowledge graph gains a fixed-overlay 全屏 mode; `prettySourcePath` shortens corpus source paths at three display sites (citations, graph nodes, KB doc list) with raw paths kept on hover; two dead test KBs (`uat-check`, `walkthrough-tmp`) deleted from the registry; 39 corpus files renamed to readable bases (sec suffixes preserved) with a full Milvus-recreate + ECL rebuild verified live.

## Stats

| Metric | Value |
|---|---|
| Tasks completed | 4 / 4 |
| Commits | 4 (2 feat, 2 chore) |
| FE gates | `tsc --noEmit` clean; vitest: gate scope 94 passed + affected suites 59 passed (8 KnowledgeGraphBrowser incl. 2 new fullscreen, 4 new prettySourcePath) |
| KB registry after cleanup | 5 live bases: authority, draft, historical, personal, sql-guides |
| Corpus renames | 39 files (28 sec-group + 5 single + 6 `其他/`→`操作手册/` moves), 202 .md files intact, 0 content edits, all `100%` git renames |
| secRegex integrity | `__sec(\d{2})_(.+)\.md$` matches all sec files post-rename (0 violations) |
| Rebuild | RECREATE boot OK; `knowledge corpus indexed` in boot log; ECL pipeline 26 batches → stage `done` (~17 min, GLM slow on batch 25) |
| Graph verify | 100 doc groups; 19 docs carry the new names (incl. `操作手册/037_*`); 0 old-base names; old `其他/` prefix gone |
| SPARQL | `referencesTable` query (X-Skill-Name: sql-guides) → 57 rows |
| SHACL | error_count 7 (unchanged), quarantined 7 — pre-existing `belongsToTier` sh:class violations, one per dir |

## Files

| Path | Status |
|---|---|
| `web/src/components/graph/KnowledgeGraphBrowser.tsx` | modified — 全屏 button + fixed inset-0 z-50 overlay (portal to body, bg-bg, X/Esc close), shared graph body for rail + fullscreen |
| `web/src/__tests__/KnowledgeGraphBrowser.test.tsx` | modified — +2 tests (overlay open/portal/single-canvas + close; Esc close) |
| `web/src/lib/pretty-source.ts` | created — basename + .md strip + `__secNN_<section>` → `base · section` (underscores dropped) |
| `web/src/lib/pretty-source.test.ts` | created — 4 cases incl. the locked 032 example |
| `web/src/lib/chunk-lookup.ts` | modified — `fetchChunkContent` now surfaces the payload's raw `source` |
| `web/src/components/center/CitationCluster.tsx` | modified — popover meta row shows pretty source (`citation-source`), raw on title |
| `web/src/components/center/ChunkSourceView.tsx` | modified — `来源:` meta row (`chunk-source-path`) for async-fetched chunks |
| `web/src/components/graph/GraphTree.tsx` | modified — doc summary pretty label, raw on title |
| `web/src/components/graph/GraphCanvas.tsx` | modified — doc + neighbor node labels pretty; SVG `<title>` keeps raw path |
| `web/src/components/layout/KnowledgeModal.tsx` | modified — KB doc list pretty label + title; `data-file` stays raw |
| `knowledge/sql-guides/**` | 39 renames (commit 1aaf853) |
| `.planning/quick/260929-d5w2-*.sh` | execution scripts (kb-cleanup, rename, rename-groups-fix, rebuild) — untracked per repo convention |

## Commits

| Hash | Message |
|---|---|
| `2a05f69` | `feat(quick-260929-d5w2): knowledge graph fullscreen overlay in detail panel` |
| `dfc5231` | `feat(quick-260929-d5w2): prettySourcePath display layer for corpus source paths` |
| `ec870aa` | `chore(quick-260929-d5w2): drop dead test KBs uat-check + walkthrough-tmp from registry` (allow-empty — DB is untracked runtime data) |
| `1aaf853` | `chore(quick-260929-d5w2): rename long-titled corpus files for readable source paths` |

## Task evidence

### Task 1 — graph fullscreen
- `graph-fullscreen-btn` (Maximize2) in the narrow rail; overlay portaled to `document.body` (KnowledgeModal B7 posture — panel ancestors with backdrop-filter would clip `fixed`), `bg-bg`, `graph-fullscreen-close` + Esc.
- vitest 8/8 in `KnowledgeGraphBrowser.test.tsx` (portal target = body, exactly one canvas in DOM, Esc exits).

### Task 2 — pretty source paths
- Locked example verified: `sql-guides/取数口径/032_保单号查支付方式与开户行__sec02_SQL_正文.md` → `032_保单号查支付方式与开户行 · SQL正文`.
- Raw path always on `title` / SVG `<title>` / `data-file` — machine identity untouched.
- Citation popover + ChunkSourceView get source via the extended `fetchChunkContent` (async path; the sync fixture carries no source, so the row renders only when known).

### Task 3 — test-KB cleanup
- Order: pkill → sqlite `.backup` → DELETE → `integrity_check ok` → restart → `GET /api/admin/kb` slugs `['authority','draft','historical','personal','sql-guides']` → backup removed. Script: `.planning/quick/260929-d5w2-kb-cleanup.sh`.

### Task 4 — rename + full rebuild
- Pre-rename disk backup (persistent, NOT deleted, user decides): **`/home/ninedrunk/sql-guides-pre-rename-backup`** (WSL home, 202 files, 988K).
- One wsl invocation end-to-end: pkill → `SKILLFORGE_MILVUS_RECREATE=1` boot (WARN line confirms recreate) → READY 32s → boot log `[OK] knowledge corpus indexed into skillforge_knowledge` → `POST /api/admin/pipeline/run` `{"status":"started"}` → poll to `"stage":"done"` → verify graph/SPARQL/SHACL (numbers above). Script: `.planning/quick/260929-d5w2-rebuild-rename.sh`.
- Rename note: sec01 files keep the old long title as the section echo (spec: suffix untouched) — e.g. `001_机构部门科室信息更新__sec01_公司业务部、武汉中支部门组部门科室等信息更新.md`; old-name verification therefore ran on the base (pre-`__secNN_`) only.

## Deviations from Plan

**[Rule 1 - Bug] Rename script first pass skipped all sec-group files**
- **Found during:** Task 4
- **Issue:** `${base#__sec}` strips a *leading* `__sec` (never matches), so 28 group files were SKIPped while single-file moves succeeded.
- **Fix:** suffix extraction `suffix="sec${base##*__sec}"` + re-run group script; verified 8+6+3+4+7 files moved, 0 secRegex violations.
- **Commit:** 1aaf853

### Observations (no action, out of scope)
- Graph API shows 100 doc groups vs 202 disk files: steady-state of boot-index + ECL extraction in this environment (extraction batches cover 26 note groups; some section files carry no extracted chunks). Pre-existing behavior, not introduced here.
- Milvus sources mix two path forms (`sql-guides/取数口径/...` from boot index vs `取数口径/...` from pipeline runs). Both render correctly through `prettySourcePath` (basename step). Pre-existing heterogeneity.
- GLM stalled ~9 min on batch 25 (`031_取数工作汇总`) before completing — known intranet flakiness; no retry/error logged.

## Known Stubs

None — all display sites render live backend data; no placeholder content added.

## Duration
~35 min (2026-09-29T02:24Z → 03:00Z), rebuild pipeline ~17 min of that.

## Self-Check: PASSED
- Commits verified: `git log` shows 2a05f69, dfc5231, ec870aa, 1aaf853 on `phase-20-p10-6-admin-console`.
- Files verified: pretty-source.ts/.test.ts exist; all 5 modified FE files compile (`tsc --noEmit` clean).
- Live verification: KB list 5 bases; graph new names 19 / old 0; SPARQL 57 rows; SHACL 7; backup present at `/home/ninedrunk/sql-guides-pre-rename-backup`.
