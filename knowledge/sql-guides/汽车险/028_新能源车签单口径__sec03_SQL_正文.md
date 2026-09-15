---
title: 新能源车签单口径
source_idx: 028
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



WITH current_date_cte AS (
    SELECT DATE('2025-01-09') AS calc_date
),
-- 将原始日期字段转换为标准日期，并提取年、月维度
date_converted AS (
    SELECT 
        *,
        -- 将数字格式的dd转换为日期，例如 20250106 -> 2025-01-06
        -- 使用CAST将dd转换为VARCHAR类型，然后使用TO_DATE转换为日期
        TO_DATE(CAST(dd AS VARCHAR), 'yyyyMMdd') AS sign_date,
        EXTRACT(YEAR FROM TO_DATE(CAST(dd AS VARCHAR), 'yyyyMMdd')) AS sign_year,
        EXTRACT(MONTH FROM TO_DATE(CAST(dd AS VARCHAR), 'yyyyMMdd')) AS sign_month,
        EXTRACT(DAY FROM TO_DATE(CAST(dd AS VARCHAR), 'yyyyMMdd')) AS sign_day
    FROM wuh_dw.FR_FGS_signature_005
    WHERE bf IS NOT NULL AND dd IS NOT NULL -- 确保关键字段不为空
      -- 过滤掉运营服务部和个信保
      AND department_group_name NOT LIKE '%运营服务本部%'
      AND department_group_name NOT LIKE '%个信保%'
),
-- 创建分公司本部映射表（根据您的实际业务规则调整）
department_mapping AS (
    SELECT 
        department_group_name,
        -- 提取分公司本部名称：假设部门名称中包含"中心支公司"或"支公司"的为分公司本部
        CASE 
            WHEN department_group_name LIKE '%中心支公司%' THEN 
                -- 提取中心支公司名称
                SUBSTRING(department_group_name FROM 1 FOR POSITION('中心支公司' IN department_group_name) + 4)
            WHEN department_group_name LIKE '%支公司%' THEN 
                -- 提取支公司名称
                SUBSTRING(department_group_name FROM 1 FOR POSITION('支公司' IN department_group_name) + 2)
            ELSE department_group_name -- 如果没有匹配，保持原样
        END AS branch_headquarters
    FROM date_converted
    GROUP BY department_group_name
),
-- 计算当日保费，按分公司本部分组
daily_premium AS (

    SELECT 
        dm.branch_headquarters,
        SUM(d.bf) AS daily_premium
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    CROSS JOIN current_date_cte c
    WHERE d.sign_date = c.calc_date
    GROUP BY dm.branch_headquarters
),
-- 计算当月保费 (截至计算日所在月份)，按分公司本部分组
monthly_premium_current AS (
    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        d.sign_month,
        SUM(d.bf) AS monthly_premium
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    CROSS JOIN current_date_cte c
    WHERE d.sign_year = EXTRACT(YEAR FROM c.calc_date)
      AND d.sign_month = EXTRACT(MONTH FROM c.calc_date)
    GROUP BY dm.branch_headquarters, d.sign_year, d.sign_month
),
-- 计算去年同月保费，用于同比，按分公司本部分组
monthly_premium_last_year AS (
    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        d.sign_month,
        SUM(d.bf) AS monthly_premium_ly
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    CROSS JOIN current_date_cte c
    WHERE d.sign_year = EXTRACT(YEAR FROM c.calc_date) - 1
      AND d.sign_month = EXTRACT(MONTH FROM c.calc_date)
    GROUP BY dm.branch_headquarters, d.sign_year, d.sign_month
),
-- 计算当年累计保费，按分公司本部分组
yearly_premium_current AS (

    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        SUM(d.bf) AS yearly_premium
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    CROSS JOIN current_date_cte c
    WHERE d.sign_year = EXTRACT(YEAR FROM c.calc_date)
    GROUP BY dm.branch_headquarters, d.sign_year
),
-- 计算去年全年保费，用于同比，按分公司本部分组
yearly_premium_last_year AS (
    SELECT 
        dm.branch_headquarters,
        d.sign_year,
```
