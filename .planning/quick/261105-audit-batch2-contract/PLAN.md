---
type: quick-task
status: in-progress
date: 2026-11-05
scope: 批次 2 契约对齐 — CODE-AUDIT-2026-11-05.md §5.3 批次 2
fixes:
  - A-03 resolver 顶替（env 回落 + LocalThinkOffModel 丢失）
  - A-04 keyless 切模型端点 base_url 硬门槛
  - A-05/A-06/A-10 thinking 策略按实际端点判定（能力化）
  - B-01 RETRIEVER_BACKEND=none 上传路径 nil panic
  - B-02 local(512) vs milvus(1024) 维度启动断言
  - B-03/B-07 MONGO_URI / BGE_M3_URL 日志泄漏
  - C-06 span 覆盖前无原文子串兜底
  - C-04 降级答案仍以「结论」呈现
  - C-02 [ref-N] 分帧头可伪造（转义）
  - D-03 ModelPicker localStorage 无异常保护
  - D-05 DataSubprogressPart 空值白屏
  - D-01 附件静默丢弃（最小修复：禁用发送 + 明示）
---

# 批次 2：契约对齐

实施顺序：后端（A-03 → A-04 → A-05/A-06/A-10 → B-01 → B-02 → B-03/B-07 → C-06 → C-04 → C-02）
→ 前端（D-03 → D-05 → D-01）。每完成一组跑对应包测试。
