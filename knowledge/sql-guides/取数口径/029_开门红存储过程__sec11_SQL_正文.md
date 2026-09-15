---
title: 开门红存储过程
source_idx: 029
section: SQL / 正文
---

## SQL / 正文

```sql
insert into FR_FGS_KMH_2025_TMP_WANG_Result_wj 
with v_case as(

SELECT bmz,'汇总' ,SUM(bf) as bf,'全险种'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 GROUP BY bmz
UNION ALL
SELECT bmz,'汇总' ,SUM(bf) as bf,'个客整体'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE class_name = '个客' GROUP BY bmz
UNION ALL
SELECT bmz,'汇总' ,SUM(bf) as bf,'车险整体'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl = '车险' GROUP BY bmz
UNION ALL
SELECT bmz,qd ,SUM(bf) as bf,'车险分渠道'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl = '车险' GROUP BY bmz,qd
UNION ALL
SELECT bmz,usage_name_b ,SUM(bf) as bf,'车险分使用性质' 
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl = '车险' GROUP BY bmz,usage_name_b
UNION ALL
SELECT bmz,'汇总' ,SUM(bf) as bf,'个非车' 
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE class_name = '个客' AND xzdl <> '车险' GROUP BY bmz
UNION ALL
SELECT bmz,'汇总' ,SUM(bf) as bf,'团客整体' 
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl <> '车险' AND class_name = '团客' GROUP BY bmz
UNION ALL
SELECT bmz,lines ,SUM(bf) as bf,'团客分条线' 
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl <> '车险' AND class_name = '团客' GROUP BY bmz,lines
UNION ALL
SELECT bmz,'汇总' ,SUM(bf) as bf,'团客常规业务'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl <> '车险' AND class_name = '团客' AND IS_REGULAR = '1' GROUP BY bmz
UNION ALL
SELECT bmz,'汇总',SUM(bf) as bf,'三农整体'
FROM FR_FGS_KMH_2025_TMP_WANG_0010 WHERE xzdl = '农险' GROUP BY bmz
)
select a.*,to_char(sysdate-1,'yyyymmdd') workdate
from v_case a;


/*
 * 当季结果
*/
--入账清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_001';

insert into FR_FGS_KMH_2025_TMP_WANG_001 
SELECT a.policy_no,a.endorsement_no,SUM(a.rzbf_notax) rzbf_notax 
FROM etl_payment_due_t a
WHERE dd BETWEEN to_char(date_trunc('year',sysdate-1),'yyyymmdd') AND to_char(sysdate-1,'yyyymmdd')
GROUP BY a.policy_no,a.endorsement_no;

--起保清单
EXECUTE IMMEDIATE 'truncate table FR_FGS_KMH_2025_TMP_WANG_002';
insert into FR_FGS_KMH_2025_TMP_WANG_002 
select a.policy_no,a.endorsement_no,a.currency_code,to_char(a.inception_date,'YYYYMMDD') dd,
       sum(nvl(a.premium_amount,0)) premium_amount,sum(nvl(a.taxamount,0)) tax_amount
from ids_wuh.nonauto_premium_t a
WHERE a.inception_date BETWEEN date_trunc('year',sysdate-1)
AND  date_trunc('month',sysdate-1)+ INTERVAL '1' MONTH - INTERVAL '1' SECOND
and a.workdate <= to_char(sysdate-1,'yyyyMMdd')-------workdate小于当天
group by a.policy_no,a.endorsement_no,a.currency_code,to_char(a.inception_date,'YYYYMMDD')
union all
SELECT policy_no,endorsement_no,'01' currency_code,to_char(REQUESTED_DATE,'YYYYMMDD') dd,
       sum(nvl(premium_amount,0)) premium_amount,sum(nvl(tax_amount,0)) tax_amount
FROM etl_policy_request_issue_t WHERE xz = '车险' AND BUSINESS_DATE <= to_char(sysdate-1,'yyyyMMdd')
AND REQUESTED_DATE BETWEEN date_trunc('year',sysdate-1)
AND date_trunc('month',sysdate-1)+ INTERVAL '1' MONTH - INTERVAL '1' SECOND
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
```
