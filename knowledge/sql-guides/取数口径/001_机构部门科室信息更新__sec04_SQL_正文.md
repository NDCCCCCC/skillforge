---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: 001
section: SQL / 正文
---

## SQL / 正文

```sql
create table tmp_ycb_002 as 
select 
a.policy_no 保单号,
a.endorsement_no 批单号,
a.dd 入账日,
a.vat_tax 增值税,
a.travel_tax 车船税,
a.rzbf_with_tax 含税入账保费,
a.rzbf_notax 不含税入账保费,
a.xzdl 险种大类,
a.product_code 险种代码,
a.product_name 险种名称,
a.department_group_code 部门组代码,
a.department_group_name 部门组名称,
a.department_code 部门代码,
a.tjbmmc 部门名称,
a.section_office_code 科室代码,
a.section_office_name 科室名称,
a.operator_code 经办人代码,
a.operator_name 经办人名称,
a.qd 渠道,
a.kq 客群,
t.xzhdlmc 险种分类,
t.COINSURANCE_INDICATOR 共保标志,
t.coinsurance_indicator_name 共保标志名称,
t.COINSURANCE_PERCENT 共保比例,nvl(t.tb_name,h.tb_name) 投保人名称,nvl(t.bb_name,h.bb_name) 被保人名称,
f.reinsurance_percent_2 再保自留比例,--s.DOCUMENTARYBONUSRATE 非车险绩效费率, --s.DOCUMENTARYBUSINESSRATE 非车险业务费率,--s.DOCUMENTARYCOMMISSIONRATE 非车险手续费佣金率,--s.DOCUMENTARYSERVICERATE  非车险增值服务费率,--g.DOCUMENTARYBONUSRATE 车险绩效费率, --g.DOCUMENTARYBUSINESSRATE 车险业务费率,--g.DOCUMENTARYCOMMISSIONRATE 车险手续费佣金率--,--g.DOCUMENTARYSERVICERATE  车险增���服务费率s.DOCUMENTARYBONUSRATE*a.rzbf_notax 非车险绩效费,
s.DOCUMENTARYBUSINESSRATE*a.rzbf_notax 非车险业务费,
s.DOCUMENTARYCOMMISSIONRATE*a.rzbf_notax 非车险手续费佣金,
--s.DOCUMENTARYSERVICERATE  非车险增值服务费率,
g.DOCUMENTARYBONUSRATE*a.rzbf_notax 车险绩效费, 
g.DOCUMENTARYBUSINESSRATE*a.rzbf_notax 车险业务费,
g.DOCUMENTARYCOMMISSIONRATE*a.rzbf_notax 车险手续费佣金--,
from etl_payment_due_t a
left join etl_nonauto_policy_t t on a.policy_no=t.policy_no
left join etl_policy h on h.policy_no = a.policy_no 
left join (select policyno,nvl(endorseno,'无') endorseno
            ,max(nvl(DOCUMENTARYBONUSRATE,0)) DOCUMENTARYBONUSRATE ----绩效费率
            ,max(nvl(DOCUMENTARYBUSINESSRATE,0)) DOCUMENTARYBUSINESSRATE ----业务费率
            ,max(nvl(DOCUMENTARYCOMMISSIONRATE,0)) DOCUMENTARYCOMMISSIONRATE -----手续费佣金率
            ,max(nvl(DOCUMENTARYSERVICERATE,0)) DOCUMENTARYSERVICERATE ----增值服务费率
            from ids_wuh.CSCMFAUTPOLICYFEE
            group by policyno,nvl(endorseno,'无')) s on a.policy_no=s.policyno and a.endorsement_no=s.endorseno
left join (select policyno,nvl(endorseno,'无') endorseno
            ,max(nvl(DOCUMENTARYBONUSRATE,0)) DOCUMENTARYBONUSRAT

E ----绩效费率
            ,max(nvl(DOCUMENTARYBUSINESSRATE,0)) DOCUMENTARYBUSINESSRATE ----业务费率
            ,max(nvl(DOCUMENTARYCOMMISSIONRATE,0)) DOCUMENTARYCOMMISSIONRATE -----手续费佣金率
            ,max(nvl(DOCUMENTARYSERVICERATE,0)) DOCUMENTARYSERVICERATE ----增值服务费率
            from ids_wuh.CSCMAUTPOLICYFEE
            group by policyno,nvl(endorseno,'无')) g on a.policy_no=g.policyno and a.endorsement_no=g.endorseno
left join ids_wuh.reinsurance_t f on a.policy_no =f.REINSURANCE_POLICY_NO
where EXISTS (

    SELECT 1 
    FROM tmp_ycb_001 b 
    WHERE b.policy_no = a.policy_no
)
and a.dd>=20260101
and a.dd<=20260228and a.rzbf_notax<>0

;


update tmp_ycb_002 a
set 部门组名称='业务'
   ,部门代码=(select b.department_code from tmp_ycb_001 b where a.保单号=b.policy_no)
   ,科室代码=(select b.section_office_code from tmp_ycb_001 b where a.保单号=b.policy_no)
   ;
update tmp_ycb_002 a
set 部门组名称='业务'
   ,部门名称=(select b.department_name from ids_wuh.bm_t b where a.部门代码=b.department_code)
   ,科室名称=(select b.section_office_name from ids_wuh.ks_t b where a.科室代码=b.section_office_code)
   ;
   
select * from tmp_ycb_002;  
```
