---
title: 农险查询
source_idx: 009
section: SQL / 正文
---

## SQL / 正文

```sql
/*
```

理赔：报案号、立案号、赔案号、出险时间、出险地点、赔案进度（已受理 查勘中 定损中 理算中 核赔中 支付中 已支付）

*/
/*


```sql
select a.REPORT_CASE_NO 报案号,t.CLAIM_FOLDER_NO 立案号,s.INDEMNITY_NO 赔案号,a.POLICY_NO 保单号
     ,a.REPORTER_NAME 报案人姓名,a.REPORTER_CERT_TYPE 报案人证件类型,a.REPORTER_CERTIFICATE_NO 报案人证件号,REPORTER_PHONE 报案人联系方式
       ,a.LOSS_EVENT_DATE 出险时间,a.LOSS_EVENT_PLACE 出险地点,a.CASE_DESCRIPTION 出险经过描述
     ,a.REGISTER_STATUS 立案状态--0，未立案；1，已立案；2，已注销 
       ,t.CLAIM_REGISTER_STATUS 立案复核进度--立案状态：0，未复核；1，复核通过；2，复核退回      
       ,s.PAYMENT_STATUS 赔付状态--赔付状态 9 其他 1 已赔付 0 未赔付    
from "ids_wuh".dwd_ag_report_case_t a
left join "ids_wuh".dwd_ag_claim_folder_t t on t.REPORT_CASE_NO=a.REPORT_CASE_NO
left join "ids_wuh".dwd_ag_cf_indemnity_t s on s.REPORT_CASE_NO=a.REPORT_CASE_NO
;
*/

select t.CLAIM_FOLDER_NO  立案号 , t.INDEMNITY_NO  赔案号 ,t.REPORT_CASE_NO   报案号,t.POLICY_NO   保单号 
       ,  a.LOSS_EVENT_DATE 出险时间,a.LOSS_EVENT_PLACE 出险地点,a.CASE_DESCRIPTION 出险经过描述
       ,  t.REMIT_ACCOUNT  收款人账号 ,t.REMIT_BANK_CODE  收款人开户行 ,t.REMIT_CERT_NO  收款人证件号 ,t.REMIT_NAME  收款人姓名 ,t.REMIT_TEL  收款人电话 
       ,  t.PAY_DATE  支付时间 ,t.MAKE_AMOUNT  定损金额 ,  t.MAKE_COUNT  定损户数 ,t.AFFECTED_AREA  受灾面积 ,t.AREA_NO  地块号 
       ,  t.INSURED_NAME  被保人姓名 ,  t.INSURED_CERTIFICATE_TYPE  被保人证件类型 ,  t.INSURED_CERTIFICATE_NO  被保人证件号码 
       ,a.REGISTER_STATUS 立案状态--0，未立案；1，已立案；2，已注销 
       ,g.CLAIM_REGISTER_STATUS 立案复核进度--立案状态：0，未复核；1，复核通过；2，复核退回      
       ,s.PAYMENT_STATUS 赔付状态--赔付状态 9 其他 1 已赔付 0 未赔付 
from "ids_wuh".dwd_ag_lpfhqd_t t
left join "ids_wuh".dwd_ag_report_case_t a on a.REPORT_CASE_NO=t.REPORT_CASE_NO
left join "ids_wuh".dwd_ag_claim_folder_t g on g.REPORT_CASE_NO=t.REPORT_CASE_NO
left join "ids_wuh".dwd_ag_cf_indemnity_t s on s.REPORT_CASE_NO=t.REPORT_CASE_NO
;

/*赔案支付数据*/

select a.POLICY_NO,a.ACCIDENT_NO 事故号,a.ACCIDENT_ID 事故ID ,a.CLAIM_NO,t.REPORT_CASE_NO 报案号,t.CLAIM_FOLDER_NO  立案号,a.STATUS_PAYMENT_PLAN 赔付状态
       ,a.PAYEE_REFUND 收款人户,a.REMIT_NAME 收款人户名,a.REMIT_ACCOUNT 收款人帐号,a.REMIT_BANK_CODE 收款人开户行代码,a.TYPE -- 1：保单，2：赔案
       a.PAID_DATE 支付时间,a.PAY_DATE 支付完成时间,a.PAYMENT_AMOUNT 赔付金额
       ,a.PAY_AMOUNT 支付金额,a.PAY_AMOUNT_REFUND  赔付金额1,a.PAY_CURRENCY  支付币种  
from "ids_wuh".dwd_ag_payment_account_t a
left join "ids_wuh".dwd_ag_cf_indemnity_t t on a.ACCIDENT_NO=t.REPORT_CASE_NO
;
```
