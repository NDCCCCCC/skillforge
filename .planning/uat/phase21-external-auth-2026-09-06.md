# Phase 21 External Authorized Channel — UAT 闭环（2026-09-06）

> **目标**：落地 DATA-04/05/06/10/11/13/14 全部 7 个 DATA 需求
> （外部授权通道全套），in-memory mock 数据源 + 持续挂起 + 双审计
> 端到端可跑。

## 交付物

### 新增代码

| 文件 | 内容 |
|------|------|
| `internal/externalauth/types.go` | QueryPreview / ExternalAuthRequest / TTLToken / AuditRow（DATA-14 schema）/ PrecompiledView / MaskSpec + Mask 函数（account/phone/email/generic）+ ViewRegistry + RejectNonSelect 守卫 |
| `internal/externalauth/source.go` | Source 接口（Execute / AppendExternalAudit / ListExternalAudit）+ MemorySource mock + clock 注入 |
| `internal/externalauth/manager.go` | Manager 编排器（RequestApproval / Approve / Reject / ExecuteWithToken / Reconcile）+ fakeStore + Store 接口 |
| `internal/nl2sql/generate.go` | DATA-11 NL2SQL 预览生成器（schema-driven prompt + SELECT-only AST gate + canonical SQL） |
| `internal/server/external_auth.go` | 4 个端点 handler（request/pending/approve/reject/execute/reconcile） + Deps.ExternalAuthManager 字段 + 路由表挂载 |

### 数据契约

DATA-14 audit row 9 字段（additive + omitempty）：
`query_id, user_id, business_reason, sql_preview, decision, data_officer_id, ttl_token_id, ts, chunk_ids_returned, mask_applied`

### 测试覆盖（`go test ./internal/externalauth/`）

| 测试 | 验证 |
|------|------|
| TestMask_AccountLast4 | DATA-05 账号 last-4 |
| TestMask_EmailKeepsFirstTwo | DATA-05 email 前 2 char |
| TestMask_AccountShortFullMask | 输入短于 KeepLen 全 mask |
| TestMask_UnknownKindFullMask | 未知 kind 全 mask（保守） |
| TestRejectNonSelect | DATA-11 SELECT-only AST gate（拒绝 INSERT/UPDATE/DELETE/DDL/CALL 等） |
| TestManager_FullApprovalAndExecute | 完整流程 + DATA-04/05/06 验证 |
| TestManager_RejectDoesNotMintToken | 拒绝路径不签发 token |
| TestManager_NonSelectRejected | DATA-11 拦截 |
| TestManager_UnregisteredViewRejected | DATA-10 视图未注册 |
| TestManager_TokenExpiryEnforced | DATA-04 TTL 过期 |
| TestManager_Reconcile_PairMismatch | DATA-06 双审计对账 |
| **TestUAT_FullChannelFlow** | **7-step 端到端 release gate** |

### TestUAT_FullChannelFlow 步骤

1. **request → pending**：NL2SQL 预览 + 视图 v_user_consumption → StatePending
2. **DATA-13 验证**：pending 状态不变（无 auto-reject / 无 auto-timeout）
3. **approve → token**：mint TTLToken (IssuedAt=now, ExpiresAt=now+5min)
4. **execute → masked rows**：account/phone/email 全部按 MaskSpec 脱敏
5. **reconcile**：内/外 audit QueryID 匹配 + ExternalOnly/InternalOnly = ∅
6. **reject 路径**：rejected audit row，TTLTokenID 为空字符串
7. **DATA-11 negative**：非 SELECT SQL 直接 ErrSQLReadOnly 拒绝

✅ UAT PASS — 1 approved + 1 rejected, all 7 DATA invariants verified

## v1.1 release gate 状态

| 需求 | 实现 | 测试 | UAT |
|------|------|------|-----|
| DATA-04 TTL token | ✅ TTLToken + ExecuteWithToken + TTL expiry | TestManager_TokenExpiryEnforced | UAT step 3,4 |
| DATA-05 二级脱敏 | ✅ MaskSpec + ApplyMask + View.Mask | 4 个 mask test | UAT step 4 |
| DATA-06 双重审计 | ✅ Source + AuditRow + Reconcile | TestManager_Reconcile_PairMismatch | UAT step 5 |
| DATA-10 预编译视图 | ✅ ViewRegistry + ErrViewNotAllowed | TestManager_UnregisteredViewRejected | UAT step 1 |
| DATA-11 NL2SQL 预览 | ✅ nl2sql.Generate + RejectNonSelect | TestRejectNonSelect + TestManager_NonSelectRejected | UAT step 1,7 |
| DATA-13 持续挂起 | ✅ State pending 不变 + 仅显式 resolve | TestUAT_FullChannelFlow step 2 | UAT step 2,6 |
| DATA-14 audit row schema | ✅ AuditRow 9 字段 + MaskApplied | UAT step 5 | UAT step 5 |

## 已知边界 / 不做

- ❌ 真数仓驱动（Postgres / MySQL / Oracle）— `Source` 接口已留，v1.2
  切换零代码改动
- ❌ TTL token store vault 集成 — 当前 fakeStore 内存版，生产 vault 留 v1.2
- ❌ 自动 SQL 风险分级 — v1.1 强制人工审批，自动分级留 v1.2
- ❌ FE 面板（ExternalChannelPanel）— Wave 4 已写 server 端点；
  curl UAT 足够覆盖 trust surface，FE 面板留给 v1.2 或用户使用需求时

## 运行方式

```bash
# Build server (WSL only — Windows native gcc has known emutls issue):
wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && \
  /usr/local/go/bin/go build -o skillforge-server-dev-new ./cmd/server'

# Run tests (works on both Windows + WSL):
go test -v -run TestUAT ./internal/externalauth/
```
