---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
--加入湖北分公司汇总及中支汇总数据
INSERT INTO v_pro_qdrbt0_new_case_result1 
SELECT * FROM v_pro_qdrbt0_new_case_result0
UNION ALL
--湖北各险种汇总数据
SELECT '4020100' department_group_code
       , '湖北分公司' department_group_name
       , '4020100' department_code
       , '湖北分公司' department_name
       , a.product_code
       , SUM(drbf) drbf
       , SUM(dybf) dybf
       , SUM(dnbf) dnbf
       , SUM(wdybf) wdybf
       , SUM(wnbf) wnbf
FROM v_pro_qdrbt0_new_case_result0 a GROUP BY product_code
UNION ALL
--湖北汇总数据
SELECT '4020100' department_group_code
       , '湖北分公司' department_group_name
       , '4020100' department_code
       , '湖北分公司' department_name
       , '汇总' product_code
       , SUM(drbf) drbf
       , SUM(dybf) dybf
       , SUM(dnbf) dnbf
       , SUM(wdybf) wdybf
       , SUM(wnbf) wnbf
FROM v_pro_qdrbt0_new_case_result0 a
UNION ALL
--中支各险种汇总数据
SELECT a.department_group_code
       , a.department_group_name
       , a.department_group_code AS department_code
       , a.department_group_name AS department_name
       , a.product_code
       , SUM(drbf) drbf
       , SUM(dybf) dybf
       , SUM(dnbf) dnbf
       , SUM(wdybf) wdybf
       , SUM(wnbf) wnbf
FROM v_pro_qdrbt0_new_case_result0 a
GROUP BY department_group_code,department_group_name,product_code
UNION ALL
--中支汇总数据

SELECT a.department_group_code
       , a.department_group_name
       , a.department_group_code AS department_code
       , a.department_group_name AS department_name
       , '汇总' product_code
       , SUM(drbf) drbf
       , SUM(dybf) dybf
       , SUM(dnbf) dnbf
       , SUM(wdybf) wdybf
       , SUM(wnbf) wnbf
FROM v_pro_qdrbt0_new_case_result0 a
GROUP BY department_group_code,department_group_name
--2021.3.3加入部门汇总数据
UNION ALL
SELECT a.department_group_code,
       a.department_group_name,          
       a.department_code,
       a.department_name,
       '汇总' product_code,
       SUM(drbf) drbf,
       SUM(dybf) dybf,
       SUM(dnbf) dnbf,
       SUM(wdybf) wdybf,
       SUM(wnbf) wnbf
FROM v_pro_qdrbt0_new_case_result0 a
GROUP BY a.department_group_code,
         a.department_group_name,
         a.department_code,
         a.department_name;

--往结果表里插入最终数据
INSERT INTO pro_qdrbt0_new_case_result 
SELECT a.*
       , (case when wdybf=0 and (dybf-wdybf)=0 then 0
               when wdybf=0 and (dybf-wdybf)>0 then 1
               when wdybf=0 and (dybf-wdybf)<0 then -1
               else (dybf-wdybf)/ABS(wdybf) end) dytb
       , (case when wnbf=0 and (dnbf-wnbf)=0 then 0
               when wnbf=0 and (dnbf-wnbf)>0 then 1
               when wnbf=0 and (dnbf-wnbf)<0 then -1
               else (dnbf-wnbf)/ABS(wnbf)  end) dntb
			 , etl_batch_date AS workdate				 
FROM v_pro_qdrbt0_new_case_result1 a;

UPDATE pro_qdrbt0_new_case_result
SET department_group_name = '本部'
WHERE department_group_name = '分公司本部';

DELETE FROM pro_qdrbt0_new_case_result WHERE department_group_code = '423333';



/**
 * 大保单提醒 abs>=100W
*/
perform p_pro_qdrbt0_new_attention(etl_batch_date);


/**
 * 2021.3.3将个信保门店瓜分到每个中支,此存储过程只会更新个信保数据（只需更新汇总和非车险，重客、业务、湖北分公司这些都不用改）
 */
--更新个信保的部门组代码及名称
UPDATE pro_qdrbt0_new_case_result a
SET department_group_code = (SELECT b.department_group_code FROM fgs_fc_gxb b WHERE a.department_code = b.department_code)
WHERE department_code IN (select department_code from fgs_fc_gxb);

UPDATE pro_qdrbt0_new_case_result a
SET department_group_name = (SELECT b.department_group_name FROM fr_bmz_t b WHERE a.department_group_code = b.department_group_code)
WHERE department_code IN (select department_code from fgs_fc_gxb);
```
