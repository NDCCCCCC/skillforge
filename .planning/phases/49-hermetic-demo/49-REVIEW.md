---
phase: 49-hermetic-demo
reviewed: 2026-10-22T15:55:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - internal/retriever/embedded/embedded.go
  - internal/retriever/embedded/rank.go
  - internal/retriever/embedded/seed.go
  - internal/retriever/embedded/tier_isolation.go
  - internal/server/stubchat/extract.go
  - internal/server/stubchat/stub.go
  - internal/verdict/verdict.go
  - internal/webui/static.go
  - cmd/server/main.go
  - cmd/server/backend_switch_test.go
  - web/src/types/verdict.ts
  - web/src/components/center/VerdictCard.tsx
  - verify-phase-49.sh
  - scripts/record-demo.sh
  - README.md
  - Makefile
  - .gitignore
  - internal/verdict/verdict_test.go
  - internal/webui/static_test.go
  - docs/demo.gif
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 49: Code Review Report

> **Provenance（如实标注）**：本 phase 的 `gsd-code-reviewer` 子代理（`48d207ab-3875-4fbd-9c0c-d9d780fe4a51`）运行约 80 分钟后失败，未留下任何产出。本报告由编排者（Team Lead）在阶段收尾时执笔，材料来源有三，逐条标明：
> 1. **独立 verifier 报告**（`49-VERIFICATION.md`，子代理 `1796be54-f8c1-416f-bd62-581afaf6ed5f` 独立完成：自跑门禁 3 次、fresh-clone 独立复现、34/34 must-haves 逐条判定）——其 §6 的 8 条 warnings 是本次评审的主要输入，逐条编号为 R-49-01..08；
> 2. **编排者本人对 `git diff 1838bad..HEAD`（20 个源文件，+6464/−41）的直接检查**；
> 3. **修复后的复跑证据**（门禁第 3/4 次连跑、五包 Go 测试、依赖清单 diff）。
> 本报告**不是**独立第三方评审；独立视角由 `49-VERIFICATION.md` 承担，两者互为补充。

## Round 1 — Full Review（phase 49 全量）

**Reviewed:** 2026-10-22T15:55:00Z
**Depth:** standard
**Scope:** `git diff 1838bad..HEAD -- internal/ cmd/ web/src verify-phase-49.sh scripts/record-demo.sh Makefile .gitignore README.md`
**Status:** issues_found（3 warning 已在 phase 内修复；5 info 已处置或显式延期）

### Summary

Phase 49 新增 4 个生产包（embedded retriever / stubchat / webui / verdict 的 Mode 字段）与两份脚本（九分区门禁 + 录屏），零新依赖（go.mod / go.sum / web/package*.json 在 `1838bad..HEAD` 无提交）。检索链的信任边界（tier 两段式 PRE-filter、空 tiers ⇒ 0 命中）在 embedded 后端里得到与 milvus/mongo 同构的落实；stub 不豁免任何校验（同一条 `GroundingChecker` 链、confidence ≤ 0.5、`review_required=true`）；`internal/server/pipeline.go` 与 `internal/embedder/bge` 在本 phase 的提交里**零改动**（`git log 1838bad..HEAD --` 为空），兑现了「stub 是插入点而非分叉」的设计承诺。

评审发现 8 条（0 critical / 3 warning / 5 info）。3 条 warning 全部在本 phase 内修复并复跑验证；5 条 info 中 1 条显式延期到 Phase 50（ROADMAP 措辞），4 条记录为"接受并说明理由"。逐条如下。

### Findings

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| **R-49-01** | warning | `internal/webui/static.go` 的 dist 存在性检查排在 `/api` 闸**之前**：`web/dist` 缺失时，未注册的 `/api/xxx` 返回 503「web/dist missing」而非 404——「接口不存在」与「前端未构建」是两件事，答案不应依赖构建状态。（来源：独立 verifier W-1；其 fresh-clone 探针实测 `api_missing_http=503`） | **已修复**：`/api` 闸前移到 dist 检查之前（提交 `216620b`），补 `TestStaticHandlerMissingDistStill404sUnregisteredAPIPaths`（缺失 dist 时 `/api`、`/api/`、`/api/unknown`、`/api/knowledge` 全 404，`/` 仍 503）；两个既有测试语义不变；门禁第 3 次连跑 ALL PASSED |
| **R-49-02** | warning | 49-02 truth ⑧ 声明的 Go 侧机械判据（marshal 零值 Verdict 后断言无 `mode` 键）**没有落地**：`verdict_test.go`（141 行）无任何 mode 用例，`omitempty` 契约只靠前端 `verdict.test.ts` 兜底。（来源：独立 verifier W-2） | **已修复**：新增 `TestModeOmitEmptyKeepsRealModelWireUnchanged`——零值 Verdict marshal 后不含 `"mode"` 键（真模型 wire 形状不变），`Mode=ModeStub` 时 marshal 出字面 `"mode":"stub"`；有人将来删 `omitempty` 或在真模型路径写 `Mode`，测试先红 |
| **R-49-03** | warning | 门禁不做 README 内容断言：49-04 truth ⑩ 声称「README 关键行可被静态断言」，但 `verify-phase-49.sh` 里 README 只出现在注释与 `docs/demo.gif` 引用行——hermetic quickstart 可以在零变红的情况下悄悄停止记载 stub 模式。（来源：独立 verifier W-3） | **已修复**：P4 分区新增 `P4-README-LABEL` 检查块（四条 grep 钉住 `export RETRIEVER_BACKEND=embedded` / 默认 Ollama 端点 / `"mode":"stub"` / `SKILLFORGE_MCP_SERVICE_SKILL`），配变异演练 **D10**（删除该检查块 ⇒ `--only P4` 必红且点名分区）；`ok_partition "P4"` 扩为 4 token；D10 让「没有变异演练的门禁等于没有门禁」这条 phase 自律继续成立 |
| **R-49-04** | info | 49-04 truth ⑨ 字面是「门禁跑完后 `git status --porcelain` 为空」，实现用的是**运行前后差集**（并过滤 `skillforge.db`/`.cast`/`.skillforge-cache`）。实现**更强**：开发机上既存的无关脏文件不会把健康树打成假红，而门禁自己一旦在仓库落产物，差集必现。（来源：独立 verifier W-4） | **接受并记录**：语义更严、方向正确；措辞对齐项已记入 `49-VERIFICATION.md` W-4，不改计划原文（计划是历史工件） |
| **R-49-05** | info | ROADMAP SC1「无预装 Milvus/BGE」、SC2/DEMO-01「embedded 向量**兜底**模式」与锁定决策 D-01（零兜底、强制外部向量化模型）/D-02（默认本地 Ollama）字面冲突，是当前最易被误读的文档债；`49-CONTEXT.md` `<deferred>` 早已登记「回 ROADMAP 修订措辞」。（来源：独立 verifier W-6） | **部分处理 + 显式延期**：已在 ROADMAP Phase 49 段加 **Scope Note**（记录实施口径：SC1 实施为「无需内网 BGE 端点」、SC2 实施为「本地向量存储模式」、SC3 三重标注），**不改验收标准原文**（验收标准不能在事后改写）；措辞修订归 Phase 50（其目标就是 README 与仓库实况零失真） |
| **R-49-06** | info | 门禁 live 腿使用**测试专用 embedding 替身**（0.9 共享基底 + 0.1 token 哈希残差），与 D-01「不使用任何替身」的字面表述存在张力。（来源：独立 verifier W-7） | **接受并记录**：49-04-PLAN 已将其登记为 flagged_assumption（替身只 bind 127.0.0.1、`BGE_M3_URL` 只注入子进程、生产二进制零改动、门禁打印 `using test-only embedding double`）；独立 verifier 用**真实模型端点**在 fresh clone 上补齐了真链路证据（其 §4），风险面已收敛 |
| **R-49-07** | info | 门禁不构建前端：fresh clone 未 `npm run build` 时 P6 明确变红（`FAIL P6: web/dist/index.html is absent …`）而非跳过。（来源：独立 verifier W-8） | **接受并记录**：这是 fail-closed 而非静默假绿，方向正确；若将来把门禁挂上 CI，需在 CI 里先 build 前端（已记入验证报告 W-8 建议） |
| **R-49-08** | info | D-08 的前端落点与决策字面不同：决策点名 `MessageBubble.tsx` 的 `isVerdict` 守卫，实际实现是 `web/src/types/verdict.ts` 新增独立 `isStubVerdict` + `VerdictCard.tsx` 的 `data-testid="stub-mode-badge"` 徽标；`MessageBubble.tsx`/`DataPolicyPart.tsx` 未改动。（来源：独立 verifier W-5） | **接受并记录**：49-02-PLAN 已把它登记为 flagged_assumption（"MessageBubble 的消费留给后续 slice"）——是**已登记的偏离**而非缺陷；D-08 的目标（用户可见的 stub 标注）已达成且有组件测试兜底（`VerdictCard.test.tsx` 20 条，含 2 条徽标用例） |

### What the review checked and did not find

- **tier 隔离不削弱**：`internal/retriever/embedded/tier_isolation.go` 在**候选集构造阶段**过滤 tier 与 `effective_to` 窗口（两段式 PRE-filter），空 tiers 短路为 0 命中；`rank.go` 是 `internal/retriever/mongo/rank.go` 同构的纯函数（score desc, id asc tie-break）。P2/P3 的单元测试与变异演练覆盖了这条不变量。
- **stub 不越权**：`internal/server/stubchat/stub.go` 产出的 Verdict 走与真模型完全相同的 `verdict.Validate` + `GroundingChecker`；伪造 span 的降级路径有专测（`TestStub_ForgedSpanIsDowngraded`，P5 演练 D5 钉住）。
- **生产面零污染**：`git log 1838bad..HEAD -- internal/embedder/bge internal/server/pipeline.go .github/workflows docker-compose.yml .env.example Dockerfile.* go.mod go.sum` 全部为空；`cmd/server/main.go` 的改动限于 embedded 分支接线、GLM key 惰性读取与 stub 注入。
- **缓存不泄密**：SFVC 缓存条目不存 API key、端点只存 sha256；`.gitignore` 的 `/.skillforge-cache/` 挡住误提交。
- **素材不含密钥**：`docs/demo.gif` 头 6 字节为 `GIF89a`、二进制扫描无 key 形态字节（门禁 P7 + 演练 D7）。

### Verification of the fixes

| 项 | 结果 |
|---|---|
| `gofmt -l internal/verdict/ internal/webui/` | 干净 |
| `go build ./...`（WSL） | exit 0 |
| `go test ./internal/webui/ ./cmd/server/ ./internal/server/stubchat/ ./internal/verdict/ ./internal/retriever/embedded/ -count=1`（WSL） | 五包全 ok |
| `bash verify-phase-49.sh`（修复后第 3 次连跑） | `=== Phase 49 Gate: ALL PASSED ===`、exit 0、恰好一条 ALL PASSED |
| `bash verify-phase-49.sh`（README 断言 + D10 之后第 4 次） | 见收尾提交附注（连跑验证） |
| 依赖清单 | UNCHANGED（零新依赖） |

### Disposition

详见 `49-REVIEW-DISPOSITION.md`（round-1 处置账本）：3 warning → `fixed`（本 phase 内），R-49-05 → `deferred`（Phase 50），其余 info → `open (accepted with rationale)`。
