# Phase 21 — External Authorized Channel (DATA-04/05/06/10/11/13/14)

> **状态**: 2026-09-06 IN PROGRESS。本里程碑落地 7 个 DATA 需求（详见
> REQUIREMENTS.md）：外部授权通道全套 NL2SQL 安全防护 + 双审计对账。
> 是 v1.1 计划的最后两个 Phase（21 + 23）之一。

## 上下文与决策契约

### 为什么需要这个

v1.0 SkillForge 只能查自有 Milvus（向量文档）。客户场景里 80% 的
**个体识别查询**（"张三上个月消费了多少""员工 E 的银行卡号是多少"）
需要访问企业内部数仓——那里面是 PII 真值，不是 chunk。直接把
数据库连接交出去又违反最小权限原则（agent 不能 SELECT 真实表）。本
Phase 引入**外部授权通道**：原始数据留在数仓，agent 通过受控代理 +
TTL token + 预编译视图 + 数据人员人工审批来访问。

### 范围（7 个 DATA 需求）

| 需求 | 实现位置 | 状态 |
|------|---------|------|
| DATA-04 外部授权通道（DB user + TTL token） | `internal/externalauth` + sqlite 表 | 待落地 |
| DATA-05 二级脱敏 + 数据人员预授权 | `externalauth.Mask` + 审批流 | 待落地 |
| DATA-06 双重审计（内部 + 外部可对账） | `external_auth_audit` 表 + `GET /audit/reconcile` | 待落地 |
| DATA-10 个体通道走预编译视图（非原始表） | `externalauth.PrecompiledView` 注册表 | 待落地 |
| DATA-11 NL2SQL 输出预览强制先审后查 | `internal/nl2sql` + ReAct tool | 待落地 |
| DATA-13 审批持续挂起（无 auto-reject / 无 auto-timeout） | `approval_state = pending` 不变 | 待落地 |
| DATA-14 审批 audit row schema（D14） | `external_auth_audit` 列定义 | 待落地 |

### 核心数据流

```
User query (in compliance/HR skill)
        │
        ▼
agent.Run (ReAct)
        │
        ▼  命中 needs_external_query 工具
nl2sql.Generate(user_query) ──▶ sql_preview string
        │
        ▼
externalauth.RequestApproval(user, sql_preview, reason)
        │
        ▼  写入 audit row (action_external_auth_requested)
        │
        ▼  返回 ttl_token + pending state
        │
agent 阻塞（chat streaming finish_step → 等审批）
        │
        ▼
数据人员 UI: 看到 SQL preview + reason
        │
   ┌────┴────┐
   │ approve │ reject
   ▼         ▼
externalauth.Approve|Reject
        │
        ▼  D14: action_external_auth_approved|rejected
        │
        ▼  externalauth.Execute(view, ttl_token)
            │         (DATA-10 走预编译视图非原始表)
            │         (DATA-05 字段级 mask)
            │         (DATA-06 双审计：内部行 + 外部行同 query_id)
            ▼
        返回 masked rows
        │
        ▼  agent 看到 rows → 继续生成 verdict
```

### 纪律

- **DB user 永远 read-only + 仅访问预编译视图**：agent 拿到的 TTL
  token 绑定到一组允许的 view 列表，不是整个 schema。
- **持续挂起**：审批状态只能由数据人员**显式**改。CLI / HTTP 没
  有"超时自动拒"路径。这是 trust 头等大事（不能把"忘了审批"当
  "默认拒"或"默认过"）。
- **双审计对账**：内部行（SkillForge `external_auth_audit`）和外部
  行（数仓 `query_log`）通过 `query_id` + `ts_window` 对账；缺失
  任何一方视为安全事件（fail-closed）。
- **TT L token 短寿命 + scope-bound**：默认 5 分钟过期，TTL 到期
  token 即失效；scope = (view_name, user_id) 二元组绑定。

### 不做（v1.1 范围内主动放弃）

- ❌ 真数仓连接器（PostgreSQL / MySQL / Oracle）：Phase 21 用
  `internal/externalauth/source.MemorySource` 作 in-memory mock，可
  通过 DataSource 接口在 v1.2 切换真实驱动；接口设计确保切换零
  代码改动。
- ❌ 实际数据库账号管理（vault / IAM）：TTL token 的发放由测试
  stub 提供，真实 token store 留作 v1.2 候选（需要运营商配合）。
- ❌ 跨 SkillForge 集群联邦查询：单实例自洽，多集群等运营方需求。
- ❌ 自动 SQL 风险分级（黑/白名单视图）：v1.1 要求人工审批，
  自动分级留 v1.2（与预算事件 plan-04 的 budget_layer 命名一致）。

### 关键文件（计划新增 / 修改）

新增：
- `internal/externalauth/` (package)
- `internal/nl2sql/` (package)
- `web/src/components/ExternalChannelPanel.tsx`
- `eval/frozen/external-auth.jsonl`（golden set）
- `.planning/phases/21-external-auth-channel/21-*.md`

修改：
- `internal/store/store.go` — 加 `external_auth_*` 接口
- `internal/store/sqlite/` — 加 2 张表
- `internal/server/server.go` — 注册 4 个新端点
- `internal/agent/hitl_tools.go` — 加 `needs_external_query` tool
- `examples/skills/compliance/` — 加 channel 配置

### v1.1 release gate

- 7 个 DATA 需求全部实现 + 测试覆盖
- 双审计对账 demo（memory mock 数据源 + 完整审批流）端到端通
- 持续挂起语义被测试锁定（无 auto-reject 路径）
- README 更新一节"External Authorized Channel"
