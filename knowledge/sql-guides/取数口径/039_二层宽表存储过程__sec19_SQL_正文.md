---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('85') THEN '特种车二-清障车、清洁（清扫）车'
        WHEN mvt.VEHICLEUSAGEDETAIL = '86' THEN '特种车二-其他罐式货车'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('87') THEN '特种车二-其他工程机械'
        WHEN mvt.VEHICLEUSAGEDETAIL IN ('91') THEN '特种车一'
        WHEN mvt.VEHICLEUSAGEDETAIL = '92' THEN '特种车三'
        WHEN mvt.VEHICLEUSAGEDETAIL = '93' THEN '特种车四'
        WHEN mvt.VEHICLEUSAGEDETAIL = '94' THEN '特种车挂车'
        ELSE '0'
    END AS  usage_xf--使用性质细分
from v_auto_agreement_t6 a
left join etl_new_channel nch on a.caliberofnewchannels=nch.ywfldm
left join ids_wuh.AUTO_AGREEMENT_EXTEND_T aat on a.policy_no=aat.policy_no
left join ids_wuh.motorised_vehicle_t mvt on a.policy_no=mvt.policy_no;
```
