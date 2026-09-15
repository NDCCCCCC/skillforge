---
title: 车险报价清单
source_idx: 036
section: SQL / 正文
---

## SQL / 正文

```sql
               BUSINESSVARIETY 商业险车辆种类,
               TRAFFICVARIETY 交强险车辆种类,
               BRANDNAME 品牌名称,
               BRANDCODE 品牌代码,
               SERIESNAME 车系名称,
               SERIESCODE 车系代码,
               INDUSTRYVEHICLECODE 车型代码,
               BUSINESSMAKERMODEL 商业险厂牌型号,
               TRMAKERMODEL 交强险厂牌型号,
               LOANVEHICLEFLAG 多年车贷投保标志,
               JYCARMODELCATEGORYNAME 精友车型分类名称,
               FLEETAGREEMENTNO 车队协议号,
               BUSSNONCLAIMDISCOUNTRATE 平台返回NCD系数,
               BUSSNONCLAIMFLOATINGREASON 无赔优浮动原因,
               BUSSNONCLAIMNOFLOATINGREASON 无赔优不浮动原因,
               THIRDANDDAMAGESPREADING 三责加车损分档,
               TRAFFICSPREADING 交强分档,
               DAMAGESPREADING 车损分档,
               THIRDPARTYSPREADING 三责分档,
               PRIVATECARSPREADING 私家车分档,
               ONLINECARSPREADING 网约车分级,
               CONSISTENT 车牌车架一致性,
               CARRECORD 车籍,
               DUMPTRAILERFLAG 自卸挂车分类,
               MOLDCHARACTERCODE 精友车型编码,
               BINDINGSELLCOVERAGE 是否投保意外险,
               CHDFACTOR1 车惠达因子1,
               CHDFACTOR2 车惠达因子2,
               CHDFACTOR3 车惠达因子3,
               ONLINECARSPREADINGSOURCE 网约车分级来源,
               BUSSORDERCOMMISSION 商业险跟单手续费,
               BUSSORDERBUSIFEE 商业险跟单业务费,
               BUSSORDERPERFORMANCE 商业险跟单绩效,
               TRAFORDERCOMMISSION 交强险跟单手续费,
               TRAFORDERBUSIFEE 交强险跟单业务费,
               TRAFORDERPERFORMANCE 交强险跟单绩效,
               BUSSSTATISTICPREMIUM 纯风险保费,
               INDEPENDENTPRICERATE 自主定价系数,
               BUSINESSCODE 业务代码,
               NONAUTOSUMPREMIUMEXCLUD 非车险保费,
               NONAUTOSUMPURERISKPREMIUM 非车险纯风险保费,
               NONAUTOSUMESPCOMPENSATIONRATE 非车险预期预期赔付率,
               NONAUTOSUMFEERATE 非车险费用率,
               ISBIGPOLICY 是否大保单,
               ISTRAFFONLY 是否交商共保,
               (select decode(max(s.policy_no), null, '未成交', '成交')
                  from etl_policy@ora_247 s
                 where a.VIN = s.VIN
                   and s.signature_date >
                       sysdate - 30) 是否成交,
               count(1) over(partition by a.VIN ) 报价次数,
               --QUOTATIONNUMBER 报价次数,
               row_number() over(partition by VIN order by lastquotationdate desc) rn
          from TBL_QUOTATION_IDS_MASTER a
         where a.lastquotationdate >
               to_char(sysdate - 30, 'yyyymmdd') || '000000') t
 where t.rn = 1

-----------------------------


```sql
drop table tmp_ycb_001; 
```
