---
title: 代理业务部车商费用
source_idx: 015
section: SQL / 正文
---

## SQL / 正文

```sql
```

签单时间，计提手续费

```sql
WITH v_case_3 AS ( 
    SELECT 
        a.policyno AS 保单号,           -- 注意：这里定义了别名“保单号”
        SUM(NVL(a.extractamount, 0)) AS 计提手续费,
        MAX(a.extractdate) AS 手续费计提时间
    FROM "ids_wuh".cmextractcommission_t a
    WHERE a.commissiontype IN ('0','1')
      AND a.extractdate BETWEEN TO_DATE('20250101 00:00:00','yyyymmdd hh24:mi:ss')
                            AND TO_DATE('20251231 23:59:59','yyyymmdd hh24:mi:ss')
    GROUP BY a.policyno
)
SELECT 
    a.department_group_code AS 部门组代码,
    a.department_group_name AS 部门组名称,
    COALESCE(a.department_code, '') AS 部门代码,
    COALESCE(a.department_name, '') AS 部门名称,
    a.product_code AS 险种代码,
    a.product_name AS 险种名称,
    a.agent_code AS 代理点代码,
    COALESCE(a.dldmc, '') AS 代理点名称,
    a.selling_channel_type AS 展业方式代码,
    a.selling_channel_name AS 展业方式名称,
    COALESCE(a.selling_shop_code, '') AS 车商代码,
    COALESCE(a.selling_shop_name, '') AS 车商名称,
    COUNT(a.policy_no) AS 保单数,
    SUM(COALESCE(a.premium_amount, 0) - COALESCE(a.tax_amount, 0)) AS 保费,
    SUM((COALESCE(a.premium_amount, 0) - COALESCE(a.tax_amount, 0)) * COALESCE(c.COMMISSIONRATE, 0)) AS 佣金,
    SUM(COALESCE(f."计提手续费", 0)) AS 计提手续费总和  
FROM wuh_dw.etl_policy a 
LEFT JOIN (

    SELECT policyno, MAX(COMMISSIONRATE) AS COMMISSIONRATE
    FROM ids_wuh.CSCMAUTPOLICYFEE
    GROUP BY policyno
) c ON a.policy_no = c.policyno
LEFT JOIN v_case_3 f ON a.policy_no = f."保单号"        -- 修正2：改为引用别名“保单号”
WHERE a.issue_date BETWEEN TO_DATE('2025-01-01 00:00:00', 'yyyy-mm-dd hh24:mi:ss')
                          AND TO_DATE('2025-12-31 23:59:59', 'yyyy-mm-dd hh24:mi:ss')
GROUP BY 
    a.department_group_code,
    a.department_group_name,
    a.department_code,
    a.department_name,
    a.product_code,
    a.product_name,
    a.agent_code,
    a.dldmc,
    a.selling_channel_type,
    a.selling_channel_name,
    a.selling_shop_code,
    a.selling_shop_name
UNION ALL
SELECT 
    a.department_group_code AS 部门组代码,
    a.department_group_name AS 部门组名称,
    COALESCE(a.department_code, '') AS 部门代码,
    COALESCE(a.department_name, '') AS 部门名称,
    a.product_code AS 险种代码,
    a.product_name AS 险种名称,
    a.agent_code AS 代理点代码,
    COALESCE(a.agent_name, '') AS 代理点名称,
    a.selling_channel_type AS 展业方式代码,
    a.selling_channel_name AS 展业方式名称,
    '' AS 车商代码,
    '' AS 车商名称,
    COUNT(a.policy_no) AS 保单数,
    SUM(COALESCE(a.premium_amount, 0) - COALESCE(a.tax_amount, 0)) AS 保费,
    SUM((COALESCE(a.premium_amount, 0) - COALESCE(a.tax_amount, 0)) * COALESCE(c.COMMISSIONRATE, 0)) AS 佣金,
    SUM(COALESCE(f."计提手续费", 0)) AS 计提手续费总和  
FROM wuh_dw.etl_nonauto_policy_t a 
LEFT JOIN (

    SELECT policyno, MAX(COMMISSIONRATE) AS COMMISSIONRATE
    FROM ids_wuh.CSCMFAUTPOLICYFEE
    GROUP BY policyno
) c ON a.policy_no = c.policyno
LEFT JOIN v_case_3 f ON a.policy_no = f."保单号"  
WHERE a.issue_date BETWEEN TO_DATE('2025-01-01 00:00:00', 'yyyy-mm-dd hh24:mi:ss')
    AND TO_DATE('2025-12-31 23:59:59', 'yyyy-mm-dd hh24:mi:ss')
GROUP BY 
    a.department_group_code,
    a.department_group_name,
    a.department_code,
    a.department_name,
    a.product_code,
    a.product_name,
    a.agent_code,
    a.agent_name,
    a.selling_channel_type,
    a.selling_channel_name;
```
