# Quick Task 260929-d5w Summary — 批量修复全部遗留隐藏缺陷 (6 项)

**One-liner:** D-NZK-DEF-01 姊妹 resolver cat_ 缺陷修复 + TBox 浏览卡片动态化 + eval frozen 扫描修复(锁 schema 对齐 + D6-13 sidecar 过滤) + KB 乱码数据修复 + store/entity_iri 质量扫描(IN-01/03/04/05 + 2 lint);活体服务 rebuild+restart 全链路验证。

**Date:** 2026-09-29
**Duration:** ~40 min (first commit 09:37 → last 10:00)
**Branch:** phase-20-p10-6-admin-console

## Tasks Executed

| # | Task | Type | Commit | Key Files |
|---|------|------|--------|-----------|
| 1 | 姊妹 resolver `(*Deps).resolveSkillTiers` cat_ 缺陷 (TDD) | test+fix | `c4cb264` (RED) → `d7f4553` (GREEN) | `internal/server/admin_ontology.go`, `internal/server/admin_ontology_def01_test.go` (new) |
| 2 | TBox 浏览类卡片动态化 (6→10 类) | fix | `b7f6664` | `web/src/components/admin/ontology/TBoxBrowser.tsx`, `OntologyTab.test.tsx` |
| 3a | eval frozen lock 字段名错位 + 扫描修复 | fix | `1303db6` | `eval/frozen/data.lock.json`, `internal/server/eval.go` |
| 3b | KB uat-check 乱码 | data | (live DB, 不入 git) | `skillforge.db` knowledge_bases.display_name |
| 4 | store.go + entity_iri.go 质量扫描 (IN-01/03/04/05 + lint) | chore | `4c8a076` | `internal/ontology/store/store.go`, `internal/ontology/store/store_label_passthrough_test.go`, `internal/knowledge/ecl/entity_iri.go` |
| — | gofmt 余量 (admin_ontology.go 既有 struct tag 对齐) | style | `9891375` | `internal/server/admin_ontology.go` |

## Task Details + Evidence

### Task 1 — 姊妹 resolver cat_ 缺陷 (D-NZK-DEF-01 family)

`(*Deps).resolveSkillTiers` (喂 `/admin/ontology` KG 查询面板 `?skill=`) 与 260928-wzl 修掉的 `resolveSkillTiersFromKG` 是同一缺陷: hasReadAccess 对象直接 TrimPrefix → tier 名为 `cat_sql-guides` → QueryScoped 匹配不到任何 belongsToTier → 面板全 ABox fail-closed,而同一 KG 走 `/api/sparql` 有行。

- **RED** (`c4cb264`): 6 测试,运行确认失败 (`"cat_sql-guides"` 泄漏 + ABox 不可见)。
- **GREEN** (`d7f4553`): 移植 76b5290 归一化 — Tier individual 直取 / KBCategory 经 belongsToTier 展开 / 未知对象跳过 (fail-closed);去重保序;补 nil-guard 与姊妹 resolver 对齐。
- 活体验证: `GET /api/admin/ontology/kg/query?skill=sql-guides` → 200, **669 matches / 26 ABox 行可见** (note_001/006/007...);同一时刻 `/api/sparql` X-Skill-Name=sql-guides → count 669。**一个 KG、两个 resolver、同一答案。**

### Task 2 — TBox 浏览类卡片动态化

TBOX_CLASSES 硬编码 Phase 38 六类;TBox v7 十类 (InsuranceProduct/Organization/NoteChunk/TableSchema 不显示)。挂载时 `fetchKGQuery('', RDF_TYPE, owl:Class)` 取全部类,与静态元数据合并: 静态有的用静态中文 label,新发现生成兜底卡片 (local name 即 label,`localeCompare` 排到尾部)。`data-testid=tbox-class-${local}` 形状不变;发现失败时静态卡集兜底 (best-effort)。

- Verify: `npx tsc --noEmit` 0 错误;`vitest run src/components/admin` **7 files / 58 tests 全绿** (含新增动态卡片用例: 4 兜底卡渲染 + 排序断言 + 总数 10)。

### Task 3a — eval frozen lock 字段名错位 (见 Deviation #1)

`eval/frozen/data.lock.json` 用 `"model_version"`,Go 侧 `internal/eval/frozen.go` LockFile 期望 `json:"model_ver"` → `FrozenSets()` 全体失败 → `GET /api/eval/frozen` 500 + WARN。

- JSON: key `model_version` → `model_ver`,**值 v0.8 与其余字节全部不动** (完整性文件,schema 字段名由 Go 侧定义)。
- 活体验证: rebuild+restart 后 `GET /api/eval/frozen` → **200** `[{skill: compliance, path: eval/frozen/compliance.jsonl}]`;`~/sf44.log` 无 `frozen sets scan failed` WARN。

### Task 3b — KB 乱码

`knowledge_bases` 表 uat-check 行 display_name 为 mojibake「UAT????」(历史编码事故)。服务停止状态下: `.backup /tmp/sf-backup.db` → `UPDATE display_name = 'UAT Test KB' WHERE slug='uat-check'` (ASCII 名,不再编码事故;行保留,phase-26 审计轨迹不动) → 验证 → 删临时备份。

- 活体验证: `GET /api/admin/kb` → 200,uat-check `display_name == 'UAT Test KB'`。

### Task 4 — store.go + entity_iri.go 质量扫描

- **IN-01**: Path B seen map 补容量 hint `make(map[string]bool, len(s.all))` (与 nil-tier dedup path 一致)。
- **IN-03**: Path B 循环 `for i, t := range` → `for _, t := range`,删 `_ = i`。
- **IN-05**: schemaClasses var+init() 两步 → 编译期 map literal 单步 (确认包内唯一 init() 无其他副作用)。
- **lint**: `interface{}` → `any` ×2 导出签名 (SetShaclReport/GetShaclReport) + 相邻两处注释 (别名,零调用点破坏)。
- **lint**: `asciiPascalCase` range `strings.Split` → `strings.SplitSeq` (Go 1.25.5,与 knowledge.go:534 先例一致)。
- **IN-04**: 新增 `TestStore_SchemaIndividualLabelTierVisibility` — skill individual 有 label 无 belongsToTier: rdf:type 行 (schemaClasses arm) 任何视图可见;label 行 tier 视图 fail-closed 排除、admin baseline (Path A) 可见。
- **IN-02 不做** — 见下方 rationale。
- 全量门: `CGO_ENABLED=1 go test ./internal/... -count=1` **全绿**;11 SPARQL + 9 HybridSearchScoped + 11 def01-family 回归全绿;gofmt clean。

### IN-02 Skip Rationale (dedup key '|' 分隔符)

`QueryScoped`/nil-tier path 的 dedup key 为 `S+"|"+P+"|"+O`。理论碰撞要求 IRI 或 literal 内含 `|` 且按特定错位拼接重合 — 而 SkillForge 的 S/P 恒为 ontology IRI (命名空间 `http://skillforge.local/ontology#`,合法 IRI 不含 `|`),O 的 literal 形态带引号包裹。改 key 格式 (如 length-prefix 或 JSON 编码) 对零个可达输入产生不同结果,却触碰 D-SPARQL-02/44 全部回归断言的性能路径。**收益为零、风险为正 → 不改**;若未来允许自由文本进入 S/P,再重访。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Task 3a 处方不足 — FrozenSets 收窄为 D6-13 sidecar 过滤**
- **Found during:** Task 3a 验证 (rename 后 re-probe)
- **Issue:** 改名只解决 data.lock.json 的第一个缺失字段;诊断遗漏了同类事实 — fanout/hitl/platform-floor 三个 milestone 完整性 pin 同样缺 LockFile 必填字段 (prompt_hash/eval_set_version/skill_name),deep/kpi 锁的是 eval/golden/ 下的 set (eval/frozen 无 same-stem .jsonl)。仅改名后扫描仍 500,验证目标「200 + 无 WARN」不可达。
- **Fix:** `FrozenSets()` 在 LoadLock 前过滤 — 仅 same-stem `.jsonl` 存在的锁视为 D6-13 frozen-set sidecar (与函数自身 doc comment 契约一致);无 same-stem set 的 milestone pin 跳过;真 sidecar (compliance) 保留 malformed → 整体 STOP 语义。
- **Files modified:** `internal/server/eval.go`
- **Commit:** `1303db6`
- **结果:** 扫描返回 1 set (compliance),200,无 WARN。FE 侧此前从未拿到过 200 (data.lock 按字母序先炸),无回归面。

**2. [Rule 1 - Test assertion] IN-04 测试按已记录的 D-SPARQL-08 语义校正断言**
- **Found during:** Task 4 首轮 suite
- **Issue:** 我最初断言 `QueryScoped(S=skill_x, P=label, tiers)` 为空;实际 Path B 中 schema 三元组在 S/P/O pattern filter 之前短路 (44-05/44-06 已记录并校正过的既有语义),rdf:type 行会被返回。
- **Fix:** 载荷断言改为 subject 级 (Path B 对 skill_x 只返回 1 行且 P=rdf:type,label 不泄漏);pred-scoped 短路行为属既有文档语义,不在本任务范围。
- **Files modified:** `internal/ontology/store/store_label_passthrough_test.go`
- **Commit:** `4c8a076`

**3. [style] admin_ontology.go 既有 struct tag 对齐**
- **Found during:** Task 4 gofmt 全文件扫描
- **Fix:** 纯空白 diff (changelog/rollback response struct tag 对齐),单独 style commit `9891375`。

## Verification Summary

| Gate | Result |
|------|--------|
| RED (Task 1) | 6 tests fail as expected (`cat_sql-guides` 泄漏) |
| `go test ./internal/server -run ResolveSkillTiers...` | 11/11 PASS (6 新 + 5 def01 既有) |
| `CGO_ENABLED=1 go test ./internal/... -count=1` | 全绿 (无 FAIL 输出) |
| 10 SPARQL + 9 Hybrid + def01 回归 | 11 + 9 + 11 PASS |
| `npx tsc --noEmit` | 0 errors |
| `npx vitest run src/components/admin` | 7 files / 58 tests PASS |
| Live: rebuild + restart (pid 743, port 8888) | BUILD_OK, 29s 起 |
| Live: `GET /api/eval/frozen` | 200, 1 set (compliance), 无 WARN |
| Live: `GET /api/admin/kb` | uat-check = 'UAT Test KB' |
| Live: `?skill=sql-guides` ontology 面板 | 200, 669 matches, 26 ABox 行可见 |
| Live: `/api/sparql` (sql-guides) | 200, count 669 (与面板一致) |
| sqlite 操作 | backup → UPDATE → verify → backup 删除 |

验证脚本: `.planning/quick/260929-d5w-rebuild-restart.sh`,`.planning/quick/260929-d5w-verify-live.sh` (与 sf44-*.sh 家族同类,按惯例不入 git)。

## Out-of-Scope Observations (不修,记录)

- 重启后 boot log 有 `[WARN] SHACL validation: health=error, errors=7, warnings=0, quarantined=7` — 来自 260928-nzk 实体个体批量入 KG 的既有数据 (D-SHACL-02 隔离路径按设计工作,不阻塞 boot);本任务 diff 不触碰 SHACL shapes/KG 内容,不属本任务范围。
- `web/pnpm-lock.yaml` / `pnpm-workspace.yaml` 及 `internal/server/skillforge-server-test-*` 目录为会话前既有 untracked 文件,未动。

## Commits

| Commit | Message |
|--------|---------|
| `c4cb264` | test(quick-260929-d5w): RED — sister resolver (*Deps).resolveSkillTiers cat_ defect |
| `d7f4553` | fix(quick-260929-d5w): normalize hasReadAccess objects in (*Deps).resolveSkillTiers |
| `b7f6664` | fix(quick-260929-d5w): TBox browser discovers owl:Class individuals beyond static cards |
| `1303db6` | fix(quick-260929-d5w): align data.lock.json schema key + scope frozen scan to D6-13 sidecars |
| `4c8a076` | chore(quick-260929-d5w): Phase 44 review IN-01/03/04/05 + interface{}→any/SplitSeq lint |
| `9891375` | style(quick-260929-d5w): gofmt admin_ontology.go struct tag alignment |

## Self-Check: PASSED

- [x] 6 commits exist in `git log` (c4cb264 → 9891375)
- [x] `internal/server/admin_ontology_def01_test.go` exists (new, committed)
- [x] Working tree clean for all task-touched tracked files (`git status` — 仅会话前既有 untracked)
- [x] Live server rebuilt from HEAD task code and serving on :8888 (pid 743)
- [x] data.lock.json diff = single key rename (value v0.8 untouched)
- [x] TDD gate: test commit `c4cb264` precedes feat commit `d7f4553`
