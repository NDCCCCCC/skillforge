---
phase: 14-p10-0-cgo-trust-floor
verified: 2026-08-23T03:35:00Z
status: gaps_found
score: 9/11 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 9/11
  gaps_closed:
    - "Gap 1 hermetic half: cmd/eval refusal-accounting defect fixed with classifyRunOutcome + TestClassifyRunOutcome RED→GREEN"
    - "Gap 3: verify-phase-13 Stage 7 compliance pin synchronized to 1dc6f148… (lockstep with verify-phase-14.sh Stage 11)"
  gaps_remaining:
    - "Gap 1 live half: compliance replay faithfulness 0.888 < 0.9 gate; platform-floor.lock.json 1.1.0 formal re-sign + eval/frozen/README.md caveat deferred per STOP branch"
    - "Gap 2 live half: Dockerfile.backend libstdc++/libgcc fix shipped; live docker build/run blocked on host (no Docker installed)"
gaps:
  - truth: "compliance 冻结 replay 门在 1.1.0 地板下 PASS（faithfulness ≥ 0.9）——D14-B4 全冻结集重跑纪律的 live 半"
    status: partial
    reason: "14-11 修复了测量层根因（cmd/eval 识别 verdict status:\"refused\"）并 hermetic 锁定；单次授权 replay 50/50 完成，AssertPin held，refusal-required 19/20 识别、该类 faith 0.9727，但 overall faithfulness = 0.888（<0.9 门），落入 plan 预声明的 0.83–0.90 区间。per no-falsify 契约未重签、未重跑，已按 14-10 Operator-Gate Addendum Alternative 升级至 planner/user 对 gate 语义 re-baseline 的裁决。"
    artifacts:
      - path: "cmd/eval/main.go"
        issue: "FIXED：classifyRunOutcome 识别 v.Status==\"refused\"，四模态优先级守卫（err > 结构化 Refusal > in-schema refused > scored verdict）"
      - path: "cmd/eval/main_test.go"
        issue: "FIXED：TestClassifyRunOutcome 6 行表驱动 RED→GREEN 回归锁"
      - path: "eval/frozen/platform-floor.lock.json"
        issue: "DEFERRED：platform_floor_version=1.1.0 但 metrics/operator 签名仍为 2026-08-22 1.0.0-era（WR-08 provenance 矛盾未重签）"
      - path: "eval/frozen/README.md"
        issue: "DEFERRED：未落档 platform-floor reproducibility caveat 与第 5 锁条目"
    missing:
      - "planner/user 裁决 14-10 Operator-Gate Addendum Alternative（re-baseline gate 语义：拒答 prose 按 ground-truth 拒答判据判 faith，而非检索 chunks）"
      - "在 PASS 的 replay 后执行 fill-lock 正式 1.1.0 重签（operator_signed_at/operator_name 刷新 + lock_sha256 重算）+ README caveat/第 5 锁条目"
  - truth: "SC-1 Docker 半：Dockerfile.backend CGO_ENABLED=1 + build-base 镜像构建成功产出可运行的 CGO 二进制"
    status: partial
    reason: "Dockerfile.backend runtime stage 已补 libstdc++ libgcc（go-duckdb C++ 引擎动态链接依赖），builder 段 CGO_ENABLED=1 + build-base 已就位；但本机无 Docker（git-bash/WSL 均 command not found），live build + ldd LINKS_OK 验证仍 environment-blocked，按 14-08 先例记 blocked-with-notes。"
    artifacts:
      - path: "Dockerfile.backend"
        issue: "FIXED：runtime stage RUN apk add ... libstdc++ libgcc，注释说明 CR-01/14-11 Gap 2"
    missing:
      - "operator 在含 Docker 环境执行 docker build -f Dockerfile.backend -t skillforge-backend:cgoflip . + docker run --rm --entrypoint /bin/sh 验证 ldd /app/skillforge 无 not found"
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: gaps_found
---

# Phase 14: CGO 地基 + 平台信任地板（P10.0）Verification Report

**Phase Goal:** 把 CGO 工具链（MSYS2 gcc + go-duckdb + mattn/go-sqlite3）钉进本机与 Docker 双构建环境，并把平台级隐藏系统提示词地板从数据维度 day-1 内建——后续所有数据能力生长在「可构建的地基 + 不可绕过的信任地板」之上。
**Verified:** 2026-08-23T03:35:00Z
**Status:** gaps_found
**Re-verification:** Yes — Round-3 closure assessment after 14-11 gap-closure plan

## Goal Achievement

验证方法：全部结论基于代码库直接证据与 WSL Ubuntu-22.04（gcc 11.4.0, go1.25.0 linux/amd64）独立重跑。Windows 原生 gcc 无法链接 go-duckdb（emutls ABI 不匹配），本机 CGO 复核一律走 WSL——与执行期实际构建环境一致。

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 本机半：CGO 下 `go build ./... && go vet ./...` 全绿，go-duckdb v2.4.3 与 mattn/go-sqlite3 v1.14.50 同时可链接 | ✓ VERIFIED | 独立重跑 verify-phase-14.sh Stage 01-02 PASS；go.mod 双 pin 精确匹配 |
| 2 | SC-1 Docker 半：Dockerfile CGO 翻转后镜像构建成功产出可运行二进制 | ⚠️ PARTIAL | Dockerfile.backend:16,32,44 代码面已翻转（build-base + CGO_ENABLED=1 + libstdc++ libgcc ✓）；但 Docker 构建/运行从未 live 执行（environment-blocked，14-08 先例口径） |
| 3 | SC-2 sqlite 驱动替换零回归：mattn 全量 store 套件绿 + DSN pragma 语义 | ✓ VERIFIED | verify-phase-14.sh Stage 04 PASS |
| 4 | SC-3 DuckDB spike：boot-queries + WAL 刷盘 + Appender ≥10 万行/s + 50 并发读不阻塞写 | ✓ VERIFIED | verify-phase-14.sh Stage 05 PASS |
| 5 | SC-4/PLAT-09：地板代码级 prepend、Skill 不可遮蔽、热重载存活（chat+deep 双路径） | ✓ VERIFIED | verify-phase-14.sh Stage 06/10 PASS |
| 6 | SC-4/PLAT-10：审计行携带 platform_prompt 版本 + admin 预览端点 | ✓ VERIFIED | verify-phase-14.sh Stage 08 PASS |
| 7 | SC-4/PLAT-11：platform-floor.jsonl 冻结集断言——诱导写 raw SQL → 拒答 | ✓ VERIFIED | verify-phase-14.sh Stage 09 PASS；live judge 20/20 @1.1.0 已有证据 |
| 8 | （14-09）旧库迁移 additive + 真实 server boot | ✓ VERIFIED | verify-phase-14.sh Stage 04 覆盖 store 迁移回归 |
| 9 | （14-10 Task 1）地板 1.1.0 重锚定 + compliance 重锁 + 无涟漪验证 | ✓ VERIFIED | verify-phase-14.sh Stage 11 PASS；4 锁 sha 与 pins 一致 |
| 10 | （14-10 Task 2 步骤 3-5）platform-floor 锁 1.1.0 正式 live 重签 + README caveat 落档 | ✗ FAILED | 14-11 STOP 分支：未重签、未写 caveat；锁保留 1.0.0-era metrics（WR-08 开放） |
| 11 | （14-10 Task 2 步骤 2）compliance 冻结 replay 门 PASS（faithfulness ≥0.9） | ✗ FAILED | 14-11 单次授权 replay 50/50 完成，overall faithfulness 0.888（<0.9），已按预声明路径升级至 re-baseline 裁决 |

**Score:** 9/11 truths verified

### Round-3 Closure Assessment (after 14-11)

**What 14-11 delivered:**

1. **Gap 1 测量层根因已永久修复。** `cmd/eval/main.go` 新增 `classifyRunOutcome`，识别 `verdict.status == "refused"` 并路由进与 Run-error / 结构化拒答完全相同的冻结约定（`refused=true`, `answer=""`, reason="verdict_status_refused"）。`TestClassifyRunOutcome` 6 行表驱动 RED→GREEN 已独立重跑 PASS。该修复使 refusal-required 类目从 0/20 提升到 19/20，faith 从 0.6763 提升到 0.9727。
2. **Gap 2 静态缺陷已修复。** `Dockerfile.backend` runtime stage 已安装 `libstdc++ libgcc` 并附注释依据（go-duckdb v2.4.3 C++ 引擎动态链接依赖）。builder 段 `build-base` + `CGO_ENABLED=1` 原已就位。代码面完整。
3. **Gap 3 涟漪回归已修复。** `scripts/verify-phase-13.sh` Stage 7 `EXPECTED_COMPLIANCE_SHA` 已从过期的 `e77877bb…` 同步到 `1dc6f148…`（与 `verify-phase-14.sh` Stage 11 锁步），注释块如实记录 14-01/14-10 两次 T-8-12 记录式重锁。
4. **14-11 授权 replay 诚实记录。** 50/50 完成，AssertPin held（`frozen pin holds`, `prompt_hash=5b12acd9…`），archive `eval/reports/2026-08-23T03-13-46Z.json` + log `build/1411-replay.log` 原样在盘。overall faithfulness = **0.888**（<0.9 门），per-category：adversarial 0.7683 | multi-hop 0.9581 | refusal-required 0.9727；refusal-required 19/20 refused。 mid-run BGE-M3 endpoint `connect: no route to host` 影响 mh-004..mh-010。

**Residual open items (not new regressions; tracked in STATE.md Blockers):**

- **D14-B4 gate 语义 + platform-floor.lock.json 1.1.0 正式重签 + README caveat：** 14-11 按 STOP 纪律未执行重签/caveat，将 0.888 结果（落入预声明 0.83–0.90 区间）直接路由至 14-10-SUMMARY Operator-Gate Addendum Alternative，待 planner/user 裁决是否 re-baseline gate 语义。当前锁诚实保留 1.0.0-era metrics（WR-08）。
- **Docker live build/run：** 代码修复已无条件提交；本机无 Docker，live 验证 environment-blocked，按项目既有 `blocked-with-notes` 先例处理。

**Verifier verdict:** Phase 14 行为面与 hermetic 锁链已全部达成（CGO 本机地基、平台地板 1.1.0、11/11 gate、13/9 gate、零涟漪）。两个 residual 项均为已记录的 deferred/operator 决策，非代码回归。但按对抗式验证标准，compliance replay ≥0.9 gate、平台锁正式重签、Docker live 验证这三个 must-have 仍未 observably true，故 status 保持 `gaps_found`。建议将剩余项作为 milestone-level operator gates 继续跟踪，不阻塞 Phase 15 数据面工作（STATE.md 已按此口径记录）。

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/platformfloor/platform_floor.go | 七条铁律 + ComposePlatformPrompt 唯一出口 + 1.1.0 | ✓ VERIFIED | verify-phase-14 Stage 06/10 |
| internal/agent/platform_floor.go | 镜像字面量 + re-export | ✓ VERIFIED | source-grep PASS |
| internal/store/sqlite/sqlite.go | mattn 装配 + dsnWithPragmas | ✓ VERIFIED | verify-phase-14 Stage 04 |
| internal/store/sqlite/schema.go | platform_floor_version 列 + 索引-after-ALTER | ✓ VERIFIED | verify-phase-14 Stage 04/08 |
| internal/server/platform_prompt.go | admin 预览 handler | ✓ VERIFIED | verify-phase-14 Stage 08 |
| internal/agent/hitl/audit_writer.go | floorVersion 注入 + fail-closed | ✓ VERIFIED | verify-phase-14 Stage 08 |
| internal/duckdb/connector.go | boot-queries + ValidateDatasetID + Close 双关 | ✓ VERIFIED | verify-phase-14 Stage 05 |
| cmd/eval/main.go | classifyRunOutcome 四模态分类 | ✓ VERIFIED | TestClassifyRunOutcome PASS |
| cmd/eval/main_test.go | 6 行表驱动回归锁 | ✓ VERIFIED | RED→GREEN evidence |
| Dockerfile.backend | runtime libstdc++ libgcc | ✓ VERIFIED (static) | :44 在案 + 注释 |
| scripts/verify-phase-13.sh | Stage 7 pin 1dc6f148… | ✓ VERIFIED | make verify-phase-13 9/9 PASS |
| eval/frozen/README.md | platform-floor caveat + 第 5 锁条目 | ✗ MISSING | STOP 分支未执行 |
| eval/frozen/platform-floor.lock.json | 1.1.0 正式 fill-lock 重签 | ⚠️ PARTIAL | version 1.1.0 + lock_sha256 已刷新；metrics/operator 签名仍为 1.0.0-era |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| cmd/eval/main.go runEvalPass 3b 段 | classifyRunOutcome | 函数调用 | ✓ WIRED | :428 |
| classifyRunOutcome refused=true | internal/eval faith 空答案 + crecall REFUSAL 哨兵 | answer="" + reason="verdict_status_refused" | ✓ WIRED | TestClassifyRunOutcome + replay 19/20 refusal faith 1.0 实证 |
| scripts/verify-phase-13.sh:184 EXPECTED_COMPLIANCE_SHA | scripts/verify-phase-14.sh Stage 11 pin | 同一文件 sha 双脚本锁步 | ✓ WIRED | 1dc6f148… 逐字节一致 |
| cmd/spike-platform-floor --fill-lock | reports/platform_floor_live.jsonl | Mode 3 回填 | ⚠️ NOT_WIRED | 未执行（STOP 分支） |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| cmd/eval 拒答口径回归锁 | WSL: `go test ./cmd/eval/ -run TestClassifyRunOutcome -count=1 -v` | 6/6 subtests PASS | ✓ PASS |
| Phase 13 门复跑 | WSL: `export PATH=/usr/local/go/bin:$PATH && bash scripts/verify-phase-13.sh` | 9/9 stages PASS | ✓ PASS |
| Phase 14 门复跑 | WSL: `export PATH=/usr/local/go/bin:$PATH && bash scripts/verify-phase-14.sh` | 11/11 stages PASS | ✓ PASS |
| Dockerfile runtime 依赖 | `grep "apk add" Dockerfile.backend` | libstdc++ libgcc 在案 | ✓ PASS |
| compliance replay 数字 | `build/1411-replay.log` + `eval/reports/2026-08-23T03-13-46Z.json` | overall 0.888；refusal 19/20；BGE no-route-to-host | ✗ FAIL vs ≥0.9 gate |
| platform-floor 锁 diff | `git diff --stat eval/frozen/platform-floor.lock.json eval/frozen/README.md` | 无 diff（未重签/未写 caveat） | ✗ DEFERRED |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PLAT-09 | 14-01, 14-05, 14-06, 14-10 | 平台级隐藏系统提示词：代码级固定、prepend、不可遮蔽、热重载存活、数据铁律内建 | ✓ SATISFIED | verify-phase-14 Stage 06/10/11 + live judge 20/20 |
| PLAT-10 | 14-02, 14-03, 14-09 | 审计行携带 platform_prompt 版本；admin 预览合成有效 prompt | ✓ SATISFIED | verify-phase-14 Stage 08 + UAT Test 6 证据 |
| PLAT-11 | 14-04, 14-07, 14-08, 14-10 | platform-floor.jsonl 冻结集断言平台层不变量（诱导写 raw SQL → 拒答） | ✓ SATISFIED（断言行为面） | verify-phase-14 Stage 09 + live judge 20/20 @1.1.0；锁重签递延 |

Orphaned requirements：无。

### Anti-Patterns Found

债务标记扫描（TBD/FIXME/XXX）：本 phase 全部触碰文件 0 命中。

14-REVIEW.md 遗留项中未在本次 14-11 修复的仍保持原评级（WR-01..07 等），均非 phase 收口阻塞项；其中 WR-08 并入 Gap 1 residual。

### Human Verification Required

无新增人工验证项；Gap 2 的 Docker live 验证为 environment-blocked，待 operator 在含 Docker 环境补跑。

### Gaps Summary

14-11 关闭了两个半 gap（Gap 1 测量层根因、Gap 2 静态修复、Gap 3 全部），但留下两个 residual 开放项：

1. **Gap 1 residual（D14-B4 纪律 live 半）：** compliance replay 0.888 < 0.9，platform-floor.lock.json 1.1.0 正式重签与 README caveat 未执行。已按 14-10 Operator-Gate Addendum Alternative 升级至 planner/user 对 gate 语义的 re-baseline 裁决；非代码缺陷，是已记录的待决策项。
2. **Gap 2 residual（Docker live 半）：** Dockerfile 修复已提交，但本机无 Docker，live build/run 未验证；按 14-08 先例为 environment-blocked。

Phase 14 核心目标（本机 CGO 地基 + 平台地板 1.1.0 行为面）已真实达成；剩余项为 frozen-baseline 形式纪律与 operator 环境验证，建议以 milestone/STATE.md Blockers 形式继续跟踪，不阻塞后续数据面工作。

---

_Verified: 2026-08-23T03:35:00Z_
_Verifier: Claude (gsd-verifier)_
