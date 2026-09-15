---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  

  --匹配二层保单表
merge into
  merge into etl_nonauto_policy_t a
  using (select b.* from v_nonauto_agreement_t5 b) c
  on (a.policy_no = c.policy_no)
  when matched then
    update
       SET  a.policy_application_no   = c.policy_application_no   ,
            a.branch_company_code   = c.branch_company_code   ,
            a.department_code   = c.department_code   ,
            a.department_name   = c.department_name   ,
            a.section_office_code   = c.section_office_code   ,
            a.section_office_name   = c.section_office_name   ,
            a.policy_type   = c.policy_type   ,
            a.policy_type_name  = c.policy_type_name  ,
            a.number_of_insured_object  = c.number_of_insured_object  ,
            a.offer_site_code   = c.offer_site_code   ,
            a.offer_time  = c.offer_time  ,
            a.offer_operator_code   = c.offer_operator_code   ,
            a.offer_operator_name   = c.offer_operator_name   ,
            a.agent_code  = c.agent_code  ,
            a.agent_name  = c.agent_name  ,
            a.short_term_rate   = c.short_term_rate   ,
            a.risk_category   = c.risk_category   ,
            a.risk_category_name  = c.risk_category_name  ,
            a.physical_object_type  = c.physical_object_type  ,
            a.coinsurance_indicator   = c.coinsurance_indicator   ,
            a.coinsurance_indicator_name  = c.coinsurance_indicator_name  ,
            a.operator_code   = c.operator_code   ,
            a.operator_name   = c.operator_name   ,
            a.input_operator_code   = c.input_operator_code   ,
            a.INPUT_OPERATOR_NAME = c.INPUT_OPERATOR_NAME ,
            a.application_date  = c.application_date  ,
            a.entry_date  = c.entry_date  ,
            a.issue_date  = c.issue_date  ,
            a.inception_date  = c.inception_date  ,
            a.check_date  = c.check_date  ,
            a.planned_end_date  = c.planned_end_date  ,
            a.blanket_customer_code   = c.blanket_customer_code   ,
            a.policy_num  = c.policy_num  ,
            a.business_type   = c.business_type   ,
            a.original_policy_no  = c.original_policy_no  ,
            a.selling_channel_type  = c.selling_channel_type  ,
            a.selling_channel_name  = c.selling_channel_name  ,
            a.product_code  = c.product_code  ,
            a.policy_status   = c.policy_status   ,
            a.is_vip  = c.is_vip  ,
            a.is_policy_card  = c.is_policy_card  ,
            a.group_business_indicator  = c.group_business_indicator  ,
            a.number_of_covered_person  = c.number_of_covered_person  ,
            a.population_timestamp  = c.population_timestamp  ,
            a.is_agriculture_related  = c.is_agriculture_related  ,
            a.workdate  = c.workdate  ,
            a.is_important_customer   = c.is_important_customer   ,
            a.farmer_num  = c.farmer_num  ,
            a.commercial_agreement_no   = c.commercial_agreement_no   ,
            a.source_timestamp  = c.source_timestamp  ,
            a.source_system_label   = c.source_system_label   ,
            a.source_datasource   = c.source_datasource   ,
            a.source_policy_status  = c.source_policy_status  ,
            a.last_endorsement_no   = c.last_endorsement_no   ,
            a.if_system   = c.if_system   ,
            a.lif

e_salesperson  = c.life_salesperson  ,
            a.temporary_policy_no   = c.temporary_policy_no   ,
            a.department_group_code   = c.department_group_code   ,
            a.department_group_name   = c.department_group_name   ,
            a.short_coefficience  = c.short_coefficience  ,
            a.business_channel  = c.business_channel  ,
            a.entry_type  = c.entry_type  ,
            a.coinsurance_percent   = c.coinsurance_percent   ,
            a.num_of_list_person  = c.num_of_list_person  ,
            a.charge_type   = c.charge_type   ,
```
