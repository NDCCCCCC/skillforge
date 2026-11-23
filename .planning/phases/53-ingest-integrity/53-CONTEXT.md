# Phase 53 Context: 摄取链路完整性

**Goal**: 消灭摄取链路三个数据完整性断点（断点 A 上传蒸发 / 断点 B 分块不一致 / 断点 C 无生命周期）。
**Milestone**: v1.8（`.planning/milestones/v1.8-ROADMAP.md` — 执行前必读其 Phase Details 与 Key Design Decisions）
**Requirements**: ING-01, ING-02, ING-03
**Wave 结构**: Wave1 = 53-01(落盘) ∥ 53-02(分块器统一)；Wave2 = 53-03(doc_id 生命周期)；Wave3 = 53-04(等价性测试 + verify 收口)

## 断点证据（2026-11-23 实证，回源码验证）

- **断点 A**：`internal/server/upload.go` processUploadedFile 全程内存处理，`IndexerBackend.Insert` 后无落盘——上传 chunk 只存在于向量库；而 `internal/knowledge/indexer.go` seedDocs 启动时 `milvus.DeleteTierRows(seedTiers(docs))` **整 tier 清空重灌**（只灌目录 docs）；embedded 后端 `internal/retriever/embedded/cache.go` corpusHash 只对 `knowledge/` 目录 WalkDocs 结果计算。→ **上传文档重启必蒸发，且静默**（audit 行还写着上传成功）。
- **断点 B**：目录路径 `Index()` 用 `buildSplitter`（CJK-aware recursive splitter，3500 rune，语义边界）；上传路径 `UploadIndex()` 用 `chunkBody`（4000 **byte** 固定窗口 + 200B overlap，会切在句子中间）。同一 slug 两种粒度混存，检索分布不可比。
- **断点 C**：`retriever.Document` 无 doc_id/chunk_index/content_hash；路由表无 DELETE 端点；SHA-256 只防完全重复——改一个字新旧 chunk 并存，检索可能命中过期版本。

## 关键设计决策（引用 v1.8-ROADMAP Key Design Decisions）

- **D-53-01 单一事实来源**：`knowledge/` 是唯一事实来源，Insert 只是热路径加速。落盘让 purge 重灌从数据丢失变幂等刷新——embedded 缓存 hash、GraphBuilder、`/api/knowledge/docs` 全部自动生效，零新机制。
- **D-53-02 分块器唯一**：CJK-aware recursive splitter 唯一，`chunkBody` 整体退役不保留开关。
- **D-53-03 PK 保持自增**：upsert = 「按 doc_id expr 删旧插新 + content_hash 未变跳过重嵌」（LlamaIndex 删旧插新模式），**不改** Milvus INT64 PK（备选 hash64(doc_id,chunk_index) 方案涉及 collection 重建与 chunk_id wire 契约，独立立项）。已知 Milvus 删除异步可见：upsert 后短暂窗口新旧并存可容忍。

## doc_id 语义

- 目录路径：doc_id = 相对 `knowledge/` 的 slash 路径（如 `authority/travel_expense.md`）——与 Source 字段同值但语义独立（Source 是展示引用标题，doc_id 是生命周期键）。
- 上传路径：doc_id = 落盘文件的相对路径（落盘后与目录路径统一！即 `authority/<sha8>-<filename>.md`）。这是 D-53-01 的关键推论：**落盘后两条路径的 doc_id 语义归一**。

## 坑（历史教训）

- Milvus 写放大：本项目已踩过 reseed 重复写（2931/256 事件）——53-03 的 upsert 必须先删后插，不得裸 append。
- `maxDocBytes`(4000B) 是告警阈值不是丢弃阈值（历史修复），重构分块器时不得回退成丢弃逻辑。
- eclMode 开启时 `__secNN` 片段归 ECL 管线，Index 分支跳过（indexer.go 承重注释）——落盘文件若含该形态片段，行为与目录文件一致即可，不需要特判。
- fail-closed 哲学：落盘失败 = 上传失败（422），不得「入库成功但落盘失败」的半状态；落盘必须发生在 Insert **之前**（顺序：落盘 → Insert），使「Insert 成功但盘上无文件」不可能出现。
