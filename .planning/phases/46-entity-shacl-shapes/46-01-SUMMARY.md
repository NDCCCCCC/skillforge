---
phase: 46-entity-shacl-shapes
plan: 01
subsystem: ontology-shacl-engine
tags: [shacl, severity, shapes-list, engine, d-46-01, d-46-02]
requires:
  - Phase 40 SHACL 引擎（7-constraint switch + quarantine 分流）
  - D-PROV-10 sourcePath warning 语义（退役对象）
provides:
  - sh:severity 数据驱动解析（Warning/Info→warning；Violation/未声明/未知→error）
  - Validate(kg, ...string) variadic + ValidateWithTrigger(kg, []string, trigger)
  - ShapesPathsFromEnv()（SKILLFORGE_SHACL_SHAPES 唯一读取点，默认双文件列表）
  - 多文件 parse + errors.Join 聚合（缺失路径不静默）
  - nodeShape.Layer parse 期源文件追踪（tbox/entity；LayerGlobal 常量就位）
affects:
  - 46-02（entities.ttl 交付后默认列表即生效）
  - 46-03（boot/rollback 装配切 ShapesPathsFromEnv）
  - 46-04（FE layer 徽章消费 Layer 枚举字符串）
tech-stack:
  added: []   # 零新外部包（Go stdlib errors / path/filepath）
  patterns:
    - severity 单字段流经全部 check*（Phase 40 留好的扩展点，本 plan 零 check* 改动）
    - filepath.SplitList / ListSeparator 对偶（env 解析与报告 join 同一分隔符语义）
    - parse 期源文件 basename → Layer 盖戳（零类名硬编码）
key-files:
  created: []
  modified:
    - internal/ontology/shacl/validate.go
    - internal/ontology/shacl/shapes.ttl
    - internal/ontology/shacl/shacl_test.go
    - internal/ontology/induction/induction.go
decisions:
  - "D-46-01 引擎侧：severity 解析仅在 PropertyShape 粒度（NodeShape 级不读，Pitfall 3）；sh:Warning/sh:Info→warning，其余一律 error；默认 error 保证既有 8 TBox shapes 行为字节不变"
  - "D-46-02 引擎侧：任一 shapes 路径 parse/read 失败 → errors.Join 聚合全部失败路径后整体 error（绝不半套 shapes 静默校验）"
  - "Layer 派生 = parse 期 filepath.Base(path)==\"entities.ttl\" 盖 LayerEntity、否则 LayerTBox（用户决策 2026-10-04，零类名硬编码）"
  - "Report.ShapesPath = strings.Join(paths, filepath.ListSeparator)（与 SplitList 对偶；单路径显示与旧行为一致，FE 类型零变化）"
  - "induction.go:384 仅做机械 []string 签名适配，硬编码路径保留 + TODO(46-03) 标注（env 列表化切换归 46-03）"
  - "REQUIREMENTS SHACL-01/02 checkbox 不在本 plan 勾选（照 45-01 先例：需求由 46-02/46-03 收口后才 mark-complete）"
metrics:
  duration: 10 min
  completed: 2026-10-08
  tasks: 3
  files: 4
---

# Phase 46 Plan 01: SHACL 引擎机制扩展（severity 归位 + 路径列表化）Summary

**One-liner:** `sh:severity` 政策从引擎硬编码归位 shapes 数据（W3C §2.1.4 三值映射，sourcePath 特例退役）+ shapes 路径列表化基建（variadic Validate、ShapesPathsFromEnv env 单点、errors.Join 多文件聚合、nodeShape.Layer 源文件追踪），既有 13 测试断言零改动全绿。

## What Was Built

### Task 1 — severity 解析 + sourcePath 特例退役（D-46-01）→ `52ca61e`
- `parsePropertyShape` 删除 `if c.Path == pf("sourcePath")` 硬编码分支（D-PROV-10 引擎特例退役），替换为 `sh:severity` triple 扫描：`sh:Warning`/`sh:Info` → `SeverityWarning`；`sh:Violation`/未知 IRI/未声明 → `SeverityError`（默认先置 error 再解析，W3C SHACL §2.1.4 "sh:Violation is the default if sh:severity is unspecified"）。severity 仅在 PropertyShape 粒度解析（Pitfall 3：NodeShape 级不读，本 plan 不改粒度）。
- 包 docstring 追加 D-46-01 三值映射说明 + 默认 error 的向后兼容保证。
- `shapes.ttl`：`skf:NoteChunk_sourcePath` / `skf:TableSchema_sourcePath` 显式补 `sh:severity sh:Warning`（政策归位数据，retrofit 后行为字节不变）；头注释修正——陈旧的「default data/ontology/shacl.ttl」改为 D-46-02 路径列表描述，D-SHACL-04 行按实际 9 个 NodeShape（8 targetClass + `skf:IRIPropertyShape` 无 targetClass，Pitfall 10）重写。
- `shacl_test.go` `TestShaclViolation`：fixture 单行补丁 `sh:severity sh:Warning`（Pitfall 1 红线口径内唯一允许的数据补丁），`WarningCount==1 / 0 quarantined / health=warn` 断言零改动通过。
- 验收 grep：`pf("sourcePath")` severity 语境命中 0；shapes.ttl severity 行 =2；validate.go D-46-01 引用 =2。

### Task 2 — Validate variadic + ShapesPathsFromEnv + merge + Layer（D-46-02）→ `2c8369b`
- `Validate(kg *store.Store, shapesPaths ...string)`：13 处既有测试单路径调用零改动编译通过；转发 `ValidateWithTrigger(kg, shapesPaths, "boot")`。
- `ValidateWithTrigger(kg, []string, trigger)`：逐路径 `parseShapes` → 单个 `parsedShapes` append merge（NodeShapes + 8 个 Global* slice）；任一路径失败 → `errors.Join` 聚合全部失败路径 → 整体 `return nil, fmt.Errorf("shacl: parse shapes: %w", ...)`——不静默跳过、不产半套 shapes（Pitfall 5，main.go 既有 WARN 路径消费）。
- `nodeShape.Layer` 字段 + 包级常量 `LayerTBox`/`LayerEntity`/`LayerGlobal`（46-UI-SPEC ViolationLayer 枚举锁定）；merge 期按 `filepath.Base(path)=="entities.ttl"` 盖 Layer（用户决策 2026-10-04：零类名硬编码）。
- `Report.ShapesPath = strings.Join(shapesPaths, string(filepath.ListSeparator))`（与 SplitList 对偶；单路径无分隔符，FE 类型零变化）。
- `ShapesPathsFromEnv()`：env 非空 → `filepath.SplitList`；空/未设置 → 默认 `[]string{shapes.ttl, entities.ttl}`（先判空再 SplitList，Pitfall 4：SplitList("") 返回空 slice 会导致零 shapes 静默通过）；docstring 注明唯一读取点（boot/rollback 共用防组合漂移）。
- `induction.go:384` 机械适配 `[]string{shapesPath}` + `TODO(46-03)`（不改硬编码路径本身）。
- 验证：CGO_ENABLED=1 全树 `go build ./...` exit 0；shacl + induction 包测试全绿。

### Task 3 — TestShapesPaths + TestSeverity 回归钉子 → `1bfe9a8`
- `TestShapesPaths`（SHACL-01 Test Map 行）：env 空串→默认双文件列表（后缀断言 shapes.ttl/entities.ttl）；单路径→单元素；多路径以 `filepath.ListSeparator` join→两元素（跨 OS 安全构造）；缺失单路径→非 nil error；多文件列表含缺失路径→整体失败不静默（Pitfall 5 钉子）。
- `TestSeverity`（SHACL-02 Test Map 行）：`sh:Warning`/`sh:Info` → WarningCount==1、0 quarantined、health=warn；未声明（W3C 默认）/`sh:Violation` → ErrorCount>=1、kb1 进 Quarantined、health=error。
- `TestSeveritySourcePathRetired`：sourcePath PropertyShape 无显式 severity → error 级违规 + 隔离——证明引擎路径名特例已退役，行为由 shapes 数据决定。
- `writeTempShapes` helper（CreateTemp idiom + 错误处理 + t.Cleanup）；既有 13 测试 diff 纯新增、零删改（git 证实 0 deletions）。

## Verification

- `CGO_ENABLED=0 go test ./internal/ontology/shacl/... -count=1`：16 顶级测试全 PASS（13 既有 + 3 新增，含子测试共 24 PASS）
- `CGO_ENABLED=1 go build ./...`：exit 0（induction.go 适配后全树可编译）
- `CGO_ENABLED=0 go test ./internal/ontology/shacl/... ./internal/ontology/induction/... -count=1`：两包全绿（induction 零回归）
- 最终 gate 复跑（build + 双包测试）：GATE-GREEN

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Process Notes（非代码偏差）

1. **Task 3 `tdd="true"` 的 RED-first 不适用**：plan 有意将实现（Task 1/2）排在钉子测试（Task 3）之前，`<behavior>` 全部是已落地语义的回归锚定——强行 RED 需回滚 Task 1/2，与 plan 排序自相矛盾。按 plan `<action>` 原文「在 shacl_test.go 末尾追加两个测试函数」执行，以 `test(46-01)` 提交。测试名可被 verify-phase-46.sh P3 正则命中（anti-vacuous 前提保持）。
2. **gsd-sdk CLI 不在本机 PATH**：STATE.md / ROADMAP.md 按 SDK handler 等价语义手工更新（advance-plan / update-progress / record-metric / add-decision / record-session / roadmap update-plan-progress）。
3. **REQUIREMENTS.md 不动**：SHACL-01/02 由 46-01（引擎地基）+ 46-02（entities.ttl）+ 46-03（boot/rollback 装配）跨 plan 收口，照 45-01 先例 checkbox 留待收口 plan 勾选。

## Known Stubs

None. `entities.ttl` 尚不存在是 plan 显式设计（Task 2 注：`ShapesPathsFromEnv` 默认列表含 entities.ttl，但本 plan 后仅测试调用该 helper 且不 parse；boot 链仍走 main.go 旧单路径调用——树保持全绿）。entities.ttl 由 46-02 交付，env 列表化消费点切换由 46-03 完成。

## Threat Posture

- T-46-01（env 指向缺失/非法文件）已按 mitigate 落地：errors.Join 聚合、整体 error、绝不静默跳过。
- T-46-02：parse 失败不阻断 boot 的既有语义未触碰（main.go 本 plan 零改动）。
- T-46-SC：零新外部包（stdlib errors / path/filepath），无安装动作。
- 无计划外新增攻击面 → 无 Threat Flags。

## Commits

| Task | Commit | Files |
| ---- | ------ | ----- |
| 1 | `52ca61e` feat(46-01): parse sh:severity from shapes data, retire sourcePath engine special case (D-46-01) | validate.go, shapes.ttl, shacl_test.go |
| 2 | `2c8369b` feat(46-01): variadic Validate + ShapesPathsFromEnv + multi-file merge with layer tracking (D-46-02) | validate.go, induction.go |
| 3 | `1bfe9a8` test(46-01): pin ShapesPaths env-list and Severity three-value semantics | shacl_test.go |

## Self-Check: PASSED

- [x] internal/ontology/shacl/validate.go 存在且含 ShapesPathsFromEnv / SplitList / ListSeparator / Layer 常量（grep 全命中）
- [x] internal/ontology/shacl/shapes.ttl 含 2 处 `sh:severity sh:Warning`
- [x] internal/ontology/shacl/shacl_test.go 含 TestShapesPaths / TestSeverity / TestSeveritySourcePathRetired
- [x] commits 52ca61e / 2c8369b / 1bfe9a8 均在 git log
- [x] 最终 gate：CGO_ENABLED=1 build ./... + CGO_ENABLED=0 双包测试 GATE-GREEN
