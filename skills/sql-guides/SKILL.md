---
description: SQL 取数/操作手册检索。当用户问「这条 SQL 怎么写」「取数口径是什么」「字段含义」「存储过程」「某个表的取数」「某个险种的保费/赔款怎么提数」等数据库操作问题时触发。返回操作步骤 + 原文 SQL + 涉及表/字段提示，**不参与合规引用裁决**。
agent: ChatModelAgent
reads:
  - sql-guides/**
writes: []
steps: |
  1. 检索 sql-guides KB（retriever 自动按 tier=personal 过滤，仅命中本 KB）
  2. 提取 top-k（建议 3-5）chunk，按相关性排序
  3. 逐条呈现：标题 + 涉及表/字段 + 完整 SQL（code block）+ 关键过滤条件
  4. 如果用户问「XX 怎么改」「加个字段」「换张表」，基于原 SQL 做局部修改而非整段重写
  5. 不要凭空补全 SQL 细节（表名/字段名/库名必须出现在命中 chunk 里）
  6. 多个候选 chunk 相关时，列出来让用户选，不要自己合并
guardrails:
  prohibitions:
    - "Never cite sql-guides chunks as policy/regulation evidence (tier=personal, 不在 citation 白名单)"
    - "Never fabricate table/column names not present in retrieved chunks"
    - "Never rewrite the SQL semantics — preserve table joins, filters, aggregations verbatim"
    - "Never present these as 'authoritative compliance answers' — they are operational guides only"
    - "Never suppress source attribution (always show which note/table the SQL comes from)"
  required_refusal_phrases:
    - "no_matching_guide"
    - "out_of_scope"
  tier_limits:
    sql-guides: ["read-only; no citation"]
temperature: 0.0
---

# SQL 取数操作手册 Skill

仅从 `sql-guides/**` 检索团队沉淀的 SQL/取数操作笔记。返回操作步骤 + 原文 SQL + 涉及表/字段，**不参与合规引用裁决**（tier=personal）。

## Wire schema

返回结构化三段呈现：

- `guide_title` (string)：命中笔记标题
- `tables_or_views` (string[])：涉及表/视图（来自 frontmatter `**涉及表/视图**`）
- `sql` (string, code block)：原始 SQL，**逐字保留**——不要改 JOIN、不要改字段名、不要改 WHERE 条件
- `key_filters` (string[])：关键过滤条件（如日期范围、险种代码、机构代码等）
- `source_note` (string)：原始笔记标识（来自 `source_idx` frontmatter）

## 何时不触发此 Skill

- 用户问"X 合规吗""是否违反制度"——这是 compliance Skill 的工作，**不要用本 Skill**
- 用户问"贷款余额""不良率"——这是 kpi-dashboard Skill 的工作
- 用户问"帮我写一段新 SQL"且没有任何参考 chunk——本 Skill 不能凭空生成，应如实说明"未在 sql-guides 中找到匹配模板"

## 已知边界

- 索引范围：43 篇清洗后的笔记（截至 2026-09-14），新增需重启 server 触发 walker
- tier 限制：即使 sql-guides KB 未来被改成 role=authority，本 Skill 仍不应作为 verdict 引用源（guardrails 显式禁止）
