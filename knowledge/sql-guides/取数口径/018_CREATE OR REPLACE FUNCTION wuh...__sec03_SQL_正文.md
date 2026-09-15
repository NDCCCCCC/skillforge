---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
section: SQL / 正文
---

## SQL / 正文

```sql
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

  INSERT INTO FGS_FG_FXQ_Tmp_CB
  SELECT '车险' AS 险种大类,
         a.tb_credential_type,
         a.policy_no,
         a.tb_name,
         wuh_dw.fun_card_getGender_wang(a.tb_credential_no) AS 投保人性别,
         CASE a.tb_credential_type
           WHEN '1' THEN
             '身份证'
           WHEN '10' THEN
             '法人证书\企业代码\营业执照'
           WHEN '2' THEN
             '军官证'
           WHEN '3' THEN
             '军官退休证'
           WHEN '4' THEN
             '护照'
           WHEN '5' THEN
             '居民户口薄'
           WHEN '7' THEN
             '学生证'
           WHEN '8' THEN
             '驾驶证'
           WHEN '9' THEN
             '台胞证'
           WHEN '999' THEN
             '团体其他'
           ELSE
             '其他'
         END AS 投保人证件类型,
         wuh_dw.fun_card_code_wang(a.tb_credential_no) AS 投保人证件号,
         wuh_dw.fun_phone_code_wang(a.tb_mobilephone) AS 投保人电话,
         p.tb_address AS 投保人地址,
         a.bb_name AS 被保人,
```
