# Spike Manifest

## Idea

第一阶段 (001–004)：验证 SKILL.md `temperature` 字段在 SkillForge 技术栈中能否起作用（已 ✓ 部分成立：byte-determinism 不可达，转向 semantic-determinism）。

第二阶段 (005a–005e)：**研究本体论在知识库方面的应用**——回答"使用本体论之后是否还需要 Milvus 向量数据库"这一终极假设，并打通 Go 端的 OWL/RDF 工具链。5 个 spike 顺序：005a TBox 工具链 → 005b SKILL.md 提升 → 005c ontology-aware chunker → 005d 轻量级 KG 存储 → 005e post-ontology 向量必要性。

## Requirements

- **Stack**: Go + CloudWeGo Eino (`cloudwego/eino`) + Eino ADK (`eino-ext/adk`) + OpenAI 兼容 API（DeepSeek / GPT-4 / Qwen 任选）
- **Query**: 5 个中文合规真实场景 query（报销判定 / 竞业限制 / 审批权限 / 假期政策 / 拒答测试）
- **Structured output**: JSON schema（合规 verdict `{status: 合规|不合规|需补充, confidence, citations, review_required}`）
- **独立验证**: 透传 / 确定性 / structured output 三个假设必须独立 spike
- **通过标准**（spike spec 见 `.planning/notes/spike-temperature-field-validation.md`）:
  - 透传：HTTP wire body 包含 `temperature:0`
  - 确定性：同 query 10 次输出 token-level 一致率 ≥95%
  - Structured output：JSON schema 解析成功率 100%
- **失败 fallback**: 若透传失败 → Eino API 排查 → 备选 OpenAI SDK 直接验证；若都失败 → temperature 字段降级为 opt-in

## Spikes

| # | Name | Type | Validates | Verdict | Tags |
|---|------|------|-----------|---------|------|
| 001 | eino-temperature-passthrough | standard | Given SKILL.md declares `temperature:0`, when Eino ChatModelAgent runs, then HTTP wire body contains `temperature:0` | ✓ VALIDATED | eino, openai-compat, temperature |
| 002 | temperature-zero-determinism | standard | Given `temperature:0` + 同 query 跑 10 次 × 5 合规 query, when 提取所有输出, then token-level 一致率 ≥95% | ✗ INVALIDATED | temperature, determinism, compliance, reasoning-model |
| 002b | temperature-zero-no-reasoning | comparison | Given `temperature:0` + reasoning DISABLED via `thinking:{type:disabled}`, when invoke glm-5.2 via Eino 10 times per query, then token-level unique output rate should drop vs spike 002 (reasoning ON) | ✗ INVALIDATED | temperature, determinism, reasoning-disabled |
| 003 | structured-output-temperature-zero | standard | Given `temperature:0` + `response_format:{type:json_object}` + 合规 verdict schema, when 跑 10 次 × 5 合规 query, then JSON schema 解析成功率 100% | ✓ VALIDATED | temperature, structured-output, json-schema |
| 004 | milvus-v3-milvusclient-tier-contract | standard | Given Milvus 3.0 + new milvusclient SDK, when build tier-filter retriever path (dial→create→insert→search), then `tier in ["权威"]` returns only 权威 (server-side PRE-filter, no 个人 leak) & empty tiers → filter `"false"` → 0 hits (D-09) | ✓ VALIDATED | milvus, milvusclient, sdk-migration, tier-contract, v3 |
| 005a | owl-tbox-toolchain | standard | Given a minimal OWL/Turtle TBox (5 Classes + 3 OP + 2 DP + 1 Restriction), when loaded via tggo/goRDFlib v0.1.0, then (a) all Classes enumerated, (b) subClassOf hierarchy walked incl. BNode parents, (c) ObjectProperty domain/range, (d) DatatypeProperty domain/range, (e) Restriction 'P some D' enumerated, (f) roundtrip lossless | ✓ VALIDATED | owl, rdf, turtle, tbox, goRDFlib, spike-005 |
| 005b | skillschema-rdfs-lift | standard | Given SKILL.md 7 fields + 5-tier KB + sql-guides directory tree (取数口径/操作手册/...) real sample, when lifted via rdf2go + semantix into RDFS/OWL TBox, then TBox serializes to .ttl and is consumable as Milvus metadata filter | ✓ VALIDATED | skillschema, rdfs, lift, kb-metadata, yaml, spike-005 |
| 005c | ontology-aware-chunker | standard | Given sql-guides/取数口径/_001_..._公司业务部__sec01..sec08.md all 8 split files, when h2/section-aware markdown chunker merges, then (1) boundary detection 100%, (2) concatenated output contains complete SQL (no truncation), (3) chunk.metadata fields (section/title/section_idx) writable to Milvus | ✓ VALIDATED | chunker, semantic, h2-boundary, sql, ontology-aware, spike-005 |
| 005d | kg-storage-eval | comparison | Given SkillForge KB scale assumption (< 10k nodes / < 50k edges), when comparing Kuzu (embedded Go via cgo) vs memory map (Go stdlib), then latency/memory/startup/devX comparable basis | PENDING | kg, kuzu, memory-map, storage, spike-005 |
| 005e | post-ontology-vector-necessity | standard | Given TBox (005a/005b) + ontology-aware chunks (005c) + KG storage (005d), when analyzing OntoRAG/LightRAG/Semantica literature + existing SkillForge tier-PRE-filter architecture, then evaluate under what conditions Milvus vector retrieval remains necessary (vs pure SPARQL/Cypher replacement) | PENDING | vector-necessity, hybrid-retrieval, post-ontology, spike-005 |

## Related Artifacts

- `.planning/notes/spike-temperature-field-validation.md` — 完整 spike spec（query 列表 / 执行步骤 / 时间预算）
- `.planning/todos/pending/spike-temperature-field-validation.md` — 执行 checklist
- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 schema（含 temperature 字段）
- `.planning/REQUIREMENTS.md`（SYSK-08）— temperature REQUIRED 需求
- `.planning/notes/skillforge-design-philosophy.md` — "确定性正解"哲学