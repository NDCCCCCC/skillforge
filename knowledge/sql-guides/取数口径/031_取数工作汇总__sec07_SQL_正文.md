---
title: 取数工作汇总
source_idx: 031
section: SQL / 正文
---

## SQL / 正文

```sql
from INDISPUTE_T t,etl_policy@ora_247 s
where t.nf='2020'
and t.qsh='12'
and t.polno=s.policy_no
;---- 未决清单

SELECT 
    t.nf AS 年,
    t.qsh AS 月,
    t.caseno AS 报案号,
    t.indemno AS 赔案号,
    t.bzhdm AS 币种代码,
    t.gsje AS 年末估损金额,
    t.xzhdm AS 险种代码,
    t.polno AS 保单号,
    t.qbrq AS 起保日期,
    s.ISSUE_DATE AS 签单日期,
    coinsurance_indicator_name AS 共保标志,
    COINSURANCE_PERCENT AS 共保比例,
    s.SELLING_CHANNEL_NAME AS 展业方式,
    t.zhzhrq AS 终止日期,
    t.chxrq AS 出险日期,
    t.larq AS 立案日期,
    t.bbrmc AS 被保险人名称,
    t.gsry AS 估损人员,
    t.gsrymc AS 估损人员姓名,
    t.ywry AS 业务人员,
    t.ywrymc AS 业务人员名称,
    t.bmdm AS 部门代码,
    t.bmmc AS 部门名称,
    t.kshdm AS 科室代码,
    t.kshmc AS 科室名称,
    t.dlddm AS 代理点代码,
    t.dldmc AS 代理点名称,
    t.zlje AS 自留金额,
    s.xzdl AS 险种大类,
    NULL AS 使用性质,
    NULL AS 车辆类型
FROM 
    INDISPUTE_T t,
    etl_nonauto_policy_t@ora_247 s
WHERE 
    t.nf = '2025'
    AND t.qsh = '08'
    AND t.polno = s.policy_no
    AND s.xzdl = '农险'

UNION ALL

SELECT 
    t.nf AS 年,
    t.qsh AS 月,
    t.caseno AS 报案号,
    t.indemno AS 赔案号,
    t.bzhdm AS 币种代码,
    t.gsje AS 年末估损金额,
    t.xzhdm AS 险种代码,
    t.polno AS 保单号,
    t.qbrq AS 起保日期,
    s.ISSUE_DATE AS 签单日期,
    coinsurance_indicator_name AS 共保标志,
    COINSURANCE_PERCENT AS 共保比例,
    s.SELLING_CHANNEL_NAME AS 展业方式,
    t.zhzhrq AS 终止日期,
    t.chxrq AS 出险日期,
    t.larq AS 立案日期,
    t.bbrmc AS 被保险人名称,
    t.gsry AS 估损人员,
    t.gsrymc AS 估损人员姓名,
    t.ywry AS 业务人员,
    t.ywrymc AS 业务人员名称,
    t.bmdm AS 部门代码,
    t.bmmc AS 部门名称,
    t.kshdm AS 科室代码,
    t.kshmc AS 科室名称,
    t.dlddm AS 代理点代码,
    t.dldmc AS 代理点名称,
    t.zlje AS 自留金额,
    s.xzdl AS 险种大类,
    NULL AS 使���性质,
    NULL AS 车辆类型
FROM 
    INDISPUTE_T t,
    etl_nonauto_policy_t@ora_247 s
WHERE 
    t.nf = '2025'
    AND t.qsh = '08'
    AND t.polno = s.policy_no
    AND s.xzdl = '非车险'

UNION ALL

SELECT 
    t.nf AS 年,
    t.qsh AS 月,
    t.caseno AS 报案号,
    t.indemno AS 赔案号,
    t.bzhdm AS 币种代码,
    t.gsje AS 年末估损金额,
    t.xzhdm AS 险种代码,
    t.polno AS 保单号,
    t.qbrq AS 起保日期,
    s.ISSUE_DATE AS 签单日期,
    NULL AS 共保标志,
    NULL AS 共保比例,
    s.SELLING_CHANNEL_NAME AS 展业方式,
    t.zhzhrq AS 终止日期,
    t.chxrq AS 出险日期,
    t.larq AS 立案日期,
    t.bbrmc AS 被保险人名称,
    t.gsry AS 估损人员,
    t.gsrymc AS 估损人员姓名,
    t.ywry AS 业务人员,
    t.ywrymc AS 业务人员名称,
    t.bmdm AS 部门代码,
    t.bmmc AS 部门名称,
    t.kshdm AS 科室代码,
    t.kshmc AS 科室名称,
    t.dlddm AS 代理点代码,
    t.dldmc AS 代理点名称,
    t.zlje AS 自留金额,
    '车险' AS 险种大类,
    usage_name AS 使用性质,
    vehicle_type_name AS 车辆类型
FROM 
    INDISPUTE_T t,
    etl_policy@ora_247 s
WHERE 
    t.nf = '2020'
    AND t.qsh = '12'
    AND t.polno = s.policy_no;
```
