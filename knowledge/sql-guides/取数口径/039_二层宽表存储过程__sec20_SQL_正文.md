---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
v_auto_agreement_t7_num := sql%rowcount;
  if v_auto_agreement_t7_num <> v_auto_agreement_t6_num then
    RETURN;
  end if;
```
