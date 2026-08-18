---
phase: 8
slug: p8-0-eino-v0-7-7-v0-9-13
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-18
---

# Phase 8 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 依赖供应链 | go.mod/go.sum 声明的第三方模块（eino v0.9.14 / eino-ext openai v0.1.13） | 编译期进入产物的代码 |
| 本地 sqlite 文件 | internal/store/sqlite 单一 DB 文件（audit/session_turn/checkpoint 表） | 结构化 verdict + checkpoint 序列化 bytes（含 InterruptContext payload） |
| live 外部服务 | 真 GLM 端点（Coding Plan）、内网 Milvus 10.62.x.x、本地 BGE-M3 embeddings | 查询文本 / 检索结果（凭据仅经 shell 环境变量） |
| eval 冻结基线 | eval/frozen/compliance.lock.json（prompt_hash + model_ver pin） | 判定基准（不得静默刷新） |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-8-01 | Tampering (supply-chain) | go.mod/go.sum 依赖升级 | mitigate | 零新 module path——仅 eino v0.9.14 + eino-ext openai v0.1.13 两个既有 direct bump（hertz v0.10.6 / milvus v2.4.2 / sqlite v1.56.0 原样保留，go.mod 实测核对）；升级提交 36845c0 单原子 | closed |
| T-8-02 | Information Disclosure | scripts/test（live key）/ 提交卫生 | mitigate | 全 phase 显式列举 git add；scripts/test untracked（git ls-files = 0，全部历史提交 = 0）；verifier 独立复核「secrets never appear in any commit」 | closed |
| T-8-03 | Availability (回归漂移) | 升级后运行时行为 | mitigate | 四道 hermetic 门全量重跑（08-01-SUMMARY 证据）+ orchestrator 执行后与 review-fix 后两次全仓 `go test ./... -count=1` EXIT=0 复核 | closed |
| T-8-04 | Tampering (SQLi) | checkpoint Get/Set 的 id/payload | mitigate | checkpoint.go 零 fmt.Sprintf（实测 0 命中）、100% `?` 参数化 const SQL；TestCheckpointSQLInjection DROP TABLE 存活断言在档（checkpoint_test.go） | closed |
| T-8-05 | Tampering (checkpoint payload 注入) | 未来 resume 数据面 | accept | 本 phase store 只透传 bytes 不解析、无 HTTP 暴露；Phase 11 HITL-04 落地 resume-data schema 校验 | closed |
| T-8-06 | Information Disclosure (checkpoint ID 可猜测越权) | 未来 HTTP Get 面 | accept | 本 phase 无 HTTP 暴露（仅测试访问）；DDL 已预留 session_id 列，Phase 11 绑定 session | closed |
| T-8-07 | Tampering (禁令门失效/假绿) | TestNoLegacyInterruptAPI | mitigate | 负向证明强制（探针注入 → FAIL → 删除 → PASS，08-03-SUMMARY 5 处证据）；verifier 二次对抗探针复现（08-VERIFICATION.md） | closed |
| T-8-08 | Tampering (Elevation via resume 串台) | 未来 HITL 代码 | mitigate | 三标识符禁令清单（NewInterruptAndRerunErr + deprecatedInterruptAndRerun 防绕行 + sentinel，no_legacy_interrupt_test.go 实测在档）；_test.go 亦在扫描内；排除清单仅非产品代码路径 | closed |
| T-8-09 | Tampering (序列化注册半截) | schema.RegisterName vs 裸 gob | mitigate | checkpoint_resume_test.go 3 处 schema.RegisterName[T]（双注册）；internal/store/sqlite/ 零裸 gob.RegisterName（实测 0 命中）；round-trip 测试即延迟引爆面检出器 | closed |
| T-8-10 | Repudiation (重启后 resume 无法证明) | SC-5 验收 | mitigate | 「重启」三重建：s1.Close()（:361 注释明示 simulates process exiting）→ 同 DSN s2 重开 → 全新 Runner；断言链含 store 持久化 + 终答 approved=true + 注入值可见 | closed |
| T-8-11 | Information Disclosure | live GLM key / 内网拓扑 | mitigate | 凭据只经 shell 环境变量；SUMMARY 只记指标与计时；scripts/test 保持 untracked（实测 0/0） | closed |
| T-8-12 | Tampering (基线掩盖) | eval/frozen/compliance.lock.json | mitigate | `git status eval/frozen/` 干净（实测 0 dirty）；TestShippedFrozenLockIsInSync（internal/eval/frozen_test.go）锁定 prompt_hash 同步机制；CR-01 review fix 的合法重 hash 经该测试 PASS 验证（53f6ca5 后复核全绿） | closed |
| T-8-13 | Availability (live 行为漂移未检出) | cancel/tier-filter/eval 三门 | mitigate | 三门全部在 phase 最终 HEAD 重跑留档（08-05-SUMMARY 16 处指标证据），用户 2026-08-18 human-verify approved | closed |
| T-8-SC (×5 plans) | Tampering (npm/pip/cargo installs) | 各 plan 供应链 | accept | 不适用——零新包安装（Go module bump 仅既有 direct deps；stub/mock 用标准库 + 既有 testify/eino） | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-8-01 | T-8-05 | resume payload 注入面本 phase 不存在（store 透传 bytes、无 HTTP 暴露）；Phase 11 HITL-04 落地 schema 校验 | plan-time 决策（08-02-PLAN） | 2026-08-17 |
| AR-8-02 | T-8-06 | checkpoint ID 越权面本 phase 不存在（无 HTTP Get）；DDL 预留 session_id，Phase 11 绑定 | plan-time 决策（08-02-PLAN） | 2026-08-17 |
| AR-8-03 | T-8-SC ×5 | 零新包安装，供应链 install 面不适用 | plan-time 决策（5 个 PLAN） | 2026-08-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-18 | 18 | 18 | 0 | orchestrator inline（gsd-security-auditor 端点 400 modelCode 故障，按运行时回退规则内联执行；全部 mitigate 项以一手 grep/git/test 证据核验） |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-18
