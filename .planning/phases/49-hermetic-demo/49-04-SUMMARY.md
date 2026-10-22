---
phase: 49-hermetic-demo
plan: 04
subsystem: demo-assembly
tags: [hermetic, demo, gate, recording, readme, makefile]
requires:
  - 49-01 (embedded retriever backend)
  - 49-02 (stub LLM path + triple annotation)
  - 49-03 (same-origin static hosting)
provides:
  - verify-phase-49.sh (9-partition acceptance gate, 29 evidence tokens, 10 mutation drills)
  - scripts/record-demo.sh (real terminal recording -> docs/demo.gif)
  - docs/demo.gif (38-46 frame GIF recorded off the real hermetic quickstart)
  - README "Run without Docker / without an API key" quickstart
  - Makefile verify-phase-49 / demo-gif targets
  - .gitignore rules for the local vector cache and cast intermediates
affects:
  - README.md
  - Makefile
  - .gitignore
  - docs/demo.gif
actuals:
  tokens: 61000
  tasks: 3
  commits: 1
tech-stack:
  patterns:
    - One partition per invariant, each depositing named evidence tokens (a claim without a token is not a claim).
    - Every partition carries a mutation drill; a gate that has never been seen to go red is not a gate.
    - Marker-wrapped assertion blocks so a drill can delete a whole assertion while the copy stays syntactically valid.
    - Test-only embedding double with a shared base component so absolute cosine clears retrievalScoreThreshold while the hashing residual keeps the ranking discriminating.
    - Recording is a read-only observer: it never writes to skills/, never exports an API key, and refuses a placeholder animation.
key-files:
  created:
    - verify-phase-49.sh
    - scripts/record-demo.sh
    - docs/demo.gif
  modified:
    - README.md
    - Makefile
    - .gitignore
key-decisions:
  - "D-19 landed as docs/demo.gif (the README already referenced the missing file)."
  - "Skill switching is demonstrated through the request's `skill` field, because cmd/server/main.go:459/:820 hardcode the skills directory as \"skills/\"."
  - "The gate's Go partitions run through `wsl()` on Windows hosts (native go test cannot link the duckdb static library there); nothing in the gate requires docker/ollama/jq/agg."
---

# 49-04 — 装配层门禁、真实录屏与 README

## Accomplishments

1. **`verify-phase-49.sh`（仓库根，47 KB）** — 九个分区，29 个证据 token，10 条变异演练。
   - P1 语料与缓存键：`knowledge/` 下 218 篇可索引 `.md`、tier fail-closed、缓存键绑定 model+dim+corpus hash。
   - P2 embedded 检索语义：空 tiers ⇒ 0 命中（refusal-by-construction）、topK 上界、未知 chunk id ⇒ `ErrChunkNotFound`、排序确定性。
   - P3 tier 隔离：候选集先过滤后打分（两段式 PRE-filter），非授权 tier 绝不出现在结果里。
   - P4 无 key 启动：stub WARN + 合法 Verdict + wire 上的 `"mode":"stub"`。
   - P5 grounding：stub 的 span 过同一条 `GroundingCheck`，伪造 span 必降级为 `needs_input`。
   - P6 同源静态托管：`/api` 不被遮蔽、SPA fallback 逐字节、穿越拒绝。
   - P7 demo 素材：GIF magic、二进制内无 key 字节、录制脚本可复现。
   - P8 不动性：`.env.example`/`docker-compose.yml`/`Dockerfile*`/CI 与生产 embedder 零改动、零新依赖。
   - P9 自证：注册表一致、空分区必红、九条变异演练、工作树干净、替身端点公告。
2. **`scripts/record-demo.sh`** — 在 WSL 内起真实后端（真实 embedding 端点、无任何 API key）、录 asciinema、渲 GIF（路径 A agg / B svg-term-cli / C python+Pillow+ffmpeg，内嵌渲染器）。录制全程对工作树只读。
3. **`docs/demo.gif`** — 真实录制：46 帧、736×328、211 KB，画面含 `status/mode/confidence/review_required/citations`、`[ref-N] tier=…`、Skill 目录（真实 `reads` 列）与"同一内核换 Skill 换知识域"的第二轮问答。
4. **README** — 新增 `## Run without Docker / without an API key`，含可照抄六步、两种端点形态、stub 三重标注说明；原 Docker 流程一字未动。
5. **Makefile** — 新增 `verify-phase-49` 与 `demo-gif`；**`.gitignore`** — 新增 `/.skillforge-cache/`、`/*.cast`、`/docs/demo.*.tmp`。

## 门禁实证（两次连跑）

| 运行 | 结果 | 用时 |
|---|---|---|
| 第 1 次 | `=== Phase 49 Gate: ALL PASSED ===`，九个分区全 PASS | 8m00s |
| 第 2 次 | 同上（幂等） | 见 Verification |

P9 的九条演练（D1 改语料期望 218→220 / D2 删 P2 空 tiers 断言 / D3 删 P3 先过滤断言 / D4 删 P4 `mode` 落 token / D5 删 P5 伪造 span 断言 / D6 删 P6 `/api` 不遮蔽断言 / D7 删 P7 GIF magic 断言 / D8 删 P8 生产 embedder 不动断言 / D9 删一次 `done_partition` 调用）全部变红并点名对应分区；D0 未变异基线保持 exit 0 且只打印一条 ALL PASSED。

## Deviations

1. **D-01 装配层拆成 49-04，门禁与素材归本 plan** — 原计划把两者放在同一份里；实际执行时 49-04 由子代理写成 `cmd/server/main.go` 装配层，门禁/录屏/README/Makefile 另起一份承接（计划集因此从 4 份变 5 份，随后合并为 4 份：49-01 embedded、49-02 stub、49-03 静态托管、49-04 装配+门禁+素材）。
2. **D-02 替身端点加"共享基底"分量** — 纯词面哈希在长 chunk 下相似度仅约 0.1，会被 `internal/server/pipeline.go:64` 的 `retrievalScoreThreshold = 0.3` 永久拦成拒答，stub Verdict 根本不会产生。最终实现 = `0.9 × 共享基底 + 0.1 × token 哈希残差`：基底把绝对相似度抬过阈值，残差保住相对排序（因此不违反 Pitfall 17 的"排序退化"禁令）。替身只在门禁内部、只监听 127.0.0.1，`BGE_M3_URL` 仅注入子进程环境，生产二进制零改动。
3. **D-03 README 六步必须含 `SKILLFORGE_MCP_SERVICE_SKILL`** — 计划原文没写这一步，但门禁的 live 驱动器第一次启动就撞上 `SKILLFORGE_MCP_SERVICE_SKILL is required (D-TRUST-02: no admin baseline bypass)`（`cmd/server/main.go:1027-1029` 在 `SKILLFORGE_MCP_ENABLED` 判断**之前**就 `log.Fatalf`）。这是本次门禁抓到的真实 quickstart 断点，已写进 README 与 SUMMARY。
4. **D-04 Skill 切换改用请求字段而非文件替换** — `cmd/server/main.go:459` 与 `:820` 把 skills 目录写死成 `"skills/"`，`SKILLFORGE_SKILLS_DIR` 只喂 ontology emitter、不驱动 loader。于是"换上 compliance / 卸下 simple-qa"的目录替换对运行中的服务无效，演示改为同一条命令只换 `skill` 字段（simple-qa → authority 域；sql-guides → sql-guides 域），既真实又让录制对工作树完全只读。
5. **D-05 asciinema 2.x 没有 `--cols/--rows`** — 那是 agg 的选项；终端尺寸改用 `COLUMNS=100 LINES=30` 传给被录制命令。
6. **D-06 渲染走路径 C（python + Pillow + ffmpeg）** — 本机 `agg` 不可得（crates.io 403、github.com 不可达）、WSL 无 node（svg-term-cli 需 npm）；渲染器内嵌进录制脚本，用 DejaVu/Liberation 等宽字体排 ASCII 网格，另配 Microsoft YaHei 等 CJK 字体并按 `east_asian_width` 做双宽推进（否则中文全渲染成方框）。
7. **D-07 方案外的本机开发依赖** — 真实 embedding 端点用宿主 `fastembed`（`mixedbread-ai/mxbai-embed-large-v1`，1024 维，正好匹配 `internal/embedder/bge/bge.go:50`；`bge.go` 不校验返回维度）包一层 OpenAI `/v1/embeddings`。这些工具**不入库**，只在录屏时使用。
8. **D-08 `DEMO_CACHE_DIR` 与 `DEMO_READY_TRIES`** — 冷索引 218 篇真实向量约 200 秒，而门禁/录制脚本的就绪轮询原本 180 秒；改成可复用缓存目录 + 可调轮询上限后，第二次录制秒级就绪。
9. **D-09 P9 的"工作树干净"改成前后差集** — 开发机本来就有既存未跟踪文件（`skillforge.db.bak-*` 等），拿绝对干净度当判据会把既存垃圾误判成门禁产物；现在只判"本次运行新增了什么"。
10. **D-10 演练 marker 用变量拼 sed 表达式** — marker 字面量原先同时出现在断言块和 sed 命令行里，导致 sed 范围自匹配、把后面上百行一起删掉（副本语法错误而非断言失效）。现在 sed 表达式在运行时拼装，命令里不含 marker 字面量；marker 也必须包住**整个 if/else**，否则删除后留下孤立 `else`。

## Verification

- `bash verify-phase-49.sh`（WSL 内，两次连跑）→ 两次均 `=== Phase 49 Gate: ALL PASSED ===`，exit 0。
- `docs/demo.gif`：`head -c 6` = `GIF89a`；Pillow 读到 46 帧、736×328；二进制内不含 `sk-`/`GLM_API_KEY=`/`LOCAL_API_KEY=`。
- 录制复现：`DEMO_DRY_RUN=1` 打印完整命令序列；真实录制 `wrote docs/demo.gif (211794 bytes)`。
- 前端与 Go 回归见 phase 级回归门（execute-phase 收尾）。

## Notes for the next agent

- 门禁在 Windows 宿主必须经 WSL 跑（`wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && bash verify-phase-49.sh"`）：Go 分区需要 Linux 链接器，宿主 `go test ./...` 会撞 duckdb 静态库 TLS-ABI。
- 冷索引 218 篇真实向量约 200 秒；调试时用 `DEMO_CACHE_DIR` 复用缓存。
- `demo.gif` 重录需要活的 embedding 端点（`BGE_M3_URL`）。没有端点时脚本会明确报错并提示 `ollama pull bge-m3`，绝不产出占位动画。
