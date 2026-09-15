---
title: 财务现金流小程序实现方案
source_idx: 010
section: SQL / 正文
---

## SQL / 正文

```sql
LEFT JOIN (    SELECT         org_name,        SUM(business_receipt) AS total_business_receipt,        SUM(net_business_flow) AS total_net_business_flow    FROM wuh_dw.fr_fgs_cx_cash_flow    WHERE workdate <= '${workdate}'    GROUP BY org_name) ad ON tm.org_name = ad.org_nameWHERE tm.workdate = '${workdate}'ORDER BY ad.total_net_business_flow DESC;
