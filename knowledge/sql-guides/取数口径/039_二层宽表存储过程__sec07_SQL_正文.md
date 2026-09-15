---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  

   
	--用子查询的方式添加投保人和被保人信息，因为party_t下发可能不同步
  insert into v_nonauto_agreement_t3 
  WITH party_t AS (
    SELECT 
        policy_no, name, address, credential_type, credential_no, mobilephone, telephone, ROLE,
        row_number() over (partition BY policy_no, ROLE order by ROLE) RANK
    FROM 
        ids_wuh.party_t 
    WHERE 
        policy_no IN (SELECT policy_no FROM v_nonauto_agreement_t1)
),
WORKLOAD_T AS (
    SELECT 
        policy_no,
        MAX(policy_no) AS policy_no_wl
    FROM 
        ids_wuh.WORKLOAD_T
    GROUP BY 
        policy_no
),
RYX_TBL_PLCINFO_ADD_T AS (

    SELECT 
        TBDH,
        MIN(TASK_ID) AS TASK_ID_ryx
    FROM 
        ids_wuh.RYX_TBL_PLCINFO_ADD_T
    GROUP BY 
        TBDH
),
CIBS_P_POLICY_EXTEND_T AS (
    SELECT 
        applyno,
        MIN(TASKNO) AS TASK_ID_cibs
    FROM 
        ids_wuh.CIBS_P_POLICY_EXTEND_T
    GROUP BY 
        applyno
),
RYX_AGREEMENT_T AS (
    SELECT 
        policy_no,
        MAX(plan_code) AS plan_code_ryx,
        MAX(plan_name) AS plan_name_ryx,
        MAX(diysalecode) AS diysalecode_ryx,
        MAX(partner_code) AS partner_code_ryx,
        MAX(partner_name) AS partner_name_ryx
    FROM 
        ids_wuh.ryx_agreement_t
    GROUP BY 
        policy_no
),
CIBS_AGREEMENT_T AS (
    SELECT 
        policy_no,
        MAX(plan_code) AS plan_code_cibs,
        MAX(plan_name) AS plan_name_cibs,
        MAX(diysalecode) AS diysalecode_cibs,
        MAX(partner_code) AS partner_code_cibs,
        MAX(partner_name) AS partner_name_cibs
    FROM 
        ids_wuh.cibs_agreement_t
    GROUP BY 
        policy_no
)
SELECT 
    a.*,
    b1.name AS tb_name,
    b1.address AS tb_address,
    b1.credential_type AS tb_credential_type,
    b1.credential_no AS tb_credential_no,
    b1.mobilephone AS tb_mobilephone,
    b1.telephone AS tb_telephone,
    b2.name AS bb_name,
    b2.address AS bb_address,
    b2.credential_type AS bb_credential_type,
    b2.credential_no AS bb_credential_no,
    b2.mobilephone AS bb_mobilephone,
    b2.telephone AS bb_telephone,
    CASE 
        WHEN c.policy_no_wl IS NULL THEN '否'
        ELSE '是'
    END AS is_moreopera,
    COALESCE(d1.TASK_ID_ryx, d2.TASK_ID_cibs) AS TASK_ID,
    COALESCE(f1.plan_code_ryx, f2.plan_code_cibs) AS plan_code,
    COALESCE(f1.plan_name_ryx, f2.plan_name_cibs) AS plan_name,
    COALESCE(f1.diysalecode_ryx, f2.diysalecode_cibs) AS diysalecode,
    COALESCE(f1.partner_code_ryx, f2.partner_code_cibs) AS partner_code,
    COALESCE(f1.partner_name_ryx, f2.partner_name_cibs) AS partner_name
FROM 
    v_nonauto_agreement_t2 a
LEFT JOIN 
    party_t b1 ON a.policy_no = b1.policy_no AND b1.role = '1' AND b1.RANK = '1'
LEFT JOIN 
    party_t b2 ON a.policy_no = b2.policy_no AND b2.role = '2' AND b2.RANK = '1'
LEFT JOIN 
    WORKLOAD_T c ON a.policy_no = c.policy_no
LEFT JOIN 
    RYX_TBL_PLCINFO_ADD_T d1 ON a.policy_application_no = d1.TBDH
LEFT JOIN 
    CIBS_P_POLICY_EXTEND_T d2 ON a.policy_application_no = d2.applyno
LEFT JOIN 
    RYX_AGREEMENT_T f1 ON a.policy_no = f1.policy_no
LEFT JOIN 
    CIBS_AGREEMENT_T f2 ON a.policy_no = f2.policy_no;
  v_nonauto_agreement_t3_num := sql%rowcount;
  if v_nonauto_agreement_t_num <> v_nonauto_agreement_t3_num then
    RETURN;
  end if;
```
