---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  --加上部门组名称\代理点等字段  v_auto_agreement_t1

  insert into v_auto_agreement_t1
  select a.*,
		       f_basic_tjbmzmc(a.department_group_code,a.department_code)department_group_name,
           f_basic_tjbmmc(a.department_group_code_original,a.department_code) department_name,
           (select max(section_office_name)
              from ids_wuh.ks_t d
             where d.section_office_code = a.section_office_code) section_office_name,
           (select product_name
              from xzh e
             where e.product_code = a.product_code) product_name,
           (select staff_name
              from ids_wuh.rydm_t f
             where f.staff_code = a.input_operator_code) input_operator,
           (select staff_name
              from ids_wuh.rydm_t f
             where f.staff_code = a.operator_code) operator_name,
           (select b.name
              from hb_channel b
             where a.selling_channel_type = b.code) selling_channel_name,
           (select dldmc from ids_wuh.dld_t e where e.dlddm = a.agent_code) dldmc,
           (select staff_name
              from ids_wuh.rydm_t f
             where f.staff_code = a.offer_operator_code) offer_operator_name
      from v_auto_agreement_t a
;	
  v_auto_agreement_t1_num := sql%rowcount;
  if v_auto_agreement_t_num <> v_auto_agreement_t1_num then
    RETURN;
  end if;

  --加上八大渠道和保费等字段v_auto_agreement_t2
  insert into v_auto_agreement_t2
     SELECT 
    a.*,
    b.ch_lv_third AS eight_channel,
    F_BASIC_QUDAO(a.CALIBEROFNEWCHANNELS,a.selling_shop_code,s.YWFLZXMC)new_channel, -----拖慢速度的主要点
    g.signature_date,
    e.insurance_flag,
    p.premium_amount,
    c.insured_amount
FROM 
    v_auto_agreement_t1 a
left join etl_new_channel s
       on s.ywfldm = a.CALIBEROFNEWCHANNELS
LEFT JOIN 
    hb_channel b ON a.selling_channel_type = b.code
LEFT JOIN 
    (SELECT 
         policy_no, 
         SUM(premium_amount) AS premium_amount
     FROM 
         ids_wuh.auto_premium_t
     GROUP BY 
         policy_no
    ) p ON a.policy_no = p.policy_no
LEFT JOIN 
    (SELECT 
         policy_no, 
         SUM(insured_amount) AS insured_amount
     FROM 
         ids_wuh.AUTO_COVERAGE_COMPONENT_T
     GROUP BY 
         policy_no
    ) c ON a.policy_no = c.policy_no
LEFT JOIN 
    (SELECT 
         policy_no, 
         insurance_flag
     FROM 
         ids_wuh.AUTO_AGREEMENT_EXTEND_T
    ) e ON a.policy_no = e.policy_no
LEFT JOIN 
    (SELECT 
         policy_no, 
         MAX(signature_date) AS signature_date 
     FROM 
         ids_wuh.auto_agreement_request_t
     WHERE 
         endorsement_no = '无'
     GROUP BY 
         policy_no
    ) g ON a.policy_no = g.policy_no;	
    
  v_auto_agreement_t2_num := sql%rowcount;
  if v_auto_agreement_t1_num <> v_auto_agreement_t2_num then
    RETURN;
  end if;

  --加上机动车表的字段v_MOTORISED_VEHICLE_T

  insert into v_MOTORISED_VEHICLE_T
    SELECT 
    a.*,
    b.usage,
    f_basic_usage(b.usage) AS usage_name,
    b.plate_number,
    b.frame_number,
    b.vin,
    b.first_registration_date,
    b.usage_tb,
    b.vehicle_brand,
    b.vehicle_owner,
    b.serial_number,
    b.value_new,
    b.brand_model,
    e.customer_clm_times,
    b.vehicle_type,
    b.quick_search_code,
    b.passenger_capacity,
    b.weighting_capacity,
    b.license_color,
    b.engine_capacity,
    f_basic_usage_b(b.usage,a.product_code) AS usage_name_b,
    b.Special_Vehicle_Flag,
    b.Dump_Trailer_Flag,
	fr_basci_vehicle_typename(b.vehicle_type, a.product_code) AS vehicle_type_name
FROM 
    v_auto_agreement_t2 a
left JOIN 
    ids_wuh.motorised_vehicle_t b ON a.policy_no = b.policy_no
LEFT JOIN 
    (SELECT policy_no, customer_clm_times FROM ids_wuh.AUTO_AGREEMENT_EXTEND_T) e ON a.policy_no = e.policy_no;
    
  v_MOTORISED_VEHICLE_T_num := sql%rowcount;
  if v_auto_agreement_t2_num <> v_MOTORISED_VEHICLE_T_num then
    RETURN;
  end if;
```
