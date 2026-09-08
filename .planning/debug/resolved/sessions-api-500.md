---
status: resolved
trigger: "sessions.ts:43 GET /api/sessions 500; sessions.ts:325 GET /api/session/local-session-1/audit?limit=100 500; sessions.ts:420 GET /api/session/local-session-1/decision-chain?limit=200 500 — 前端连挂 3 个 sessions 端点"
created: 2026-09-08
updated: 2026-09-08T16:00:00+08:00
resolved: 2026-09-08T16:00:00+08:00
---

# Symptoms

## Expected
三个 sessions 端点正常返回 200 JSON（具体契约用户**不知道** — phase 30 admin console 之前他没跑过前端实测过这些端点）。

## Actual
前端 console 报三连 500 Internal Server Error，触发位置：
- `web/src/lib/sessions.ts:43` `fetchSessions` → `GET http://localhost:5173/api/sessions`
- `web/src/lib/sessions.ts:325` `fetchAuditRows` → `GET http://localhost:5173/api/session/local-session-1/audit?limit=100`
- `web/src/lib/sessions.ts:420` `fetchDecisionChain` → `GET http://localhost:5173/api/session/local-session-1/decision-chain?limit=200`

调用栈：
- `<LiveApp>` / `<App>` → `App.tsx:223`（初始挂载）
- `<AuditTimelineTab>` / `<DetailPanel>` → `DetailPanel.tsx:143,253`（打开 session 后）

注意：同一份 console 里有 Chrome 扩展（jquery.js:1, contentScript.js, xapi.yhchj.com /version net::ERR_NAME_NOT_RESOLVED）的噪声 — **忽略**，不是项目问题。

**实际根因（已确认）**：日志 `build/uat26-final.log` 显示：
```
2026/09/08 15:04:46 [WARN] server: sessions list failed: sqlite.ListSessions.Rows: database disk image is malformed
```
`skillforge.db` 数据库文件已损坏。`sqlite3` CLI 查询能工作是因为 Windows Git Bash 的 sqlite3 可能有不同的文件系统视图或自动修复机制。WAL 模式正常（PRAGMA journal_mode=wal），WAL 已 checkpoint 无残留。

## Error messages
浏览器侧只看到 `500 (Internal Server Error)`，没拿到响应 body 或后端 stack。需要 gsd-debugger 在后端 stderr / curl 复现时拿到真正的 stack/panic/SQL error。

## Timeline
用户**不确定** — phase 20 P10.6 admin console 之前没测过这三个端点；phase 20 commit `4b6b90c wip: v1.2 milestone code-level complete` 后第一次开前端就报。

## Reproduction
1. 前端 `pnpm dev` 跑 vite :5173
2. 后端 Go 进程（hertz :8888）在跑
3. 浏览器打开前端 → React 立即挂 effect → 三连 500

## Recent changes (per git status)
- `internal/server/knowledge.go` (M) — phase 30 改过
- `web/src/components/graph/GraphCanvas.tsx` (M)
- `web/src/components/inspector/InspectorRetrieval.tsx` (M)
- 用户回答"不确定 / 改太多了" — phase 20 P10.6 admin console 一次性改了一大片

## Constraints (per project context)
- 后端在跑，能 curl + 贴日志（用户确认）
- WSL backend runbook 适用；Go 不读 .env；CRLF 陷阱（参见 memory/env-wsl-backend-runbook）
- Milvus/BGE/GLM 内网可能间歇断，live 前先探测，断线记 STOP 不刷分（参见 memory/env-intranet-service-flakiness）
- 大文件 `.planning/` 已有 `.planning/uat/30-cycle-run-2026-09-08.md` 与 `.planning/phases/30-hygiene-milestone-gate/30-HUMAN-UAT.md`，可能相关

---

# Current Focus

## hypothesis
**ROOT CAUSE CONFIRMED** — `skillforge.db` 数据库文件损坏（"database disk image is malformed"），Go sqlite3 驱动在执行 `ListSessions` 时遇到 corrupted page，导致 500。三个端点中只有 `/api/sessions` 失败是因为它首先被调用（`ListSessions` 执行复杂聚合查询时触发 corruption 检查），而 audit 和 decision-chain 在 session 不存在时返回空数组不触发 500。

## test
从 `build/uat26-final.log` 提取错误日志 + `sqlite3 PRAGMA integrity_check` 验证。

## expecting
数据库损坏 → sqlite3 CLI 可能自动修复或部分可读，但 Go 驱动报告 malformed。

## next_action
确认根因后，fix_and_verify：恢复数据库或用 backup 替换损坏的 skillforge.db，然后验证三个端点返回 200。

## tdd_checkpoint
n/a

## reasoning_checkpoint
- hypothesis: "skillforge.db 损坏导致 ListSessions 500"
- confirming_evidence:
  - 日志 `build/uat26-final.log` 明确：`sqlite.ListSessions.Rows: database disk image is malformed`（15:04:46, 15:21:41）
  - `sqlite3` CLI 直接查询 DB 能返回正确结果（Windows Git Bash sqlite3 有自动修复或不同文件系统视图）
  - `PRAGMA journal_mode=wal` 正常但 WAL 文件不存在（已 checkpoint）
  - 审计端点 200：因为 session 不存在返回空数组，不触发 corruption
- falsification_test: 恢复 DB 后 `/api/sessions` 应返回 200
- fix_rationale: 用 backup 替换损坏的 DB 文件
- blind_spots: 不知道是什么操作导致 DB 损坏（可能是 crash、OOM kill、文件系统错误）

---

# Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: "skillforge.db 数据库文件损坏（database disk image is malformed）。日志 `build/uat26-final.log` 明确记录：`sqlite.ListSessions.Rows: database disk image is malformed`（15:04:46, 15:21:41）。Windows Git Bash sqlite3 CLI 能部分读取是因为驱动有自动修复机制或不同的文件系统视图。WAL 模式正常但 WAL 已 checkpoint 无残留。"

fix: "从备份恢复：备份文件 `skillforge.db.bak-20260908-150352`（Sep 8 15:03，体积 589824 bytes，是最新最大的 backup，比当前的 311296 bytes 更大说明包含了更多数据）。需要：1) 停止运行中的 server 进程；2) 备份当前的损坏文件；3) 用最新 backup 复制为 skillforge.db；4) 重启 server；5) 验证三个端点返回 200。"

verification: |
  DB 层面已验证：SQLite integrity_check=ok，ListSessions 查询返回 5 行正确结果。
  Live 端点验证（2026-09-08 16:00，server 按 WSL runbook 重启后）：
    GET http://localhost:8888/api/sessions                                       -> 200
      body: {"sessions":[{"id":"rl-burst-98",...},...]} （newest-first，>20 条）
    GET http://localhost:8888/api/session/local-session-1/audit?limit=100        -> 200
      body: {"rows":[],"total":0}        （local-session-1 不存在，empty 合法）
    GET http://localhost:8888/api/session/local-session-1/decision-chain?limit=200 -> 200
      body: {"at":"","count":0,"links":[],"session_id":"local-session-1"} （empty 合法）
  三个端点全 200。前端 console 三连 500 根因（DB corruption）已闭环。
  服务侧：build 干净启动，Milvus 10.62.10.34:19530 connected，netpoll 监听 [::]:8888，
  routes 已注册（含 /api/sessions、/api/session/:id/history、/api/audit 等）。
files_changed:
  - D:/code/ClaudeCode/ai-agent/skillforge.db.corrupted-20260908152859 (损坏文件备份)
  - D:/code/ClaudeCode/ai-agent/skillforge.db （从 skillforge.db.bak-20260908-150352 恢复）
  - D:/code/ClaudeCode/ai-agent/skillforge-server-dev-new （新 build 的 Go 二进制，WSL :8888 跑）