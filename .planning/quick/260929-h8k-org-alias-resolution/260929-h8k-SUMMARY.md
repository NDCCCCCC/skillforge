---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick Task 260929-h8k: 机构实体别名消解 Summary

**机构别名确定性归一化 —— 武汉/武汉中心支公司 式 16 对重复 org 实体在投影层与抽取层双路合并**

- **Date:** 2026-09-29
- **Branch:** phase-20-p10-6-admin-console
- **Commits:** `8660cbf` (test RED) → `ee1b10f` (feat GREEN)

## What Was Done

### Task 1 — BE: 机构名归一化规则 + 投影合并 (TDD)

- **新文件 `internal/server/entity_alias.go`**:
  - `orgSuffixes` / `orgSuffixRe`: 封闭机构后缀语法 `(中心支公司|支公司|分公司|分部|部门组|科室)$`。
  - `canonicalOrgLabel(label, all)`: 后缀结尾 → 原样(terminal);否则枚举 label+单后缀,命中取最长(武汉 → 武汉中心支公司,即使 武汉支公司 同时存在);无命中原样。幂等(canonical(canonical(x))==x),不可能链式/成环。
  - `mergeOrgAliases(kg, acc)`: 两遍。Pass 1 建立 **全库级** label set(所有 rdf:type Organization individual 的 rdfs:label + 投影内 org 节点)——canonical 解析与 notes= 过滤范围无关,过滤视图与全库视图对同一实体渲染同一 canonical 名。Pass 2 按 canonical 分组,代表 = label==canonical 的 individual(不在投影范围时物化其节点),边重指向代表,**边多重性保留**,自环剔除。
  - 三条设计承诺已写入代码注释: 为什么只对 org(产品名无后缀语法,责任险 vs 雇主责任险是修饰语差异)、为什么 canon=长名(长名是具体法律机构,持有 parentOrganization 边)、为什么确定性规则而非模糊匹配(信任层不做猜测性合并,编辑距离/向量会融合 武汉江南 vs 江北中心支公司 这类刻意的不同拼写)。
- **`internal/server/admin_ontology.go`**: `projectOntologyGraph` 在构建响应前调用 `mergeOrgAliases`。保险产品/table/note 节点不受影响。FE (`OntologyGraphView.tsx`) 零改动。

### Task 2 — BE: 抽取期同规则

- **`internal/knowledge/ecl/stub.go`**: `normalizeOrgAliases` 挂在 `sanitizeEntities` 尾部(D-NZK-04 门禁之后): 批内存在另一 org 名 = name+后缀 → 丢弃短名实体(note→entity 边随之坍缩到长名实体自己的边,即"边指向长名"语义),**Parent 引用同步改写**(否则 abox.go 的 orgByName 精确名解析会静默丢 parent 边)。后缀结尾名 terminal 永不丢弃;批内无长名则短名原样(批间由既有 InsertUnique + 投影合并兜底)。规则副本与 server 侧交叉引用,注释声明保持同步。

## Tests (RED → GREEN)

- `internal/server/entity_alias_test.go`: canonicalOrgLabel 8 例表驱动(最长命中 / terminal / 公安支公司→自身 / 个信保→个信保分部 / 责任险不参与 / 总公司不受"公司"误伤 / 无命中原样);投影集成测 orgAliasFixtureKG(武汉+武汉中心支公司+短名带 parent hop)→ 2 org 节点(代表+父)、2 条 involves_org 同指代表、parent_org 改写后来自代表。
- `internal/knowledge/ecl/org_alias_test.go`: 批内 武汉+武汉中心支公司 → 只出长名;某支公司(Parent:武汉) → Parent 改写为武汉中心支公司;责任险/雇主责任险产品对不合并;无长名批 → 短名存活;多候选取最长;terminal 名在有更长兄弟时仍保留。

## Verification

- WSL `/usr/local/go/bin/go build ./...` — OK
- `CGO_ENABLED=1 go test ./internal/...` — 全绿,除 1 个 **pre-existing** 失败(见 Deferred Issues,与本次改动无依赖关系)
- 活体(rebuild `~/skillforge-server-44` + `sf44-restart.sh`,port 8888 up 29s):
  - `GET /api/admin/ontology/graph`: **org 节点 56 → 38**(≤40 达标,规则比人工审计的 16 对多合并 2 对)
  - `label=武汉` org 节点: **不存在**(已并入 武汉中心支公司 = `ent_e4f27761d1552`,sql-guides tier)
  - 4 条 involves_org 边(note_001/027/031/037)全部指向合并后代表节点;rep 的 parent hop(→ 湖北分公司)完整;dangling 边 = 0
  - 剩余 16 个无后缀 org label 均为合法非别名(部门/供应商/全称: AWS、腾讯云、代理业务部、众诚汽车保险股份有限公司…)

## Deviations from Plan

**1. [Rule 1 - Bug] 重指向后的边不再二次去重(初版实现误删)**
- **Found during:** Task 1 GREEN
- **Issue:** 任务描述同时写了"重复边去重"与集成测期望"2 条边同目标"。初版 merge 在重指向后按 from|to|type 去重,把 note→武汉 与 note→武汉中心支公司 重指向后的两条边坍缩成 1 条,集成测失败。
- **Fix:** 精确重复在 accumulator 阶段已去重;合并阶段保留边多重性(每条 involves_org 边 = 一次真实提及),自环仍剔除。FE cytoscape 的元素 id 为 from|to|type,重复边在渲染层自然收敛为一条,无副作用。
- **Files modified:** internal/server/entity_alias.go
- **Commit:** ee1b10f

## Deferred Issues

- `internal/ontology/emitter` `TestEmitSubCategoryInstanceCount` 失败(SubCategory instances = 6, want 7): 该测试 pin 的是**磁盘上 skills/ + knowledge/ 语料**(git 未跟踪目录)派生的实例数,近期语料重组(1aaf853 等)后计数漂移。与本次改动无依赖关系(emitter 不 import ecl/server;本次未触碰 skills/knowledge)。pre-existing,详见 deferred-items.md。

## TDD Gate Compliance

- RED gate: `8660cbf` test(quick-260929-h8k) — 两个测试文件在实现前运行均为失败(编译失败 + 断言失败已实测)
- GREEN gate: `ee1b10f` feat(quick-260929-h8k) — 全部转绿

## Self-Check: PASSED

- [x] internal/server/entity_alias.go 存在(commit ee1b10f)
- [x] internal/server/entity_alias_test.go、internal/knowledge/ecl/org_alias_test.go 存在(commit 8660cbf)
- [x] git log 含 8660cbf 与 ee1b10f
- [x] 活体指标: org 38 ≤ 40;武汉 已消失;4 条 note 边指向武汉中心支公司;dangling 0
