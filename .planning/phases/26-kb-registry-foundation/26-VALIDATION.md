---
phase: 26
slug: kb-registry-foundation
status: active
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-07
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 26-RESEARCH.md「Validation Architecture」+ 全部 10 个 plan 的 `<verify>` 块（26-01 … 26-10）。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + testify v1.11.1（BE）；vitest + @testing-library（FE） |
| **Config file** | 无 Go 配置文件（约定优于配置）；FE `web/vitest.config.*` + `web/package.json` |
| **Quick run command** | `go test ./internal/kb/ ./internal/skill/ ./internal/knowledge/ -count=1`（< 30s，纯 hermetic） |
| **Full suite command** | Windows hermetic 子集：`go test ./... -count=1` + `cd web && npx vitest run` + `npx tsc --noEmit`；CGO 全量（WSL，D15-01-1 / env-wsl-backend-runbook）：`wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && make check-cgo && go build ./... && go vet ./internal/... ./cmd/... && go test ./... -count=1"` |
| **Estimated runtime** | quick ~30s；Windows full ~几分钟；WSL full（CGO + store/sqlite）~几分钟；FE vitest ~26s |

**环境注记：** `internal/data/`、`internal/store/sqlite/`、`cmd/server`（CGO 链）在 WSL 跑；Windows 主机跑不动时记录 blocked-with-notes 并以 WSL 输出为证。Milvus / BGE-M3 / MongoDB 内网间歇（env-intranet-service-flakiness）——hermetic 路径一律走 indexSeedHook / stub / fake store。

---

## Sampling Rate

- **After every task commit:** `go test ./internal/kb/ ./internal/skill/ ./internal/knowledge/ ./internal/server/ -count=1`（< 30s，纯 hermetic）
- **After every plan wave:** `go test ./... -count=1` + `bash scripts/verify-phase-26.sh`（wave 7 起存在）+ `cd web && npx vitest run`
- **Before `/gsd:verify-work`:** Full suite must be green — WSL `make check-cgo` + `go build ./...` + `go vet ./internal/server/ ./cmd/server/` + FE `npx tsc --noEmit` exit 0（HYG-03 预演）
- **Max feedback latency:** ~30 seconds

---

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| KBM-01 | seed 加载 fail-closed + Registry 接口语义 + 原子快照 | unit | `go test ./internal/kb/ -count=1` | ❌ Wave 0（26-01） |
| KBM-01 | `skill.Validate` 经接口校验（未注册/停用=失败；role 通配展开） | unit | `go test ./internal/skill/ -run TestValidate -count=1` | ✅（改断言，26-04） |
| KBM-02 | create/deactivate 403/400/409/201 + 审计行 + 停用后 Lookup 不可见 | integration（httptest） | `go test ./internal/server/ -run TestKBAdmin -count=1` | ❌ Wave 0（26-05） |
| KBM-03 | draft 永不检索（开关不可覆盖）+ citability 门（非 authority 不入 chunksByID） | unit | `go test ./internal/kb/ ./internal/agent/ -run 'TestDraftRole\|TestCitable' -count=1` | ❌ Wave 0（26-01/26-06） |
| KBM-03 | D5-13 反证通道回归（chunksByID 快照不变）保持绿 | unit | `go test ./internal/server/ -run TestCounterEvidence -count=1` | ✅（改字面量断言，26-05） |
| KBM-04 | 既有 7 个 SKILL.md 迁移后可加载 + calculator PLACEHOLDER 注释更新 | integration | `go test ./internal/skill/ -run TestExamplesValidate -count=1` | ✅（26-04/26-07） |
| KBM-05 | milvus filter slug 表达式 + 空 slug 集 `"false"`；mongo `$in` + KNOW-02 双侧拒 | unit | `go test ./internal/retriever/... ./internal/knowledge/ -run 'TestBuildMilvusFilter\|TestIndexer\|TestSearch' -count=1` | ✅（改断言，26-03） |
| KBM-06 | 未知目录 WARN+skip+audit；停用目录 skip；共享 walker 两侧一致 | unit | `go test ./internal/knowledge/ -run TestIndex -count=1` | ✅（扩用例，26-03） |
| HYG-01 | TODO-zero / TierEnum-zero / CN 字面量 scoped ban 三门 | structural gate | `bash scripts/verify-phase-26.sh` | ❌ Wave 0（26-10，先证红） |
| 全局 | 冻结 lock 字节一致性（compliance/hitl/platform-floor 不动）+ fanout 重签 | gate | `go test ./internal/eval/ -count=1` | ✅（26-07） |

---

## Per-Task Verification Map

全部 10 个 plan、共 27 个任务元素（26 个实现任务 + 1 个 operator checkpoint）逐一映射。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 26-01-01 | 01 | 1 | KBM-01, KBM-03 | T-26-01 / 门3 | Role 封闭词表 + CitableRole=authority + slug 白名单/保留字拒收（穿越面源头） | unit | `go test ./internal/kb/ -run 'TestRoles\|TestCitableRole\|TestValidateSlug' -count=1 -v` | ❌ W0 | ⬜ pending |
| 26-01-02 | 01 | 1 | KBM-01, KBM-03 | T-26-03（停用复活）/ A3 | seed 受控文件 fail-closed 加载；draft 角色 seed 层即不可检索 | unit | `go test ./internal/kb/ -run 'TestLoadSeed\|TestDraftRole' -count=1 -v` | ❌ W0 | ⬜ pending |
| 26-01-03 | 01 | 1 | KBM-01, KBM-02, KBM-03 | T-26-01 / T-26-04-1 | ResolveReads fail-closed（unknown/停用/data: 守卫/draft 丢弃）+ 原子快照 + 变更面经 Store seam | unit | `go test ./internal/kb/ -count=1 && go vet ./internal/kb/` | ❌ W0 | ⬜ pending |
| 26-02-01 | 02 | 2 | KBM-01, KBM-02 | T-26-05-1 | knowledge_bases additive DDL + role CHECK 约束 + 无 DELETE（停用不删）；WSL/CGO | integration（WSL） | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/sqlite/ -run 'TestKBRegistry\|TestUpsertBase\|TestSetRetrieval\|TestLoadBases\|TestRoleCheck\|TestMigrateIdempotent\|TestExistingDBUpgrades' -count=1 -v"` | ❌ W0 | ⬜ pending |
| 26-02-02 | 02 | 2 | KBM-01, KBM-02 | T-26-03（Pitfall 6 双向） | 「结构归配置、开关归运营」——管理员停用不被 seed 复活；seed 新条目可达存量部署 | unit | `go test ./internal/kb/ -run 'TestOpen' -count=1 -v` | ❌ W0 | ⬜ pending |
| 26-03-01 | 03 | 2 | KBM-05, KBM-06 | T-26-03-1 / T-26-03-2 | validateTiers 注册表 fail-closed；未知目录 WARN+skip+INSERT-only 审计；停用=索引侧消失 | unit | `go test ./internal/knowledge/ -run 'TestWalk\|TestIndex\|TestValidateTiers' -count=1 -v` | ✅（walker/registry 测试新增） | ⬜ pending |
| 26-03-02 | 03 | 2 | KBM-05 | T-26-03-4（注入） | walker 两侧收敛；filter expr 模板逐字节不变、空集 `"false"` default-deny | unit | `go test ./internal/knowledge/ ./internal/retriever/... -count=1 && go vet ./internal/retriever/... ./internal/knowledge/` | ✅ | ⬜ pending |
| 26-03-03 | 03 | 2 | KBM-05, KBM-06 | T-26-03-3 | graph 节点集=活跃注册表（N 动态）；全部 cmd 调用点编译一致 | unit + build | `go build ./... 2>&1 \| head -20; go test ./internal/knowledge/ -run 'TestGraph' -count=1 -v` | ✅ | ⬜ pending |
| 26-04-01 | 04 | 3 | KBM-01, KBM-04 | T-26-04-1 / T-26-04-3 | reads 契约注册表 fail-closed；CN 字面量被拒；既有 7 SKILL.md 免改加载 | unit | `go test ./internal/skill/ -count=1 -v` | ✅（改断言） | ⬜ pending |
| 26-04-02 | 04 | 3 | KBM-04 | T-26-04-2 | datarules tier 校验注册表化 + loans.yaml 同步迁移（无「代码 slug/文件 CN」窗口） | unit | `go test ./internal/runtime/ ./internal/data/ -count=1`（data 在 WSL） | ✅ | ⬜ pending |
| 26-04-03 | 04 | 3 | KBM-01, KBM-04 | T-26-04-4（半迁移 DoS） | TierEnum 全仓清零 + 5 处 NewLoader 调用点接线 + 端点 nil registry 503 fail-closed | unit + build gate | `go build ./... 2>&1 \| head -20; go test ./internal/server/ -run 'TestValidateSkill' -count=1 -v` | ✅ | ⬜ pending |
| 26-05-01 | 05 | 4 | KBM-02 | T-26-05-1 / T-26-05-2 | token 门变更面 + 403 先于 503（CHECK ORDER）+ INSERT-only 审计行 + 公开投影信息面最小化 | integration（httptest） | `go test ./internal/server/ -run 'TestKBAdmin\|TestKnowledgeBasesPublic' -count=1 -v` | ❌ W0 | ⬜ pending |
| 26-05-02 | 05 | 4 | KBM-02, KBM-03 | T-26-05-4 | 反证检索集 = SlugsByRole(historical)（空集=0 命中）；chunksByID 快照不变 | unit | `go test ./internal/server/ -run 'TestCounterEvidence\|TestPipeline' -count=1 -v` | ✅（改断言） | ⬜ pending |
| 26-05-03 | 05 | 4 | KBM-03 | T-26-05-3（穿越） | 双 map 删除；docs/doc 走 ValidateSlug + Lookup 双闸；`tier=../cmd` 回归 | unit | `go test ./internal/server/ -run 'TestDocs\|TestDocContent' -count=1 -v` | ✅ | ⬜ pending |
| 26-06-01 | 06 | 4 | KBM-03 | T-26-06-1 / T-26-06-3 | 插入点 citability 门控；historical-only grounding 拒答（新信任契约锁定） | unit | `go test ./internal/agent/ -run 'TestCitable\|TestTrustFloor\|TestRuleBased\|TestBuildRunner' -count=1 -v` | ✅ | ⬜ pending |
| 26-06-02 | 06 | 4 | KBM-03 | T-26-06-2 | hitl Enum=ActiveSlugs 快照 + authority 默认；BuildToolsForSkill 双调用点传 reg，nil fail-closed | unit | `go test ./internal/agent/ ./internal/agent/hitl/ ./internal/agent/tools/ -count=1` | ✅（改字面量） | ⬜ pending |
| 26-07-01 | 07 | 4 | KBM-04 | T-26-07-1 / T-26-07-3 | golden 词汇迁移 + lock 按 convention 显式重签（metrics/thresholds 零改动） | gate | `go test ./internal/eval/ -count=1 -v && grep -c '"tier_ground_truth":"authority"\|"tier_ground_truth":"historical"\|"tier_ground_truth":"personal"\|"tier_ground_truth":"draft"' eval/golden/fanout.jsonl` | ✅ | ⬜ pending |
| 26-07-02 | 07 | 4 | KBM-04 | T-26-07-2（正文误伤） | spike 断言与 golden 同词汇；golden 每行 JSON 可解析 | unit | `go test ./cmd/spike-fanout/ -count=1 2>&1 \| tail -5; grep -rn "历史\|权威" cmd/spike-fanout/main.go \| head -5` | ✅ | ⬜ pending |
| 26-08-01 | 08 | 5 | KBM-02 | T-26-08-2（未知 tier 渲染） | 类型放宽 string + kb-bases 缓存 + TierBadge `[slug]` fallback + chunk-lookup 硬编码删除 | unit（vitest） | `cd web && npx vitest run src/components/common/TierBadge.test.tsx src/lib --reporter=basic` | ❌ W0 | ⬜ pending |
| 26-08-02 | 08 | 5 | KBM-02 | T-26-08-1（变更动词） | /admin KB 视图：X-Admin-Token 全写请求 + 前端 slug 正则双层校验 + 无删除/恢复 | unit（vitest） | `cd web && npx vitest run src/components/admin --reporter=basic` | ❌ W0 | ⬜ pending |
| 26-08-03 | 08 | 5 | KBM-02 | T-26-08-2 / T-26-08-3 | 6-KB fixture N 动态证据；FE 残留 CN tier 字面量/过时注释清零；tsc exit 0 | unit + type gate | `cd web && npx vitest run 2>&1 \| tail -8 && npx tsc --noEmit; echo "tsc exit=$?"` | ✅（既有套件） | ⬜ pending |
| 26-09-01 | 09 | 6 | KBM-01, KBM-02, KBM-06 | T-26-09-1 / T-26-09-3 | 装配顺序 kb.Open → skill.NewLoader 可证；审计钩子绑 PutAudit；PROVISIONAL 清除 | unit + build | `go build ./... 2>&1 \| head; go test ./cmd/server/ -count=1 -v 2>&1 \| tail -15; grep -c "PROVISIONAL (26-03)" cmd/server/main.go` | ✅（main_test.go） | ⬜ pending |
| 26-09-02 | 09 | 6 | KBM-05 | T-26-09-2（RECREATE 误触发） | RECREATE 门仅 env=="1" 触发 + Drop 仅在 RecreateCollection 路径；WSL 全量门（HYG-03 BE 证据） | unit + WSL gate | `go test ./internal/retriever/milvus/ -run 'TestRecreate' -count=1 -v && wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go build ./... && go vet ./internal/... ./cmd/... && echo WSL-GATE-OK"` | ❌ W0（client_recreate_test.go） | ⬜ pending |
| 26-10-01 | 10 | 7 | KBM-03 | T-26-10-2（冻结基线） | 四角色语义双锚点落档；platformfloor 零字节改动（git diff --quiet 断言） | doc + structural | `grep -c "authority" README.md; git diff --stat internal/platformfloor/platform_floor.go \| wc -l; ls docs/threat-model/STRIDE-26.md` | ❌ W0（STRIDE-26.md） | ⬜ pending |
| 26-10-02 | 10 | 7 | HYG-01 | T-26-10-1（门疲劳/误报） | 三门先证红（exit 1 点名 long.go:54）+ deep.TODO 零误报断言 + Makefile 目标 | structural gate（RED 证明） | `bash scripts/verify-phase-26.sh; echo "gate exit=$? (expect 1 — RED proof before TODO cleanup)"; grep -c "verify-phase-26" Makefile` | ❌ W0 | ⬜ pending |
| 26-10-03 | 10 | 7 | HYG-01 | T-26-10-4（事实性） | TODO/FIXME 清零（long.go:54 事实性改写）+ 全门转绿 + UAT 手册成文 | structural gate（GREEN） | `bash scripts/verify-phase-26.sh && echo GATE-GREEN && grep -rnIE 'TODO\(\|TODO:\|FIXME\(\|FIXME:' --include='*.go' internal cmd \| grep -v '_test.go' \| wc -l` | ✅（long.go） | ⬜ pending |
| 26-10-04 | 10 | 7 | KBM-05（live 验收） | T-26-10-3 | live 向量库迁移 + 管理面 UAT 由 operator 执行——见 Manual-Only Verifications | **manual checkpoint** | —（operator gate） | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/kb/`（包 + registry/seed/slug/open/store 契约 + 全套单测）——KBM-01/02/03 核心 → **26-01（wave 1）/ 26-02（wave 2）**
- [ ] `internal/server/kb_admin_test.go`——KBM-02 端点 + 审计 + CHECK ORDER 断言（create 端点**必须**有 POST，勿照抄 KPI 只读锁）→ **26-05 Task 1（wave 4）**
- [ ] `scripts/verify-phase-26.sh` + Makefile `verify-phase-26` 目标——HYG-01 三门，先证红后转绿（D8-04 负向证明）→ **26-10 Task 2（wave 7）**
- [ ] `internal/knowledge/indexer_registry_test.go`——未知目录/停用目录/审计回调三态 + 共享 walker 两侧各一条 → **26-03 Task 1/2（wave 2）**
- [ ] FE：`web/src/components/common/TierBadge.test.tsx` + `web/src/components/admin/KnowledgeBaseList.test.tsx` + graph 参数化 fixture → **26-08（wave 5）**
- [ ] `internal/retriever/milvus/client_recreate_test.go`——RECREATE 门序列/解析两测试 → **26-09 Task 2（wave 6）**

*Wave-0 deliverables 全部已落位于对应 plan 的首个触面任务；无「MISSING — Wave 0 must create」悬空引用。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| live Milvus 向量库迁移（tier 列 CN → slug，drop+recreate+重嵌）后检索非零命中 | KBM-05 | Milvus/BGE-M3 为内网间歇服务，drop+recreate 是破坏性生产操作，仅 operator 可执行（26-10 checkpoint；env-intranet-service-flakiness 纪律） | 1) 探测 Milvus/BGE-M3 可达；2) `.env` 设 `SKILLFORGE_MILVUS_RECREATE=1` 启动 cmd/server，观察 RECREATE WARN + 重索引完成行；3) 移除 env 并重启；4) 验收：① tier 查询非零命中 ② /eval 无 100% refusal ③ 图谱节点 chunkCount>0 ④ /admin 建库+停用+重启不复活 ⑤ `bash scripts/verify-phase-26.sh` exit 0；不可达时记 blocked-with-notes 于 STATE Blockers，不刷分 |

*除此之外全部 phase 行为均有自动化验证（CGO 包的 WSL 依赖是环境约束而非人工步骤）。*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies（27/27：26 个 `<automated>` + 1 个 operator checkpoint 手册化）
- [x] Sampling continuity: no 3 consecutive tasks without automated verify（每任务均有 hermetic 命令）
- [x] Wave 0 covers all MISSING references（见 Wave 0 Requirements 落位表）
- [x] No watch-mode flags（vitest 一律 `run`，非 `--watch`）
- [x] Feedback latency < 30s（quick 子集）
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-07（regenerated in revision round 1）
