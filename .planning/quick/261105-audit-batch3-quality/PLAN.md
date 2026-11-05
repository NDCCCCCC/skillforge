---
type: quick-task
status: in-progress
date: 2026-11-05
scope: 批次 3 工程质量（第一波）— CODE-AUDIT-2026-11-05.md §5.3
fixes:
  - F-05 仓库跟踪 4 个 22MB spike 二进制
  - A-12 分类器 prompt 规则编号断档 + empty 枚举口径
  - A-14 rewrite Timeout 0 值兜底
  - A-15 historyCharBudget 字节截断劈裂 UTF-8
  - C-07 概述「共 N 条语句」用截断后条数
  - C-08 每 token 一条 INFO 日志
  - C-09 流式中途失败不发 draft-end
  - C-11 X-Session-Id 无格式校验
  - E-04 useAdminData 丢弃错误对象（boolean → string，403 可辨）
  - E-13 useAdminData url===null loading 卡死 + AbortController
  - E-05 AdminDashboard Refresh 空函数按钮
  - E-10 StatusBadge drifted 英文
  - E-11 StatCard tone 类型约束失效
  - E-12 Toolbar 受控/非受控混用
---

# 批次 3（第一波）：工程质量

第二波候选：E-06/E-07/E-08/E-14（llmApi 类型 + onSave 覆盖 + 轮询）、
D-02（admin token key 统一）、G-02..G-05/G-07（面板竞态）、A-16/D-12（补测试）。
