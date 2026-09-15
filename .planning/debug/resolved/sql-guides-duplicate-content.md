---
slug: sql-guides-duplicate-content
status: resolved
trigger: 用户报告 sql-guides 知识库中 SQL 相关出现很多重复内容，要求检查原因。时间上紧跟 7c7db99（语料重组为 7 个业务子目录）+ cf26c04（递归 KB walker + subdirectory-aware graph display）两次变更；用户是第一次使用该 KB，无法对比重组前表现
created: 2026-09-15
updated: 2026-09-15
---

## Symptoms

- Expected behavior: sql-guides 知识库中每份文档在检索结果/图谱中只出现一次，无重复条目
- Actual behavior: 用户观察到知识库中 SQL 相关出现很多重复内容（具体 UI 形态未提供：可能是检索候选重复、图谱重复节点、或引用列表重复）
- Error messages: 无报错
- Timeline: 不确定 / 用户第一次使用该 KB；时间上紧跟 7c7db99 语料重组 + cf26c04 递归 walker 上线
- Reproduction: 查看 sql-guides KB（图谱页或检索结果）；已从相邻 debug 会话（sql-guides-needs-input）的 live 探针取得一条硬证据，见下方 Evidence 首条
- Scope: 本会话只查重复内容的成因与修复；needs_input 降级误报已在 sql-guides-needs-input（resolved）中修复，勿动其修复面（pipeline.go operational-mode / redline.go operational gate）

## Current Focus

reasoning_checkpoint:
  hypothesis: CONFIRMED — Milvus collection skillforge_knowledge 物理持有 2931 行（GetCollectionStatistics row_count），而 Query 只见 256 行；重复 boot 的 knowledge.Index → milvus.SeedDocuments 全量 Insert（caller 赋 id、无任何 delete/upsert/cleanup）使每代语料（平铺期根路径 + 子目录期 + 中间态）全部累积为独立 entity。Milvus 对重复 PK 的 Insert 不去重；Query（ListMetadata/graph）按 PK 归并返回最新代 → graph 显得干净；Search（向量检索）不去重 → 所有代同台竞争 → 同一内容以不同 id + 新旧 source 路径同时召回 = 用户看到的重复内容
  confirming_evidence:
    - "tmp_milvus_probe（WSL go run，直连 10.62.10.34:19530）：row_count=2931；QUERY-ALL rows=256 uniqueIds=256 dupIdCount=0；同一查询 top-60 里出现成对/成组同分命中：id=31 取数口径/001_…__part09.md 与 id=25 根路径 001_…__part09.md 同分 0.0642；id=45/118/124/130/136（029_开门红存储过程 同 part 同分 0.0192）→ 根路径行物理存在且参与检索"
    - "根路径 source（sql-guides/001_…__part04.md）指向磁盘不存在的文件布局（根目录只有 7 个子目录）→ 只能是旧代残留行"
    - "live graph（graph-now.json）chunk 20 = sql-guides/其他/037_…__part04.md，而 sibling 会话探针同 id=20 返回根路径 001_…__part04.md → 同一 PK 两代并存、Query 归并最新代的直接实证"
    - "代码考古：SeedDocuments godoc 自述 're-running inserts duplicate IDs (Milvus INT64 PK is not auto-deduped)'；全仓 grep 无任何 milvus delete/upsert 路径；唯一清理 = SKILLFORGE_MILVUS_RECREATE=1 一次性人工闸门；boot 每次启动都跑 knowledge.Index"
    - "Mongo 后端对照：SeedFromKnowledgeDir 每 seed 前 Reset 整 collection（mongo.go Reset）→ mongo 无此故障形态，Milvus 路径缺等价物"
  falsification_test: "若 Milvus row_count == 256（无多余物理行）或 Search 结果无成对同分/无根路径 source，则本假设被证伪——实测 row_count=2931 且 Search 成对同分，假设成立"
  fix_rationale: "根因是 boot 全量 reseed 无清理（写放大累积），不是 graph 显示或检索排序问题 → 修复必须在写入侧让 knowledge.Index 的 seed 语义变为 replace-by-tier：seed 前按 tier 表达式 Delete 旧代行（限定在本次即将写入的 tier 集合，绝不动未写入 tier），再 Insert 当前语料。上传增量路径（upload.go → IndexerBackend.Insert → SeedDocuments）不得触发 purge（只写子集会清掉全 tier），故 purge 放在 knowledge.Index→seedDocs 接缝而非 SeedDocuments 内部。一次带修复的 boot 即顺带清空存量 2675 行残留"
  blind_spots: "Milvus row_count 统计含 tombstone 行，delete 后 num_entities 未必立刻下降——验证以 Query 行数 + Search 无重复为准而非 row_count；tier 被完全清空（某 KB 文件删光）的 purge 边界本次不覆盖（scope=docs 内 tier 的取舍已在 fix 注明）；Delete 需要 collection 已 load，实现里补 LoadCollection"

hypothesis: （已确认并修复，见 reasoning_checkpoint + Resolution）
test: TDD（新增 6 个 purge 单测，修复面 mock client 直测 delete-before-insert 顺序）+ Milvus 行级探针（修前/修后对照）+ live SSE 探针
expecting: 修复 + 重启后：QUERY 行数回归当前语料行数（authority 16 + sql-guides 240 = 256）、Search top-60 无根路径 source、无成对同分重复、graph 240 docs 不变、live SSE 探针 source-document 无重复
next_action: （完成）human-verify 检查点已满足 → 已归档 + 提交 daf98d1 + 知识库条目；无后续动作

## Evidence

- timestamp: 2026-09-15 orchestrator 预调查（live 探针 + 只读 API + 文件系统）
  - 硬证据（探针 1，query=公司业务部相关数据查询 skill=sql-guides 的 SSE）：两个 source-document 标题 —— "sql-guides/取数口径/001_公司业务部、武汉中支部门组部门科室等信息更新__part01.md · chunk 23" 与 "sql-guides/001_公司业务部、武汉中支部门组部门科室等信息更新__part04.md · chunk 20"。后者是**根路径形态**（sql-guides/ 下无子目录），而磁盘根目录只有 7 个子目录、无散文件 → 根路径 chunk 指向磁盘上不存在的文件布局，只能是旧索引残留
  - GET /api/knowledge/graph：sql-guides tier docCount=240 chunkCount=240（每 doc 恰 1 chunk），240 个 doc id **全部**为 2+ 层斜杠的子目录路径，0 个根路径；doc id basename 无重复
  - 文件系统：knowledge/sql-guides/ 下 7 个子目录（其他/取数口径/操作手册/汽车险/理赔与赔付/财务对账/非车险），共 240 个 .md，md5 内容级零重复
  - SQLite（skillforge.db）无 chunk/knowledge 向量表（.tables 仅业务表：audit/feedback/session_turn/knowledge_bases 等）→ chunk 元数据与向量在 Milvus（collection 名启动日志为 skillforge_knowledge，MILVUS_ADDRESS=10.62.10.34:19530）
  - WSL 无 pymilvus（Milvus 行数直查待装包或走其他途径）
  - implication: graph 数据源（240 全新路径）与检索数据源（含根路径 chunk）不一致 → 两源之一含旧数据；若 Milvus 同时含新旧两套，则用户看到的重复 = 同内容双 chunk 被同时召回
- timestamp: 2026-09-15 相关变更考古（git log）
  - 394c2be feat(kb): register sql-guides knowledge base（yaml + Skill）—— 首次注册
  - 7c7db99 data(kb): sql-guides corpus reorganized into 7 business-domain subdirs —— 语料搬家
  - cf26c04 feat(knowledge): recursive KB walker + subdirectory-aware graph display —— 递归索引 + 图谱显示适配
  - d9f93be docs(999.2): phase plan, verification and summary for KB recursive walker
  - implication: 若 reindex 路径只 upsert 不按前缀清理，旧根路径向量必然残留；indexer 清理逻辑（internal/knowledge/indexer.go，git status 显示 modified）是首要读点
- timestamp: 2026-09-15 本会话（代码读点 + Milvus 行级直查）
  checked: internal/knowledge/indexer.go + internal/retriever/milvus/seed.go + client.go + cmd/server/main.go buildRetriever
  found: Index 每 boot 全量 Insert（id 从 1 起caller 赋值）；SeedDocuments godoc 自述 "re-running inserts duplicate IDs (Milvus INT64 PK is not auto-deduped)"；milvus 包零 delete/upsert 路径，唯一清理 = SKILLFORGE_MILVUS_RECREATE=1 一次性闸门；graph（ListMetadata）与 Search 同一 client 同一 collection → 源分裂不在配置层
  implication: 排除"graph/检索读不同源"假设 → 分裂必在 Milvus 行级（Query vs Search 对重复 PK 的可见性差异）
- timestamp: 2026-09-15 本会话（tmp_milvus_probe 直连 10.62.10.34:19530，WSL go run）
  checked: GetCollectionStatistics + Query(all / tier=sql-guides) + 随机向量 Search(tier=sql-guides, topK=60)
  found: row_count=2931；QUERY-ALL rows=256（uniqueIds=256, dupIdCount=0, 全部新子目录 source）；QUERY-SQLGUIDES rows=240 无根路径；Search top-60 中成组同分重复：id=31(取数口径/001_…__part09.md) 与 id=25(根路径 001_…__part09.md) 同分 0.0642，id=46/255 同分 0.0334，id=45/118/124/130/136（029_开门红存储过程 part05/11/17/23）同分 0.0192（≥4 代），id=50/220 同分 0.0267，id=98/171 同分 0.0210
  implication: ROOT CAUSE CONFIRMED — collection 物理持有 ~11.4x 当前语料行数的多代残留；Query 按 PK 归并最新代（graph 干净假象），Search 全代竞争（用户可见重复）。同 id 两代并存直接实证：graph-now chunk 20 = 其他/037_…__part04.md，而 sibling 探针同 id 20 = 根路径 001_…__part04.md
- timestamp: 2026-09-15 增量上传路径风险确认
  checked: internal/server/upload.go:264 → s.IndexerBackend.Insert(ctx, docs)（GetMaxID+1 增量 id）
  implication: purge 不能放 milvus.SeedDocuments/Insert（上传只写子集会误清全 tier）→ 必须放 knowledge.Index 的 seedDocs 接缝（boot 全量 reseed 专属）
- timestamp: 2026-09-15 修复后验证（WSL 重建 + 重启）
  checked: (1) 全仓 go test ./... 42 包全绿（含 sibling 会话 pipeline/redline 测试，无回归）；(2) 重启日志 "[OK] knowledge corpus indexed into skillforge_knowledge"（purge 生效）；(3) tmp_milvus_probe 修后对照：QUERY-ALL 仍 256 行 unique 全新路径；Search top-60 仅返回 33 个真实实体、零根路径 source、零成对同分重复（修前 60 命中含多组同分双代）；row_count 3187 = 2931 旧行 + 256 新行（tombstone 待 compaction 回收，属 Milvus 正常行为）；(4) graph 240 docs/240 chunks 不变；(5) live SSE 探针（公司业务部相关数据查询, skill=sql-guides）：两条 source-document 帧均为子目录路径（取数口径/…__part01.md · chunk 23、取数口径/…__part04.md · chunk 26），根路径帧消失；data-policy needs_input@0.95 带真实引用 + 无 data-redline 帧 —— sibling 会话修复面（operational-mode）完好
  implication: 原始复现症状（检索返回根路径 chunk/重复内容）已消失；写侧 replace 语义使后续每次 boot 语料收敛到磁盘真值，残留不再累积
- timestamp: 2026-09-15 human-verify 检查点满足（orchestrator 探针矩阵；server 重建重启为 PID 8362 后探测，用户委托验证）
  checked: 3 条 sql-guides 真实查询（跨 3 个业务子目录）+ 图谱对照 + 全矩阵断言（零根路径引用标题、零同响应重复引用）
  found: probe1 公司业务部相关数据查询 → 2 条引用全子目录路径（取数口径/001…part01 chunk 23、取数口径/001…part04 chunk 26），零重复，needs_input@0.95 带真实引用；probe2 车险保单信息在哪些表里 → 2 条引用（汽车险/011…part02 chunk 185、汽车险/036…part01 chunk 204），零重复，status=ok@0.95；probe3 理赔赔付时效要求 → 诚实 no_authorized_source 拒答卡（护城河生效，零引用无从重复，非回归）；graph 240 docs/240 chunks/0 根路径不变
  implication: 用户工作流验证通过 → CHECKPOINT human-verify 满足，会话收口

## Eliminated

- hypothesis: 磁盘语料文件本身重复（重组时复制未删）—— ELIMINATED：240 个文件 md5 内容级零重复，根目录无散文件
- hypothesis: graph 显示层重复（doc id 重复渲染）—— ELIMINATED 作为 graph 源头：240 doc id 无重复 basename、全子目录路径。但"graph 一套 + 检索一套"的源分裂仍是主嫌疑
- hypothesis: graph 与检索读不同源（双 collection / 双 retriever 配置分裂）—— ELIMINATED：Deps.Knowledge = knowledge.New(s.Retriever)，graph 的 ListMetadata 与 chat 的 Search 走同一 MilvusClient 同一 collection（internal/retriever/milvus/client.go：两者都用 m.cfg.CollectionName）。源分裂只能发生在行级
- hypothesis: Milvus Query 行级不对称之外的写入路径（admin 上传、eval/spike 双写）为主因 —— ELIMINATED 作为主因：根因是 boot reseed 无清理的写放大；upload.go 增量路径只影响未来行为，不是存量多代的来源（2931 行 ≈ 历代全量 boot 累积）


## Resolution

root_cause: Milvus 写侧无清理的全量重索引。knowledge.Index 在每次 boot 全量 Insert（caller 赋 1..N id），milvus.SeedDocuments 只 insert 不 delete/upsert，而 Milvus 对重复主键的 Insert 不去重 → 语料的每一代（394c2be 平铺期根路径 → 7c7db99 重组中间态 → 现子目录期）全部堆积为可检索 entity（实测 skillforge_knowledge row_count=2931，当前语料仅 256 行）。Milvus 的 Query 按 PK 归并返回最新行（graph/ListMetadata 因此显示干净的 240 docs —— 假象），Search 不去重 → 所有代同台竞争 → 同一内容以不同 id 多次命中 + 旧根路径 source 引用 = 用户看到的"SQL 相关很多重复内容"。7c7db99 语料重组改变 id→文件映射 + 期间多次重启，使残留代数放大。
fix: boot reseed 语义从 append 改为 replace-by-tier：knowledge.Index 的 seedDocs 接缝在生产路径先调 milvus.DeleteTierRows（`tier in [...]` 表达式，scope=本批 docs 携带的 tier 集合、升序确定，绝不动未写入 tier 与停用 tier 行），flush 后再 SeedDocuments。增量上传路径（internal/server/upload.go → IndexerBackend.Insert → SeedDocuments 裸调）刻意不经该接缝（只写子集不可清 tier）。purge 前全部 embed/validate 已完成，purge 成功 + insert 失败的窗口退化为 no_authorized_source 拒答（D-09），与 Mongo 后端 Reset-before-seed 同一取舍。一次带修复的 boot 即顺带清空存量残留（本次重启已执行，无需 SKILLFORGE_MILVUS_RECREATE）。
verification: 四层验证全过（详见 Evidence 2026-09-15 修复后验证 + human-verify 条目）：(1) 全仓 go test ./... 42 包全绿（含 6 个新增 purge 单测与 sibling 会话全部回归）；(2) Milvus 行级探针修后对照：Search 零根路径、零成对同分重复（修前 60 命中含 ≥4 组双代同分）；(3) graph 240 docs/240 chunks 不变；(4) live SSE 探针 + orchestrator 3 探针矩阵（跨 3 子目录）全为子目录路径引用、零重复，probe2 status=ok@0.95，probe3 诚实拒答 —— sibling operational-mode 修复面完好。
commit: daf98d1 fix(knowledge): purge tier rows before Milvus boot reseed (replace-by-tier)
commit_note: 部分暂存 —— 工作树中 indexer.go 的 diff 含 5 个 hunk，仅 3 个属本修复（Index godoc / seedDocs godoc / seedDocs body + seedTiers），其余 2 个（imports + walkTier→WalkDocs 重写）是 Phase 999.2 WalkDocs 重构遗留，留在工作树待该阶段收尾提交。首提 90a12ed 曾漏掉第 5 个 hunk（git diff 输出 >200 行被截断导致漏数 hunk），worktree HEAD 编译验证暴露 undefined: seedTiers 后已 amend 为 daf98d1；提交验证用 git worktree 在 WSL go build + go test internal/knowledge + internal/retriever 通过。
files_changed:
  - internal/retriever/milvus/seed.go — 新增 DeleteTierRows（tier-scoped purge：HasCollection → LoadCollection → Delete(`tier in [...]`) → Flush；缺 collection no-op；godoc 载明 scope 规则与"禁用于增量上传"）+ SeedDocuments godoc 更新（Query/Search 重复 PK 不对称说明）
  - internal/knowledge/indexer.go — seedDocs 生产路径改为 purge→insert；新增 seedTiers 纯函数（去重 + 升序，expr 字节稳定）；Index godoc 更新（replace 语义 + RecreateCollection 仍为跨 tier 重置之门）
  - internal/knowledge/indexer_purge_test.go — 新增：TestSeedTiers_DeduplicatesAndSorts / TestSeedTiers_EmptyDocs / TestSeedDocs_PurgesTiersBeforeInsert（mock client 锁 delete-before-insert 顺序 + expr 精确断言）/ TestSeedDocs_PurgeFailureAbortsInsert / TestSeedDocs_HookStillInterceptsWholeOperation
  - internal/retriever/milvus/delete_tier_rows_test.go — 新增：目标 collection + expr + 调用顺序锁定、缺 collection no-op、空 scope no-op、nil client 显式报错、Delete 失败传播
  - 会话工件（不提交）：.planning/debug/graph-now.json（修前）/ graph-after.json（修后）/ chat-sse-after.txt（修后 live SSE）/ probe-query.json（探针请求体）；throwaway tmp_milvus_probe/ 已删除
