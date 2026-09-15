---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE OR REPLACE FUNCTION wuh_dw.etl_nonauto_agreement_t(etl_batch_date character varying, etl_sequenceno character varying)
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$ DECLARE  v_nonauto_agreement_t_num    number;
  v_nonauto_agreement_t1_num   number;
  v_nonauto_agreement_t2_num   number;
  v_nonauto_agreement_t3_num   number;
  v_nonauto_agreement_t4_num   number;
  v_nonauto_agreement_t5_num   number;
begin
  execute immediate 'truncate table v_nonauto_agreement_t ';
  execute immediate 'truncate table v_nonauto_agreement_t1 ';
  execute immediate 'truncate table v_nonauto_agreement_t2 ';
  execute immediate 'truncate table v_nonauto_agreement_t3 ';
  execute immediate 'truncate table v_nonauto_agreement_t4 ';
  execute immediate 'truncate table v_nonauto_agreement_t5 ';
```
