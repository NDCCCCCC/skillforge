---
title: 车险报价清单
source_idx: 036
section: SQL / 正文
---

## SQL / 正文

```sql
```sql
drop table tmp_ycb_002;
create table tmp_ycb_002 nologging as
select distinct a.中支,
                a.中支名称,
                a.部门,
                a.部门名称,
                a.代理点代码,
                a.经办人工号,
                a.经办人,
                a.电销坐席工号,
                a.报价人员工号,
                a.报价人员,
                a.车牌号,
                a.车架号,
                a.车型名称,
                a.初登日期,
                a.新车购置价,
                a.协商实际价值,
                a.车主名称,
                a.商业使用性质,
                max(a.最新报价时间) 最新报价时间
from tmp_ycb_001 a
group by a.中支,
                a.中支名称,
                a.部门,
                a.部门名称,
                a.代理点代码,
                a.经办人工号,
                a.经办人,
                a.电销坐席工号,
                a.报价人员工号,
                a.报价人员,
                a.车牌号,
                a.车架号,
                a.车型名称,
                a.初登日期,
                a.新车购置价,
                a.协商实际价值,
                a.车主名称,
                a.商业使用性质
;

select * from tmp_ycb_002
;

```

查询交商合计NCD标费用预赔，末次报价自主系数，交商合计太保分

```sql
select b.total_bfyp_ncd 交商合计NCD标费预赔,a.* from (
select
vin
,BUSINESSCPICSCORE 商业险太保分
,TRAFFICCPICSCORE 交强险险太保分
,TOTALCPICSCORE 交商合计太保分
,BUSSNONCLAIMDISCOUNTRATE 平台返回NCD系数,
UNDERWRITINGRATE 自主核保系数,
SELFCHANNELCOE 自主渠道系数,
INDEPENDENTPRICERATE 自主定价系数
,row_number() over(partition by VIN order by lastquotationdate desc) rn
from ids_wuh.TBL_QUOTATION_IDS_MASTER
--where lastquotationdate > to_char(sysdate - 30, 'yyyymmdd') || '000000'
) a
left join ids_wuh.ix_quotation_transrate_list b on a.vin = b.vin
where a.vin in ('LM8F7D79XNA035265','LSVZM60T3N2093033')
--where a.vin = 'LC0CE6CD9M1084180'
and a.rn = 1
```
