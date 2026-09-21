---
source_idx: 018
title: CREATE OR REPLACE FUNCTION wuh...
sections: 2
---

# CREATE OR REPLACE FUNCTION wuh...

## CREATE OR REPLACE FUNCTION wuh...

# CREATE OR REPLACE FUNCTION wuh...



**涉及表/视图**: `ids_wuh.party_t`, `ids_wuh.AUTO_FEE_RECORD_T`, `wuh_dw.etl_policy`, `precomputed_party`, `precomputed_payment`, `wuh_dw.etl_nonauto_policy_t`, `FGS_FG_FXQ_Result_CB`, `FGS_FG_FXQ_Tmp_CB`

**输出字段示例**: 险种大类, 投保人性别, 投保人证件类型, 投保人证件号, 投保人电话, 投保人地址, 被保人, 被保人证件类型

## SQL / 正文

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
## SQL / 正文

```sql
         wuh_dw.fun_card_getGender_wang(a.bb_credential_no) 被保人性别,
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
         END AS 被保人证件类型,
         wuh_dw.fun_card_code_wang(a.bb_credential_no) 被保人证件号,
         wuh_dw.fun_phone_code_wang(a.bb_mobilephone) 被保人电话,
         p.bb_address 被保人地址,
         a.sp_name 索赔人,
         wuh_dw.fun_card_getGender_wang(a.sp_credential_no) 索赔人性别,
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
         END AS 索赔人证件类型,
         wuh_dw.fun_card_code_wang(a.sp_credential_no) 索赔人证件号,
         wuh_dw.fun_phone_code_wang(a.sp_mobilephone) 索赔人电话,
         p.sp_address 索赔人地址,
         a.product_name 险种名称,
         a.issue_date 签单日期,
         a.signature_date 签发日期,
         a.department_group_code 部门组代码,
         a.department_group_name 部门组名称,
         a.department_code 部门代码,
         a.department_name 部门名称,
         a.operator_code 经办人代码,
         a.operator_name 经办人名称,
         a.premium_amount AS 不含税保费,
         a.new_channel AS 渠道,
         a.agent_code AS 代理点代码,
         a.dldmc AS 代理点名称,
         pmt.pay_way AS 支付方式
    FROM wuh_dw.etl_policy a
    LEFT JOIN precomputed_party p ON p.policy_no = a.policy_no
    LEFT JOIN precomputed_payment pmt ON pmt.application_no = a.policy_application_no
   WHERE a.signature_date >= v_begin_timestamp
     AND a.signature_date <= v_end_timestamp
     AND a.premium_amount >= 10000 UNION ALL
  union all

            SELECT a.xzdl 险种大类,
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
```
## SQL / 正文

```sql
                   wuh_dw.fun_card_code_wang(a.tb_credential_no) AS 投保人证件号,
                   wuh_dw.fun_phone_code_wang(a.tb_mobilephone) AS 投保人电话,
                   p.tb_address AS 投保人地址,
                   a.bb_name AS 被保人,
                   wuh_dw.fun_card_getGender_wang(a.bb_credential_no) 被保人性别,
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
                   END AS 被保人证件类型,
                   wuh_dw.fun_card_code_wang(a.bb_credential_no) 被保人证件号,
                   wuh_dw.fun_phone_code_wang(a.bb_mobilephone) 被保人电话,
                   p.bb_address 被保人地址,
                   a.sp_name 索赔人,
                   wuh_dw.fun_card_getGender_wang(a.sp_credential_no) 索赔人性别,
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
                   END AS 索赔人证件类型,
                   wuh_dw.fun_card_code_wang(a.sp_credential_no) 索赔人证件号,
                   wuh_dw.fun_phone_code_wang(a.sp_mobilephone) 索赔人电话,
                 

  p.sp_address 索赔人地址,
                   a.product_name 险种名称,
                   a.issue_date 签单日期,
                   a.signature_date 签发日期,
                   a.department_group_code 部门组代码,
                   a.department_group_name 部门组名称,
                   a.department_code 部门代码,
                   a.department_name 部门名称,
                   a.operator_code 经办人代码,
                   a.operator_name 经办人名称,
                   a.premium_amount AS 不含税保费,
                   a.new_channel AS 渠道,
                   a.agent_code AS 代理点代码,
                   a.dldmc AS 代理点名称,
                   pmt.pay_way AS 支付方式
              FROM wuh_dw.etl_nonauto_policy_t 
              LEFT JOIN precomputed_party p ON p.policy_no = a.policy_no
              LEFT JOIN precomputed_payment pmt ON pmt.application_no = a.policy_application_no
             WHERE a.signature_date >= v_begin_timestamp
               AND a.signature_date <= v_end_timestamp
               AND a.premium_amount >= 10000
-- 3. 使用变量避免重复的workdate计算[9](@ref)
 DELETE
              FROM FGS_FG_FXQ_Result_CB
             WHERE workdate = v_workdate;
```
## SQL / 正文

```sql
  INSERT INTO FGS_FG_FXQ_Result_CB
  SELECT *, v_workdate
    FROM FGS_FG_FXQ_Tmp_CB
    
EXCEPTION
        WHEN OTHERS THEN
            RAISE EXCEPTION '存储过程执行失败：%', SQLERRM;
            ROLLBACK;
END;
$$;
```

