---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
--重新计算含个信保门店的中支汇总的当日、当月、当年、往月、往年数据及同比
UPDATE pro_qdrbt0_new_case_result a
SET drbf = drbf + (SELECT nvl(SUM(drbf),0) FROM pro_qdrbt0_new_case_result b
            WHERE a.department_group_code = b.department_group_code
            AND b.department_code IN (select department_code from fgs_fc_gxb)
            AND a.product_code = b.product_code),
    dybf = dybf + (SELECT nvl(SUM(dybf),0) FROM pro_qdrbt0_new_case_result b
            WHERE a.department_group_code = b.department_group_code
            AND b.department_code IN (select department_code from fgs_fc_gxb)
            AND a.product_code = b.product_code),
    dnbf = dnbf + (SELECT nvl(SUM(dnbf),0) FROM pro_qdrbt0_new_case_result b
            WHERE a.department_group_code = b.department_group_code
            AND b.department_code IN (select department_code from fgs_fc_gxb)
            AND a.product_code = b.product_code),
    wdybf = wdybf + (SELECT nvl(SUM(wdybf),0) FROM pro_qdrbt0_new_case_result b
            WHERE a.department_group_code = b.department_group_code
            AND b.department_code IN (select department_code from fgs_fc_gxb)
            AND a.product_code = b.product_code),
    wnbf = wnbf + (SELECT nvl(SUM(wnbf),0) FROM pro_qdrbt0_new_case_result b
            WHERE a.department_group_code = b.department_group_code
            AND b.department_code IN (select department_code from fgs_fc_gxb)
            AND a.product_code = b.product_code)
WHERE department_group_code=department_code
AND department_group_name NOT IN('湖北分公司','业务','重客','个信保')
AND product_code IN ('汇总','非车险');



--重新计算同比

UPDATE pro_qdrbt0_new_case_result a
SET a.dytb = (case when a.dybf=0 and (a.dybf-a.wdybf)=0 then 0
                   when a.dybf=0 and (a.dybf-a.wdybf)>0 then 1
                   when a.dybf=0 and (a.dybf-a.wdybf)<0 then -1
                   WHEN a.wdybf = 0 THEN 1
              else (a.dybf-a.wdybf)/ABS(a.wdybf) END),
    a.dntb = (case when a.dnbf=0 and (a.dnbf-a.wnbf)=0 then 0
                   when a.dnbf=0 and (a.dnbf-a.wnbf)>0 then 1
                   when a.dnbf=0 and (a.dnbf-a.wnbf)<0 then -1
                   WHEN a.wnbf = 0 THEN 1
              else (a.dnbf-a.wnbf)/ABS(a.wnbf) END);


EXCEPTION
  WHEN NO_DATA_FOUND THEN
    DBMS_OUTPUT.PUT_LINE('erro');
end$$
/
```
