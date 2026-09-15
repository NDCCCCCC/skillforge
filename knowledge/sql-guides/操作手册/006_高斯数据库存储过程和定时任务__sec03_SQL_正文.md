---
title: 高斯数据库存储过程和定时任务
source_idx: 006
section: SQL / 正文
---

## SQL / 正文

```sql
-- 假设当前计算日期为 '2025-01-09'，您可以通过变量或函数动态获取
WITH current_date_cte AS (
    SELECT DATE('${yearmonth}') AS calc_date
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
      and fuel_type = '新能源车'
      -- 过滤掉运营服务部和个信保
      AND department_group_name NOT LIKE '%运营服务本部%'
      AND department_group_name NOT LIKE '%个信保%'
),
-- 创建分公司本部映射表（根据您的实际业务规则调整）
department_mapping AS (
     SELECT 
        department_group_name,
        -- 提取分公司本部名称：假设部门名称中包含"中心支公司"或"支��司"的为分公司本部
        CASE 
            WHEN department_group_name = '业务' THEN 
                '分公司本部' -- 提取中心支公司名称
            when department_group_name like '%支公司%' then
              REPLACE(REPLACE(department_group_name, '中心支公司', ''), '支公司', '')
            ELSE department_group_name -- 如果没有匹配，保持原样
        END AS branch_headquarters
    FROM date_converted
    GROUP BY department_group_name
),
ssbf AS (

select department_group_name,sum(bf) as bf from "pro_qdrbt0_bt01_gfc_and_xny" where fueltype in ('1','3','4') and xz = '车险' group by department_group_name
),
-- 计算当日保费，按分公司本部分组
daily_premium AS (
    SELECT 
        dm.branch_headquarters,
        SUM(d.bf)/10000 AS daily_premium
--    FROM date_converted d
    from ssbf d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    GROUP BY dm.branch_headquarters
),
-- 计算当月保费 (截至计算日所在月份)，按分公司本部分组
monthly_premium_current AS (
    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        d.sign_month,
        (SUM(d.bf)+ss.daily_premium)/10000 AS monthly_premium
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    left join daily_premium ss on ss.branch_headquarters=dm.branch_headquarters
    CROSS JOIN current_date_cte c
    WHERE d.sign_year = EXTRACT(YEAR FROM c.calc_date)
      AND d.sign_month = EXTRACT(MONTH FROM c.calc_date)
    GROUP BY dm.branch_headquarters, d.sign_year, d.sign_month, ss.daily_premium
),
-- 计算去年同月保费，用于同比，按分公司本部分组
monthly_premium_last_year AS (
    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        d.sign_month,
        SUM(d.bf)/10000 AS monthly_premium_ly
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
    CROSS JOIN current_date_cte c
    WHERE d.sign_year = EXTRACT(YEAR FROM c.calc_date) - 1
      AND d.sign_month = EXTRACT(MONTH FROM c.calc_date)
      and cast(d.sign_day as int) <= cast(EXTRACT(DAY FROM c.calc_date) as int)
    GROUP BY dm.branch_headquarters, d.sign_year, d.sign_month
),
-- 计算当年累计保费，按分公司本部分组
yearly_premium_current AS (

    SELECT 
        dm.branch_headquarters,
        d.sign_year,
        (SUM(d.bf)+ss.daily_premium)/10000 AS yearly_premium
    FROM date_converted d
    JOIN department_mapping dm ON d.department_group_name = dm.department_group_name
```
