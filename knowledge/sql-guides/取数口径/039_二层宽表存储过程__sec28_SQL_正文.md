---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  ---非车险加上生效日期、签发日期v_request_issue_t2

  insert into v_request_issue_t2
    select a.*
		       , b.request_date as request_date
           --, (SELECT b.signature_date FROM ids_wuh.NONAUTO_AGREEMENT_REQUEST_T b WHERE b.policy_no=a.policy_no AND b.endorsement_no=a.endorsement_no) signature_date
           , b.requested_date as requested_date
					 --, (SELECT b.workdate FROM ids_wuh.nonauto_premium_t b WHERE b.policy_no=a.policy_no AND b.endorsement_no=a.endorsement_no) premium_workdate
					 , to_char(a.signature_date,'yyyymmdd') as business_date
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),1,4) as nf
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),5,2) as yf
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),7,2) as dd
					 , b.request_reason as request_reason
					 , b.approval_document as approval_document
					 , '' CPICSCORE 
           , '' INDEPENDENTPRICERATE
           , '' LASTNCDRATE
           , '' NONCLAIMDISCOUNTRATE 
           , '' policy_costrate
           , '' PURERISK_PREMIUM
           , '' ACTUALDISCUTE_RATE
           , '' underwritingrate
           , '' channelrate
           , '' RULE_GROUP_NO
           , '' RULE_GROUP_NAME
           , '' TOTALCPICSCORE
           , '' TOTALECOMPENSATION_RATE
           , '' business_fee
           , '' changeable_feerate
           , '' ecompensation_rate
           , '' fixed_feerate
           , '' goalcompenstation_rate
           , '' lastyear_acctimes
    from v_request_issue_t1 a
    left join ids_wuh.NONAUTO_AGREEMENT_REQUEST_T b
    on b.policy_no=a.policy_no AND b.endorsement_no=a.endorsement_no;
  v_request_issue_t_num2 := sql%rowcount;
  if v_request_issue_t_num1 <> v_request_issue_t_num2 then
    RETURN;
  end if;

  insert into v_request_issue_t3
  ---车险
    WITH C_QDBF1 as(
		     select  a.policy_no
								 ,a.endorsement_no
								 ,b.signature_date
								 ,b.workdate as workdate_request
								 ,a.workdate as workdate_premium
                 ,sum(a.premium_amount) as premium_amount
								 ,sum(a.tax_amount) as tax_amount
    from ids_wuh.AUTO_PREMIUM_T a
    join ids_wuh.AUTO_AGREEMENT_REQUEST_T b
    on a.policy_no=b.policy_no and a.endorsement_no=b.endorsement_no
		where b.workdate = etl_batch_date
		AND b.sequenceno = etl_sequenceno
    GROUP BY a.policy_no,a.endorsement_no,b.signature_date,b.workdate,a.workdate
    ) 
    select b.*,
    COALESCE(b.premium_amount,0)-COALESCE(b.tax_amount,0)as bf,
    a.department_code,
    '车险' as xz,
    F_BASIC_TJBMZDM(a.department_group_code , a.department_code )as department_group_code,
    F_BASIC_TJBMZMC(a.department_group_code , a.department_code )as department_group_name,
    F_BASIC_TJBMMC(a.department_group_code,a.department_code)as department_name,
    a.section_office_code,
		c.section_office_name,
    F_BASIC_QUDAO(a.CALIBEROFNEWCHANNELS,a.selling_shop_code,s.ywflzxmc)as qd
		
    from C_QDBF1 b
    join ids_wuh.AUTO_AGREEMENT_T a
    on a.policy_no=b.policy_no
    LEFT JOIN 
    (SELECT section_office_code, MAX(section_office_name) AS section_office_name
     FROM ids_wuh.ks_t
     GROUP BY section_office_code) c ON c.section_office_code = a.section_office_code
     left join etl_new_channel s
       on s.ywfldm = a.CALIBEROFNEWCHANNELS;

  v_request_issue_t_num3 := sql%rowcount;
```
