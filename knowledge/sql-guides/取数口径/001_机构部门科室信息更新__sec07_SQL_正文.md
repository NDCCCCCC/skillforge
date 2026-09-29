---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: 001
section: SQL / 正文
---

## SQL / 正文

```sql
create table tmp_ycb_002 as
SELECT o.policy_no 保单号
       ,t.product_code 险种代码
       ,t.product_name 险种名称
       ,t.department_group_code 部门组代码,t.department_CODE 部门代码,t.section_office_code 科室代码
    --, (SELECT xzdl FROM FR_FGS_ZDWDQ_PRODUCTCODE b WHERE a.financial_product_code = b.productcode) 险种大类
    , t.department_group_name  部门组名称
      , t.department_name 部门名称
      , t.section_office_name  科室名称
       , '20260631' 评估时点
       --, last_day(add_months(to_date(ESTIMATE_DATE,'yyyyMMdd'),1)) 评估时点1
       ,t.inception_date 起保日期, t.PLANNED_END_DATE 终保日期
       ,t.kq_new 客群,t.tb_name 投保人名称,t.bb_name 被保人名称
       ,f.reinsurance_percent_2 再保自留比例
    , a.ADJUST_AFTER_BRANCHREIN_UPR 调整后法定分公司再保后未到期
       , a.ADJUST_LSBPI_UPR 调整后法定分公司再保后保费不足
    , a.ADJUST_AFTER_BRANCHREIN_UPR2  调整后二号分公司再保后未到期
    , a.ADJUST_LSBPI_UPR2 调整后二号分公司再保后保费不足
    ,(--入账

    select sum(rzbf_notax)
      from etl_payment_due_t s
      where s.dd<=20260631
      and s.policy_no=o.policy_no
      and s.rzbf_notax<>0
    ) 入账不含税金额
    ,(--已决清单
      select sum(b.insured_object_indemnity+b.investigation_fee
           +b.investigation_agent_fee+b.inside_investigation_agent_fee
           +b.claim_audit_fee+b.salvage_fee+b.other_fee
           -b.salvage_recovery) 
      from ids_wuh.nonAUTO_CF_INDEMNITY_T b --非车险
      where b.policy_no =o.policy_no
      and b.posted_date<=to_date('2026-06-31 23:59:59','yyyy-mm-dd hh24:mi:ss')) 赔付金额
     ,(--未决
     select sum(gsje) from "ids_wuh".INDISPUTE_T_1214 t
      where t.nf='2026'
        and t.qsh='06'
        and t.polno=o.policy_no
     ) 估损金额
FROM tmp_ycb_001 o
left join 
(
  select y.policy_no,sum(y.ADJUST_AFTER_BRANCHREIN_UPR) ADJUST_AFTER_BRANCHREIN_UPR
       , sum(y.ADJUST_LSBPI_UPR) ADJUST_LSBPI_UPR
    , sum(y.ADJUST_AFTER_BRANCHREIN_UPR2)  ADJUST_AFTER_BRANCHREIN_UPR2
    , sum(y.ADJUST_LSBPI_UPR2) ADJUST_LSBPI_UPR2
    from ids_wuh.op_uprcollection_all_t_adj y
    where EXISTS (
        SELECT 1 
        FROM tmp_ycb_001 b 
        WHERE b.policy_no = y.policy_no
    ) and y.ESTIMATE_DATE = '20260631' 
    group by y.policy_no
) a on o.policy_no = a.policy_no
left join ids_wuh.reinsurance_t f on o.policy_no =f.REINSURANCE_POLICY_NO
left join etl_nonauto_policy_t t on t.policy_no = o.policy_no 
where o.policy_no='AWUHBSGJ0125QAAAAA2C'

SELECT t.policy_no 保单号
       ,t.product_code 险种代码
       ,t.product_name 险种名称
       ,t.department_group_code 部门组代码,t.department_CODE 部门代码,t.section_office_code 科室代码
    --, (SELECT xzdl FROM FR_FGS_ZDWDQ_PRODUCTCODE b WHERE a.financial_product_code = b.productcode) 险种大类
    , t.department_group_name  部门组名称
      , t.department_name 部门名称
      , t.section_office_name  科室名称
       , '20251231' 评估时点
       --, last_day(add_months(to_date(ESTIMATE_DATE,'yyyyMMdd'),1)) 评估时点1
       ,t.inception_date 起保日期, t.PLANNED_END_DATE 终保日期
       ,t.kq_new 客群,t.tb_name 投保人名称,t.bb_name 被保人名称
       ,f.reinsurance_percent_2 再保自留比例
    , a.ADJUST_AFTER_BRANCHREIN_UPR 调整后法定分公司再保后未到期
       , a.ADJUST_LSBPI_UPR 调整后法定分公司再保后保费不足
    , a.ADJUST_AFTER_BRANCHREIN_UPR2  调整后二号分公司再保后未到期
    , a.ADJUST_LSBPI_UPR2 调整后二号分公司再保后保费不足
    ,(--入账
    select sum(rzbf_notax)
      from wuh_dw.etl_payment_due_t s
      where s.dd<=20251231
      and s.policy_no=t.policy_no
      and s.rzbf_notax<>0
    ) 入账不含税金额
    ,(--已决清单
```
