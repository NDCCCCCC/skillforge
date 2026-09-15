---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  

EXCEPTION
  WHEN NO_DATA_FOUND THEN
    DBMS_OUTPUT.PUT_LINE('erro');
end$$
/保批单存储过程
CREATE OR REPLACE FUNCTION wuh_dw.etl_request_issue_t(etl_batch_date character varying, etl_sequenceno character varying)
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$ DECLARE  v_request_issue_t_num1   number;
  v_request_issue_t_num2   number;
  v_request_issue_t_num3   number;
	v_request_issue_t_num4   number;
	v_etl_batch_date DATE := to_date(etl_batch_date,'yyyymmdd');

begin
  execute immediate 'truncate table v_request_issue_t1 ';
  execute immediate 'truncate table v_request_issue_t2 ';
  execute immediate 'truncate table v_request_issue_t3 ';
	execute immediate 'truncate table v_request_issue_t4 ';
	execute immediate 'truncate table v_request_issue_t5 ';
```
