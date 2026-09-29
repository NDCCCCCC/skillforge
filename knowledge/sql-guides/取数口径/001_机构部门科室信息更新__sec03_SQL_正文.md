---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: 001
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



update etl_payment_due_t
set department_group_code='420100'
  , department_group_name='武汉中心支公司'
  , department_code='EAJ'
  , tjbmmc='团客中心经纪业务部'
  , section_office_code='EAJ'
  , section_office_name='团客中心经纪业务部'
where department_group_name='业务'
and section_office_code not in ('OVD','CCI');
 update etl_policy_request_issue_t
set department_group_code='420100'
  , department_group_name='武汉中心支公司'
  , department_code='WFV'
  , department_name='团客中心轨道交通业务部'
  , section_office_code='WFV'
  , section_office_name='团客中心轨道交通业务部'
where department_group_name='业务'
  and department_code='06X'
  and section_office_code in ('OVD','CCI');
  
update etl_policy_request_issue_t
set department_group_code='420100'
  , department_group_name='武汉中心支公司'
  , department_code='EAJ'
  , department_name='团客中心经纪业务部'
  , section_office_code='EAJ'
  , section_office_name='团客中心经纪业务部'
where department_group_name='业务'
and section_office_code not in ('OVD','CCI');
 update etl_NONAUTO_POLICY_T
set department_group_code='420100'
  , department_group_name='武汉中心支公司'
  , department_code='WFV'
  , department_name='团客中心轨道交通业务部'
  , section_office_code='WFV'
  , section_office_name='团客中心轨道交通业务部'
where department_group_name='业务'
  and department_code='06X'
  and section_office_code in ('OVD','CCI');
  

update etl_NONAUTO_POLICY_T
set department_group_code='420100'
  , department_group_name='武汉中心支公司'
  , department_code='EAJ'
  , department_name='团客中心经纪业务部'
  , section_office_code='EAJ'
  , section_office_name='团客中心经纪业务部'
where department_group_name='业务'
and section_office_code not in ('OVD','CCI');
```

注：1、目前所有二层数据的部门和部门组已改为重组后的武汉中支数据，要想查询历史公司业务部数据需要查询原始ids下发数据表，带上条件department_code in ('050','06X','C19')，其中“06X”为历史最新公司业务部部门代码，部门名称为“公司业务部new”。2、需要定期对公司业务部退保数据进行更新二层！财务提取公司业务部历史数据进行成本核算：--公司业务部保单清单

```sql
drop table tmp_ycb_001;
create table tmp_ycb_001 as 
select policy_no,department_code,section_office_code from ids_wuh.auto_agreement_t t where t.department_code in ('050','06X','C19')
union all
select policy_no,department_code,section_office_code from ids_wuh.nonauto_agreement_t t where t.department_code in ('050','06X','C19')
;


--入账保费收入+跟单手续费业务费绩效
drop table tmp_ycb_002;
```
