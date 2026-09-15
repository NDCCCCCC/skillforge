---
title: 财务逐单未到期
source_idx: 020
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



DECLARE
    estimate date;
    estimate_1 date;
    estimate_2 date;
    estimate_3 date;
    sql_text text;
BEGIN
    -- 计算日期变量
    estimate := DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '1 day';
    estimate_1 := DATE_TRUNC('month', estimate + INTERVAL '2 month') - INTERVAL '1 day';
    estimate_2 := DATE_TRUNC('month', estimate + INTERVAL '3 month') - INTERVAL '1 day';
    estimate_3 := DATE_TRUNC('year', CURRENT_DATE) + INTERVAL '1 year' - INTERVAL '1 day';

    RAISE NOTICE 'estimate: %, estimate_1: %, estimate_2: %, estimate_3: %', 
                  estimate, estimate_1, estimate_2, estimate_3;

    -- 创建临时表存储中间结果
    DROP TABLE IF EXISTS temp_v_FR_FGS_CW_ZDWDQ1;
    CREATE TEMP TABLE temp_v_FR_FGS_CW_ZDWDQ1 AS
    SELECT a.policy_no as 保单号
       , a.product_code as 险种代码
       , (SELECT MAX(substr(b.tjxzh,1,2)) FROM ids_wuh.typecomp_t b WHERE b.YXZH = a.product_code) as tjxzh_str
       , (case when DEPTGROUP_CODE='429021' then '宜昌中心支公司'
           when DEPT_CODE in ('050','06X','C19') then '业务' 
           else (select department_group_name from ids_wuh.bmz_t c where (DEPTGROUP_CODE=c.department_group_code)) end) as 部门组名称 
       , (case when DEPTGROUP_CODE='429021' then '神农架支公司'
           else (select department_name from ids_wuh.bm_t d where DEPT_CODE=d.department_code and d.department_group_code<'429021' and d.department_group_code>'400000') end) as 部门名称
       , estimate as 评估时点
       , estimate_1 as 评估时点1 
       , estimate_2 as 评估时点2             
       , estimate_3 as 评估时点3       
       , to_date(a.PLANNED_END_DATE,'yyyyMMdd') as 终保日期
       , ADJUST_AFTER_BRANCHREIN_UPR as 调整后法定分公司再保后未到期
       , ADJUST_LSBPI_UPR as 调整后法定分公司再保后保费不足
       , ADJUST_AFTER_BRANCHREIN_UPR2 as 调整后二号分公司再保后未到期
       , ADJUST_LSBPI_UPR2 as 调整后二号分公司再保后保费不足
       , (case when DEPT_CODE in (select department_code from fgs_fc_gxb) then '1' else '0' end) as is_gxb
       , t.FUEL_TYPE
    FROM ids_wuh.op_uprcollection_all_t_adj a 
    LEFT JOIN etl_policy t ON a.policy_no = t.policy_no
    WHERE ESTIMATE_DATE = to_char(estimate,'yyyymmdd');

    -- 创建第二个临时表

    DROP TABLE IF EXISTS temp_v_FR_FGS_CW_ZDWDQ2;
```
