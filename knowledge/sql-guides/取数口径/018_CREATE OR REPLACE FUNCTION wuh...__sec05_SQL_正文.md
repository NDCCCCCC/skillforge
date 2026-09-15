---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
section: SQL / 正文
---

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
