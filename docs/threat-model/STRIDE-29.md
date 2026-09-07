# Phase 29 — Threat Model (STRIDE-29)

**Phase:** 29 — 反馈闭环 + 工单接口 (Feedback Loop + Ticket Interface)
**Authored:** 2026-09-08 (Plan 29-09)
**Status:** active — locks the trust surface for Phase 29
**Gate:** `bash scripts/verify-phase-29.sh` (4 stages) / `make phase-29`

---

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| 匿名客户端 → POST /api/feedback | Public write; session/turn 服务端绑定，无 admin token |
| 匿名客户端 → POST /api/tickets | Public write; description 直达外部 provider，长度封顶 |
| 匿名客户端 → admin 三端点 (/api/admin/feedback/*) | admin token 门；403 先于 503 |
| SkillForge → 外部 webhook (ticket provider) | 唯一出站信任面；URL 仅来自 operator env |
| 反馈数据 → 知识库 | **结构禁止面**：反馈/复核代码零 KB 变更动词（FBK-05） |
| 出站 goroutine → 进程 | panic/重试风暴不得拖垮请求路径或进程 |
| 门脚本 → 代码库 | verify 门本身是信任面：被弱化的门等于没有门 |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-29-01 | Spoofing | 垃圾投票 / 垃圾工单（公开写端点） | mitigate | `UNIQUE(session_id, turn_id)` 幂等键（schema.go:405 feedback / :470 tickets）+ 三重服务端绑定（sessionIDRegex `^[a-zA-Z0-9-]{8,128}$` handlers_session_history.go:43 → turn 存在性 → role=assistant）+ 长度上限（comment 2000 rune feedback.go:71 / description 4000 rune tickets.go:57）；重放返回 `200 already_recorded` 绝不二次计票/二次外发 |
| T-29-02 | Information Disclosure | webhook SSRF / 密钥泄漏 | mitigate | URL 仅来自 `SKILLFORGE_TICKET_WEBHOOK_URL` env（main.go:355，零用户输入；测试断言 rawQuery==""）+ payload 只进 JSON body（用户描述永不进 URL/header）+ 可选 `X-SkillForge-Ticket-Secret` 头（webhook.go:39 `SecretHeader`，env `SKILLFORGE_TICKET_WEBHOOK_SECRET`）+ per-instance `*http.Client`（`DefaultTransport.Clone()`，不污染进程级默认 client，BGE 先例） |
| T-29-03 | Information Disclosure | admin 503 探测（未授权探测依赖装配状态） | mitigate | admin 三端点首语句 `isAdminRequest(c)`（feedback.go:436/535/663）——403 先于 503 门序，hermetic 断言锁定首语句（29-05 `TestFeedbackAdmin...` grep 锁）；未授权者无法区分「未配置」与「不存在」 |
| T-29-04 | Tampering | SQL 注入（新表查询 / 聚合 / refs 拆分） | mitigate | 全部 `?` 参数化（T-3-SQLI 姿态延续）；表名/列名常量化零拼接；refs CSV 段经 `ParseInt` 逐段转换，不可解析段跳过而非拼进 SQL（29-05 refs 归一化）；`UNIQUE`/`CHECK` 词表在 DDL 层兜底 |
| T-29-05 | Elevation of Privilege | 反馈自动改信任（FBK-05 红线回归） | mitigate | **结构锁双层**：(1) `verify-phase-29.sh` Stage 2 source-grep——feedback.go + store/sqlite/{feedback,tickets}.go 零 `UPDATE `/`DELETE `/`Insert(`、retriever 唯一引用 GetChunk、零 internal/knowledge 引用；(2) `TestFeedbackNoTrustMutation`（internal/server/feedback_test.go:1082）进程内同款断言。复核处置仅产生状态行（feedback_dispositions INSERT-only）+ 审计行，知识库变更只能由 KB 上传通道产生（见 29-UAT.md Test 8 联合验收） |
| T-29-06 | Repudiation | 跨会话投票错绑（turn id 混淆 / 重放错绑） | mitigate | turn 查询强制双键 `WHERE session_id = ? AND turn_id = ?`（store/sqlite/feedback.go:90）与 `WHERE session_id = ? AND id = ?`（:557）；`AppendTurn` 幂等重放返回 0 且永不触碰 `LastInsertId`（29-01，mattn 冲突后 rowid 静默跨轮错绑已被守卫）；conflict → `already_recorded` 读回真实 verdict（FE 锁定态只是服务端真相的缓存）；emitTurnID 对 turnID<=0 零字节输出——冲突/降级绝不编造身份 |
| T-29-07 | Denial of Service | 出站 goroutine panic / 重试风暴拖垮进程或请求路径 | mitigate | `defer recover()` 兜底（panic 落 failed 审计行，进程存活）+ 总预算 `TicketDispatchBudget`（`context.Background()` 派生 + WithTimeout，main.go:358 default 30000ms）+ 有界重试 `retries+1` 次每次独立 WithTimeout、1s ctx-aware 固定退避（webhook.go:122,141）；慢 provider 不阻塞 201 响应（29-06 `TestTicketDispatchNonBlocking` 300ms provider 下 <150ms 返回）；结果只写 INSERT-only 审计行 |
| T-29-08 | Repudiation | 威胁处置缺失（登记表某行 Disposition 为空 = 未审计面） | mitigate | 本登记表每威胁行强制 Disposition 列（29-09 验收 grep 断言非空）；门弱化面另设边界行「门脚本 → 代码库」——负向自证（注入 `UPDATE ` → Stage 2a FAIL exit 1，撤销后恢复 PASS）证明门真实把守而非摆设，记录于 29-09-SUMMARY |
| T-29-09 | Tampering | FE 渲染注入（用户 comment / 服务端文案进 DOM） | mitigate | 全部反馈/工单/admin 面板走 JSX 文本节点（React 自动转义）；5 个 phase-29 FE 文件（FeedbackRow / MessageBubble / FeedbackMetricsPanel / ReviewQueuePanel / SfToast）`dangerouslySetInnerHTML` 计数全 0（29-07/29-08 测试锁定 + 人工复查）；服务端错误原文仅作行内文本回退 |
| T-29-SC | Tampering | 供应链（依赖安装） | accept | 本 phase 零新增依赖（29-01..29-09 全 plan 合法性审计空表；internal/ticket 纯 stdlib；FE 复用既有 lucide-react / zustand）；`Provider` 窄接口恰好一个方法，外部工单系统（Jira/ServiceNow）明确 Out of Scope |

## Mitigation Summary

- **T-29-01**: 幂等键 + 三重绑定 + 长度封顶；DB `UNIQUE` 约束是唯一真相，FE 锁定态只是缓存。
- **T-29-02**: 出站面 = env-only URL + body-only payload + 可选密钥头 + per-instance client；webhook-无-URL 在装配点 `log.Fatalf`（main.go:360，D16-08-D：坏配置 fatal 而非静默降级）。
- **T-29-03**: 403 先于 503 的固定检查顺序，hermetic 测试 grep 锁定首语句。
- **T-29-04**: 参数化 + 常量化列名 + DDL 层 CHECK 封闭词表。
- **T-29-05**: 双层结构锁（verify 门 Stage 2 + 进程内测试）；负向自证证明门有效。
- **T-29-06**: 双键查询 + LastInsertId 守卫 + conflict 读回真相；轮次身份缺失时诚实禁用（FE title「该消息缺少服务端轮次标识」），不编造。
- **T-29-07**: recover + 总预算 + 有界重试 + 非阻塞受理；审计行留痕可对账。
- **T-29-08**: Disposition 列强制非空 + 门负向自证记录。
- **T-29-09**: 零 dangerouslySetInnerHTML，JSX 转义面。
- **T-29-SC**: 零新依赖标准；provider 可插拔但零第三方 SDK。

---

_See also:_ `docs/threat-model/STRIDE-27.md` (Phase 27 upload trust contract), `docs/threat-model/STRIDE-26.md` (HYG structural-ban precedent), `verify-phase-29.sh` Stage 2 (FBK-05 source-grep 门), `.planning/phases/28-dynamic-knowledge-graph/STRIDE-28.md` (Phase 28 graph contract), 29-RESEARCH.md §Security Domain.
