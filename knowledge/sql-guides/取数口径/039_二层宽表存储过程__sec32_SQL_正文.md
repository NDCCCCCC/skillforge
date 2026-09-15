---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  
	

EXCEPTION
  WHEN NO_DATA_FOUND THEN
    DBMS_OUTPUT.PUT_LINE('error');
end$$
/实时签单存储过程
CREATE OR REPLACE FUNCTION wuh_dw.p_pro_qdrbt0_new(etl_batch_date character varying)
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$ DECLARE 

begin
  execute immediate 'truncate table v_pro_qdrbt0_new_case_dr ';
  execute immediate 'truncate table v_pro_qdrbt0_new_case_dy ';
  execute immediate 'truncate table v_pro_qdrbt0_new_case_dn ';
  execute immediate 'truncate table v_pro_qdrbt0_new_case_dy_wn ';
  execute immediate 'truncate table v_pro_qdrbt0_new_case_dn_wn ';
	execute immediate 'truncate table v_pro_qdrbt0_new_case_result0 ';
	execute immediate 'truncate table v_pro_qdrbt0_new_case_result1 ';
	execute immediate 'truncate table pro_qdrbt0_new_case_result ';

--当日
INSERT INTO v_pro_qdrbt0_new_case_dr 
SELECT department_group_code
       , department_group_name
       , department_code
       , department_name
       , xz AS product_code
       , SUM(bf) AS drbf
FROM etl_policy_request_issue_t WHERE business_date = etl_batch_date
GROUP BY department_group_code,department_group_name,department_code,department_name,xz;

--当月
INSERT INTO v_pro_qdrbt0_new_case_dy 
SELECT department_group_code
       , department_group_name
       , department_code
       , department_name
       , xz AS product_code
       , SUM(bf) AS dybf
FROM etl_policy_request_issue_t WHERE nf = SUBSTR(etl_batch_date,0,4) AND yf = SUBSTR(etl_batch_date,5,2) AND dd <= SUBSTR(etl_batch_date,7,2)
GROUP BY department_group_code,department_group_name,department_code,department_name,xz;

--当年
INSERT INTO v_pro_qdrbt0_new_case_dn 
SELECT department_group_code
       , department_group_name
       , department_code
       , department_name
       , xz AS product_code
       , SUM(bf) AS dnbf
FROM etl_policy_request_issue_t WHERE nf = SUBSTR(etl_batch_date,0,4) AND business_date <= etl_batch_date
GROUP BY department_group_code,department_group_name,department_code,department_name,xz;

--往年当年

INSERT INTO v_pro_qdrbt0_new_case_dn_wn 
SELECT department_group_code
       , department_group_name
       , department_code
       , department_name
       , xz AS product_code
       , SUM(bf) AS wnbf
FROM etl_policy_request_issue_t WHERE nf = SUBSTR(etl_batch_date,0,4)-1 AND business_date <= to_char(add_months(to_date(etl_batch_date,'yyyyMMdd'),-12),'yyyyMMdd')
--AND workdate_premium <= to_char(add_months(to_date(etl_batch_date,'yyyyMMdd'),-12),'yyyyMMdd') 
GROUP BY department_group_code,department_group_name,department_code,department_name,xz;

--往年当月=往年当日的当年保费-往年上月末的当年保费(因为卡了workdate，直接卡一段时间数据对不上，只能取两个时间点的数据)  2024.3.21：不卡workdate_premium
--如果是1月的数据，那么往年当月=往年当年
IF SUBSTR(etl_batch_date,5,2)-1 = '0' 
THEN 
   INSERT INTO v_pro_qdrbt0_new_case_dy_wn  SELECT * FROM v_pro_qdrbt0_new_case_dn_wn;
```
