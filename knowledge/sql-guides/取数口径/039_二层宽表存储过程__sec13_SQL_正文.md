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
/二层车险保单存储过程
CREATE OR REPLACE FUNCTION wuh_dw.etl_auto_agreement_t(etl_batch_date character varying, etl_sequenceno character varying)
 RETURNS void
 LANGUAGE plpgsql
 NOT FENCED NOT SHIPPABLE
AS $$ DECLARE  v_auto_agreement_t_num    number;
  v_auto_agreement_t1_num   number;
  v_auto_agreement_t2_num   number;
  v_MOTORISED_VEHICLE_T_num number;
  v_party_t_num             number;
	v_auto_agreement_t6_num   NUMBER;
	v_auto_agreement_t7_num   NUMBER;

begin
  execute immediate 'truncate table v_auto_agreement_t ';
  execute immediate 'truncate table v_auto_agreement_t1 ';
  execute immediate 'truncate table v_auto_agreement_t2 ';
  execute immediate 'truncate table v_MOTORISED_VEHICLE_T ';
  execute immediate 'truncate table v_party_t ';
	execute immediate 'truncate table v_auto_agreement_t6 ';
	execute immediate 'truncate table v_auto_agreement_t7 ';		

  --建立workdate为etl_batch_date的临时保单表v_auto_agreement_t
  insert into v_auto_agreement_t
    select a.policy_no,
           a.policy_application_no,
					 a.department_group_code AS department_group_code_original,
           f_basic_tjbmzdm(a.department_group_code,a.department_code) department_group_code,
           a.department_code,
           a.section_office_code,
           a.product_code,
           a.input_operator_code,
           a.operator_code,
           a.selling_channel_type,
           a.selling_shop_code,
           a.AGENT_CODE,
           a.total_insured_amount, --总保额(该字段不准，保额请用insured_amount)
           a.check_date,
           a.offer_operator_code,
           a.offer_site_code,
           a.CALIBEROFNEWCHANNELS,
           a.blanket_customer_code,
           a.issue_date,
           a.inception_date,
           a.planned_end_date,
           a.policy_status,
           a.policy_type,
           a.application_date,
           a.LIFE_SALESPERSON_NAME,
           a.SOURCETYPE,
           a.SUB_ORIGIN_TYPE,
           a.THIRDANDDAMAGESPREADING,
           a.THIRDPARTYSPREADING,
           a.TRAFFICSPREADING,
           a.aid_foundation,
           a.aid_foundation_rate,
           a.arbitration_institution,
           a.area_code,
           a.branch_company_code,
           a.business_channel,
           a.business_kind,
           a.business_type,
           a.coinsurance_indicator,
           a.cross_reference,
           a.entry_date,
           a.floating_amount,
           a.floating_rate_accident,
           a.floating_rate_indemnity,
           a.floating_rate_num,
           a.floating_rate_peccancy,
           a.floating_rate_renewal,
           a.group_business_indicator,
           a.input_way,
           a.insure_indicator,
           a.is_agriculture_related,
           a.is_important_customer,
           a.is_policy_card,
           a.is_vip,
           a.last_endorsement_no,
           a.lawsuit_way,
           a.life_salesperson,
           a.loanvehicleflag,
           a.number_of_covered_person,
           a.number_of_insured_object,
           a.offer_time,
           a.physical_object_type,
           a.policy_adjust_ratio,
           a.policy_num,
           a.workdate,
					 a.source_datasource source_datasource_code,
					f_basic_source_datasource(a.source_datasource) source_datasource_name,
					 a.commercial_agreement_no,
					 b.taskapplicationno
      from ids_wuh.auto_agreement_t a
      left join ids_wuh.AUTO_AGREEMENT_EXTEND_T b on (a.policy_no = b.policy_no)
     where a.workdate = etl_batch_date
		   AND a.sequenceno = etl_sequenceno;
  v_auto_agreement_t_num := sql%rowcount;
```
