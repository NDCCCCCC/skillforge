---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
split_part: 2/6
source_split: true
---

## SQL / 正文

```sql
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

  -- 2. 使用CTE优化承保数据查询，减少重复关联和计算


    WITH precomputed_party AS
           (SELECT policy_no,
                   MAX(CASE
                         WHEN role = '1' THEN
                           address
                       END ) AS tb_address,
                   MAX(CASE
                         WHEN role = '2' THEN
                           address
                       END ) AS bb_address,
                   MAX(CASE
                         WHEN role = '3' THEN
                           address
                       END ) AS sp_address
              FROM ids_wuh.party_t
             WHERE role IN('1', '2', '3')
             GROUP BY policy_no),
         precomputed_payment AS
           (SELECT application_no,
                   MAX(CASE pay_way
                         WHEN '1' THEN
                           '支票'
                         WHEN '2' THEN
                           '划卡'
                         when '3' then
                           '银行柜面现金'
                         when '3A' then
                           '集中支付'
                         when '4' then
                           '现金'
                         when '5' then
                           '网银转账'
                         when '6' then
                           '刷卡'
                         when '7' then
                           '现金结算'
                         when '8' then
                           '网上银行'
                         when '9' then
                           '电话银行'
                         when '10' then
                           '柜台转账'
                         when '11' then
                           '实时代扣'
                         when '12' then
                           '支付宝'
                         when 'chinapay' then
                           '银联电子支付'
                         when 'weixin' then
                           '微信'
                         when '07' then
                           '快钱'
                         when '09' then
                           '电销通联支付'
                         when 'SAICPAY' then
                           '上汽支付'
                         when 'TENPAY' then
                           '财付通'
                         when 'alipay' then
                           '支付宝'
                         when 'ALIPAY' then
                           '支付宝'
                         when 'mpay' then
                           '聚合支付'
                         when 'BILL' then
                           '划卡'
                         ELSE
                           '其他'
                       END ) AS pay_way
              FROM ids_wuh.AUTO_FEE_RECORD_T
             GROUP BY application_no)