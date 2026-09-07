# Phase 29 Acceptance Runbook (29-UAT)

**Phase:** 29 — 反馈闭环 + 工单接口 (Feedback Loop + Ticket Interface)
**Operator checkpoint:** HUMAN VERIFY (mirrors 27-UAT.md / 28-UAT.md)
**Authored:** 2026-09-08 (Plan 29-09)

Items marked **[operator]** require a live stack / browser / external webhook and are
operator-facing; everything in Test 9 is hermetic and re-runnable by anyone.

## Prerequisites

- **[operator]** 服务已启动且 `SKILLFORGE_ADMIN_TOKEN` 已设置：
  `SKILLFORGE_ADMIN_TOKEN=<token> go run ./cmd/server`（cgo 包在 WSL 构建：
  `wsl -e bash -lc "go run ./cmd/server"`）
- **[operator]** 至少一个 active KB（Knowledge Bases 面可上传文档）+ 一个已注册 Skill
- **[operator]** 浏览器打开 chat 主界面与 `/admin` 管理台
- 以下命令假设 `BASE=http://localhost:8888`、`TOKEN=<SKILLFORGE_ADMIN_TOKEN>`

---

### Test 1: 投票链路 — chat 一轮 → data-turn 帧 → FeedbackRow 201

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** chat 界面发一轮提问（如「差旅报销标准是什么？」） | 助理回复正常渲染，回复下方出现 FeedbackRow（有帮助 / 没帮助 / 知识可能过时） | |
| 2 | **[operator]** `curl -N -X POST $BASE/api/chat -H 'Content-Type: application/json' -d '{"session_id":"uat-fb-0001","message":"差旅报销标准是什么？"}'` | SSE 帧流中可见 `"type":"data-turn"` 帧，`data.turnId` 为正整数（位于 text-end 之前） | |
| 3 | **[operator]** 在 FeedbackRow 点击「有帮助」 | Network 面板：`POST /api/feedback` → **201**，body `{"status":"recorded","verdict":"helpful"}`；pill 变 active、其余两项禁用；toast「已记录反馈：有帮助」 | |
| 4 | `curl -s -X POST $BASE/api/feedback -H 'Content-Type: application/json' -d '{"session_id":"uat-fb-0001","turn_id":<turnId>,"verdict":"helpful"}'` | 201（脚本侧复现同链路） | |

### Test 2: 幂等 + 锁定态回显 — already_recorded + history voted_verdict

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** 对 Test 1 同一条消息再点任一反馈项 | `POST /api/feedback` → **200** `{"status":"already_recorded","verdict":"helpful"}`；FE 渲染服务端 verdict + info toast「该消息已反馈过：有帮助」；**绝不**出现第二个反馈行 | |
| 2 | `curl -s -X POST $BASE/api/feedback -H 'Content-Type: application/json' -d '{"session_id":"uat-fb-0001","turn_id":<turnId>,"verdict":"unhelpful"}'` | 200 `already_recorded`，`verdict` 仍为首投值 `helpful`（服务端真相，不被覆盖） | |
| 3 | **[operator]** 刷新页面 → 重新打开该会话（history 恢复） | 该消息 FeedbackRow 直接呈锁定态（active pill = helpful），零额外 POST 请求（Network 面板确认）；来源是 history 条目的 `voted_verdict` 字段经 `data-turn` part 回显 | |
| 4 | `curl -s "$BASE/api/sessions/<session_id>/history" -H "X-Admin-Token: $TOKEN"` | 对应 assistant 条目含 `"turn_id"` 与 `"voted_verdict":"helpful"` | |

### Test 3: outdated → /admin 复核 tab → 三处置 → 关闭数 + 处置历史

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** chat 里对一轮带引用的回复点「知识可能过时」，可填一行说明后提交 | 201 recorded；说明随 body.comment 发送并收起 | |
| 2 | **[operator]** `/admin` → 复核 tab（第 7 个 tab） | 该 chunk 出现为聚合行（同一 chunk 多次报告合并，report_count 计数）；FBK-05 信任横幅常驻：「不会自动修改知识库内容」 | |
| 3 | `curl -s "$BASE/api/admin/feedback/review" -H "X-Admin-Token: $TOKEN"` | 200 `{"pending":[…]}`，行含 chunk_id / report_count / 说明列表 | |
| 4 | **[operator]** 点「说明 (n)」展开 | 逐条报告可见（空说明显示「（未填写说明）」）；chunk 预览可加载 | |
| 5 | **[operator]** 点击三处置之一（已更新 / 误报 / 忽略） | **201** `{"closed_count":N}`；success toast 报关闭条数；行从 pending 消失（服务端真相 refetch） | |
| 6 | `curl -s -X POST $BASE/api/admin/feedback/dispositions -H "X-Admin-Token: $TOKEN" -H 'Content-Type: application/json' -d '{"chunk_id":<id>,"disposition":"ignored","note":"uat"}'` | 201 `{"closed_count":…}`（脚本侧复现） | |
| 7 | **[operator]** 复核 tab 切到已处置过滤（disposition 过滤 + badge 三色） | 处置历史可见：已更新=compliant / 误报=non-compliant / 忽略=unanswered 三色 badge | |
| 8 | `curl -s "$BASE/api/admin/feedback/review?filter=resolved" -H "X-Admin-Token: $TOKEN"` | 200 `{"dispositions":[…]}` 含刚创建的处置行（disposition + note 逐字留档） | |

### Test 4: 重开语义 — 处置后再报告 → pending 重现（latest-disposition-wins）

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** 对 Test 3 已处置的同一 chunk 再次报告 outdated（新一轮对话） | 投票 201 | |
| 2 | `curl -s "$BASE/api/admin/feedback/review" -H "X-Admin-Token: $TOKEN"` | 该 chunk **重新出现**在 pending（处置后新报告重开 = Open Q1），report_count 从新批次起算 | |
| 3 | **[operator]** 再次处置 | 新处置行生效（latest-disposition-wins），历史列表同时保留两条处置记录 | |

### Test 5: 指标 tab — 口径注 / 0 票「—」/ 日周切换 / per-KB 脚注

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** `/admin` → 反馈 tab（第 6 个 tab） | 页面常驻**全票口径注**（帮助率/过时率的分母=全部投票，D-29-10）；FE 永不自行计算比率 | |
| 2 | **[operator]** 选一个 0 票维度（如未投过票的 Skill） | 帮助率/过时率渲染「**—**」，绝不出现 0% 或 NaN | |
| 3 | `curl -s "$BASE/api/admin/feedback/metrics?dim=skill&days=30" -H "X-Admin-Token: $TOKEN"` | 200 `{dim,days,totals,by_key,trend}`；0 票行比率为 JSON `null` 而非 0 | |
| 4 | **[operator]** 时间范围 7/30 切换 + 维度 Skill/知识库切换 | 数据重取（Network 计数 +1/次）；趋势粒度 日↔周 为纯客户端重分桶，**零**额外请求 | |
| 5 | **[operator]** 查看 per-KB 区域 | 双计脚注逐字可见（一票涉多 KB 每 KB 各计一次，totals 仍取严格单计聚合） | |
| 6 | **[operator]** 点「待复核 N 条」cross-link | 跳到复核 tab | |

### Test 6: 工单 provider=none — TKT-n 受理编号 + 留档文案

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** 默认启动（`SKILLFORGE_TICKET_PROVIDER` 未设 = none），chat 里对一条回复展开「创建工单」 | 表单预填问题描述（chatText 截 200 字）+ mono 引用条（会话短 id · 轮次 #n · chunk chips） | |
| 2 | **[operator]** 填写并提交 | **201** `{"ticket_number":"TKT-<n>","dispatch_status":"none"}`；文案为**留档**语义（已受理落库、无外部系统）；trigger 变「已建单」禁用 | |
| 3 | `curl -s -X POST $BASE/api/tickets -H 'Content-Type: application/json' -d '{"session_id":"uat-fb-0001","turn_id":<turnId>,"description":"uat ticket"}'` | 201 TKT 编号；同参重放 → **200** `{"status":"already_exists","ticket_number":<同一编号>}` 且 `dispatch_status` 字段消失，**绝不**二次外发 | |
| 4 | `curl -s -X POST $BASE/api/tickets -H 'Content-Type: application/json' -d '{"session_id":"uat-fb-0001","turn_id":<turnId>,"description":""}'` | 400（description 必填）；>4000 rune → 400 | |

### Test 7: 工单 provider=webhook — 本地假端点 → queued + 审计行 **[operator]**

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** 起一个本地假 webhook（任一 httptest / `nc` 脚本，2xx 即可）并带密钥校验头：`SKILLFORGE_TICKET_PROVIDER=webhook SKILLFORGE_TICKET_WEBHOOK_URL=http://127.0.0.1:9090/hook SKILLFORGE_TICKET_WEBHOOK_SECRET=s3cret go run ./cmd/server` | 服务正常启动（缺 URL 时应 `log.Fatalf` 拒绝启动——可另测一次负向） | |
| 2 | **[operator]** chat 里创建一张工单 | **201** 立即返回 `{"ticket_number":"TKT-<n>","dispatch_status":"queued"}`（响应不等外发）；假端点随后收到 POST：JSON body 含 ticketNumber/sessionId/turnId/description/chunkIds/skillId + `X-SkillForge-Ticket-Secret: s3cret`；URL 无 query（rawQuery==""） | |
| 3 | `curl -s "$BASE/api/admin/audit?..." -H "X-Admin-Token: $TOKEN"`（既有审计查询面） | 可见 `action_ticket_dispatch_succeeded` 审计行（FrozenArgs 快照 ticket_id/ticket_number/provider/attempts/external_ref）；失败场景为 `action_ticket_dispatch_failed` | |
| 4 | **[operator]** 把假端点换成 5xx/拒绝连接再建一单 | 工单**仍** 201 queued（受理与外发解耦）；审计行落 failed；服务进程存活（panic/retry 风暴不外溢） | |

### Test 8: Phase 27→29 联合验收 — 知识库变更只由上传产生 **[operator]**

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | **[operator]** 在复核 tab 对某 chunk 执行「已更新」处置 | 201 closed_count；`GET /api/admin/kb` 的文档/分块集合**零变化** | |
| 2 | `curl -s "$BASE/api/admin/kb" -H "X-Admin-Token: $TOKEN"` | 与处置前快照一致（FBK-05：复核动作零 KB 变更） | |
| 3 | **[operator]** 经 Knowledge Bases 面板上传一份新文档（Phase 27 通道） | 上传成功、审计行 `action_doc_uploaded` | |
| 4 | `curl -s "$BASE/api/admin/kb" -H "X-Admin-Token: $TOKEN"` | 知识库**仅**因这次上传产生变更（新文档/新分块出现）——知识库变更的唯一入口是上传通道 | |
| 5 | **[operator]** chat 提问命中新上传文档 | 回复引用新 chunk（[ref-N]）；GraphCanvas / InspectorRetrieval 面板同步（Phase 27→28→29 链路贯通） | |

### Test 9: 结构门 — verify-phase-29.sh（hermetic，任何人可跑）

| # | 操作 / 命令 | 期望结果 | 实测记录 |
|---|-------------|----------|----------|
| 1 | `bash scripts/verify-phase-29.sh` | 退出码 0，四 stage 全 PASS：TODO 清零（deep.TODO 4 处引用不误伤）/ FBK-05 结构锁 / D15 配对双侧存在 / DDL 纪律（零 DO UPDATE、DO NOTHING ≥4、4 张新表） | |
| 2 | `make phase-29` | 退出码 0，末行 `Phase 29 verification passed`（Windows host 无 make 时用 WSL：`wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && make phase-29"`） | |
| 3 | **负向自证（已执行，2026-09-08）**：向 `internal/server/feedback.go` 注入一行含 `UPDATE ` 的注释再跑门 | Stage 2a **FAIL** 并点名文件+行号，退出码 1；撤销后恢复 PASS——证明门真实把守而非摆设（T-29-08） | |
| 4 | `wsl -e bash -lc "go test ./... -count=1"` + `cd web && npm run test && npx tsc --noEmit` | 全量回归全绿 | |

---

## Pass Criteria

- Test 1–5（投票/幂等/复核/重开/指标）与 Test 6–7（工单双 provider）全部期望结果成立。
- Test 8 联合验收成立：处置动作零 KB 变更；知识库变更仅由上传产生。
- Test 9 结构门 PASS 且负向自证记录在案。
- 任何偏差记入 `.planning/STATE.md` Blockers。

_See also:_ `docs/threat-model/STRIDE-29.md`（信任边界与处置）、`scripts/verify-phase-29.sh`（4-stage 结构门）、`.planning/phases/28-dynamic-knowledge-graph/28-UAT.md`（Phase 28 runbook 惯例来源）。
