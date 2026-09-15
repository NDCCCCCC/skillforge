---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  

  --匹配二层保单表
merge into
  merge into etl_policy a
  using v_auto_agreement_t7 c
  on (a.policy_no = c.policy_no)
  when matched then
    update
       set a.policy_application_no    = c.policy_application_no,
           a.department_group_code    = c.department_group_code,
           a.department_code          = c.department_code,
           a.section_office_code      = c.section_office_code,
           a.product_code             = c.product_code,
           a.input_operator_code      = c.input_operator_code,
           a.operator_code            = c.operator_code,
           a.selling_channel_type     = c.selling_channel_type,
           a.selling_shop_code        = c.selling_shop_code,
           a.AGENT_CODE               = c.AGENT_CODE,
           a.total_insured_amount     = c.total_insured_amount,
           a.check_date               = c.check_date,
           a.offer_operator_code      = c.offer_operator_code,
           a.offer_site_code          = c.offer_site_code,
           a.CALIBEROFNEWCHANNELS     = c.CALIBEROFNEWCHANNELS,
           a.blanket_customer_code    = c.blanket_customer_code,
           a.issue_date               = c.issue_date,
           a.inception_date           = c.inception_date,
           a.planned_end_date         = c.planned_end_date,
           a.policy_status            = c.policy_status,
           a.policy_type              = c.policy_type,
           a.application_date         = c.application_date,
           a.LIFE_SALESPERSON_NAME    = c.LIFE_SALESPERSON_NAME,
           a.SOURCETYPE               = c.S

OURCETYPE,
           a.SUB_ORIGIN_TYPE          = c.SUB_ORIGIN_TYPE,
           a.THIRDANDDAMAGESPREADING  = c.THIRDANDDAMAGESPREADING,
           a.THIRDPARTYSPREADING      = c.THIRDPARTYSPREADING,
           a.TRAFFICSPREADING         = c.TRAFFICSPREADING,
           a.aid_foundation           = c.aid_foundation,
           a.aid_foundation_rate      = c.aid_foundation_rate,
           a.arbitration_institution  = c.arbitration_institution,
           a.area_code                = c.area_code,
           a.branch_company_code      = c.branch_company_code,
           a.business_channel         = c.business_channel,
           a.business_kind            = c.business_kind,
           a.business_type            = c.business_type,
           a.coinsurance_indicator    = c.coinsurance_indicator,
           a.cross_reference          = c.cross_reference,
           a.entry_date               = c.entry_date,
           a.floating_amount          = c.floating_amount,
           a.floating_rate_accident   = c.floating_rate_accident,
           a.floating_rate_indemnity  = c.floating_rate_indemnity,
           a.floating_rate_num        = c.floating_rate_num,
           a.floating_rate_peccancy   = c.floating_rate_peccancy,
           a.floating_rate_renewal    = c.floating_rate_renewal,
           a.group_business_indicator = c.group_business_indicator,
           a.input_way                = c.input_way,
           a.insure_indicator         = c.insure_indicator,
           a.is_agriculture_related   = c.is_agriculture_related,
           a.is_important_customer    = c.is_important_customer,
           a.is_policy_card           = c.is_policy_card,
           a.is_vip                   = c.is_vip,
           a.last_endorsement_no      = c.last_endorsement_no,
           a.lawsuit_way              = c.lawsuit_way,
           a.life_salesperson         = c.life_salesperson,
           a.loanvehicleflag          = c.loanvehicleflag,
           a.number_of_covered_person = c.number_of_covered_person,
           a.number_of_insured_object = c.number_of_insured_object,
           a.offer_time               = c.offer_time,
           a.physical_object_type     = c.physical_object_type,
           a.policy_adjust_ratio      = c.policy_adjust_ratio,
           a.policy_num               = c.policy_num,
           a.workdate                 = c.workdate,
```
