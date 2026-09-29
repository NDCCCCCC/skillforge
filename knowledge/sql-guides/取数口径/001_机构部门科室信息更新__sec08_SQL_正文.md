---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: 001
section: SQL / 正文
---

## SQL / 正文

```sql
      select sum(b.insured_object_indemnity+b.investigation_fee
           +b.investigation_agent_fee+b.inside_investigation_agent_fee
           +b.claim_audit_fee+b.salvage_fee+b.other_fee
           -b.salvage_recovery) 
      from ids_wuh.nonAUTO_CF_INDEMNITY_T b --非车险
      where b.policy_no =t.policy_no
      and b.posted_date<=to_date('2025-12-31 23:59:59','yyyy-mm-dd hh24:mi:ss')) 赔付金额
     ,(--未决

     select sum(gsje) from "ids_wuh".INDISPUTE_T_1214 i
      where i.nf='2025'
        and i.qsh='12'
        and i.polno=t.policy_no
     ) 估损金额
FROM wuh_dw.etl_nonauto_policy_t t
left join 
(
  select y.policy_no,sum(y.ADJUST_AFTER_BRANCHREIN_UPR) ADJUST_AFTER_BRANCHREIN_UPR
       , sum(y.ADJUST_LSBPI_UPR) ADJUST_LSBPI_UPR
    , sum(y.ADJUST_AFTER_BRANCHREIN_UPR2)  ADJUST_AFTER_BRANCHREIN_UPR2
    , sum(y.ADJUST_LSBPI_UPR2) ADJUST_LSBPI_UPR2
    from ids_wuh.op_uprcollection_all_t_adj y
    where EXISTS (
        SELECT 1 
        FROM tmp_ycb_001 b 
        WHERE b.policy_no = y.policy_no
    ) and y.ESTIMATE_DATE = '20251231' 
    group by y.policy_no
) a on t.policy_no = a.policy_no
left join ids_wuh.reinsurance_t f on t.policy_no = f.REINSURANCE_POLICY_NO
where t.policy_no in
```
