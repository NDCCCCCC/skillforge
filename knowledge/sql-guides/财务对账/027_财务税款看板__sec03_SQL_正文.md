---
title: 财务税款看板
source_idx: 027
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE TABLE tax_revenue_stats (
    org_info VARCHAR(100) COMMENT '机构信息，如机构代码或名称',
    org_info2 VARCHAR(100) COMMENT '附加机构信息，如机构类型或区域',
    corporate_income_tax DECIMAL(15,2) COMMENT '企业所得税金额（单位：元）',
    value_added_tax DECIMAL(15,2) COMMENT '增值税金额（单位：元）',
    urban_maintenance_construction_tax DECIMAL(15,2) COMMENT '城市维护建设税金额（单位：元）',
    property_tax DECIMAL(15,2) COMMENT '房产税金额（单位：元）',
    land_use_tax DECIMAL(15,2) COMMENT '土地使用税金额（单位：元）',
    stamp_tax DECIMAL(15,2) COMMENT '印花税金额（单位：元）',
    vehicle_purchase_tax DECIMAL(15,2) COMMENT '车辆购置税金额（单位：元）',
    withholding_value_added_tax DECIMAL(15,2) COMMENT '代扣代缴增值税金额（单位：元）',
    withholding_individual_income_tax DECIMAL(15,2) COMMENT '代扣代缴个人所得税金额（单位：元）',
    withholding_urban_tax DECIMAL(15,2) COMMENT '代扣代缴城市维护建设税金额（单位：元）',
    withholding_vehicle_tax DECIMAL(15,2) COMMENT '代扣代缴车船使用税金额（单位：元）',
    education_surcharge DECIMAL(15,2) COMMENT '教育费附加金额（单位：元）',
    local_education_surcharge DECIMAL(15,2) COMMENT '地方教育附加金额（单位：元）',
    total DECIMAL(15,2) COMMENT '所有税种合计金额（单位：元）',
    corporate_income_tax_mom DECIMAL(10,2) COMMENT '企业所得税月同比增长率（百分比，如10.50表示10.5%）',
    value_added_tax_mom DECIMAL(10,2) COMMENT '增值税月同比增长率（百分比）',
    urban_maintenance_construction_tax_mom DECIMAL(10,2) COMMENT '城市维护建设税月同比增长率（百分比）',
    property_tax_mom DECIMAL(10,2) COMMENT '房产税月同比增长率（百分比）',
    land_use_tax_mom DECIMAL(10,2) COMMENT '土地使用税月同比增长率（百分比）',
    stamp_tax_mom DECIMAL(10,2) COMMENT '印花税月同比增长率（百分比）',
    vehicle_purchase_tax_mom DECIMAL(10,2) COMMENT '车辆购置税月同比增长率（百分比）',
    withholding_value_added_tax_mom DECIMAL(10,2) COMMENT '代扣代缴增值税月同比增长率（百分比）',
    withholding_individual_income_tax_mom DECIMAL(10,2) COMMENT '代扣代缴个人所得税月同比增长率（百分比）',
    withholding_urban_tax_mom DECIMAL(10,2) COMMENT '代扣代缴城市维护建设税月同比增长率（百分比）',
    withholding_vehicle_tax_mom DECIMAL(10,2) COMMENT '代扣代缴车船使用税月同比增长率（百分比）',
    education_surcharge_mom DECIMAL(10,2) COMMENT '教育费附加月同比增长率（百分比）',
    local_education_surcharge_mom DECIMAL(10,2) COMMENT '地方教育附加月同比增长率（百分比）',
    total_mom DECIMAL(10,2) COMMENT '合计金额月同比增长率（百分比）',
    corporate_income_tax_yoy DECIMAL(10,2) COMMENT '企业所得税年同比增长率（百分比）',
    value_added_tax_yoy DECIMAL(10,2) COMMENT '增值税年同比增长率（百分比）',
    urban_maintenance_construction_tax_yoy DECIMAL(10,2) COMMENT '城市维护建设税年同比增长率（百分比）',
    property_tax_yoy DECIMAL(10,2) COMMENT '房产税年同比增长率（百分比）',
    land_use_tax_yoy DECIMAL(10,2) COMMENT '土地使用税年同比增长率（百分比）',
    stamp_tax_yoy DECIMAL(10,2) COMMENT '印花税年同比增长率（百分比）',
    vehicle_purchase_tax_yoy DECIMAL(10,2) COMMENT '车辆购置税年同比增长率（百分比）',
    withholding_value_added_tax_yoy DECIMAL(10,2) COMMENT '代扣代缴增值税年同比增长率

```

百分比）',

    withholding_individual_income_tax_yoy DECIMAL(10,2) COMMENT '代扣代缴个人所得税年同比增长率（百分比）',
```
