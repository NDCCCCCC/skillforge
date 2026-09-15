---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  
  --新增细分字段等字段
  insert into v_auto_agreement_t7
  select a.*,'个客' class_name,
       nch.zyfsmc, --新渠道小类
       CASE 
        WHEN aat.fuel_type = '0' THEN '燃油'
        WHEN aat.fuel_type = '1' THEN '纯电动'
        WHEN aat.fuel_type = '2' THEN '燃料电池'
        WHEN aat.fuel_type = '3' THEN '插电式混合动力'
        WHEN aat.fuel_type = '4' THEN '其他混合动力'
        WHEN aat.fuel_type = 'A' THEN '汽油'
        WHEN aat.fuel_type = 'B' THEN '柴油'
        WHEN aat.fuel_type = 'C' THEN '电'
        WHEN aat.fuel_type = 'D' THEN '混合油'
        WHEN aat.fuel_type = 'D1' THEN '燃油'
        WHEN aat.fuel_type = 'D10' THEN '燃油'
        WHEN aat.fuel_type = 'D11' THEN '燃油'
        WHEN aat.fuel_type = 'D12' THEN '插电式混合动力'
        WHEN aat.fuel_type = 'D13' THEN '燃油'
        WHEN aat.fuel_type = 'D14' THEN '燃油'
        WHEN aat.fuel_type = 'D15' THEN '燃油'
        WHEN aat.fuel_type = 'D2' THEN '燃油'
        WHEN aat.fuel_type = 'D3' THEN '燃油'
        WHEN aat.fuel_type = 'D4' THEN '燃油'
        WHEN aat.fuel_type = 'D5' THEN '其他混合动力'
        WHEN aat.fuel_type = 'D6' THEN '纯电动'
        WHEN aat.fuel_type = 'D7' THEN '燃油'
        WHEN aat.fuel_type = 'D8' THEN '燃料电池'
        WHEN aat.fuel_type = 'D9' THEN '燃油'
        WHEN aat.fuel_type = 'E' THEN '天然气'
        WHEN aat.fuel_type = 'F' THEN '液化石油气'
        WHEN aat.fuel_type = 'L' THEN '甲醇'
        WHEN aat.fuel_type = 'M' THEN '乙醇'
        WHEN aat.fuel_type = 'N' THEN '太阳能'
        WHEN aat.fuel_type = 'O' THEN '混合动力'
        WHEN aat.fuel_type = 'P' THEN '氢'
        WHEN aat.fuel_type = 'Q' THEN '生物燃料'
        WHEN aat.fuel_type = 'Y' THEN '无'
        WHEN aat.fuel_type = 'Z' THEN '其他'
        ELSE '0'
    END AS  fuel_type, --能源类型
       CASE 
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('11') THEN '党政机关事业团体客车-教练车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('12') THEN '党政机关事业团体客车-校车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('13') THEN '党政机关事业团体客车-其他'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('21') THEN '企业客车-教练车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('22') THEN '企业客车-校车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('23') THEN '企业客车-其他'
        WHEN mvt.VEHICLEUSAGEDETAIL = '31' THEN '出租租赁客车-出租'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('32') THEN '出租租赁客车-租赁'
        WHEN mvt.VEHICLEUSAGEDETAIL = '

33' THEN '出租租赁客车-其他'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('34') THEN '出租租赁客车-预约出租客运'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('41') THEN '城市公交'
        WHEN mvt.VEHICLEUSAGEDETAIL = '51' THEN '公路客运-旅游客运'
        WHEN mvt.VEHICLEUSAGEDETAIL = '52' THEN '其他客运'
        WHEN mvt.VEHICLEUSAGEDETAIL = '61' THEN '非营业货车-渣土运输（环卫）'
        WHEN mvt.VEHICLEUSAGEDETAIL = '62' THEN '非营业货车-邮递快递'
        WHEN mvt.VEHICLEUSAGEDETAIL = '63' THEN '非营业货车-其他'
        WHEN mvt.VEHICLEUSAGEDETAIL = '71' THEN '营业货车-渣土运输（环卫）'
        WHEN mvt.VEHICLEUSAGEDETAIL = '72' THEN '营业货车-危险品运输'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('73') THEN '营业货车-物流'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('74') THEN '营业货车-搬运'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('75') THEN '营业货车-其他'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('81') THEN '特种车二-搅拌车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('82') THEN '特种车二-起重车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('83') THEN '特种车二-泵车'
        WHEN mvt.VEHICLEUSAGEDETAIL = '84' THEN '特种车二-冷藏保温'
```
