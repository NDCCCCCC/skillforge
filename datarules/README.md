# datarules/ — 数据集脱敏规则（受控文件）

D15-A1：每数据集的「列 → 规则」映射放在本目录的 `{dataset}.yaml`，数据人员可写、
随仓库版本化、改动走 PR 评审（这是 passthrough 滥声明的第一道防线）。标准 PII
识别规则（哈希前规范化链、HMAC、定宽桶）在 Go 代码 `internal/data/`，与规则文件
分开版本化。规则文件不落 `data/`（那是运行态目录，被 gitignore）。

## 字段

| 字段 | 说明 |
|------|------|
| `dataset` | 逻辑数据集名；注册名为 `{dataset}__{period}`，故 dataset 禁含 `__` |
| `tier` | `权威` \| `历史` \| `个人`（草稿态不适用导入） |
| `k_min` | 查询侧 k-匿名下限 override；缺省 0 = 用全局默认 `KMinDefault=5` |
| `columns.<名>.rule` | `hash` \| `passthrough` \| `bucket_fixed`（v1 L2 仅数值定宽桶） |
| `columns.<名>.profile` | hash 列的规范化档：`person_name` / `phone_cn` / `id_card_cn`（可空 = 默认链） |
| `columns.<名>.width` | bucket_fixed 桶宽（必须 > 0） |
| `columns.<名>.duckdb_type` | 可选：`VARCHAR` / `BIGINT` / `DOUBLE` / `TIMESTAMP` |

## fail-closed 语义

- CSV 出现规则外的列 → 整批拒绝，错误列出全部未知列名（D15-A2）；
  规则里有 CSV 没有的列 → 同样拒绝（schema 漂移双向检查）。
- `profile` 只认 `internal/data/normalize.go` 的 `KnownProfiles()` 白名单，
  写错名字 = 加载期报错（防规则名幻觉）。
- 站点盐 `SKILLFORGE_DATA_SALT` 未设置 → 导入端点直接拒绝（fail-closed）。
  轮换站点盐 = 既有哈希与新导入不可 join，需全量重导。

## 审计与修正

- 每批导入的审计行携带本文件字节的内容哈希 `rules_sha256`（D15-A4）：
  「这批数据按哪一版规则脱敏」可回放可举证。
- 规则修正（如某列事后发现敏感）= 删除该数据集 + 从企业源文件重导
  （D15-A3）。SkillForge 内无 raw、不可恢复是设计而非缺陷；二次脱敏迁移
  不在 v1 范围。
