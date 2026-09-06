# Phase 21 — Plan Outline (5 waves)

## Wave 1 — 数据基础 + 包骨架 (Done 落地)
- `internal/externalauth/` 包：
  - `externalauth.ExternalChannel` 接口（RequestApproval / Approve /
    Reject / Execute）
  - `externalauth.TTLToken` 类型（token_id, view_name, user_id, ttl,
    scope_hash）
  - `externalauth.QueryPreview` 类型（sql_preview, estimated_rows,
    business_reason, requested_at）
  - `externalauth.AuditRow` 类型（对应 DATA-14 的 9 字段 schema）
  - `externalauth.Source` 接口 + `source.MemorySource` mock（用
    in-memory table 模拟数仓，便于本地 UAT）

## Wave 2 — Store + 脱敏 (Done 落地)
- `internal/store/store.go` 接口加 3 个方法（issue ttl / list pending /
  reconcile audit）
- `internal/store/sqlite/` 加 2 张表 + adapter：
  - `external_auth_tokens` (token_id PK, view_name, user_id,
    ttl_unix_ms, scope_hash, created_at)
  - `external_auth_audit` (D14 schema：query_id PK, user_id,
    business_reason, sql_preview, decision, data_officer_id,
    ttl_token_id, ts, chunk_ids_returned, mask_applied)
- `externalauth.Mask` 函数：账号 last-4、手机 mid-4→*、邮箱本地
  部分前 2 char；registry-driven（白名单 view + 字段 regex map）

## Wave 3 — NL2SQL 预览 (Done 落地)
- `internal/nl2sql/` 包：
  - `nl2sql.Generate(ctx, agent, schema, query) (QueryPreview, error)`
  - schema = PrecompiledView 列表 + 字段类型（避免 hallucinate）
  - 输出预览 = 单 SELECT SQL 字符串 + estimated_rows（粗略）
  - 强制 DDL-only / read-only AST check（拒绝 INSERT/UPDATE/DELETE/DDL）
- `internal/agent/hitl_tools.go` 新增 `needs_external_query` tool，
  强制先调 NL2SQL → 拿到 preview → 调 externalauth.RequestApproval
  → 阻塞等审批

## Wave 4 — Server 端点 + FE 面板 (Done 落地)
- `internal/server/external_auth.go`：
  - `POST /api/external-auth/request` — 创建审批请求 + 返回 TTL token + pending state
  - `GET /api/external-auth/pending?data_officer_id=` — 数据人员看板
  - `POST /api/external-auth/approve` / `/reject` — D13 持续挂起；审批后落地 D14 audit row + 触发 externalauth.Execute
  - `GET /api/external-auth/audit/reconcile?from=&to=` — DATA-06 双审计对账
- `web/src/components/ExternalChannelPanel.tsx`：
  - SQL preview 卡片（高亮 view_name + 字段）
  - 批 / 拒按钮（带 reason 必填）
  - TTL 倒计时（无超时 = 0，仅展示当前 token 寿命）
  - 双审计对账 tab

## Wave 5 — Live UAT + 闭环 (Done 落地)
- 端到端 demo：mock 数仓 + 问"员工 E 上个月消费多少" → NL2SQL 预览 →
  数据人员批 → masked rows 返回 → 双审计可对账
- golden set：`eval/frozen/external-auth.jsonl` (10 题，覆盖 7 需求)
- `.planning/uat/phase21-external-auth-2026-09-06.md` 闭环报告
- README 增加"External Authorized Channel"节，标题对标 DATA-04..14
