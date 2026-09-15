---
title: 高斯数据库存储过程和定时任务
source_idx: 006
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE OR REPLACE FUNCTION wuh_dw.pro_qdrb0_bt01_gfc_and_xny()
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$
DECLARE
    v_count INT;
BEGIN
/*
 * BT01实时签单报表：个非车和新能源实时签单保费
*/

    -- 新增逻辑：检查目标表是否存在，不存在则创建
    SELECT COUNT(*)
    INTO v_count
    FROM information_schema.tables
    WHERE table_schema = 'wuh_dw'
      AND table_name = 'pro_qdrbt0_bt01_gfc_and_xny';

    IF v_count = 0 THEN
        EXECUTE IMMEDIATE '
            CREATE TABLE wuh_dw.pro_qdrbt0_bt01_gfc_and_xny (
                department_group_code VARCHAR(50),
                department_group_name VARCHAR(100),
                department_code VARCHAR(50),
                department_name VARCHAR(100),
                xz VARCHAR(20),
                fueltype VARCHAR(50),
                customerdept VARCHAR(50),
                bf DECIMAL(15, 2)
            )
        ';
    END IF;

--更新BT01实时签单
EXECUTE IMMEDIATE 'TRUNCATE TABLE PRO_QDRBT0_BT01_GFC_AND_XNY';
INSERT INTO PRO_QDRBT0_BT01_GFC_AND_XNY
WITH case_data AS (
SELECT 
        a.policyno AS policy_no,
        a.departmentGroupCode,
        a.departmentCode,
        DECODE(a.autoFlg,'3','车险','7','农险','非车险')as xz,
        a.fueltype,
        a.customerdept,
        SUM(
            (CASE 
                WHEN a.autoflg = '3' AND NVL(a.endorsementno, '无') = '无' THEN NVL(b.totalPremiumChange, 0) - NVL(b.taxAmount, 0)
                WHEN a.autoflg = '3' AND NVL(a.endorsementno, '无') <> '无' THEN NVL(b.totalPremiumChange, 0) - NVL(b.PREMIUMCHANGETAX, 0)
                WHEN a.autoflg <> '3' AND NVL(a.endorsementno, '无') = '无' THEN NVL(b.BAKTOTALPREMIUM, 0) - NVL(b.TAXAMOUNT, 0)
                WHEN a.autoflg <> '3' AND NVL(a.endorsementno, '无') <> '无' THEN NVL(b.TOTALPREMIUMCHANGE, 0) - NVL(b.PREMIUMCHANGETAX, 0)
                ELSE 0 
            END) * cc.rate / 100
        ) AS bf
    FROM 
        ids_wuh.msg_dwa_policy_summarys a
    JOIN 
        ids_wuh.msg_dwa_policy_summarys_tot_prem b 
        ON a.data_id = b.data_parent_id
    JOIN 
        ids_wuh.exrate_month_t cc 
        ON b.origcurrency = cc.bzh 
        AND cc.theyear || LPAD(cc.themonth, 2, '0') = TO_CHAR(SYSDATE, 'yyyymm')
    WHERE 
        a.signatureDate LIKE TO_CHAR(SYSDATE, 'yyyy-mm-dd') || '%'
        and a.autoflg != '7'
    GROUP BY 
        a.policyno, a.departmentGroupCode, a.departmentCode, a.autoFlg, a.fueltype, a.customerdept
),
department_info AS (

    SELECT 
        a.departmentGroupCode,
        a.departmentCode as department_code,
        f_basic_tjbmzdm(a.departmentGroupCode, a.departmentCode) AS department_group_code,
        f_basic_tjbmzmc(a.departmentGroupCode, a.departmentCode) AS department_group_name,
        f_basic_tjbmmc(a.departmentGroupCode, a.departmentCode) AS department_name,
        a.xz,
        a,fueltype,
        a.customerdept,
        a.bf
    FROM 
        case_data a
)   
SELECT department_group_code,department_group_name,department_code,department_name,xz,fueltype,customerdept,SUM(bf) bf FROM department_info GROUP BY department_group_code,department_group_name,department_code,department_name,xz,fueltype,customerdept;

END$$
/SELECT DBMS_JOB.SUBMIT(
    'CALL wuh_dw.pro_qdrb0_bt01_gfc_and_xny();',
    sysdate,
    'SYSDATE + 10/1440'
); dbms_job.isubmit(
        101,
        'CALL wuh_application.fgs_fg_fxq_20260414();',
        TO_DATE('2026-01-05 00:00:00', 'YYYY-MM-DD HH24:MI:SS') - INTERVAL '8 hour',
        'ADD_MONTHS(sysdate, 3)'
    );


SELECT * FROM pg_jobs where what like '%BT01%'

CALL DBMS_JOB.REMOVE(46)
call DBMS_JOB.WHAT(41, 'CALL WUH_DW.PRO_QDRB0_BT01_GFC_AND_XNY();');
CALL DBMS_JOB.INTERVAL(41, 'INTERVAL ''10'' MINUTE');
```
