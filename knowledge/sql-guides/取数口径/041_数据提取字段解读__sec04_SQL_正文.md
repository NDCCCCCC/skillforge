---
title: 数据提取字段解读
source_idx: 041
section: SQL / 正文
---

## SQL / 正文

```sql
```

若为交强险，则再保自留比例=1非车险：若险种代码指认为政保客户，则再保自留比例=1；否则再保自留比例=0.8农险：若险种代码指认为农险，则再保自留比例=0.73分保和再保的操作不是分公司层面操作的，而应该是总公司统一执行的，根本目的还是为了降低或转嫁风险理赔摊回应收看共保保单对应跟单方的理赔摊回应收情况根据跟单方名称查到[某范围]的保单号、立案号根据保单号、立案号可以查到赔款金额根据保单号查到其共保比例，计算出跟单方对应的应付理赔金额最后根据赔案表里的负数赔款计算出对应的理赔摊回应收（最大的问题是如何确定对应的跟单方）下面直接取收付系统的表，但待付款类型只能根据赔案号的项目类型来看（个人感觉理论上应该是根据待付款流水号来匹配）--CIBS共保表

```sql
SELECT * FROM COINSURANCE_T WHERE  policy_no = 'AWUHC1923421Q000006L'  --赔案表
SELECT policy_no,INSURED_OBJECT_INDEMNITY,CALCULATION_FORMULA FROM NONAUTO_CF_INDEMNITY_T WHERE indemnity_no = 'EWUHC192342200000003'--待付款表
SELECT SUM(DEALED_AMOUNT-PENDING_AMOUNT) 理赔应收 FROM PAYMENT_GENERATION_T WHERE pending_type in ('386','396') and pending_cause='1'

 AND client_name LIKE '%众诚汽车保险股份有限公司%'已决和未决：根据保单号汇总 / 这里也可以根据立案号来查，根据情况汇总车险：需要注意已决和未决要取同一期，即未决qsh=7时，赔付日期(posted_date)要卡7.31之前
SELECT a.policy_no, ( a.insured_object_indemnity+a.investigation_fee +a.investigation_agent_fee+a.inside_investigation_agent_fee +a.claim_audit_fee+a.salvage_fee +a.other_fee -a.salvage_recovery) 已决金额, (select min(gsje) from INDISPUTE_T b where b.polno=a.policy_no and b.nf=2020 and b.qsh=4) 未决金额 //选择当年最新的期数
FROM AUTO_CF_INDEMNITY_T a估损金额
