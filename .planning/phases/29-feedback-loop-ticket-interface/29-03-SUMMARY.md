---
phase: 29-feedback-loop-ticket-interface
plan: 03
subsystem: ticket-service
tags: [go, ticket-provider, webhook, registry, httptest, pure-stdlib, tkt-01]

# Dependency graph
requires:
  - phase: 15-p10-1-kpi-aggregation
    provides: narrow-interface + NotWired-constant discipline (retriever.IndexerBackend shape mirror)
  - phase: 02-compliance-vertical-slice
    provides: per-instance *http.Client + ctx-aware bounded-retry precedent (internal/embedder/bge/bge.go)
provides:
  - "ticket.Provider narrow interface — exactly one method Dispatch(ctx, Ticket) (DispatchResult, error), terminal semantics"
  - "ticket.NewProvider(name, cfg) config-driven registry: none (zero side-effect store-only) / webhook (HTTP outbound) / unknown -> error listing valid values"
  - "ticket.Ticket / ticket.DispatchResult / ticket.Config / ticket.ProviderNotWiredError 503-body constant"
  - "webhookProvider: per-instance client (cloned DefaultTransport), retries+1 bounded attempts, 1s ctx-aware backoff, per-attempt timeout, 2xx = success, body-only payload, optional X-SkillForge-Ticket-Secret header"
affects: [29-06 POST /api/tickets + main.go SKILLFORGE_TICKET_* assembly, 29-09 phase hygiene gate]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "one-method Provider interface + ProviderNotWiredError constant — IndexerBackend narrow-surface discipline second application"
    - "per-instance http.Client via http.DefaultTransport.(*http.Transport).Clone() — keeps proxy/TLS defaults with a private connection pool, never shares the process-wide default client"
    - "config-string factory returns error on bad values; the fatal decision (log.Fatalf) stays at the assembly point (D16-08-D posture)"
    - "user text only in the JSON body; optional secret only in a header; the URL is operator config and stays byte-untouched (T-29-02)"

key-files:
  created:
    - internal/ticket/ticket.go
    - internal/ticket/registry.go
    - internal/ticket/registry_test.go
    - internal/ticket/webhook.go
    - internal/ticket/webhook_test.go
  modified: []

key-decisions:
  - "webhook without a WebhookURL is rejected by NewProvider with an error naming the missing knob — this gives 29-06's main.go a single error path that covers both webhook-no-URL and unknown-provider-name log.Fatalf branches"
  - "chunkIds marshals as [] (not null) when Ticket.ChunkIDs is nil — friendlier to generic webhook consumers"
  - "DispatchResult.ExternalRef stays empty for the webhook provider: a generic webhook has no agreed receipt shape, and an invented ref would be a small lie in the audit stream"
  - "no TLS-skip-verify knob: intranet webhooks verify certificates like everything else (bge's InsecureSkipVerify is operator opt-in for a legacy endpoint, not a default worth copying)"
  - "webhook.go landed in Task 1's GREEN commit because registry.go's webhook branch constructs webhookProvider — the package cannot compile with the factory but without the provider (Task 1's own behavior asserts var _ Provider = (*webhookProvider)(nil))"

patterns-established:
  - "Provider interface is reflect-locked to exactly one method (TestProviderInterfaceIsNarrow) — future integrations (Jira/ServiceNow) satisfy it with a single function"
  - "secret header name is the exported constant ticket.SecretHeader (X-SkillForge-Ticket-Secret) so 29-06's docs/tests reference one symbol"

requirements-completed: [TKT-01]

# Metrics
duration: 14min
completed: 2026-09-08
tasks-completed: 2
files-created: 5
files-modified: 0
tests-added: 14  # TestNewProvider(5 sub) + TestProviderInterfaceIsNarrow + TestProviderNotWiredErrorConstant + TestWebhookDispatch(7 sub)

commits:
  - "625b143 test(29-03): add failing TicketProvider registry contract tests (TKT-01)"
  - "d585b14 feat(29-03): implement ticket contract + provider registry (TKT-01)"
  - "370944a test(29-03): lock webhook dispatch behavior with hermetic httptest matrix (TKT-01)"
---

# Phase 29 Plan 03: TicketService 接口预留面 (TKT-01) Summary

**One-liner:** `internal/ticket` 新包——单方法 `Provider.Dispatch` 窄接口 + none/webhook 配置驱动注册表 + per-instance-client webhook 出站（retries+1 有界重试 / 1s ctx-aware 退避 / 每次尝试超时 / body-only payload），14 个 Windows 原生 hermetic 测试全绿。

## What Was Built

- **ticket.go** — 契约面：`Ticket`（Number/SessionID/TurnID/Description/ChunkIDs/SkillID）、`DispatchResult`（ExternalRef 空 = 无外部回执 / Attempts）、`Provider`（恰好一个 `Dispatch` 方法，godoc 写明终态语义：成功或耗尽后才返回）、`ProviderNotWiredError`（29-06 的 503 body）、`Config`（WebhookURL/Timeout/Retries/Secret；本包零 env 读取）。
- **registry.go** — `NewProvider(name, cfg)`：`"none"` → `noneProvider{}`（Dispatch 零副作用直接成功，Attempts=0）；`"webhook"` → `newWebhookProvider(cfg)`（空 URL 拒绝）；其它值 → error 列出合法值。编译期断言 `var _ Provider = noneProvider{}`。
- **webhook.go** — 出站实现：per-instance `*http.Client`（`DefaultTransport.Clone()`，零进程级默认 client 引用）；`retries+1` 有界尝试 + 尝试间 1s 固定退避（select 监听 ctx.Done）+ 每次尝试 `context.WithTimeout`；2xx 即成功；payload 只进 JSON body（键名 ticketNumber/sessionId/turnId/description/chunkIds/skillId）；可选 `X-SkillForge-Ticket-Secret` 头。编译期断言 `var _ Provider = (*webhookProvider)(nil)`。
- **webhook_test.go** — httptest hermetic 7 用例矩阵：首次 2xx / 500-后-200 / 耗尽（Attempts==3 + 错误含 attempt 数）/ 慢端点按单次超时（elapsed 界定 1.3s vs 无超时路径 ≥4s）/ 密钥头 present-iff-configured / body-only + URL query 为空 / 父 ctx 取消打断退避返回 ctx.Err()。

## Verification

- `go test ./internal/ticket/ -count=1` → ok（14 tests，6.5s，Windows 原生，纯 stdlib 零 cgo）
- `go vet ./internal/ticket/` → clean；`gofmt -l internal/ticket/` → clean
- `go build ./internal/ticket/` → OK；`grep http.DefaultClient internal/ticket/` → 0 hits
- `go build ./...` 在 cmd/server **链接**阶段失败——已记录的 D15-01-1 环境限制（MSYS2 gcc 与 go-duckdb 的 emutls ABI，本 plan 之前就存在；全部包编译通过，仅 cgo 链接挂）。本包不在此失败面上，后端链接验证归 WSL（env-wsl-backend-runbook）。

## Deviations from Plan

**1. [Rule 2 - Missing validation] NewProvider("webhook", 空 WebhookURL) 返回 error**
- **Found during:** Task 1
- **Issue:** plan behavior 矩阵只规定未知 provider 名报错，未规定 webhook 缺 URL 的分支；空 URL 会让 Dispatch 在运行时以晦涩的 `unsupported protocol scheme` 失败，而 29-06 恰恰需要「webhook-无-URL → log.Fatalf」的装配行为。
- **Fix:** registry 的 webhook 分支对空 WebhookURL 返回点名缺失配置的 error——29-06 的 main.go 用同一条 `err != nil → log.Fatalf` 路径覆盖两种坏配置（其 plan 原文即「registry error 上抛」）。
- **Files modified:** internal/ticket/registry.go + registry_test.go（`webhook_without_url_is_rejected` 子用例）
- **Commit:** d585b14

**2. [Rule 3 - Blocking] webhook.go 随 Task 1 GREEN 提交（编译耦合）**
- **Found during:** Task 1
- **Issue:** registry.go 的 webhook 分支构造 `newWebhookProvider(cfg)`；工厂不与 provider 同提交则包不可编译（plan Task 1 behavior 自身要求 `var _ Provider = (*webhookProvider)(nil)` 编译期断言）。
- **Fix:** Task 1 GREEN 提交内含完整 webhook.go（实现按 Task 2 `<action>` 逐条落地）；Task 2 提交为纯测试提交。
- **Files modified:** internal/ticket/webhook.go（提前至 d585b14）
- **Commit:** d585b14

**3. [Style - comment] webhook.go 客户端注释改写避开 `http.DefaultClient` 字面量**
- **Found during:** Task 2 verification
- **Issue:** plan verification 要求 grep internal/ticket/ 零 `http.DefaultClient` 引用；注释中的字面量会让朴素 source-grep 误报。
- **Fix:** 措辞改为「the process-wide default client is never used」。
- **Files modified:** internal/ticket/webhook.go
- **Commit:** 370944a

## TDD Gate Compliance

- **Task 1: RED + GREEN 齐**——`625b143`（test，RED：包尚不存在，undefined: NewProvider/Provider/Config，失败原因正确=契约缺失）先于 `d585b14`（feat，GREEN）。
- **Task 2: 无独立 RED（advisory 偏差）**——hermetic 矩阵（`370944a`）落在一个已含实现的提交之后，落地即绿。根因是上述编译耦合：registry 与 webhookProvider 无法拆成两个可编译提交，因此 Task 2 的 RED 无法在不制造一个不可编译中间提交的前提下成立（本仓库纪律是每个提交可编译，29-01 先例同款）。补偿：Task 2 的 7 用例逐一对照 plan `<behavior>` 写成后才跑，任何一条实现偏差都会立即暴露。

## Threat Model Posture (T-29-02 / T-29-07 / T-29-SC)

- T-29-02 (SSRF/泄漏): URL 仅来自 Config（operator env，29-06 装配）；用户描述只进 JSON body；密钥只进 `X-SkillForge-Ticket-Secret` 头；测试以 `rawQuery == ""` + 路径不变断言锁定零 URL 注入。
- T-29-07 (重试风暴): `retries+1` 有界 + 1s 固定退避 + 每次尝试 Timeout；负 Retries 钳制为 0；goroutine 总预算归 29-06。
- T-29-SC (依赖): 仅 stdlib（context/encoding/json/net/http/time/fmt/io/bytes），零第三方新依赖。

## Known Stubs

None — 本包无占位实现；`none` provider 的「零外发」是规格行为而非 stub（store-only 语义由 29-06 落库路径消费）。

## Self-Check: PASSED

- 5 个创建文件全部存在（internal/ticket/{ticket,registry,webhook,registry_test,webhook_test}.go）
- 3 个提交 hash 全部在 git log 中（625b143 / d585b14 / 370944a）
- TKT-01 代码侧完成；REQUIREMENTS 行翻转归 phase 收口（29-06/29-09 消费端点后由 verifier 统一判 Satisfied）
