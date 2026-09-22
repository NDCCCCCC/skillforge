# Golden Set Review Checklist — eval/ecl-golden/sql-guides.jsonl (D-GT-01)

**Purpose:** one human review pass over the 26-row golden-set draft, per locked decision
**D-GT-01** (regex 起草 + 人工 review; **LLM 永不参与标注集构造**). This document is the
review RECORD: after the review, corrections land in the committed jsonl and this file
records who reviewed what, when, and what changed. It is also the anti-self-grading
mechanism for SC-2 — the reviewed jsonl becomes the F1 yardstick that the LLM extractor
is scored against, so the review IS the quality gate.

- **Draft produced by:** `go run ./cmd/ecl-draft-golden -kb knowledge/sql-guides` — deterministic regex stub (`ecl.StubExtract`) over `ecl.WalkAndMerge` output. No LLM anywhere in that path (grep-gated).
- **Draft under review:** `eval/ecl-golden/sql-guides.jsonl` (held untracked until review lands) — identical bytes also at `/tmp/ecl-golden-draft.jsonl`.
- **Row shape (D-GT-01 verbatim):** `{"source_idx":"001","tables":[{"name":"etl_payment_due_t","kind":"update"}],"related":["006"]}`
- **Kind enum (D-Ext-01):** `update | select | insert | create` — nothing else is legal.
- **Note bodies:** a note's body = the concatenation of its `__secNN` fragment files under `knowledge/sql-guides/<子目录>/<source_idx>_…__secNN_*.md` (section order = NN order).

## Review instructions (per row)

1. **Tables:** every listed `etl_*` table must be genuinely touched by the note's SQL at the claimed kind. Watch for regex artifacts (see drafter notes below).
2. **Kinds:** must match what the SQL actually does, within the D-Ext-01 enum.
3. **Related:** each related `source_idx` must point at a note this note truly cross-references (not a generic-noun collision).
4. **Completeness:** ADD anything the regex missed (tables touched, relations), DELETE false positives, FIX kinds — you may edit all three columns.
5. Mark each row `OK` or write the correction inline in the 评审结论 column.

## What the regex stub sees — and what it does not (drafter notes, machine-generated)

The stub scans only line-anchored **write-shaped** statements: `UPDATE etl_*`, `INSERT INTO etl_*`,
`CREATE OR REPLACE …` (first `etl_*` token on the line), `DELETE FROM etl_*` → kind `update`,
`MERGE INTO etl_*` → kind `update`, `SELECT … INTO etl_*` → kind `select`.
Consequences to scrutinize:

- **Plain reads are invisible by design.** `SELECT … FROM etl_*` / JOINs are NOT captured
  (the `select` kind is only `SELECT … INTO`). E.g. note **029** reads `etl_payment_due_t` /
  `etl_policy_request_issue_t` in FROM clauses and drafts as empty; note **001** joins
  `etl_policy` (no `_t`) but drafts only its 3 write targets. Decide per row whether
  read-only references belong in the golden (add as kind `select` if yes).
- **`CREATE OR REPLACE FUNCTION` false-positive class (note 039).** The create-rule grabs the
  first `etl_*` token on any `CREATE OR REPLACE` line — in 039 those tokens are PostgreSQL
  **function names / parameter names**, not tables:
  `etl_nonauto_agreement_t`, `etl_auto_agreement_t`, `etl_request_issue_t` (functions),
  `etl_batch_date` (a parameter). Confirm or delete per row.
- **Related notes: the drafter emits ZERO everywhere** — the corpus has no explicit numeric
  `见 0NN`-style cross-references today (plan 39-03 survey; one prose ref cites a sibling
  note by TITLE, not id). The relations column is entirely yours: add real relations if any
  note genuinely builds on another, leave empty otherwise.
- **Case:** names keep corpus case (`etl_NONAUTO_POLICY_T` in 001 is an uppercase UPDATE target — legal).
- **SC-1 anchor:** note 001 must keep exactly its 3 write targets
  (`etl_payment_due_t`, `etl_policy_request_issue_t`, `etl_NONAUTO_POLICY_T`) — the chunker
  success criterion cites 3/3 for note 001.

## Review table (26 rows — one per merged note)

| # | source_idx | 标题 (子目录) | 草稿 tables+kind | 草稿 related | 评审结论 (OK / correction) |
|---|-----------|---------------|------------------|--------------|---------------------------|
| 1 | 001 | 公司业务部、武汉中支部门组部门科室等信息更新 (取数口径) | etl_payment_due_t=update; etl_policy_request_issue_t=update; etl_NONAUTO_POLICY_T=update | — | OK |
| 2 | 006 | 高斯数据库存储过程和定时任务 (操作手册) | — | — | OK |
| 3 | 007 | 意外伤害医疗保险 (非车险) | — | — | OK |
| 4 | 009 | 农险查询 (非车险) | — | — | OK |
| 5 | 010 | 财务现金流小程序实现方案 (财务对账) | — | — | OK |
| 6 | 011 | 三责险，车损险保费保额 (汽车险) | — | — | OK |
| 7 | 012 | 定时任务桌面程序 - 完整使用手册 (操作手册) | — | — | OK |
| 8 | 014 | 逐单未到期准备金 (财务对账) | — | — | OK |
| 9 | 015 | 代理业务部车商费用 (汽车险) | — | — | OK |
| 10 | 016 | 视频录制api (操作手册) | — | — | OK |
| 11 | 018 | CREATE OR REPLACE FUNCTION wuh... (取数口径) | — | — | OK |
| 12 | 020 | 财务逐单未到期 (财务对账) | — | — | OK |
| 13 | 026 | 投保交强险代收代缴车船税 (汽车险) | — | — | OK |
| 14 | 027 | 财务税款看板 (财务对账) | — | — | OK |
| 15 | 028 | 新能源车签单口径 (汽车险) | — | — | OK |
| 16 | 029 | 开门红存储过程 (取数口径) | — | — | OK |
| 17 | 031 | 取数工作汇总 (取数口径) | — | — | OK |
| 18 | 032 | 根据保单号查询支付方式，开户行名称，客户账户 (取数口径) | — | — | OK |
| 19 | 033 | 分公司自有车辆赔款清单 (理赔与赔付) | — | — | OK |
| 20 | 036 | 车险报价清单 (汽车险) | — | — | OK |
| 21 | 037 | 个客综合市场部数据看板 (其他) | — | — | OK |
| 22 | 038 | 帆软升级注意事项 (操作手册) | — | — | OK |
| 23 | 039 | 二层宽表存储过程 (取数口径) | etl_nonauto_agreement_t=create; etl_nonauto_policy_t=update; etl_auto_agreement_t=create; etl_policy=update; etl_request_issue_t=create; etl_policy_request_issue_t=update; etl_batch_date=create | — | 删4函数/参数误报，留3个merge into写目标 |
| 24 | 041 | 数据提取字段解读 (取数口径) | — | — | OK |
| 25 | 042 | sql示例 (取数口径) | — | — | OK |
| 26 | 043 | MongoDB 4.4全面解析：从核心概念到实战应用 (操作手册) | — | — | OK |

Highest-density rows (review first): **039** (7 draft tables — function/parameter false-positive
candidates above) and **029** (largest note, 16 sections; reads-only → drafts empty).

## Sign-off

| Field | Value |
|-------|-------|
| Reviewer | 用户（orchestrator 提取 SQL grep 证据、用户逐项批准；LLM 未参与 golden 内容决定） |
| Date | 2026-09-22 |
| Result | approved (policy A — writes-only) |
| Corrections applied to jsonl | 039 -4 / 其余不变 |

Corrections log (note_idx → change):

- **039**: DELETE 4 false positives — all `CREATE OR REPLACE FUNCTION wuh_dw.<name>` artifacts
  (3 PostgreSQL function names + 1 function parameter): `etl_nonauto_agreement_t` (create),
  `etl_auto_agreement_t` (create), `etl_request_issue_t` (create), `etl_batch_date` (create).
  KEEP the 3 real `merge into` write targets as kind update: `etl_nonauto_policy_t`,
  `etl_policy`, `etl_policy_request_issue_t`. Evidence: deterministic SQL grep over the
  knowledge/sql-guides corpus, presented to the user, user approved.
- **All other 25 rows**: unchanged — 001 keeps its 3 write targets (SC-1 anchor); the
  empty-table rows are confirmed by a corpus-wide write-statement scan (zero missed write
  targets beyond 001/039); all `related` stay empty (corpus has zero numeric cross-references).

## Policy decision — writes-only (policy A)

The user chose **policy A: writes-only**. Read-only `FROM`/`JOIN etl_*` references
(~26 across 14 notes) are deliberately NOT added as kind `select`. Accepted implication
(recorded, not a defect to fix in this phase): the live-LLM F1 operator gate scores
against this writes-only golden while the extraction prompt uses "touches" semantics —
a read-touched table the LLM reports will count as a false positive against the
writes-only yardstick. This is a **documented live-gate risk**; revisit only if the
live gate's F1 sits uncomfortably close to the 0.8 threshold.
