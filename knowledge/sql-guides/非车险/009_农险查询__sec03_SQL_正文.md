---
title: 农险查询
source_idx: 009
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



select a.policy_no 保单号,
       case when a.policy_status='1' then '有效'
            when a.policy_status='2' then '退保'
            when a.policy_status='3' then '注销'
            when a.policy_status='4' then '终止责任'
            when a.policy_status='h' then '非正常终止'
            else '其他' end policy_status --保单状态
       ,a.product_name 险种名称,a.inception_date 起保日期,a.planned_end_date 终保日期
       ,a.tb_name 投保人姓名,a.tb_credential_type 投保人证件类型,a.tb_credential_no 投保人证件号码,a."tb_mobilephone" 投保人手机号
     ,a.bb_name 被保人姓名,a.bb_credential_type 被保人证件类型,a.bb_credential_no 被保人证件号码,a."bb_mobilephone" 被保人手机号
       ,a.premium_amount 保费,a.insured_amount 保障额度,a.department_group_name 承保机构
       ,(select max(t.INSURED_OBJECT) from "ids_wuh".dwd_ag_cbfhqd_t t where t.POLICY_NO=a.policy_no) 保险标的项目
       ,c.hylx 行业类型
       ,c.zclx 政策类型
from etl_nonauto_policy_t a
left join "ids_wuh".dwd_ag_policy_t s on s.POLICY_NO=a.policy_no
left join xzh_nx_view_new c on c.yxzh=a.product_code
where a.xzdl='农险'
;

select * from "ids_wuh".dwd_ag_cbfhqd_t t where policy_no='AWUHR5889820Q050086A' ;
*/

select t.POLICY_NO 保单号,t.INSURED_OBJECT 保险标的项目, t.INSURED_NAME  被保人姓名 ,  t.INSURED_CERTIFICATE_TYPE  被保人证件类型 
       ,t.INSURED_CERTIFICATE_NO  被保人证件号码,  t.INSURED_BANK_ACCOUNT  被保人银行账号 , t.INSURED_BANK  被保人开户银行 
       ,t.INSURED_TELEPHONE  被保人联系电话 ,t.INSURED_AMOUNT  保额 ,t.PREMIUM  保费 ,k.PREMIUM_NH as "保费-农户自缴",
       h.PREMIUM_NH as "保费-农户自缴",t.NUMBER_OF_INSURED_OBJECT  承保数量 
       ,case when a.policy_status='1' then '有效'
            when a.policy_status='2' then '退保'
            when a.policy_status='3' then '注销'
            when a.policy_status='4' then '终止责任'
            when a.policy_status='h' then '非正常终止'
            else '其他' end policy_status --保单状态
       ,a.product_name 险种名称,a.inception_date 起保日期,a.planned_end_date 终保日期
       ,a.tb_name 投保人姓名,a.tb_credential_type 投保人证件类型,a.tb_credential_no 投保人证件号码,a."tb_mobilephone" 投保人手机号
     /*,a.bb_name 被保人姓名,a.bb_credential_type 被保人证件类型,a.bb_credential_no 被保人证件号码,a."bb_mobilephone" 被保人手机号
       ,a.premium_amount 保费,a.insured_amount 保障额度,*/
       ,a.department_group_name 承保机构
       ,c.hylx 行业类型
       ,c.zclx 政策类型
from "ids_wuh".dwd_ag_cbfhqd_t t
left join "ids_wuh".dwd_ag_policy_t s on s.POLICY_NO=t.policy_no
left join wuh_dw.etl_nonauto_policy_t a on a.POLICY_NO=t.policy_no
left join wuh_dw.xzh_nx_view_new c on c.yxzh=a.product_code
left join "ids_wuh".DWD_AG_POLICY_EXTEND_T k on k.POLICY_NO=t.policy_no 
left join "ids_wuh".DWD_AG_POLICY_REQUEST_T h on h.POLICY_NO=t.policy_no and h.ENDORSEMENT_NO='无'
order by t.CREATE_DATE desc
;
```
