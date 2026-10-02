# Requirements: SkillForge — v1.6

**Defined:** 2026-10-02
**Core Value:** 同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码。
**Milestone Goal:** 闭合 v1.5 两项信任/治理积压——D-SPARQL-05 + 实体类 SHACL shapes——不削弱 D-TIER-03 fail-closed。

## v1.6 Requirements

### SPARQL Schema Visibility（D-SPARQL-05 落地）

- [ ] **SPARQL-01**: `isSchemaTriple` 切换为 subject-based detection（44-RESEARCH §5 Option α）——skill 个体及其 `hasReadAccess` triples 在 admin baseline 可见
- [ ] **SPARQL-02**: D-TIER-03 fail-closed 不回退：无 `belongsToTier` 的 ABox 实体仍被过滤，既有 tier 回归测试全绿
- [ ] **SPARQL-03**: schema 可见性规则单点化——`/api/sparql` 与 `/api/admin` 共享同一 `isSchemaTriple` 判定

### Entity SHACL Validation（实体 shapes 落地）

- [ ] **SHACL-01**: 实体类 ABox（险种/机构/TableSchema 等）SHACL shapes 文件随仓库交付，路径可配置（零硬编码）
- [ ] **SHACL-02**: 启动校验覆盖实体 shapes；违规进入现有 quarantine 流；boot 不被阻断（D-SHACL-02 保持）
- [ ] **SHACL-03**: `/admin/governance` 校验报告可呈现实体类违规（若涉 UI 改动，先 UI-SPEC）

## Future Requirements

Deferred. 不入 v1.6 roadmap，留 quick task 或后续 milestone。

### SPARQL Hardening

- **SPARQL-F01**: D-SPARQL-07 — `comment` predicate hardening（与 label 同类 trust-leak 风险，44-DECISIONS 记录）

### Accumulated Backlog

- **BL-01**: 双 resolver 去重（`resolveSkillTiers` vs `resolveSkillTiersFromKG`）
- **BL-02**: server 包 lint 清扫
- **BL-03**: live 抽取 prompt 对齐 writes-only 的残余微调
- **BL-04**: DetailPanel/emitter 两个 deferred 测试

## Out of Scope

| Feature | Reason |
|---------|--------|
| PySHACL sidecar | 打破 Apache-2.0 单 binary 护城河；Go 自实现 SHACL 校验已有 Phase 40 基础 |
| SHACL 高级约束（sh:pattern 级规则引擎） | 实体 shapes 只需必填属性/cardinality 级约束；规则引擎过度设计 |
| 42-03 operator runbook 剩余项 | 属 v1.5 operator 域，不属 v1.6 开发范围 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SPARQL-01 | — | Pending |
| SPARQL-02 | — | Pending |
| SPARQL-03 | — | Pending |
| SHACL-01 | — | Pending |
| SHACL-02 | — | Pending |
| SHACL-03 | — | Pending |

**Coverage:**
- v1.6 requirements: 6 total
- Mapped to phases: 0
- Unmapped: 6 ⚠️（roadmap 创建后归零）

---
*Requirements defined: 2026-10-02*
*Last updated: 2026-10-02 after v1.6 milestone start*
