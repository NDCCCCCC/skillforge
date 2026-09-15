---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE OR REPLACE FUNCTION wuh_application.fgs_fg_fxq_20260122(begin_date VARCHAR, end_date VARCHAR)
RETURNS VOID
LANGUAGE PLPGSQL
NOT FENCED NOT SHIPPABLE
AS $$
DECLARE
  v_begin_timestamp TIMESTAMP;
  v_end_timestamp   TIMESTAMP;
  v_workdate        VARCHAR(50);
BEGIN
-- 0. 初始化：转换日期参数并计算workdate，避免重复计算

  v_begin_timestamp := TO_DATE(begin_date, 'YYYYMMDD');
  v_end_timestamp   := TO_DATE(end_date, 'YYYYMMDD') + INTERVAL '1 day' - INTERVAL '1 second';
  v_workdate        := begin_date || '-' || end_date;

  -- 1. 清空临时表（使用变量避免硬编码表名重复书写）


  EXECUTE 'TRUNCATE TABLE FGS_FG_FXQ_Tmp_CB';
  EXECUTE 'TRUNCATE TABLE FGS_FG_FXQ_Tmp_LP';

  EXECUTE 'TRUNCATE TABLE FGS_FG_FXQ_Tmp_PT';
  EXECUTE 'TRUNCATE TABLE FGS_FG_FXQ_Tmp_GH';
```
