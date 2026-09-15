---
title: 数据提取字段解读
source_idx: 041
section: SQL / 正文
---

## SQL / 正文

```sql
SELECT SUM(estimate_amount) FROM AUTO_ESTIMATED_LOSS_T c WHERE c.claim_folder_no =b.indemnity_no AND c.is_lastest='1'学平险相关被保人清单：human_body_t学校班级信息：nonauto_plc_element_t.FACTOR_CODE in（'M9','M9000','MU',MU000'）大非车之前是ryx_plc_element_t (select decode(max(b.factor_value),'FC100352002','其它','FC100352003','1年级','FC100352004','2年级','FC100352005','3年级','FC100352006','4年级',                             'FC100352007','5年级','FC100352008','6年级',                             'FC100352009','小班','FC100352010','中班','FC100352011','大班',                             'FC100352012','初中预科班','FC100352013','初中一年级','FC100352014','初中二年级','FC100352015','初中三年级',                             'FC100352016','高中预科班','FC100352017','高中一年级','FC100352018','高中二年级','高中三年级',                       ��     max(b.factor_value))            from nonauto_plc_element_t@ora_241 b         where b.policy_no=a.policy_no           and b.factor_code in ( 'FC100352')) 年级,       (select decode(max(b.factor_value),'FC100353002','1班','FC100353003','2班','FC100353004','3班','FC100353005','4班',                             'FC100353006','5班','FC100353007','6班','FC100353008','7班','FC100353009','8班',                             'FC100353010','9班','FC100353011','10班','FC100353012','11班','FC100353013','12班',                             'FC100353014','13班','FC100353015','14班','FC100353016','15班','FC100353017','16班','FC100353018','17班',                             'FC100353019','18班','FC100353020','19班','FC100353021','20班','FC100353022','21班',                             'FC100353023','22班','FC100353024','23班','FC100353025','24班','FC100353026','25班',                             'FC100353027','26班','FC100353028','27班','FC100353029','28班','FC100353030','29班','FC100353031','30班','FC100353032','其它',                             max(b.factor_value))           from nonauto_plc_element_t@ora_241 b         where b.policy_no=a.policy_no           and b.factor_code in ('FC100353')) 班级 代步车特性信息表（已停止下发，迁移至CIBS_P_FREIGHT_SPEC_T）CIBS_P_SCOOTER_SPEC_T公路货运承运人责任特性表 ：延保 / 置换险 / 代步车CIBS_P_FREIGHT_SPEC_T：包含大非车数据（原表P_FREIGHT_SPEC属于偏源数据，只包含CIBS数据）船舶险特性表NONAUTO_SHIP_SPEC_T驾乘险特性表NONAUTO_VEHICLE_INFO_T、Agreement_Link_t(保单关联对照表)货运险特性表NONAUTO_CARGO_SPEC_T、p_cargo、NONAUTO_CARGO_AGREEMENT_T(非车货运险协议表)受益人信息表 / 

```
