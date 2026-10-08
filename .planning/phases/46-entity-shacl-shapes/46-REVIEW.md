---
phase: 46-entity-shacl-shapes
reviewed: 2026-10-08T03:33:50Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - cmd/server/main.go
  - internal/ontology/induction/induction.go
  - internal/ontology/induction/induction_test.go
  - internal/ontology/shacl/entities.ttl
  - internal/ontology/shacl/shacl_test.go
  - internal/ontology/shacl/shapes.ttl
  - internal/ontology/shacl/validate.go
  - verify-phase-46.sh
  - web/src/components/admin/governance/GovernanceTab.tsx
  - web/src/components/admin/governance/ShaclReportView.test.tsx
  - web/src/components/admin/governance/ShaclReportView.tsx
  - web/src/components/admin/governance/types.ts
findings:
  critical: 1
  warning: 8
  info: 7
  total: 16
status: issues_found
---

# Phase 46: Code Review Report

**Reviewed:** 2026-10-08T03:33:50Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

对 Phase 46（Entity SHACL Shapes，D-46-01..04）的 12 个变更文件做了 standard 深度评审，并对 `internal/ontology/store`、`internal/knowledge/ecl/abox.go`、`internal/server/admin_governance.go`、`web/.../governanceApi.ts` 做了跨文件取证。

**红线核对（全部通过，未列为缺陷）：**
- entityCode 零约束：`entities.ttl` 无 entityCode PropertyShape，且 `TestEntitiesShapesParse` 有回归钉（shacl_test.go:1443-1450）——与用户决策 2026-10-04 一致。
- `dangling_edge_count` 无 omitempty：validate.go:118；FE `!= null` 容缺（ShaclReportView.tsx:211）——符合设计。
- boot 时序：main.go :921 `WriteSnapshot` 先于 :935 `Validate`——未被动过。
- severity 进 shapes 数据：validate.go:557-565 数据驱动 + `TestSeveritySourcePathRetired` 退役钉；shapes.ttl 两处 `sh:severity sh:Warning`——D-46-01 核心意图落地。
- store.go / admin_governance.go / governanceApi.ts：git diff 证实零改动。
- FE 零推导：LayerBadge 未知值渲染原值（ShaclReportView.tsx:46，有测试钉）；无 layer 推导逻辑。
- XSS：detail/constraint 全部文本节点渲染，无 dangerouslySetInnerHTML；测试含元素注入负断言（ShaclReportView.test.tsx:186-205）。

**总体评估：** Phase 46 新增代码（path-list 合并、layer 戳、dangling 计数、severity 数据化、FE 分层渲染）质量良好、测试钉密集且红线全部落实。但评审发现 1 个**数据丢失级缺陷**（rollback 整库清空并持久化，Phase 40 遗留）、若干 induction 管线的正确性缺陷（遗留）、一处新代码里的"声明但从未执行"的 shape 集合，以及验证脚本 P2/P4 忽略退出码的空转风险。多数 Critical/Warning 标注了 `遗留`（afad037 之前已存在），供修复排期时区分归属。

---

## Critical Issues

### CR-01: RollbackTBox 清空整个 KG（含 ABox）且通过快照把删除持久化 — 数据丢失风险（遗留，Phase 40）

**File:** `internal/ontology/induction/induction.go:344-364`

**Issue:** Step 6 遍历 `kg.Query("","","")` 对**每一个 subject** 执行 `DeleteSubject`（:346-353），Step 7 仅从回滚后的 TBox TTL 重建（:356）。KG store 中 Phase 39 起持久化的全部 ABox 三元组（ECL 管线产出的 note/table/entity，D-KG-02 明确要求跨重启存活）随之全部丢失。Step 8 立即快照（:361-363）+ `fireChange` 触发 5s 防抖快照（:367）把这个 TBox-only 状态**持久化到磁盘**——下次 boot `onstore.Restore` 加载的就是 ABox 已被清空的快照，删除成为永久状态（除非逐 note 重跑 ECL）。这直接对抗 Phase 39 的 "ABox survives restarts" 护城河与 main.go:917-923 注释声明的语义。D-LOG-03 只要求 TBox 回滚 + SHACL 重验，没有任何决策授权 rollback 删除 ABox。

**Fix:** 回滚只应替换 TBox 层三元组，不动 ABox。最小修复：Step 6 改为只删除「当前 TBox 指针 TTL 里的 subject」（解析当前 `skillforge_tbox.ttl` 得到 TBox subject 集合，仅对这些 subject 做 `DeleteSubject`），Step 7 照旧 `LoadFromTurtle(newTTL)`。若整库清空+重建确为签核过的设计，需在 D-LOG-03/46 文档补记该语义、在 rollback API 响应中显式披露 ABox 丢失规模，且 changelog `What.Removed` 应如实记录被删三元组（当前 `Removed: []string{}` 为空，induction.go:375，与实际行为不符）。

---

## Warnings

### WR-01: Approve 吞掉当前 TBox 读取错误 —— 指针文件可能被「仅含新提案」的 TTL 覆盖（遗留）

**File:** `internal/ontology/induction/induction.go:244-248`

**Issue:** `if data, err := os.ReadFile(current); err == nil { base = string(data) }` —— 读指针 TTL 失败（权限、IO 抖动）时 `base=""` 静默继续，`newTTL` 退化为仅含本次提案三元组，随后 :263-265 用它**原子覆盖指针文件**：整个既有 TBox 被截断销毁，且无任何日志。文件缺失（全新部署）与读错误（异常态）被同等对待。

**Fix:** 区分 `os.IsNotExist(err)`（base="" 合法）与其他错误（返回 error，拒绝 approve）。至少也要 `log.Printf("[ERROR] ...")` 留痕：

```go
data, err := os.ReadFile(current)
switch {
case err == nil:
    base = string(data)
case os.IsNotExist(err):
    base = ""
default:
    return *p, 0, fmt.Errorf("induction: read current TBox %s: %w", current, err)
}
```

### WR-02: Approve 部分失败语义混乱 —— 文件已改、状态未持久化，却向调用方返回 error（遗留）

**File:** `internal/ontology/induction/induction.go:260-296`

**Issue:** Approve 依次写 v{N+1}.ttl → 指针 → 版本文件（:260-268），中途任一步失败即返回 error（此时前序文件已改，版本文件可能落后于指针，重启后 `readVersion` 回退导致 v{N+1} 文件名被下次 approve 复用覆盖）。若三写全 succeeds，:271-275 先插 KG，:278 先把内存态置 `StatusApproved`，**然后** `persistLocked()`（:280）——该调用失败时返回 error，但：TBox/KG 已变更、内存队列标记已批准（重试 Approve 得 "not pending"）、磁盘队列仍是 pending（重启后可再 approve → 重复插三元组、版本再 +1）。另外 `m.kg.Insert`（:273）非 InsertUnique，重复 approve 会产生重复三元组。

**Fix:** 把队列状态持久化挪到 TBox 文件写入**之前**失败窗口之外不可行（需事务），务实的修复序：先 `persistLocked()` 固化 approved 状态 → 再写 TBox 文件 → 最后插 KG；每步失败都返回带「已完成到哪一步」的错误；KG 插入改 `InsertUnique`。至少：`p.Status = StatusApproved` 仅在 `persistLocked()` 成功后设置。

### WR-03: parseTripleLine 以 " a " 分割 —— 描述含 " a " 的提案会向 KG 插入垃圾三元组（遗留）

**File:** `internal/ontology/induction/induction.go:648-671`（配合 ：628-644 proposedTriples）

**Issue:** Approve 对 `ProposedTriples` 的**label 行**（`rdfs:label "…"@zh , "…"@en .`）同样跑 `strings.SplitN(s, " a ", 2)`。只要 descEN/descZH 含 " a "（如 "rule a company"、"representing a business concept"——LLM 按 prompt 产出的单句英文描述极常见），SplitN 返回 2 段，解析出 `Triple{S: 'rdfs:label "规则"@zh , "rule', P: rdf:type, O: 'company"@en'}` 并 `kg.Insert`。垃圾三元组随快照持久化、出现在 provenance graph 中。自产自析（生成后又用文本反解析自己刚拼的行）是根因。

**Fix:** 不做文本反解析——`proposedTriples` 直接返回 `[]store.Triple`（结构化生成、序列化两用），或在 `parseTripleLine` 入口先判 `strings.HasPrefix(s, "rdfs:label ")` 直接跳过 label 行：

```go
if strings.HasPrefix(s, "rdfs:label ") {
    return store.Triple{}, false
}
```

### WR-04: previewIRILocked 前缀子串误报 IRI 冲突（遗留）

**File:** `internal/ontology/induction/induction.go:558-563`

**Issue:** `strings.Contains(string(data), m.ns+localName)` 无右边界：localName "Note" 会与已存在的 `…ontology#NoteChunk` 前缀子串匹配 → `ErrIRIConflict`。后果有二：管理员自定义改名被误拒（`…#Note` 实际空闲）；`uniqueLocalName`（:573-588）被误逼着加后缀（BusinessRule → BusinessRule2）。误报方向单向（不会漏报），但拒绝合法操作是用户可见的错误行为。

**Fix:** 全 IRI 检查加终止定界符——TTL 里 IRI 后只可能是空白、`;`、`,`、`.`、`"`、`>`：

```go
full := m.ns + localName
if strings.Contains(string(data), full) && !hasIRIBoundaryAfter(string(data), full) { ... }
```
或最简单：解析当前 TTL 的 subject/predicate/object 集合做精确匹配，而非子串 Contains。

### WR-05: 8 个 Global* shape 桶只写不读 —— shapes.ttl 声明的全局约束静默不生效（收集为遗留，D-46-02 合并循环延续）

**File:** `internal/ontology/shacl/validate.go:306-314, 429-447, 175-182, 823-854`

**Issue:** `parseShapes` 把 8 类独立 PropertyShape 分桶收集（:429-447），新增的 D-46-02 多文件合并循环把 8 个桶逐文件 append（:175-182）——但**全仓库无任何消费点**（grep 证实只出现在定义与 append 处）。反向同样成立：`checkGlobalReferences`（:823-854）完全不读 `parsed`（参数未用），硬编码只查 `referencesTable → TableSchema`。净效果：
1. shapes.ttl:65-68 声明的 `skf:belongsToNoteShape` 及 :29-62 的 belongsToTier/hasReadAccess/hasWriteAccess/poweredBy/requiresRefusal/hasSubCategory 全局 shape **从未被执行**——shapes 作者会误以为已获 enforcement（当前无活数据踩雷：abox.go 明确不产 belongsToNote，其余由 NodeShape 级 shape 覆盖，但这是数据耦合非引擎保证）；
2. 即便操作者通过 `SKILLFORGE_SHACL_SHAPES` 提供不含 referencesTable 规则的自定义 shapes 列表，硬编码的全局检查依然运行——「shapes 即数据」契约在全局层两个方向都被绕过。

**Fix:** 二选一：（a）实现通用执行——把 8 桶折叠为 `map[string][]propertyConstraint`（path→constraints），`checkGlobalReferences` 遍历执行 sh:class/sh:nodeKind 等检查，Layer 用 `LayerGlobal`；（b）若暂不实现，删除 8 桶与合并循环中的 8 行 append、给 `checkGlobalReferences` 去掉未用参数，并在 shapes.ttl 注释声明全局 shape 仅 referencesTable 受支持。

### WR-06: verify-phase-46.sh P2/P4 丢弃退出码 —— 基建故障可空转通过（新文件）

**File:** `verify-phase-46.sh:95-104, 132-141`

**Issue:** P2/P4 只跑 `waiver_ok`（对日志 grep `--- FAIL`），`wsl` 命令的退出码被丢弃且脚本未开 `set -e`。凡不产生 `--- FAIL` 行的失败一律判绿：`go test` 本身报错（坏 flag、工具链异常）、测试二进制被 OOM kill、WSL 启动间歇失败（日志为空 → `waiver_ok` 空集通过）、或未来把 P2/P4 命令改坏。P1（检查退出码）与 P3（rc + ≥7 --- PASS 下限）部分兜底，但 P2/P4 的 "ontology suite / full internal suite PASSED" 结论在上述场景下是**无证据断言**——对一个以 fail-fast 自我声明的门禁（脚本头注释）这是契约缺口。

**Fix:** 与 P3 对齐，捕获退出码并要求为 0（叠加 waiver 判定）：

```bash
wsl "CGO_ENABLED=1 $GO test ./internal/ontology/... -count=1" >"$WORK/p2.log" 2>&1
p2_rc=$?
if [ "$p2_rc" -ne 0 ] && ! waiver_ok "$WORK/p2.log"; then
  fail_detail "$WORK/p2.log" "P2"
  fail "P2 ontology suite failed (rc=$p2_rc)"
fi
```
（P4 同理；同时对空日志显式 `fail`。）

### WR-07: 治理页重试成功后不清除 shaclError —— 视图永久卡在加载失败（遗留）

**File:** `web/src/components/admin/governance/GovernanceTab.tsx:145-149`

**Issue:** `onRetry` 的 `.then(setShaclReport)` 不清 `shaclError`；`ShaclReportView` 先判 `if (error)`（ShaclReportView.tsx:127），因此一次瞬时失败后，即使重试成功拿到新报告，校验报告视图仍永远渲染「校验报告加载失败（点击重试）」。重试按钮对 view 分支完全失效（HealthBanner 因判 `shaclReport` 优先会恢复，两处状态不一致加重困惑）。

**Fix:**

```tsx
onRetry={() => {
  fetchShaclReport()
    .then((r) => { setShaclError(null); setShaclReport(r); })
    .catch((e: unknown) => setShaclError(e instanceof Error ? e.message : String(e)));
}}
```

### WR-08: Generate 双重 persistLocked —— 第一次写的是「next_id 已推进但不含新提案」的中间态（遗留）

**File:** `internal/ontology/induction/induction.go:205-209`

**Issue:** `persistLocked()`（此时 queue 尚未 append，但 `allocID()` 已把 `nextID+1`）→ `append` → 再 `persistLocked()`。第一次是纯多余的可持久化写；若第二次失败，磁盘上留下 next_id 已推进但提案缺失的队列文件，提案仅存于内存（重启即丢，而 Generate 已返回错误、管理员可能已按返回的 Proposal 行事）。无数据损坏，但属多余 IO + 崩溃窗口。

**Fix:** 删除第一次 `persistLocked()`，只保留 append 后那次；其错误即 Generate 的错误。

---

## Info

### IN-01: 被隔离实体的 warning 级违规从报告中整体消失

**File:** `internal/ontology/shacl/validate.go:259-264`

**Issue:** "Collect pure warnings" 只收集无 error 实体的 warnings；一个实体若同时有 1 error + 1 warning，其 warning 既不进 `Warnings` 也不计入 `WarningCount`，管理员看不到该实体的完整违规画像（隔离区 Violations 数组也只含 error）。

**Fix:** 把被隔离实体的 warnings 附进 `QuarantinedEntity.Violations`（带 severity 区分），或至少在 WarningCount 口径注释中声明该取舍。

### IN-02: Quarantined / Warnings 顺序非确定（map 遍历）

**File:** `internal/ontology/shacl/validate.go:241, 259`

**Issue:** 两个报告分区的构建都遍历 map，Go map 序随机——同一份图每次 boot/重验输出顺序不同，FE 表格行序在两次启动间漂移，削弱可比性（FE 测试按 data-testid 索引，不受影响）。

**Fix:** 输出前按 `entity` 字典序排序（`sort.Slice(report.Quarantined, ...)`）。

### IN-03: danglingEdgePredicates 硬编码三元组族 —— 计数口径窄于 FE 表头承诺

**File:** `internal/ontology/shacl/validate.go:862-880`（对照 `web/src/components/admin/governance/ShaclReportView.tsx:183`）

**Issue:** FE 表头 tooltip 承诺「指向该隔离实体的入边数量」，但引擎只数 `involvesInsuranceProduct / involvesOrganization / parentOrganization` 三条。当前 abox.go:66-82 确实只产这三类 entity 边、且现实可被隔离的只有这两类实体（type/label 均无条件发射），故今天计数正确；但若 (a) abox 新增 entity 边谓词、或 (b) 异常态使 NoteChunk/TableSchema 进隔离区（其 referencesTable 入边不计入），该列会静默显示 0 而实则有入边——按本控制台自己的 Honesty 红线精神属低估披露。属 D-46-04 认可的设计，列为漂移风险备忘。

**Fix:** 在 `danglingEdgePredicates` 定义处加注释指向 abox.go 谓词族作为同步点；或改为「统计全部入边谓词」的通用计数（Query("", "", entity) 按 P 去重过滤 rdf:type）。

### IN-04: Violation.SourcePath / LineStart / LineEnd 引擎从不赋值 —— FE 来源/行号列恒为 ——

**File:** `internal/ontology/shacl/validate.go:97-99`（对照 `ShaclReportView.tsx:222-230, 288-296`）

**Issue:** 结构体有字段、FE 有专列（含 title 提示），但全 shacl 包无任何赋值点（grep 证实）——引擎违规的「来源」「行号」永远渲染 '—'。shapes 文件路径在 parse 时已知（D-46-02 已做 basename 分层），行号可从 TTL 语义近似，当前两列是死 UI（渲染 '—' 至少诚实）。

**Fix:** 要么 parseShapes 记录 shape 来源文件（可顺带行号）并在 stamping 循环赋 `SourcePath`，要么从 FE 删两列、留待 D-PROV-05 真正落地再加。

### IN-05: hasBackfill 判据 'provenance-missing' 从未被任何 BE 版本发射 —— Run ECL 回填按钮不可达死 UI

**File:** `web/src/components/admin/governance/GovernanceTab.tsx:50-52, 64-80`

**Issue:** `git log -S 'provenance-missing'` 证实 BE Go 代码从未产出过 `constraint === 'provenance-missing'`（旧引擎特例只改 severity，不改 constraint 字符串）；D-46-01 之后 warning 形态变为 sourcePath 上的 sh:maxCount/sh:nodeKind（且「缺失」根本不产生 violation）。净效果：回填按钮自 40-07a 起从未出现过，46 之后更是**结构性不可达**——黄色区不再有任何 provenance 缺口信号，`runPipeline` 的治理侧入口成为死代码（PipelineTab 另有独立入口，功能本身可达）。ShaclReportView.test.tsx:170-184 钉住的徽章属「legacy 兼容」，但实际不存在 legacy 生产者。属 D-46-01 决策的连带后果，建议产品侧确认后清理或改挂新信号。

**Fix:** 确认产品意图：若回填入口仍需要，改判据为「BE 可产出的 warning 形态」（如 `w.constraint === 'sh:nodeKind' && w.detail 含 sourcePath`），否则删除 hasBackfill 分支与按钮，并在 46-UI-SPEC 记录该入口退役。

### IN-06: GovernanceTab ↔ ShaclReportView 循环 import

**File:** `web/src/components/admin/governance/ShaclReportView.tsx:18`（对照 `GovernanceTab.tsx:16`）

**Issue:** GovernanceTab 导入 ShaclReportView（default），ShaclReportView 又 re-export GovernanceTab 的 HealthBanner——真循环依赖。ESM/Vite 目前靠渲染期延迟求值容忍，但属脆弱模式（tree-shaking、模块初始化顺序变更时可能炸）。

**Fix:** 把 HealthBanner 抽到独立文件（如 `HealthBanner.tsx`），两个视图都单向导入；re-export 可短期保留兼容。

### IN-07: 默认 shapes 路径是 CWD 相对路径 —— 换工作目录启动即静默降级

**File:** `internal/ontology/shacl/validate.go:293-296`（消费点 `cmd/server/main.go:934-938`）

**Issue:** 默认列表 `internal/ontology/shacl/{shapes,entities}.ttl` 相对进程 CWD 解析。服务器若不在仓库根启动（systemd、容器 WORKDIR、脚本 cd 漂移），两个文件都读不到 → boot 校验 WARN 跳过 + 治理页 503 + rollback 重验恒 0（D-PROV-09 容忍，但三个面同时静默降级且只有一行启动 WARN）。旧代码对 shapes.ttl 已如此，D-46-02 把 entities.ttl 也纳入同一暴露面。

**Fix:** 用 `runtime.Caller(0)` 或 build-time embed（`go:embed entities.ttl shapes.ttl`）锚定仓库内默认路径；至少在 ShapesPathsFromEnv 的 WARN 路径把解析后的绝对路径打进日志（当前 main.go :938 只打 err）。

---

_Reviewed: 2026-10-08T03:33:50Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
