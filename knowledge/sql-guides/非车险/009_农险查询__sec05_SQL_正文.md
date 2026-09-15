---
title: 农险查询
source_idx: 009
section: SQL / 正文
---

## SQL / 正文

```sql
select t.POLICY_NO 保单号,a.product_name 险种名称,a.inception_date 起保日期,a.planned_end_date 终保日期
,a.tb_name 投保人姓名
, t.INSURED_NAME  被保人姓名
,t.INSURED_TELEPHONE  被保人联系电话
       ,t.INSURED_CERTIFICATE_NO  被保人证件号码,  t.INSURED_BANK_ACCOUNT  被保人银行账号
       ,t.INSURED_OBJECT 标的物名称
       ,s.object_address
,t.NUMBER_OF_INSURED_OBJECT  承保数量 
       ,t.PREMIUM  保费
       ,z.yjpf
from "ids_wuh".dwd_ag_cbfhqd_t t
left join "ids_wuh".dwd_ag_policy_t s on s.POLICY_NO=t.policy_no
left join wuh_dw.etl_nonauto_policy_t a on a.POLICY_NO=t.policy_no
left join wuh_dw.xzh_nx_view_new c on c.yxzh=a.product_code
left join "ids_wuh".DWD_AG_POLICY_EXTEND_T k on k.POLICY_NO=t.policy_no 
left join "ids_wuh".DWD_AG_POLICY_REQUEST_T h on h.POLICY_NO=t.policy_no and h.ENDORSEMENT_NO='无'
left join (select policy_no, SUM(insured_object_indemnity + investigation_fee + investigation_agent_fee + inside_investigation_agent_fee + claim_audit_fee + salvage_fee + other_fee - salvage_recovery) as yjpf
               from ids_wuh.NONAUTO_CF_INDEMNITY_T
              WHERE TRUNC(POSTED_DATE) < TRUNC(SYSDATE - 1)
              group by policy_no ) z on z.POLICY_no = a.POLICY_no
where a.department_name = '公安支公司' and a.inception_date BETWEEN DATE('2023-01-01 00:00:00') and date('2025-12-31 23:59:59')
order by t.CREATE_DATE desc
```
