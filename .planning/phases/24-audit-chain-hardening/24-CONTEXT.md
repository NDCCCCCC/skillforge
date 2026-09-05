# Phase 24 — 审计链强化 (Audit Chain Hardening) — CONTEXT

**Status:** executing (2026-09-06 起)
**Origin:** 2026-09-06 第二轮竞品验证（Semantica/WeKnora）+ v1.0 全功能 UAT。ROADMAP.md "Next Milestone (v1.1)" Phase 24 节为权威 scope。
**Sequencing:** 先于 Phase 23（OSS Release）——产出把 README 的"承认 PROV-O 是行业标准"弱势措辞翻转为"支持 PROV-O 导出"正面特性。

## 目标

对标 Semantica 的三大可审计能力，用纯确定性（零 LLM）方式补齐 SkillForge 审计链的标准出口与查询面，同时保持既有不变式不放松：

| Semantica 能力 | SkillForge 对标交付 | 状态 |
|---|---|---|
| W3C PROV-O 溯源导出（JSON/RDF） | `internal/auditprov`：会话审计行 → PROV-JSON；`GET /api/session/:id/audit/prov` | ✅ **Task 24-01 DONE（2026-09-06，本 session 实现+实机验证）** |
| `trace_decision_chain` 决策因果回溯 | `GET /api/session/:id/decision-chain?root=<tool_call_id>`：query→retrieve→verdict→approve→rule→auto-pass→revoke 链（沿 D11-C4 tool_call_id==approval_id + 规则审计痕迹）；FE 审计轨迹 tab 链式视图 | Task 24-02（待 plan） |
| 时间旅行（时间点快照回放） | `GET /api/session/:id/state?at=<RFC3339>`：从 INSERT-only 数据重建 T 时刻 turns + 事件 + 规则表（撤销规则从 action_rule_revoked.frozen_args 复原——T-3-AUDIT"审计行即证据"终极验证） | Task 24-03（待 plan） |
| （增强，超出对标） | 审计行 prev-hash 哈希链（additive 列） | 可选，planner 决策 |

## 已锁纪律（对所有 task 生效）

1. **additive migration only**（D14-E1：索引独立 const 于 ALTER 循环后）。
2. **INSERT-only 不变式不放松**：所有导出/重建为只读；source-grep 结构锁（TestAuditInsertOnly 同族）继续有效。
3. **审计路径零 LLM**——与 Semantica 同款确定性哲学：同输入同输出，可 hash 复现。
4. **过既有脱敏语义**：导出属会话面（与 `/api/session/:id/audit` 同 isolation 边界，SC-9 session 过滤）；全局 /api/audit 的 retention redaction 语义不适用于会话面（与现状一致），但 PROV 输出必须原样透传行内已脱敏哨兵（[REDACTED:...]），不得解码。
5. PROV-JSON 结构遵循 W3C PROV-JSON 惯例（entity/activity/agent + wasGeneratedBy/used/wasAttributedTo/wasDerivedFrom），命名空间前缀 `sf:`。

## PROV 映射（24-01 已实现并锁定）

对 `store.AuditFilter{SessionID}` 查出的每行：

- `sf:turn-<ID>` = prov:Activity（类型 ex:ChatTurn；Action 非空行 → ex:HITLAction，Action 值入 note）
- `sf:query-<ID>` = prov:Entity（ex:Query，text=Query）
- `sf:verdict-<ID>` = prov:Entity（ex:Verdict，VerdictJSON 非空时；RefusalReason 非空 → ex:refusal_reason note）
- `sf:chunk-<cid>` = prov:Entity（ChunkIDs CSV 展开）
- `sf:user-<UserID|anonymous>` / `sf:skill-<SkillID>` / `sf:model-<ModelVersion>` / `sf:floor-<PlatformFloorVersion>` = prov:Agent
- 关系：verdict wasGeneratedBy turn；turn used query；verdict wasDerivedFrom 各 chunk；verdict wasAttributedTo user+skill+model+floor
- 确定性：同输入行集 → 字节级相同输出（map 键排序由 encoding/json 的 key 排序保证）

## 验收口径（24-01）

- `go test ./internal/auditprov/` 绿（确定性 + CSV 展开 + HITLAction 分型）
- `go test ./internal/server/ -run TestSessionProv` 绿（会话隔离：只含本会话行；nil store 503；空会话返回空 document 非 404）
- 浏览器/curl 实测 GET 200 且 JSON 结构合法
