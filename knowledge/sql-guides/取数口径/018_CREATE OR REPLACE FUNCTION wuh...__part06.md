---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
split_part: 6/6
source_split: true
---

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
