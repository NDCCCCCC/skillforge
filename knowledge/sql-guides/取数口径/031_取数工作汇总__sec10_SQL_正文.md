---
title: 取数工作汇总
source_idx: 031
section: SQL / 正文
---

## SQL / 正文

```sql
select a.policy_no 保单号,a.policy_application_no 投保单号,a.department_group_name 部门组,a.department_group_code 部门组代码,a.agent_code 代理点,
       a.department_name 部门,a.section_office_name 科室,a.kq 客群,a.operator_name 经办人,a.operator_code 经办人工号,a.product_code 险种代码,a.product_name 险种名称,
       a.premium_amount 含税保费,a.insured_amount 保额,a.APPLICATION_DATE 投保日期,a.issue_date 签单日期,a.inception_date 起保日期,
       a.planned_end_date 终止日期,a.tb_name 投保人,
       a.tb_credential_type 投保人证件类型,
       a.tb_credential_no 投保人身份证号,
       a.tb_mobilephone 投保人手机,
       a.bb_name 被保人,
       a.bb_credential_type 被保人手机,
       a.bb_credential_no 被保人身份证号,
       case when a.group_business_indicator=0 then 1
              when a.group_business_indicator=1 then (select count(1) from (select distinct t.credential_no from "ids_wuh".HUMAN_BODY_T t where t.policy_no=a.policy_no))
           end 投保人数量,
       (select max(b.factor_value) 
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code='M9') 学校名称
from wuh_dw.etl_nonauto_policy_t a
where 
a.department_group_name='随州中心支公司';
```

第二种

```sql
select t.policy_no 保单号,t.bb_name 被保人,t."bb_credential_no",
a.factor_value 学校名称,decode(max(b.factor_value),'O2000','大中专学校学生','O2001','中学生','O2002','小学生','O2003','幼儿','O2004','其他',max(b.factor_value)) 学生类别
from wuh_dw.etl_nonauto_policy_t t
left join "ids_wuh".nonauto_plc_element_t a
on a.policy_no = t.policy_no and a.factor_code='M9'
left join "ids_wuh".nonauto_plc_element_t b
on b.policy_no=t.policy_no and b.factor_code in ('O2')
where a.policy_no in 
('AWUHH631EY25FN00000D','AWUHH631EY25FN00000E','AWUHH631EY25FN00000F','AWUHE631EY25FN000004','AWUHE631EY25FN000005','AWUHH631EY25FN00000G','AWUHO631EY25FN000001','AWUHO631EY25FN000002','AWUHO631EY25FN000004','AWUHO631EY25FN000003','AWUHO631EY25FN000005','AWUHO631EY25FN000006','AWUHO631EY25FN000007','AWUHO631EY25FN000008','AWUHO631EY25FN000009','AWUHO631EY25FN00000A','AWUHO631EY25FN00000B','AWUHO631EY25FN00000C','AWUHO631EY25FN00000D','AWUHO631EY25FN00000E','AWUHO631EY25FN00000F','AWUHO631EY25FN00000G','AWUHO631EY25FN00000H','AWUHO631EY25FN00000I','AWUHO631EY25FN00000J','AWUHO631EY25FN00000K','AWUHO631EY25FN00000L','AWUHO631EY25FN00000N','AWUHO631EY25FN00000M')
group by
t.policy_no,
t.bb_name,
t.bb_credential_no,
a.factor_value
```
