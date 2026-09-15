---
title: 财务现金流小程序实现方案
source_idx: 010
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文




SELECT     a.department_group_code,    t.department_group_name,    a.department_code,    s.department_name FROM ids_wuh.AUTO_CF_INDEMNITY_T a
LEFT JOIN ids_wuh.bmz_t t ON a.department_group_code = t.department_group_code
LEFT JOIN ids_wuh.bm_t s ON a.department_code = s.department_codeWHERE indemnity_no = 'F420100Y2026018137_1';-- 核查非车险赔案组织架构
SELECT     a.department_group_code,    t.department_group_name,    a.department_code,    s.department_name FROM ids_wuh.nonAUTO_CF_INDEMNITY_T a LEFT JOIN ids_wuh.bmz_t t ON a.department_group_code = t.department_group_code
LEFT JOIN ids_wuh.bm_t s ON a.department_code = s.department_codeWHERE indemnity_no = 'EWUHC1194926050053';-- 核查批单组织架构
SELECT     a.department_group_code,    a.department_group_name,    a.department_code,    a.department_name FROM etl_policy_request_issue_t aWHERE endorsement_no = 'BWUH00LY2026B000023J';-- 核查结算单组织架构
SELECT     a.deptgropcode,    t.department_group_name,    a.deptcode,    s.department_name FROM ids_wuh.CMPOLICYBANLANING_T a
LEFT JOIN ids_wuh.bmz_t t ON a.deptgropcode = t.department_group_code
LEFT JOIN ids_wuh.bm_t s ON a.deptcode = s.department_codeWHERE a.balancingcode = '100845116';
