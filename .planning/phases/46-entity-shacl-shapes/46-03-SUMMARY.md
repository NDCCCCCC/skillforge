---
phase: 46-entity-shacl-shapes
plan: 03
subsystem: ontology-shacl-engine
tags: [shacl, boot-wiring, rollback-parity, env-shapes-list, d-46-02, d-prov-09, tdd-red-green]
requires:
  - 46-01 引擎基建（ShapesPathsFromEnv 单点读取 + Validate variadic + ValidateWithTrigger []string）
  - 46-02 entities.ttl 交付（默认双文件列表的第二文件自此真被消费）
  - main.go boot 链时序（:921 WriteSnapshot 先于 Validate，Pitfall 6 红线）
provides:
  - boot shapes 加载 = shacl.ShapesPathsFromEnv()（SKILLFORGE_SHACL_SHAPES 在 main.go 零字面量，默认值收敛进 shacl 包单点）
  - rollback Step 11 重校验读同一 env 列表（induction.go 硬编码 data/shacl/shapes.ttl join 退役，D-46-02 parity 缺口闭合）
  - TestRollbackShaclParity 双 case 回归钉（修复前 rollback SHACL 路径零覆盖）
  - warn-and-return-0 姿态正式钉死（D-PROV-09：SHACL 失败不是 rollback 失败）
affects:
  - 46-04（FE layer/target_class/dangling_edge_count 渲染——数据自本 plan 起进生产报告）
  - 46-05（收口 plan：SHACL-01/02/03 checkbox mark-complete）
tech-stack:
  added: []   # 零新外部包
  patterns:
    - env 单点读取消费点收口（helper 持有默认值，调用方零路径字面量）
    - TDD RED→GREEN 回归钉（修复前行为可观测失败：WARN 行暴露 no-op 路径）
key-files:
  created: []
  modified:
    - cmd/server/main.go
    - internal/ontology/induction/induction.go
    - internal/ontology/induction/induction_test.go
decisions:
  - "D-46-02 收口：boot（main.go :930-931）与 rollback（induction.go Step 11）均消费 shacl.ShapesPathsFromEnv()——两条校验链从 helper 单点取路径列表，组合漂移在类型上不可能；main.go 与 induction.go 均零路径字面量"
  - "main.go 注释措辞规避 grep 自伤：不出现 shacl.ShapesPathsFromEnv() 调用字面量与 SKILLFORGE_SHACL_SHAPES 环境变量名（验收 grep 要求前者==1 仅命中代码行、后者==0），措辞用『env-driven path-list helper / single env read point』表述，语义不损"
  - "Pitfall 6 红线全守：:921 WriteSnapshot 时序零改动、:937-941 quarantine 循环 subject-only 一字未动、:943 SetShaclReport 与 health 日志一字未动、零新增 post-quarantine 快照触发器（git diff 行级核对）"
  - "rollback warn-and-return-0 姿态保持并升级为被钉状态（D-PROV-09）：case 2 t.Setenv 指向不存在路径断言 violations==0 && err==nil"
  - "env 未设置 case 不钉（测试 docstring 记录原因）：默认列表相对 repo root 而包测试 cwd 为包目录，行为依赖进程 cwd——该 fallback 由 shacl 包 TestShapesPaths 覆盖"
  - "REQUIREMENTS SHACL-01/02 checkbox 不在本 plan 勾选（照 45-01/46-01/46-02 先例：留待收口 plan 46-05 mark-complete）"
metrics:
  duration: 11 min
  completed: 2026-10-08
  tasks: 2
  files: 3
---

# Phase 46 Plan 03: boot/rollback shapes 消费点收口 ShapesPathsFromEnv（D-46-02 + D-PROV-09）Summary

**One-liner:** boot（main.go）与 rollback（induction.go Step 11）两个 shapes 消费点统一到 `shacl.ShapesPathsFromEnv()` 单点——induction.go:383 从未生效的硬编码路径（静默 no-op）退役，D-PROV-09 rollback/boot shapes parity 缺口闭合，并由真 RED→GREEN 的 TestRollbackShaclParity 双 case 钉死（修复前 rollback SHACL 重校验零覆盖）；boot 链时序与隔离语义零改动。

## What Was Built

### Task 1 — main.go boot shapes 加载切换 env 列表（D-46-02）→ `be1db74`
- `:930` 的 `shaclShapesPath := envOr("SKILLFORGE_SHACL_SHAPES", ...)` 单路径字面量替换为 `shaclShapesPaths := shacl.ShapesPathsFromEnv()`（默认双文件列表收敛进 shacl 包单点，main.go 内 `SKILLFORGE_SHACL_SHAPES` 字面量归零）；`:931` 改为 variadic `shacl.Validate(ontologyKG, shaclShapesPaths...)`。
- 块注释（:925-929）追加 D-46-02 说明（路径列表化、boot/rollback 同源、单点防漂移）。
- **红线逐条自查通过（git diff -U0 行级核对）**：:921 WriteSnapshot 与 Validate 先后不动；:937-941 quarantine 循环 subject-only 一字未动（D-46-04）；:943 SetShaclReport、:944-950 health 日志一字未动；零新增 post-quarantine onChange/快照触发器（Pitfall 6「错的一半」未犯）；envOr 函数保留（其余 ~30 处 env 仍用）。
- 验收 grep：`shacl.ShapesPathsFromEnv()` == 1（仅代码行）、`SKILLFORGE_SHACL_SHAPES` == 0；CGO_ENABLED=1 build + vet exit 0。

### Task 2 — induction.go rollback 读 env 列表 + TestRollbackShaclParity（TDD RED→GREEN）
**RED → `0f13b47`**：induction_test.go 新增 TestRollbackShaclParity（装配逐字照 TestRollbackLatency：v1/v2 TTL + pointer + tbox_version.txt + proposals.json + `kg.LoadFromTurtle(v2TTL)` + `NewManager(testNS, dir, nil, kg)` + `RollbackTBox(1, "test rollback")`；shapes fixture 用 os.WriteFile 写入 t.TempDir）：
- case 1 `env_list_consumed_violations_reported`：t.Setenv 指向 fixture（全局 `skf:referencesTableShape a sh:PropertyShape ; sh:path skf:referencesTable ; sh:class skf:TableSchema .`），v1 TTL 含 `skf:BadNote a skf:NoteChunk . skf:BadNote skf:referencesTable skf:MissingTable .` 悬空引用 → 断言 violations > 0。**修复前该 case 失败，且 WARN 行现场暴露根因**：`[WARN] induction: post-rollback SHACL validation: shacl: parse shapes: read shapes file: open /tmp/.../shacl/shapes.ttl: no such file or directory`——硬编码 join 指向的目录不存在、重校验恒 no-op 返回 0，parity 缺口被日志实锤。
- case 2 `missing_shapes_warn_and_return_zero`：t.Setenv 指向不存在路径 → 断言 violations == 0 && err == nil（D-PROV-09 warn-and-return-0 姿态）。
- 两个子测试顺序执行（无 t.Parallel，t.Setenv 约束）；docstring 标注 D-46-02 + D-PROV-09 + 修复前零覆盖背景 + env 未设置 case 不钉的原因。

**GREEN → `35a24bc`**：induction.go Step 11 删除 `shapesPath := filepath.Join(m.dataDir, "..", "shacl", "shapes.ttl")` 行及 46-01 遗留 TODO(46-03) 注释，替换为 `shacl.ValidateWithTrigger(m.kg, shacl.ShapesPathsFromEnv(), "rollback")`；warn-and-return-0 分支原样保留、注释追加 D-46-02 parity 引用。git diff 证实仅 Step 11 块变更——Step 8（快照写 :360-364）/ Step 9（fireChange）/ Step 10（changelog）零行变更；filepath import 保留（:335/:341/:361 仍用）。

## Verification

- RED gate：`-run TestRollbackShaclParity -v` → case 1 FAIL（violations==0，WARN 暴露 no-op 路径）、case 2 PASS——与 plan 预测逐字一致
- GREEN gate：同命令 → 2/2 子测试 PASS（case 1 不再出现 WARN，env fixture 真被消费）
- 验收 grep：induction.go `dataDir, "..", "shacl"` == 0（硬编码退役）、`shacl.ShapesPathsFromEnv()` == 1、`TODO(46-03)` 残留 == 0
- 全套件：`go test ./internal/ontology/induction/... ./internal/ontology/shacl/... -count=1` 双包 ok（induction 零回归、shacl 零回归）
- 最终 gate：`CGO_ENABLED=1 go build ./...` 全树 exit 0（FULL-GATE-GREEN）
- 三个 commit 零文件删除（`git diff --diff-filter=D` 空）；build 残留二进制（go/server）mtime 早于本 plan，确认非本 run 产物、未纳入 commit

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] LoadFromTurtle 双返回值编译错误**
- **发现于：** Task 2 RED 阶段
- **问题：** 测试初稿写 `if err := kg.LoadFromTurtle(v2TTL); err != nil`，但 `Store.LoadFromTurtle` 签名为 `(int, error)`——单变量接双值编译失败（TestRollbackLatency 以纯语句调用故未暴露）
- **修复：** 两处改 `if _, err := kg.LoadFromTurtle(v2TTL)`
- **文件：** internal/ontology/induction/induction_test.go
- **提交：** 0f13b47（RED commit 内已带修复，编译通过后才构成有效 RED）

### Process Notes（非代码偏差）

1. **Task 1 注释措辞自纠**：初稿注释含 `shacl.ShapesPathsFromEnv()` 调用字面量与环境变量名，会使验收 grep（==1 / ==0）翻红——提交前改写为不含字面量序列的等义表述。属验收门自洽性修正，非行为变更。
2. **gsd-sdk CLI 不在本机 PATH**：STATE.md / ROADMAP.md 按 SDK handler 等价语义手工更新（照 46-01/46-02 先例）。
3. **REQUIREMENTS.md 不动**：plan frontmatter 列 requirements: [SHACL-01, SHACL-02]，但照 45-01/46-01/46-02 三连先例，SHACL-01/02/03 checkbox 由收口 plan（46-05）统一 mark-complete。

## TDD Gate Compliance

Task 2 `tdd="true"` 走了完整真 RED→GREEN：`test(46-03)`（0f13b47，case 1 失败且 WARN 实锤 no-op 根因）先于 `feat(46-03)`（35a24bc，转绿）——修复前该路径零覆盖、修复后双 case 钉死，gates 顺序合规。REFACTOR 无需（实现 8 行新增含注释，无可清理面）。

## Known Stubs

None. 两条消费点均为真实生产路径：boot 默认双文件列表解析 shapes.ttl + entities.ttl（46-02 交付物自本 plan 起进 boot 校验与 rollback 重校验）；FE 渲染属 46-04 范围，非 stub。

## Threat Posture

- T-46-01（env 误配）mitigate 保持：boot 侧 parse 失败 WARN + 跳过（D-SHACL-02 既有）、rollback 侧 warn-and-return-0（D-PROV-09 既有，本 plan 钉死为 case 2）——两条姿态零改动。
- T-46-04（Pitfall 6 boot 时序 DoS/永久化）mitigate：acceptance 红线以 git diff -U0 行级核对通过——零新增 post-quarantine 快照触发器，隔离不落快照、报告每 boot 稳定呈现。
- T-46-SC：零新外部包、零安装动作。
- 无计划外新增攻击面 → 无 Threat Flags。

## Commits

| Task | Commit | Files |
| ---- | ------ | ----- |
| 1 | `be1db74` feat(46-03): boot shapes load via shacl.ShapesPathsFromEnv (D-46-02) | cmd/server/main.go |
| 2 RED | `0f13b47` test(46-03): add failing TestRollbackShaclParity regression (RED) | internal/ontology/induction/induction_test.go |
| 2 GREEN | `35a24bc` feat(46-03): rollback SHACL revalidation reads env shapes list (D-46-02/D-PROV-09) | internal/ontology/induction/induction.go |

## Self-Check: PASSED

- [x] cmd/server/main.go / internal/ontology/induction/induction.go / internal/ontology/induction/induction_test.go 均存在
- [x] commits be1db74 / 0f13b47 / 35a24bc 均在 git log（test 先于 feat，RED→GREEN gates 顺序合规）
- [x] 最终 gate 复跑：CGO_ENABLED=1 build ./... + induction/shacl 双包 -count=1 全绿
- [x] 零文件删除；验收 grep 全过（main.go FromEnv()==1 / env 字面量==0；induction.go 硬编码 join==0 / FromEnv()==1 / TODO 残留==0）
