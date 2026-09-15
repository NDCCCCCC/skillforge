---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
```

境外不含港、澳、台

       a.insured_sign_flag,  --记名标志,表示被保人记名，为是则必须有被保人
       a.org_code, --机构代码
       a.agent_fee_rate,  --代理手续费率（%）
       a.agreement_no,  --协议号，承保时适用的协议的编号
       to_char(SYSDATE, 'yyyy-mm-dd') myworkdate,
       to_char(a.issue_date, 'yyyy') issue_year,
       to_char(a.issue_date, 'mm') issue_month,
       to_char(a.issue_date, 'dd') issue_dd,
       to_char(a.inception_date, 'yyyy') inception_year,
       to_char(a.inception_date, 'mm') inception_month,
       to_char(a.inception_date, 'dd') inception_dd,
       to_char(a.planned_end_date, 'yyyy') planned_end_year,
       to_char(a.planned_end_date, 'mm') planned_end_month,
       to_char(a.planned_end_date, 'dd') planned_end_dd,
       a.department_group_code AS department_gtoup_code_orial         
       FROM ids_wuh.nonauto_agreement_t a
       where workdate = etl_batch_date
			 AND sequenceno = etl_sequenceno;
  v_nonauto_agreement_t_num := sql%rowcount;
  
  ---加上部门组名称\代理点等字段  v_nonauto_agreement_t1


```sql
   insert into v_nonauto_agreement_t1 
  select 
    a.*,
    F_BASIC_TJBMMC(a.department_gtoup_code_orial, a.department_code) DEPARTMENT_NAME,
    F_BASIC_TJBMZMC(a.department_group_code, a.department_code) DEPARTMENT_GROUP_NAME,
    (select max(section_office_name)
       from ids_wuh.ks_t d
      where d.section_office_code = a.section_office_code) section_office_name,
    (case 
        when a.product_code in (select yxzh from xzh_nx_view) 
        then '农险' 
        else '非车险' 
    end) xzdl,
    (select product_name
       from xzh e
      where e.product_code = a.product_code) product_name,
    (select staff_name
       from ids_wuh.rydm_t f
      where f.staff_code = a.input_operator_code) input_operator_name,
    (select staff_name
       from ids_wuh.rydm_t f
      where f.staff_code = a.operator_code) operator_name,
    (select dldmc from ids_wuh.dld_t e where e.dlddm = a.agent_code) agent_name,
    (select staff_name
       from ids_wuh.rydm_t f
      where f.staff_code = a.offer_operator_code) offer_operator_name
from v_nonauto_agreement_t a;
v_nonauto_agreement_t1_num := sql%rowcount;
  if v_nonauto_agreement_t_num <> v_nonauto_agreement_t1_num then
    RETURN;
  end if;


 
  --加上八大渠道等字段  v_nonauto_agreement_t2
  insert into v_nonauto_agreement_t2 
    WITH NONAUTO_PREMIUM_T AS (
    SELECT 
        policy_no,
        SUM(premium_amount) AS premium_amount,
        SUM(TAXAMOUNT) AS tax_amount
    FROM 
        ids_wuh.nonauto_premium_t
    GROUP BY 
        policy_no
),
NONAUTO_COVERAGE_COMPONENT_T AS (

    SELECT 
        policy_no,
        SUM(insured_amount) AS insured_amount
    FROM 
        ids_wuh.nonauto_coverage_component_t
    GROUP BY 
        policy_no
)
SELECT 
    a.*
    , b.ch_lv_third AS eight_channel --八大渠道
    , b.name AS SELLING_CHANNEL_NAME --展业方式名称
    , c.signature_date AS signature_date
    , d.premium_amount
    , d.tax_amount
    , e.insured_amount
    --, F_BASIC_ISMSB(f1.msb_1, f2.msb_2, f3.msb_3) AS is_msb
FROM 
    v_nonauto_agreement_t1 a
LEFT JOIN 
    hb_channel b ON a.selling_channel_type = b.code
LEFT JOIN 
    ids_wuh.nonauto_agreement_request_t c ON a.policy_no = c.policy_no AND c.endorsement_no = '无'
LEFT JOIN 
    NONAUTO_PREMIUM_T d ON a.policy_no = d.policy_no
LEFT JOIN 
    NONAUTO_COVERAGE_COMPONENT_T e ON a.policy_no = e.policy_no;
    v_nonauto_agreement_t2_num := sql%rowcount;
  if v_nonauto_agreement_t_num <> v_nonauto_agreement_t2_num then
    RETURN;
  end if;
```
