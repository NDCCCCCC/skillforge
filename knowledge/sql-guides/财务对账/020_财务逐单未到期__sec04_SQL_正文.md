---
title: 财务逐单未到期
source_idx: 020
section: SQL / 正文
---

## SQL / 正文

```sql
    CREATE TEMP TABLE temp_v_FR_FGS_CW_ZDWDQ2 AS
    SELECT a.保单号,a.险种代码,a.tjxzh_str,a.部门组名称,a.部门名称,a.评估时点,a.评估���点1,a.评估时点2,a.评估时点3,a.终保日期,
           a.调整后法定分公司再保后未到期,a.调整后法定分公司再保后保费不足,a.调整后二号分公司再保后未到期,a.调整后二号分公司再保后保费不足
          , (CASE WHEN tjxzh_str = '02' and fuel_type in ('燃油','燃油车','其他混合动力') THEN '车险-燃油车'		 
                  WHEN tjxzh_str = '02' THEN '车险-新能源车'		      
                  WHEN tjxzh_str IN ('01','03','04','05','06','07','08','09','10','12','13') and is_gxb = '0' THEN '非车非农'					
                  WHEN is_gxb = '1' THEN '个信保'		
                  WHEN tjxzh_str IN ('11','15','16','17','18','19','20','22','23','24') THEN '农险'
                  ELSE '非车非农' END) as 险种大类 
          , (CASE WHEN tjxzh_str = '01' THEN '企财险'
                  WHEN tjxzh_str = '02' THEN f_basic_product_name(a.险种代码)
                  WHEN tjxzh_str = '03' THEN '船舶险'  
                  WHEN tjxzh_str = '04' THEN '货运险'  
                  WHEN tjxzh_str = '05' THEN '工程险'  
                  WHEN tjxzh_str = '06' THEN '家财险'  
                  WHEN tjxzh_str = '07' THEN '保证险'  
                  WHEN tjxzh_str = '08' THEN '责任险'  
                  WHEN tjxzh_str = '09' THEN '特殊险'  
                  WHEN tjxzh_str = '10' THEN '信用险'  
                  WHEN tjxzh_str = '12' THEN '意外险'  
                  WHEN tjxzh_str = '21' THEN '意外险'  
                  WHEN tjxzh_str = '13' THEN '健康险'   
                  WHEN tjxzh_str = '11' THEN '农险'  
                  WHEN tjxzh_str = '15' THEN '农险'  
                  WHEN tjxzh_str = '16' THEN '农险'  
                  WHEN tjxzh_str = '17' THEN '农险'  
                  WHEN tjxzh_str = '18' THEN '农险'  
                  WHEN tjxzh_str = '19' THEN '农险'  
                  WHEN tjxzh_str = '20' THEN '农险'  
                  WHEN tjxzh_str = '22' THEN '农险' 
                  WHEN tjxzh_str = '23' THEN '农险' 
                  WHEN tjxzh_str = '24' THEN '农险' 
                  ELSE '其他险'  END) as 明细险种大类
          , (CASE WHEN 评估时点1 < 终保日期 THEN (终保日期-评估时点1)/(终保日期-评估时点)
                  ELSE 0 END) as 进展系数1
          , (CASE WHEN 评估时点2 < 终保日期 THEN (终保日期-评估时点2)/(终保日期-评估时点)
                  ELSE 0 END) as 进展系数2
          , (CASE WHEN 评估时点3 < 终保日期 THEN (终保日期-评估时点3)/(终保日期-评估时点)
                  ELSE 0 END) as 进展系数3				
          , (CASE WHEN 调整后法定分公司再保后未到期 = 0 THEN 1
                  ELSE 1 - 调整后二号分公司再保后未到期/调整后法定分公司再保后未到期 END) as 首日费用率								
    FROM temp_v_FR_FGS_CW_ZDWDQ1 a;

    -- 创建第三个临时表

    DROP TABLE IF EXISTS temp_v_FR_FGS_CW_ZDWDQ3;
    CREATE TEMP TABLE temp_v_FR_FGS_CW_ZDWDQ3 AS
    SELECT a.*
         , (调整后法定分公司再保后未到期*进展系数1) as 进展法定未到期金额1
         , (调整后法定分公司再保后未到期*进展系数2) as 进展法定未到期金额2
         , (调整后法定分公司再保后未到期*进展系数3) as 进展法定未到期金额3
    FROM temp_v_FR_FGS_CW_ZDWDQ2 a;

    -- 创建第四个临时表
    DROP TABLE IF EXISTS temp_v_FR_FGS_CW_ZDWDQ4;
```
