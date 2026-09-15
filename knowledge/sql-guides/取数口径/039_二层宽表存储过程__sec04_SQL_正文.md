---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  --建立workdate为etl_batch_date的临时保单表v_nonauto_agreement_t
  insert into v_nonauto_agreement_t 
    SELECT a.policy_no, --保单号
       a.policy_application_no, --投保单号
       a.branch_company_code,
       F_BASIC_TJBMZDM(a.department_group_code, a.department_code) department_group_code,
       a.department_code,
       a.section_office_code,
       a.policy_type, --保单性质
       CASE a.policy_type
        WHEN '1' THEN '新保'
        WHEN '2' THEN '续保'
        WHEN '3' THEN '公司内转保'
        WHEN '4' THEN '公司间转保'
        WHEN '5' THEN '新保暂保'
        WHEN '6' THEN '续保暂保'
        WHEN '9' THEN '其他'
        ELSE a.policy_type
    END AS policy_type_name,
       a.number_of_insured_object , --承保数量   农业险中：种植业为亩,养殖业为头
       a.offer_site_code, --出单点代码
       a.offer_time, --  出单时间
       a.offer_operator_code, --出单员代码
       a.agent_code, --代理点
       a.short_term_rate, --短期比例
       a.risk_category, --风险类别
       CASE a.risk_category
        WHEN 'gd' THEN '电力项目保险'
        WHEN 'gf' THEN '航空航天保险'
        WHEN 'dl' THEN '道路交通类保险'
        WHEN 'gq' THEN '跨江、跨海桥梁保险'
        WHEN 'gy' THEN '石油天然气项目保险'
        WHEN 'pt' THEN '普通业务'
        WHEN 'sh' THEN '石油化工项目保险'
        WHEN 'gh' THEN '核电站保险'
        WHEN '99' THEN '其他'
        ELSE a.risk_category
    END AS risk_category_name,
       a.physical_object_type, --标的类型
       a.coinsurance_indicator, --共保标志
       CASE a.coinsurance_indicator
        WHEN '0' THEN '非��保'
        WHEN '1' THEN '主出单方'
        WHEN '2' THEN '跟单方'
        WHEN '3' THEN '司内主出单方'
        WHEN '9' THEN '其他'
        ELSE a.coinsurance_indicator
    END AS coinsurance_indicator_name,
       a.operator_code, --经办人代码
       a.input_operator_code, --录单员代码
       a.application_date, --投保日期
       a.entry_date, --录入日期
       a.issue_date, --签单日期
       a.inception_date, --起保日期
       a.check_date, --核保日期
       a.planned_end_date, --终保日期
       a.blanket_customer_code, --统保代码
       a.policy_num, --业务笔数
       a.business_type, --业务性质
       a.original_policy_no, --原保单号
       a.selling_channel_type, --展业方式
       a.product_code, --主险种代码
       a.policy_status, --保单状态
       a.is_vip, --是否vip
       a.is_policy_card, --是否卡式保单
       a.group_business_indicator, --是否团单
       a.number_of_covered_person, --被保人数
       a.population_timestamp, --IDS装载数据生成时间
       a.is_agriculture_related, --是否涉农
       a.workdate,
       a.is_important_customer, --是否重大客户
       a.farmer_num, --农户次数
    

   a.commercial_agreement_no, --协议号
       a.source_timestamp,  --源时间戳
       a.SOURCE_SYSTEM_LABEL,  --IDS加载源系统标签
       a.SOURCE_DATASOURCE, --源数据来源  
       a.SOURCE_POLICY_STATUS, --源系统保单状态
       a.last_endorsement_no, --最新批单号
       a.if_system, --是否系统出单
       a.life_salesperson, --寿险人员代码
       a.temporary_policy_no, --暂保单号
       a.short_coefficience, --短期费率系数
       a.business_channel,  --业务来源
       a.entry_type, --录入类型
       a.coinsurance_percent, --共保比例
       a.num_of_list_person, --已录清单人数
       a.charge_type,  --保费计费方式
       a.product_code_casualty, --产品代码
       total_insured_amount, --总保额(该字段不准，保额请用insured_amount)
       a.total_premium,  --总保费重新算
       a.arbitration_institution, --仲裁机构
       a.planned_end_time, --止期时刻
```
