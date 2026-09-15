---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: 001
section: SQL / 正文
---

## SQL / 正文

```sql
--逐单未到期是指一单一单的算未到期，包括法定未到期和报表未到期   ADJUST_AFTER_BRANCHREIN_UPR2  调整后二号分公司再保后未到期
--法定未到期 - 首日费用率 = 报表未到期
SELECT a.policy_no 保单号
       ,a.product_code 险种代码
       ,(select product_name from xzh t where t."product_code"=a.product_code) 险种名称
       ,a.DEPTGROUP_CODE 部门组代码,a.DEPT_CODE 部门代码,a.actuary_coverage_code 科室代码
    --, (SELECT xzdl FROM FR_FGS_ZDWDQ_PRODUCTCODE b WHERE a.financial_product_code = b.productcode) 险种大类
    , '业务' 部门组名称
      , (select department_name from ids_wuh.bm_t s where s.department_code=g.department_CODE) 部门名称
      , (select section_office_name from ids_wuh.ks_t s where s.section_office_code=g.section_office_code) 科室名称
       , to_date(ESTIMATE_DATE,'yyyyMMdd') 评估时点
       , last_day(add_months(to_date(ESTIMATE_DATE,'yyyyMMdd'),1)) 评估时点1
       , to_date( a.PLANNED_END_DATE,'yyyyMMdd') 终保日期
       ,t.kq_new 客群,nvl(t.tb_name,h.tb_name) 投保人名称,nvl(t.bb_name,h.bb_name) 被保人名称
    , ADJUST_AFTER_BRANCHREIN_UPR 调整后法定分公司再保后未到期
       , ADJUST_LSBPI_UPR 调整后法定分公司再保后保费不足
    , ADJUST_AFTER_BRANCHREIN_UPR2  调整后二号分公司再保后未到期
    , ADJUST_LSBPI_UPR2 调整后二号分公司再保后保费不足
FROM ids_wuh.op_uprcollection_all_t_adj a
left join etl_nonauto_policy_t t on t.policy_no = a.policy_no left join etl_policy h on h.policy_no = a.policy_no 
left join tmp_ycb_001 g on g.policy_no = a.policy_no
--join tmp_ycb_001 b on b.policy_no = a.policy_no
--left join "etl_policy_request_issue_t" on b.policy_no = a.policy_no
where a.DEPT_CODE in ('050','06X','C19')
and ESTIMATE_DATE = '20260131'
;
--已决清单

drop table tmp_ycb_003;
```
