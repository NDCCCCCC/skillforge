---
title: 财务逐单未到期
source_idx: 020
section: SQL / 正文
---

## SQL / 正文

```sql
    CREATE TEMP TABLE temp_v_FR_FGS_CW_ZDWDQ4 AS
    SELECT a.*
         , (进展法定未到期金额1*(1-首日费用率)+调整后二号分公司再保后保费不足) as 进展二号未到期金额1
         , (进展法定未到期金额2*(1-首日费用率)+调整后二号分公司再保后保费不足) as 进展二号未到期金额2
         , (进展法定未到期金额3*(1-首日费用率)+调整后二号分公司再保后保费不足) as 进展二号���到期金额3
    FROM temp_v_FR_FGS_CW_ZDWDQ3 a;

    -- 结果表1：徐婷：逐单未到期 分险种
    DELETE FROM fr_fgs_zdwdq_result_xz WHERE 评估时点1 = estimate_1;
    
    INSERT INTO fr_fgs_zdwdq_result_xz 
    SELECT 评估时点1,评估时点2,评估时点3
           , (case when 险种大类 in ('车险-燃油车','车险-新能源车') then '车险' else 险种大类 end) as 险种大类名称
           , 明细险种大类 
           , SUM(进展法定未到期金额1) as 进展法定未到期金额1
           , SUM(进展二号未到期金额1) as 进展二号未到期金额1
           , SUM(进展法定未到期金额2) as 进展法定未到期金额2
           , SUM(进展二号未到期金额2) as 进展二号未到期金额2					 
           , SUM(进展法定未到期金额3) as 进展法定未到期金额3
           , SUM(进展二号未到期金额3) as 进展二号未到期金额3
           , '湖北分公司' as 部门组名称		 
    FROM temp_v_FR_FGS_CW_ZDWDQ4 
    GROUP BY 评估时点1,评估时点2,评估时点3,险种大类名称,明细险种大类
    UNION ALL

    SELECT 评估时点1,评估时点2,评估时点3
           , (case when 险种大类 in ('车险-燃油车','车险-新能源车') then '车险' else 险种大类 end) as 险种大类名称
           , 明细险种大类
           , SUM(进展法定未到期金额1) as 进展法定未到期金额1
           , SUM(进展二号未到期金额1) as 进展二号未到期金额1
           , SUM(进展法定未到期金额2) as 进展法定未到期金额2
           , SUM(进展二号未到期金额2) as 进展二号未到期金额2					 
           , SUM(进展法定未到期金额3) as 进展法定未到期金额3
           , SUM(进展二号未到期金额3) as 进展二号未到期金额3
           , 部门组名称					 
    FROM temp_v_FR_FGS_CW_ZDWDQ4 
    GROUP BY 评估时点1,评估时点2,评估时点3,险种大类名称,明细险种大类,部门组名称;

    -- 结果表2：罗瑜：法定未到期 分机构和险种
    DELETE FROM fr_fgs_fdwdq_result_jgxz WHERE 评估时点 = estimate;
```
