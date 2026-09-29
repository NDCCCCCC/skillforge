---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
section: SQL / 正文
---

## SQL / 正文

```sql
  INSERT INTO FGS_FG_FXQ_Result_CB
  SELECT *, v_workdate
    FROM FGS_FG_FXQ_Tmp_CB
    
EXCEPTION
        WHEN OTHERS THEN
            RAISE EXCEPTION '存储过程执行失败：%', SQLERRM;
            ROLLBACK;
END;
$$;
```
