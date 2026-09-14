---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
split_part: 5/6
source_split: true
---

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