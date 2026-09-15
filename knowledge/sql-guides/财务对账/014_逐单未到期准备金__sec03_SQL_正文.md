---
title: 逐单未到期准备金
source_idx: 014
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



DROP TABLE TMP_WANG_0011 PURGE; 
CREATE TABLE TMP_WANG_0011 NOLOGGING AS 
SELECT policy_no 保单号
       , product_code 险种代码
    , (SELECT xzdl FROM FR_FGS_ZDWDQ_PRODUCTCODE b WHERE a.financial_product_code = b.productcode) 险种大类
    , f_basic_tjbmzmc(DEPTGROUP_CODE,DEPT_CODE) 部门组名称
       , f_basic_tjbmmc(DEPTGROUP_CODE,DEPT_CODE) 部门名称
       , to_date(ESTIMATE_DATE,'yyyyMMdd') 评估时点
       , last_day(add_months(to_date(ESTIMATE_DATE,'yyyyMMdd'),1)) 评估时点1
       , to_date(PLANNED_END_DATE,'yyyyMMdd') 终保日期
    , ADJUST_AFTER_BRANCHREIN_UPR 调整后法定分公司再保后未到期
       , ADJUST_LSBPI_UPR 调整后法定分公司再保后保费不足
    , ADJUST_AFTER_BRANCHREIN_UPR2	调整后二号分公司再保后未到期
    , ADJUST_LSBPI_UPR2 调整后二号分公司再保后保费不足
FROM op_uprcollection_all_t_adj a
WHERE ESTIMATE_DATE = '20210331';


DROP TABLE TMP_WANG_0022 PURGE; 
CREATE TABLE TMP_WANG_0022 NOLOGGING AS 
SELECT a.*
      , ( CASE WHEN 评估时点1 < 终保日期 THEN (终保日期-评估时点1)/(终保日期-评估时点)
       ELSE 0 END ) 进展系数
   , ( CASE WHEN 调整后法定分公司再保后未到期 = 0 THEN 1
       ELSE 1 - 调整后二号分公司再保���未到期/调整后法定分公司再保后未到期 END) 首日费用率 
FROM TMP_WANG_0011 a; 


DROP TABLE TMP_WANG_0033 PURGE; 
CREATE TABLE TMP_WANG_0033 NOLOGGING AS    

SELECT a.*
       , (调整后法定分公司再保后未到期*进展系数) 进展法定未到期金额
FROM TMP_WANG_0022 a;    


DROP TABLE TMP_WANG_0044 PURGE;
CREATE TABLE TMP_WANG_0044 NOLOGGING AS  
SELECT a.*
       , (进展法定未到期金额*(1-首日费用率)+调整后二号分公司再保后保费不足) 进展二号未到期金额
FROM TMP_WANG_0033 a;
  

DROP TABLE TMP_WANG_0055 PURGE; 
CREATE TABLE TMP_WANG_0055 NOLOGGING AS 
SELECT 评估时点,险种大类,SUM(进展法定未到期金额) 进展法定未到期金额,SUM(进展二号未到期金额) 进展二号未到期金额 FROM TMP_WANG_0044 GROUP BY 评估时点,险种大类;

--徐婷：报表未到期 分险种
INSERT INTO fr_fgs_zdwdq_result_xz NOLOGGING SELECT * FROM TMP_WANG_0055; 
COMMIT;


/*
--逐单未到期 政保健康险
SELECT DISTINCT(险种代码) FROM TMP_WANG_0044
SELECT * FROM TMP_WANG_0044 WHERE 险种代码 IN ('22011500',
'22012000',
'22019900',
'22091500',
'22C39900',
'22652400',
'22979900',
'22D89900',
'23Z39900',
'22709900',
'22719900',
'22F99900',
'1106A002',
'1106A001',
'22J79800',
'22J89800',
'22G29900',
'1106A402',
'1106A401',
'22J49800',
'22J59800',
'22J69800',
'1106B900',
'22H69900')
*/

--罗瑜：法定未到期 分机构和险种 调整后法定分公司再保后未到期+调整后法定分公司再保后保费不足
INSERT INTO fr_fgs_fdwdq_result_jgxz  NOLOGGING 
SELECT * FROM (
SELECT 评估时点
       , 部门组名称
    , 部门名称
    , '整体' 险种大类
       , SUM(调整后法定分公司再保后未到期+调整后法定分公司再保后保费不足) "调整后法定分公司再保后(未到期+保费不足)" 
FROM TMP_WANG_0011 a
GROUP BY 评估时点,部门组名称,部门名称,险种大类
union ALL

SELECT 评估时点
       , 部门组名称
    , 部门名称
    , (CASE WHEN 险种大类 IS NULL THEN '险种为空' ELSE 险种大类 END)险种大类
       , SUM(调整后法定分公司再保后未到期+调整后法定分公司再保后保费不足) "调整后法定分公司再保后(未到期+保费不足)" 
FROM TMP_WANG_0011 a
GROUP BY 评估时点,部门组名称,部门名称,险种大类
)
PIVOT
(SUM("调整后法定分公司再保后(未到期+保费不足)") FOR 险种大类 IN ('整体' AS 整体,'车险' AS 车险,'非车非农' AS 非车非农,'农险' AS 农险,'车意险险种大类' AS 车意险险种大类));
COMMIT;
```
