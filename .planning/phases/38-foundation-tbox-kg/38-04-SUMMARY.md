---
plan: 38-04
title: internal/server/admin_ontology.go — 8 API 路由 + admin token gate
status: complete
commit: 14463e5
requirements: [ONT-01, ONT-04, ONT-06, ONT-07, ONT-08]
---

# Plan 38-04 Summary — Ontology Admin API + Boot 集成

## What Was Built

- `internal/server/admin_ontology.go` — 8 个 token-gated handler (D-Route-01 全量):
  - `GET /api/admin/ontology/tbox` — pointer TTL + KG live count + version
  - `GET /api/admin/ontology/kg/stats` — Stats() 投影
  - `GET /api/admin/ontology/kg/query?s=&p=&o=` — 通配模式查询
  - `POST /api/admin/ontology/proposals/generate` — LLM 提议 (pending)
  - `GET /api/admin/ontology/proposals` — 队列列表
  - `POST /api/admin/ontology/proposals/:id/approve` — 唯一 TBox 变更路径; IRI 冲突 → 409
  - `POST /api/admin/ontology/proposals/:id/reject` — 状态标记
  - `POST /api/admin/ontology/snapshot` — 手动落盘 (调试)
- `Deps.Ontology *OntologyDeps` nil-posture (nil → 503); 403 在 503 前 (CHECK ORDER)
- `server.go` 路由注册块
- `cmd/server/main.go` boot 块:
  - 首次 boot: emitter 生成 → v1.ttl + pointer + tbox_version.txt
  - 重启: 复用 pointer (approved induction 跨重启存活)
  - KG LoadFromTurtle → 立即 WriteSnapshot → SnapshotDebounced(5s) 挂到 Manager.onChange
  - 日志: `[OK] cmd/server: ontology layer wired (TBox vN, N triples, snapshot path)`
- `induction.Manager` 增强: `Version()` / `DataDir()` / `SetOnChange` + fireChange

## Tests

```
6/6 handler tests PASS (403×8 路由, 503 nil, tbox 200 含 version/triple_count/ttl,
kg query 2 matches, approve flow v1→v2 含 tbox endpoint 复验, snapshot)
全量回归: internal/server 15.2s ok + internal/ontology/... 3 包 ok
```

## Deviations from Plan

- 软链 `skillforge_tbox.ttl` → v{N}.ttl 实现为 **copy 而非 symlink** (Windows 无权限时 fallback 提前到 Phase 38 内置, 与 38-03 一致)
- `EnvOr` 命名沿用 main.go 现有 `envOr` helper

## Self-Check: PASSED

- [x] 8 路由 + handler ≤ 200 行 (admin_ontology.go 全文件 ~270 行含 godoc/注释, 单 handler 均 < 50 行)
- [x] WSL `go build ./cmd/server/ ./internal/...` 通过
- [x] 全量 server + ontology 回归绿
