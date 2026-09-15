---
title: 开门红存储过程
source_idx: 029
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE OR REPLACE FUNCTION wuh_dw.fr_fgs_kmh_2025()
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$ DECLARE 
begin

/*
 * 往月结果
*/
--入账清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_001';
insert into FR_FGS_KMH_2025_TMP_WANG_001
SELECT a.policy_no,a.endorsement_no,SUM(a.rzbf_notax) rzbf_notax FROM etl_payment_due_t a
WHERE dd BETWEEN to_char(date_trunc('month',sysdate-1-INTERVAL '1' YEAR),'yyyymmdd') AND to_char(sysdate-1-INTERVAL '1' YEAR,'yyyymmdd')
GROUP BY a.policy_no,a.endorsement_no;

--起保清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_002';
insert into FR_FGS_KMH_2025_TMP_WANG_002 
select a.policy_no,a.endorsement_no,a.currency_code,to_char(a.inception_date,'YYYYMMDD') dd,
       sum(nvl(a.premium_amount,0)) premium_amount,sum(nvl(a.taxamount,0)) tax_amount
from ids_wuh.nonauto_premium_t a
WHERE a.inception_date BETWEEN date_trunc('month',sysdate-1-INTERVAL '1' YEAR)
AND  date_trunc('month',sysdate-1-INTERVAL '1' YEAR)+ INTERVAL '1' MONTH - INTERVAL '1' SECOND
and a.workdate <= to_char(sysdate-1-INTERVAL '1' YEAR,'yyyyMMdd')-------workdate小于当天
group by a.policy_no,a.endorsement_no,a.currency_code,to_char(a.inception_date,'YYYYMMDD')
union all
SELECT policy_no,endorsement_no,'01' currency_code,to_char(REQUESTED_DATE,'YYYYMMDD') dd,
       sum(nvl(premium_amount,0)) premium_amount,sum(nvl(tax_amount,0)) tax_amount
FROM etl_policy_request_issue_t WHERE xz = '车险' AND BUSINESS_DATE <= to_char(sysdate-1-INTERVAL '1' YEAR,'yyyyMMdd')
AND REQUESTED_DATE BETWEEN date_trunc('month',sysdate-1-INTERVAL '1' YEAR)
AND date_trunc('month',sysdate-1-INTERVAL '1' YEAR)+ INTERVAL '1' MONTH - INTERVAL '1' SECOND
group by policy_no,endorsement_no,to_char(REQUESTED_DATE,'YYYYMMDD');

--起保清单-加上折币起保保费
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_003';

insert into FR_FGS_KMH_2025_TMP_WANG_003 
select a.policy_no,a.endorsement_no,SUM((a.premium_amount-a.tax_amount)*b.rate / 100) qbbf
from FR_FGS_KMH_2025_TMP_WANG_002 a
join ids_wuh.exrate_month_t b
on a.currency_code = b.bzh
and case
        when a.DD > to_char(sysdate-1,'yyyyMMdd') then
         to_char(sysdate-1, 'yyyymm')
        else
         substr(a.DD,1,6)
       end = trim(b.theyear) || substr('0' || trim(b.themonth), -2)
GROUP BY a.policy_no,a.endorsement_no;

--起保清单-加上起保保费
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_004';
insert into FR_FGS_KMH_2025_TMP_WANG_004 
SELECT a.*
       , nvl(b.rzbf_notax,0) as qbrzbf
FROM FR_FGS_KMH_2025_TMP_WANG_003 a
left join (SELECT policy_no,endorsement_no,SUM(rzbf_notax)as rzbf_notax
           FROM FR_FGS_KMH_2025_TMP_WANG_001
           GROUP BY policy_no,endorsement_no) b on a.policy_no = b.policy_no and a.endorsement_no = b.endorsement_no;

--起保未入账清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_005';
insert into FR_FGS_KMH_2025_TMP_WANG_005 
SELECT a.*,(a.qbbf-a.qbrzbf) as qbwrz FROM FR_FGS_KMH_2025_TMP_WANG_004 a;

--入账含起保未入账清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_006';
insert into FR_FGS_KMH_2025_TMP_WANG_006
SELECT a.policy_no,a.endorsement_no,a.rzbf_notax bf FROM FR_FGS_KMH_2025_TMP_WANG_001 a
UNION ALL
SELECT a.policy_no,a.endorsement_no,a.qbwrz bf FROM FR_FGS_KMH_2025_TMP_WANG_005 a;


--入账含起保未入账清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_007';
```
