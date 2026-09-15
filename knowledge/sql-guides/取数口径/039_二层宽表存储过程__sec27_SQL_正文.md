---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  --建立保批单workdate为etl_batch_date的临时非车险签单明细表v_request_issue_t1
  insert into v_request_issue_t1
	  ---非车险
	  with FC_QDBF1 as(
		--筛选出批改日期离谱的数据，这部分数据的签单保费不会计算，所以就设为0，但是数据保留
    select a.policy_no
           ,a.endorsement_no
           ,b.signature_date
           ,b.workdate workdate_request
           ,a.workdate workdate_premium
           ,0 premium_amount
           ,0 tax_amount
    from ids_wuh.nonauto_premium_t a
    join ids_wuh.NONAUTO_AGREEMENT_REQUEST_T b 
    on a.policy_no=b.policy_no and a.endorsement_no=b.endorsement_no
    join ids_wuh.exrate_month_t c
    on a.currency_code = c.bzh
    WHERE b.request_date < to_date('1999-01-01 00:00:00','yyyy-MM-dd hh24:mi:ss')
--and trunc(b.request_date) >= trunc(v_etl_batch_date, 'YYYY')
    AND b.workdate = etl_batch_date
		AND b.sequenceno = etl_sequenceno
    GROUP BY a.policy_no,a.endorsement_no,b.signature_date,b.workdate,a.workdate
		
		union ALL
		
    select a.policy_no,
    a.endorsement_no,
    b.signature_date,
    b.workdate AS workdate_request,
    a.workdate AS workdate_premium,
    SUM(
        CASE 
            WHEN a.business_type = '2' AND a.product_code = '11061600' 
            THEN COALESCE((a.premium_amount + a.savings_amount) * c.rate / 100, 0) * 0.0034
            WHEN a.business_type = '2' AND a.product_code <> '11061600' 
            THEN COALESCE((a.premium_amount + a.savings_amount) * c.rate / 100, 0) * 0.0198
            ELSE COALESCE(a.premium_amount * c.rate / 100, 0)
        END
    ) AS premium_amount,
    SUM(COALESCE(a.taxamount, 0) * c.rate / 100) AS tax_amount
    from ids_wuh.nonauto_premium_t a
    join ids_wuh.NONAUTO_AGREEMENT_REQUEST_T b
    on a.policy_no=b.policy_no and a.endorsement_no=b.endorsement_no
    join ids_wuh.exrate_month_t c
    on a.currency_code = c.bzh
    and case
            when b.signature_date > v_etl_batch_date then
             to_char(v_etl_batch_date, 'yyyymm')
            else
             to_char(b.signature_date, 'yyyymm')
           end = trim(c.theyear) || substr('0' || trim(c.themonth), -2)
    where a.branch_company_code='4020100'
    AND b.workdate = etl_batch_date
		AND b.sequenceno = etl_sequenceno
    GROUP BY a.policy_no,a.endorsement_no,b.signature_date,b.workdate,a.workdate),
		--如果税为空，就去子险承保明细表里查，如果都为空就说明可能该批单没有造成保费变化
    FC_QDBF2 AS (

    select a.policy_no,a.endorsement_no,a.signature_date,a.workdate_request,a.workdate_premium,a.premium_amount,
    (case when a.tax_amount is not null then a.tax_amount
          else COALESCE(b.taxamount,0) end) tax_amount
    from  FC_QDBF1 a
    left join (
            select policy_no,endorsement_no,sum(taxamount)as taxamount
            from ids_wuh.nonauto_coverage_component_t 
            group by policy_no,endorsement_no
            ) b on a.policy_no =b.policy_no AND a.endorsement_no = b.endorsement_no
          
    )
    select b.*,
    COALESCE(b.premium_amount,0)-COALESCE(b.tax_amount,0)as bf,
    a.department_code,
    case 
       when  A.product_code in (select yxzh from  xzh_nx_view) then '农险' 
       else '非车险' 
    end as xz,
    F_BASIC_TJBMZDM(a.department_group_code , a.department_code )as department_group_code,
    F_BASIC_TJBMZMC(a.department_group_code , a.department_code )as department_group_name,
    F_BASIC_TJBMMC(a.department_group_code,a.department_code)as department_name,
    a.section_office_code,
    c.section_office_name,
    '1' AS qd
FROM 
    FC_QDBF2 b
JOIN 
    ids_wuh.NONAUTO_AGREEMENT_T a ON a.policy_no = b.policy_no
LEFT JOIN 
    (SELECT section_office_code, MAX(section_office_name) AS section_office_name
     FROM ids_wuh.ks_t
     GROUP BY section_office_code) c ON c.section_office_code = a.section_office_code;
		
	v_request_issue_t_num1 := sql%rowcount;	
```
