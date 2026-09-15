---
title: 取数工作汇总
source_idx: 031
section: SQL / 正文
---

## SQL / 正文

```sql
       t.premium_amount 含税保费,t.tax_amount 税,t.insured_amount 保额,t.application_date 投保日期,t.issue_date 签单日期,t.inception_date 起保日期,
       t.planned_end_date 终止日期,t.tb_name 投保人,t.tb_credential_type 投保人证件类型,t."tb_credential_no" 投保人证件号,t.tb_mobilephone 投保人手机,t.bb_name 被保人,t.bb_credential_type 被保人证件类型,t."bb_credential_no" 被保人证件号,t.plan_name ,t.partner_name,t.diysalecode 自定义编码,
  --c.diysalecode 自定义编码1,
a.factor_value 学校名称,
decode(max(b.factor_value),'FC100352002','其它','FC100352003','1年级','FC100352004','2年级','FC100352005','3年级','FC100352006','4年级',
                             'FC100352007','5年级','FC100352008','6年级',
                             'FC100352009','小班','FC100352010','中班','FC100352011','大班',
                             'FC100352012','初中预科班','FC100352013','初中一年级','FC100352014','初中二年级','FC100352015','初中三年级',
                             'FC100352016','高中预科班','FC100352017','高中一年级','FC100352018','高中二年级', 'FC100352019','高中三年级','FC100352020','托班',
                             max(b.factor_value)) 年级,
decode(max(c.factor_value),'FC100353002','1班','FC100353003','2班','FC100353004','3班','FC100353005','4班',
                             'FC100353006','5班','FC100353007','6班','FC100353008','7班','FC100353009','8班',
                             'FC100353010','9班','FC100353011','10班','FC100353012','11班','FC100353013','12班',
                             'FC100353014','13班','FC100353015','14班','FC100353016','15班','FC100353017','16班','FC100353018','17班',
                             'FC100353019','18班','FC100353020','19班','FC100353021','20班','FC100353022','21班',
                             'FC100353023','22班','FC100353024','23班','FC100353025','24班','FC100353026','25班',
                             'FC100353027','26班','FC100353028','27班','FC100353029','28班','FC100353030','29班','FC100353031','30班','FC100353032','其它',
                             max(c.factor_value)) 班级,
decode(max(d.factor_value),'O2000','大中专学校学生','O2001','中学生','O2002','小学生','O2003','幼儿','O2004','其他',max(d.factor_value)) 学生类别
from wuh_dw.etl_nonauto_policy_t t
left join "ids_wuh".nonauto_plc_element_t a
on a.policy_no = t.policy_no and a.factor_code='M9'
left join "ids_wuh".nonauto_plc_element_t d
on d.policy_no=t.policy_no and d.factor_code in ('O2')
left join "ids_wuh".nonauto_plc_element_t c
on c.policy_no=t.policy_no and c.factor_code in ('FC100353')
left join "ids_

wuh".nonauto_plc_element_t b
on b.policy_no=t.policy_no and b.factor_code in ('FC100352')
WHERE t.inception_date >= DATE(2025-09-01)
and t.operator_code in ('015188','003097')
and t.product_code in ('24489800','13077500')
and t.department_name='蕲春支公司'
GROUP BY 
    t.policy_no,
    t.policy_application_no,
    t.department_group_name,
    t.department_group_code,
    t.agent_name,
    t.agent_code,
    t.department_name,
    t.section_office_name,
    t.kq,
    t.operator_name,
    t.operator_code,
    t.product_code,
    t.product_name,
    t.premium_amount,
    t.tax_amount,
    t.insured_amount,
    t.application_date,
    t.issue_date,
    t.inception_date,
    t.planned_end_date,
    t.tb_name,
    t.tb_credential_type,
    t.tb_credential_no,
    t.tb_mobilephone,
    t.bb_name,
    t.bb_credential_type,
    t.bb_credential_no,
    t.plan_name,
    t.partner_name,
    t.diysalecode,
    a.factor_value;
```
