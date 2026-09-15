---
title: 三责险，车损险保费保额
source_idx: 011
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



select 
--    a.issue_date 签单日期,
a.signature_date 签发日期,
a.inception_date 起保日期,
a.planned_end_date 终止日期,
a.plate_number 车牌号,
a.vin,a.product_name 险种名称,
a.policy_no 保单号,
a.tb_name 投保人名称,
a.vehicle_owner 车主名称,
a.usage_name 使用性质,
a.vehicle_brand 品牌,
a.premium_amount 保费收入,
a.INDEPENDENTPRICERATE 自主定价系数, 
a.NONCLAIMDISCOUNTRATE NCD系数  ,
max(b.vehicle_brand) 车系,
max(b.first_registration_date) 登记日期,
SUM(CASE WHEN c.product_code IN ('11025210','11025310','11027010','11025010','11025110') THEN c.insured_amount ELSE 0 END) 车损险保额
   , SUM(CASE WHEN c.product_code IN ('11025250','11025350','11027050','11025050','11025150') THEN c.insured_amount ELSE 0 END) 三责险保额
     , SUM(CASE WHEN c.product_code IN ('11025251','11025252','11025351','11025352','11027056','11027057','11025051','11025052','11025151','11025152') THEN c.insured_amount ELSE 0 END) 车上人员保额
   , SUM(CASE WHEN c.product_code IN ('11025253','11025353','11027058','11025053','11025153') THEN c.insured_amount ELSE 0 END) 司乘人员保额
     , SUM(CASE WHEN c.product_code IN ('11025210','11025310','11027010','11025010','11025110') THEN c.premium_amount ELSE 0 END) 车损险保费
   , SUM(CASE WHEN c.product_code IN ('11025250','11025350','11027050','11025050','11025150') THEN c.premium_amount ELSE 0 END) 三责险保费
   , SUM(CASE WHEN c.product_code IN ('11025251','11025252','11025351','11025352','11027056','11027057','11025051','11025052','11025151','11025152') THEN c.premium_amount ELSE 0 END) 车上人员保费
  , SUM(CASE WHEN c.product_code IN ('11025253','11025353','11027058','11025053','11025153') THEN c.premium_amount ELSE 0 END) 司乘人员保费
from wuh_dw.etl_policy a
left join CUSTOMER_VEHICLE b on a.policy_no = b.sy_policy_no
left join ids_wuh.AUTO_COVERAGE_COMPONENT_T c on a.policy_no = c.policy_no
WHERE a.issue_date 
BETWEEN to_date('2026-01-01 00:00:00', 'YYYY-MM-DD HH24:MI:SS') 
    AND to_date('2026-04-22 23:59:59', 'YYYY-MM-DD HH24:MI:SS')
and blanket_customer_code like '%DST00%'
and 
group by a.signature_date,
a.inception_date,
a.planned_end_date,
a.plate_number,
a.vin,a.product_name,
a.policy_no,
a.tb_name,
a.vehicle_owner,
a.usage_name,
a.vehicle_brand,
a.premium_amount,
a.INDEPENDENTPRICERATE, 
a.NONCLAIMDISCOUNTRATE2select 
--    a.issue_date 签单日期,
a.signature_date 签发日期,
a.inception_date 起保日期,
a.planned_end_date 终止日期,
a.plate_number 车牌号,
a.vin,a.product_name 险种名称,
a.policy_no 保单号,
a.tb_name 投保人名称,
a.vehicle_owner 车主名称,
a.usage_name 使用性质,
a.vehicle_brand 品牌,
a.premium_amount 保费收入,
a.INDEPENDENTPRICERATE 自主定价系数, 
a.NONCLAIMDISCOUNTRATE NCD系数  ,
max(b.vehicle_brand) 车系,
max(b.first_registration_date) 登记日期,
SUM(CASE WHEN c.product_code IN ('11025210','11025310','11027010','11025010','11025110') THEN c.insured_amount ELSE 0 END) 车损险保额
   , SUM(CASE WHEN c.product_code IN ('11025250','11025350','11027050','11025050','11025150') THEN c.insured_amount ELSE 0 END) 三责险保额
     , SUM(CASE WHEN c.product_code IN ('11027057') THEN c.insured_amount ELSE 0 END) 车上人员保额
   , SUM(CASE WHEN c.product_code IN ('11027058') THEN c.insured_amount ELSE 0 END) 司乘人员保额
     , SUM(CASE WHEN c.product_code IN ('11025210','11025310','11027010','11025010','11025110') THEN c.premium_amount ELSE 0 END) 车损险保费
   , SUM(CASE WHEN c.product_code IN ('11025250','11025350','11027050','11025050','11025150') THEN c.premium_amount ELSE 0 END) 三责险保费
   , SUM(CASE WHEN c.product_code IN ('11027057') THEN c.premium_amount ELSE 0 END) 车上人员保

  , SUM(CASE WHEN c.product_code IN ('11027058') THEN c.premium_amount ELSE 0 END) 司乘人员保费
from wuh_dw.etl_policy a
```
