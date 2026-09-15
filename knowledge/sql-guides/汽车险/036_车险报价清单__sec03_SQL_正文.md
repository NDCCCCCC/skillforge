---
title: 车险报价清单
source_idx: 036
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



select t.*
  from (select LICENCENO 车牌号,
               VIN 车架号,
               MODELNAME 车型名称,
               to_date(REGISTERDATE, 'yyyy-mm-dd hh24:mi:ss') 初登日期,
               PURCHASEPRICE 新车购置价,
               CURRENTVALUE 协商实际价值,
               LICENSEOWNER 车主名称,
               DEPTGROUPCODE 中支,
        (select t.department_group_name from bmz_t t where t.department_group_code=a.DEPTGROUPCODE) 中支名称,
               DEPTCODE 部门,
        (select t.department_name from bm_t t where t.department_code=a.DEPTCODE) 机构,
               THE4SCODE 渠道合作代码,
               TELPARTNERCODE 合作伙伴代码,
               AGENTCODE 代理点代码,
               TRANSACTORCODE 经办人,
               TELSALESMANCODE 电销坐席工号,
               OPTCODE 报价人员,
               BUSINESSUSAGE 商业使用性质,
               BUSSINSURANCEFLAG 商业险新转续标志,
               SELLINGCHANNEL 展业方式,
               (select b.name
                  from hb_channel b
                 where a.SELLINGCHANNEL = b.code) 展业方式名称,
               ORIGINTYPE 系统来源,
               SUBORIGINTYPE 子系统来源,
               to_date(FIRSTQUOTATIONDATE, 'yyyy-mm-dd hh24:mi:ss') 初次报价时间,
               to_date(LASTQUOTATIONDATE, 'yyyy-mm-dd hh24:mi:ss') 最新报价时间,
               BUSSAPPLICATIONNO 商业险投保单号,
               BUSSINESSQUERYSEQUENCENO 商业险投保查询码,
               to_date(BUSSSTARTDATE, 'yyyy-mm-dd hh24:mi:ss') 商业险起保时间,
               to_date(BUSSENDDATE, 'yyyy-mm-dd hh24:mi:ss') 商业险终保时间,
               BUSSFLOATINGRATE 商业险保单折扣,
               NEWCHANNEL 新渠道分类代码,
               BUSSVEHICLECLAIMTYPE 商业险费赔标签,
               TRAFVEHICLECLAIMTYPE 交强险费赔标签,
               BUSSEXPECTEDLOSSRATIO 商业险预期赔付率,
               TRAFAPPLICATIONNO 交强险投保单号,
               TRAFFICQUERYSEQUENCENO 交强险投保查询码,
               to_date(TRAFSTARTDATE, 'yyyy-mm-dd hh24:mi:ss') 交强险起止时间,
               to_date(TRAFENDDATE, 'yyyy-mm-dd hh24:mi:ss') 交强险终止时间,
               TRAFAMOUNT 交强险签单保费,
               TRAFEXPECTEDLOSSRATIO 交强险预期赔付率,
               SECTIONCODE 科室代码,
               UNIFIEDSECURITYCODE 统保代码,
               APPLICANTNAME 投保人名称,
               /*APPLICANTCUSTOMERTYPE 投保人客户类型,
               APTCERTIFICATECODE  投保人证件号码,*/
               INSUREDNAME 被保险人名称,
               /*INSUREDCUSTOMERTYPE 被保险人客户类型,
               INSCERTIFICATECODE  被保险人证件号码,*/
               CLAIMHOLDERNAME 索赔权益人名称,
               /*CLAIMHOLDERCUSTOMERTYPE 索赔权益人客户类型,
               CLAIMHOLDERCERTIFICATECODE  索赔权益人证件号码,
               LICENSECUSTOMERTYPE 行驶证车主客户类型,
               LICENSEIDNUMBER 行驶证车主证件号码,*/
               TRAFINSURANCEFLAG 交强新转续,
               BUSSSTANDARDPREMIUM 商业险标准保费,
               SCHEDULEDPREMIUM 交强险标准保费,
               BUSSAMOUNT 商业险保单保费,
               UNDERWRITINGRATE 自主核保系数,
               SELFCHANNELCOE 自主渠道系数,
               BUSSTRAFFICTRANSGRESSRATE 交通违法系数,
               BUSSQUOTATIONSCHEME 报价方案,
               TARGETPOLICYCOSTRATE 目标保单成本率,
               DEFAULTCOSTRATE 预设费用率,
               DEFAULTTOTALDISCOUNT 预设总折扣,
               BUSINESSCPICSCORE 商业险

```

保分,

               TRAFFICCPICSCORE 交强险险太保分,
               TOTALCPICSCORE 交商合计太保分,
               TRAFFICUSAGE 交强险使用性质,
```
