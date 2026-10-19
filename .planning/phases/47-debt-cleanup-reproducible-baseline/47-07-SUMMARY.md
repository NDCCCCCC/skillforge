---
phase: 47-debt-cleanup-reproducible-baseline
plan: 07
status: complete
type: docs
wave: 2
requirements: [REL-04, REL-01]
depends_on: [47-01, 47-02, 47-03, 47-04, 47-05]
files_modified:
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
subsystem: planning-records
tags: [docs, traceability, debt-disposition, root-cause-correction]
tech-stack: []
key-files:
  created: []
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
decisions:
  - REL-01 根因改写为「边方向确定性（map 迭代序）」，删除「浮点 Weight」表述而非加注解
  - 验收强度定为 -count=100 + 一次 -shuffle=on；SC-4 的「连续两跑」降级为冒烟门槛
  - Traceability 覆盖率分母为 17（13 项 v1.7 需求 + BL-01..04），非 18
metrics:
  tasks: 2
  commits: 2
  duration: ~4min
  files_changed: 2
---

# Phase 47 Plan 07: BL 补录 + 根因措辞更正 Summary

把 REL-01/SC-1 的记录改成实测根因（map 迭代序导致的边方向非确定性），并把 BL-01..04 作为带 owner/status 的行补进 v1.7 Traceability 表——v1.6 审计点名的「正文有 ID 但表里缺行」缺口至此闭合。

## What Shipped

### Task 1 — REL-01 / SC-1 根因措辞更正（commit `7e55794`）

删除（而非加注解）三处错误解释：

- `.planning/ROADMAP.md` SC-1：改为「`keyword_cooccur` **边方向**非确定性 —— `ComputeEdges` 的 `sources` 由 `sourceTokens` 的 map 迭代序构造，`i<j` 配对使 `From`/`To` 随迭代序翻转，而 `EdgeGraph.Neighbors` 是有向遍历；末尾排序键 `(From, To, EdgeType)` 无法把 `(1→4)` 与 `(4→1)` 归一」。修复手段写明：配对前确定性排序（沿用同文件 `computeCitationEdges` 既有做法）+ `Weight` 作第四排序键（`sort.Slice` 不稳定，`citation_cooccur` 同一 `(From,To)` 可因不同 audit 行产生不同 Weight）。保留 `——D-46-DEF-02 闭合` 标记。
- `.planning/ROADMAP.md` SC-4：保留「连续两跑无 flake」但显式标注**仅作冒烟门槛，不充分**，追加确定性强度段要求（`./internal/knowledge/` 的 `-run '^TestDeriveEdgesDeterministic$' -count=100` + 对触及包跑一次 `-shuffle=on`）与**空 waiver** 硬约束（`t.Skip`/`WAIVED` 与公开绿灯目标相悖；CI 裸跑 `go test ./...` 不读本地 waiver 机制）。
- `.planning/REQUIREMENTS.md` REL-01：同一根因 + 实测证据分布，`- [ ] **REL-01**:` 复选框语法不动。

证据写法直接引用 47-01 的实测：修复前 `-count=200` 两次分别 **34 败 / 50 败（17–25%）**，失败**全部**落在 `From`/`To`/`EdgeType` 错位断言，`Weight` 断言 **0 次失败**——测试的严格比较是对的，因此记录里写明不得用放宽 `InDelta` 容差或改无序比较换绿灯（沿用 D-01c 原则）。

### Task 2 — BL-01..04 补录 + 四处陈旧计数更正（commit `4a1ac2b`）

- 新增 `## Backlog Items Carried From v1.6` 小节：BL-01..04 定义**逐字**引自 `.planning/milestones/v1.6-REQUIREMENTS.md` 第 40-43 行，每条带 `来源 …:行号` 引用；归档文件本身未被修改。
- Traceability 表追加四行，Phase 与 Status 均非空，去向与 47-DEBT-DISPOSITION 主表一致（47-06 Task 1 pin 的 disposition）：
  - `BL-01` → Backlog（无 owner phase）/ **Deferred**（行为语义重构，需单独立项）
  - `BL-02` → Backlog（Phase 48 lint 落地后立项）/ **Deferred**（需 Phase 48 的 `golangci-lint` 报告才有可定义范围）
  - `BL-03` → Backlog（needs-live）/ **Deferred**（依赖内网 LLM/BGE）
  - `BL-04` → Phase 47 / **Complete**（plans 47-03 + 47-05）
- `Coverage:` 行改写为 `Coverage: 17/17 mapped items`（13 项 v1.7 需求 + 4 项 BL），并注明 BL-01..03 维持 Deferred、BL-04 Complete。
- 更正四处继承的陈旧计数，全部把 v1.7 需求数夸大了 1（id 列表 `FACE-01..03 / CICD-01..03 / DEMO-01..03 / REL-01..04` 恰好 13 项）：`.planning/ROADMAP.md` Scope 行 `14 项需求 / coverage 14/14` → `13 项 / 13/13`；`.planning/ROADMAP.md` 尾部斜体 `14 REQ … coverage 14/14` → `13 REQ … 13/13`（该行**第二个**独立错误数字，只改 `14/14` 会漏掉）；`.planning/REQUIREMENTS.md` Coverage 行（已由上条重写）；`.planning/STATE.md` —— **已在本轮 plans 编写时就是 `Coverage: 13/13 REQ`，按计划指示保留原样**，但仍留在 grep 扫描范围内，使该断言自校验、防止未来编辑时重新漂移。

## Verification

| Check | Result |
|-------|--------|
| `grep -n "浮点 Weight\|精度抖动" ROADMAP.md REQUIREMENTS.md` | 无输出 |
| `边方向` 出现于两个文件 | ROADMAP ×2 / REQUIREMENTS ×1 |
| SC-1 含 `-count=100` 与 `-shuffle=on` | 是 |
| REL-01 仍以 `- [ ] **REL-01**:` 开头 | 是 |
| Traceability 行数 = 17（13 + 4 BL） | 是 |
| `Coverage: 17/17` 存在于 REQUIREMENTS.md | 是 |
| `grep -nE "14/14\|14 项需求\|14 REQ"` 三文件 | 无输出 |
| `.planning/STATE.md` 含 `Coverage: 13/13 REQ` | 是 |
| `git status --porcelain .planning/milestones/` | 空（归档未改） |
| `git diff --stat` | 仅 REQUIREMENTS.md + ROADMAP.md |
| 13 条既有 Traceability 行的 Phase/Status | 未改动（仅追加） |

## Deviations

- **`.planning/STATE.md` 未修改**：计划要求「re-grep 后若已为 13/13 则保持原样」。实测确认已是 `Coverage: 13/13 REQ（FACE×3 / CICD×3 / DEMO×3 / REL×4）`，故不编辑，仅纳入扫描范围。
- **任务文件清单 vs 动作范围**：Task 2 的 `<files>` 列了 `REQUIREMENTS.md` 与 `STATE.md`，但动作要求改的两处陈旧计数在 `ROADMAP.md`。以动作与验收判据（`ROADMAP.md:62` Scope 行、`ROADMAP.md:159` 尾部）为准，`ROADMAP.md` 在实际编辑范围内，与 plan frontmatter 的 `files_modified` 一致。

## Notes for Downstream

- **47-06**：`47-DEBT-DISPOSITION.md` 主表的 BL-01..04 去向必须与本计划写入的 Status 一致（Deferred ×3 / Complete ×1）；STATE.md § Deferred Items 仍需逐 ID 索引行（47-06 负责，本计划未触）。
- **47-08**：门脚本 `verify-phase-47.sh` 需含 SC-4 写明的确定性强度段（`-count=100` + 一次 `-shuffle=on`）并做到空 waiver。
- **Phase verification**：REL-01 / REL-02 的 Traceability Status 由验证步骤负责翻成 Complete；本计划只把 BL-04 写成 Complete（它是本轮已交付的），未动 FACE/CICD/DEMO/REL 行。
- Phase 47 SC-1 的 flake 率记录为 **17–25%**（实测 34 与 50 两次），与 47-CONTEXT.md 早期写的「~17%」有出入，本记录以实测为准。
