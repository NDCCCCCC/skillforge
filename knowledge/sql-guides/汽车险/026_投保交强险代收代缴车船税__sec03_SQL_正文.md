---
title: 投保交强险代收代缴车船税
source_idx: 026
section: SQL / 正文
---

## SQL / 正文



```sql
select 
a.tb_credential_no as 投保人证件号码
--,a.tb_credential_type as 投保人证件类别
,c.STANDARD_CREDENTIAL_TYPE as 投保人证件类型
,(CASE 
    -- 个人证件类型
    WHEN c.STANDARD_CREDENTIAL_TYPE = '111' THEN '居民身份证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '113' THEN '户口簿'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '114' THEN '中国人民解放军军官证(包含军官证、士兵证)'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '133' THEN '学生证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '211' THEN '离休证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '335' THEN '机动车驾驶证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '411' THEN '外交护照'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '414' THEN '普通护照'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '418' THEN '外国人旅行证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '422' THEN '澳门居民身份证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '511' THEN '台湾居民来往大陆通行证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '513' THEN '往来港澳通行证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '516' THEN '港澳居民来往内地通行证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '517' THEN '大陆居民往来台湾通行证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '521' THEN '香港居民身份证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '550' THEN '港澳台居民居住证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '553' THEN '外国人永久居留身份证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '554' THEN '外国人居留证或居留许可'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '990' THEN '其他'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '901' THEN '武装警察身份证件'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '902' THEN '社保证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '903' THEN '异常身份证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '904' THEN '少儿证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '999' THEN '其他（上游证件类型为空或未映射到）'
    -- 组织机构证件类型
    WHEN c.STANDARD_CREDENTIAL_TYPE = '1' THEN '组织机构代码证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '2' THEN '税务登记证'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '3' THEN '营业执照'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '4' THEN '事业单位法人证书'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '5' THEN '社会团体��人证书'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '6' THEN '民办非企业单位登记证书'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '8' THEN '工商注册号码'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '9' THEN '其他证件'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '10' THEN '统一社会信用代码'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '11' THEN '内部编码'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '12' THEN '企业代码'
    WHEN c.STANDARD_CREDENTIAL_TYPE = '13' THEN '团体其他'
    ELSE '未知证件类型'
END ) as 投保人证件类型
,a.policy_no as 保单号
,a.tb_name as 投保人
,a.INCEPTION_DATE as 起保日期
,a.PLANNED_END_DATE as 终保日期
,a.premium_amount - a.tax_amount 签单保费
,b.travel_tax as 车船税
,a.product_name as 险种名称
,a.DEPARTMENT_GROUP_NAME as 中支
,a.DEPARTMENT_NAME as 部门
from wuh_dw.etl_policy a

left join (select policy_no, sum(travel_tax) as travel_tax from wuh_dw.etl_payment_due_t group by policy_no) b on a.policy_no = b.policy_no
left join ids_wuh.party_t c on a.policy_no = c.policy_no
where a.INCEPTION_DATE between to_DATE('20220901 00:00:00','yyyymmdd hh24:mi:ss')
and to_DATE('20250831 23:59:59','yyyymmdd hh24:mi:ss')
and a.product_name = '机动车交通事故责任强制保险条款（2020版）'
and c.role = '1'
and c.STANDARD_CREDENTIAL_TYPE != '111'
```
