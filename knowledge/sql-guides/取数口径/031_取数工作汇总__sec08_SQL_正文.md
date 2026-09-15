---
title: 取数工作汇总
source_idx: 031
section: SQL / 正文
---

## SQL / 正文

```sql
```

学平险取数

```sql
select a.policy_no 保单号,a.policy_application_no 投保单号,a.department_group_name 部门组,a.department_group_code 部门组代码,a.agent_code 代理点,
       a.department_name 部门,a.section_office_name 科室,a.kq 客群,a.operator_name 经办人,a.operator_code 经办人工号,a.product_code 险种代码,a.product_name 险种名称,
       a.premium_amount 含税保费,a.insured_amount 保额,a.APPLICATION_DATE 投保日期,a.issue_date 签单日期,a.inception_date 起保日期,
       a.planned_end_date 终止日期,a.tb_name 投保人,
       a.tb_credential_type 投保人证件类型,
       a.tb_credential_no 投保人身份证号,
       a.tb_mobilephone 投保人手机,
       a.bb_name 被保人,
       a.bb_credential_type 被保人手机,
       a.bb_credential_no 被保人身份证号,
       case when a.group_business_indicator=0 then 1
              when a.group_business_indicator=1 then (select count(1) from (select distinct t.credential_no from "ids_wuh".HUMAN_BODY_T t where t.policy_no=a.policy_no))
           end 投保人数量,
       (select max(b.factor_value) 
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code='M9') 学校名称,
       (select decode(max(b.factor_value),'FC100352002','其它','FC100352003','1年级','FC100352004','2年级','FC100352005','3年级','FC100352006','4年级',
                             'FC100352007','5年级','FC100352008','6年级',
                             'FC100352009','小班','FC100352010','中班','FC100352011','大班',
                             'FC100352012','初中预科班','FC100352013','初中一年级','FC100352014','初中二年级','FC100352015','初中三年级',
                             'FC100352016','高中预科班','FC100352017','高中一年级','FC100352018','高中二年级','高中三年级',
                             max(b.factor_value))  
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code in ( 'FC100352')) 年级,
       (select decode(max(b.factor_value),'FC100353002','1班','FC100353003','2班','FC100353004','3班','FC100353005','4班',
                             'FC100353006','5班','FC100353007','6班','FC100353008','7班','FC100353009','8班',
                             'FC100353010','9班','FC100353011','10班','FC100353012','11班','FC100353013','12班',
                             'FC100353014','13班','FC100353015','14班','FC100353016','15班','FC100353017','16班','FC100353018','17班',
                             'FC100353019','18班','FC100353020','19班','FC100353021','20班','FC100353022','21班',
                       

      'FC100353023','22班','FC100353024','23班','FC100353025','24班','FC100353026','25班',
                             'FC100353027','26班','FC100353028','27班','FC100353029','28班','FC100353030','29班','FC100353031','30班','FC100353032','其它',
                             max(b.factor_value)) 
          from "ids_wuh".nonauto_plc_element_t b
         where b.policy_no=a.policy_no
           and b.factor_code in ('FC100353')) 班级 
from wuh_dw.etl_nonauto_policy_t a
where a.operator_code in ('015188','003097')
and a.product_code in ('24489800','1107A000')
and a.department_name='蕲春支公司'and to_char(a.inception_date,'yyyy')='2025'第二种：select t.policy_no 保单号,t.policy_application_no 投保单号,t.department_group_name 部门组,t.department_group_code 部门组代码,t.agent_name 代理点,t.agent_code 代理点代码,
       t.department_name 部门,t.section_office_name 科室,t.kq 客群,t.operator_name 经办人,t.operator_code 经办人工号,t.product_code 险种代码,t.product_name 险种名称,
```
