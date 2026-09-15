---
title: 数据提取字段解读
source_idx: 041
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文




select a.保单号,sum(case when b.product_code in ('11024430','11024530','11024620','11024720') then insured_amount else 0 end) 三者险保额,sum(case when b.product_code in ('11024430','11024530','11024620','11024720') then premium_amount else 0 end) 三者险保费,sum(case when b.product_code ='11024411' then premium_amount else 0 end) 玻璃险保费,sum(case when b.product_code = ('11024424') then premium_amount else 0 end) dqxbf,sum(case when b.product_code = ('11024415') then premium_amount else 0 end) ssxbf,sum(case when b.product_code = ('11024412') then premium_amount else 0 end) zrxbf,sum(case when b.product_code = ('11024442') then premium_amount else 0 end) 三责节假日翻倍险,sum(case when b.product_code = ('11024810') then premium_amount else 0 end) jdcssbxbffrom tmp_ycb_003 a,AUTO_COVERAGE_COMPONENT_T bwhere a.保单号=b.policy_nogroup by a.保单号;RPA或者Python自动化的时候需要设置安全站点以及ActiveX弹出策略IDS里没有通赔数据，m6有下发通赔数据但无驾驶员信息ML_SADMDA_ENDCLAIM_LIST_TPST 时效通赔已决ML_SADMDA_UNENDCLAIM_LIST_TPST 时效通赔未决求最大共保单位方和共保自留比例车险：AUTO_COINSURANCE_T非车：RYX_COINSURANCE_T(人意险根据 bdh)、COINSURANCE_T(CIBS根据 policy_no)其中分公司自留比例就是上面两个表中的份额，有多少份额就表示有多少占比（或者非车表里也有coinsurance_percent）最大共保单位方就是份额最大的单位（人意险根据'主出单方标志':ZCDFBZ，CIBS根据'是否出单方':if_sign_out_company）共保和分保（分保又叫再保，临分属于其中一种再保方式的专业术语）比如某工程保费1000W，几家公司共同承保，太保占10%自留比例，那共保保费就是100W（这也是下发给我们的保费，是乘了自留比例的），但由于风险太大，按照规定需要继续分保给其他保险公司承保，此时若太保的再保自留比例是50%，那再保后保费就是50W（这也是财务需要的数据）reinsurance_t（再保表）. reinsurance_percent_2指的是我们将共保比例继续分给其他公司承保后所占的比例，再保表里有所有的保单，是分保后的再保自留比例（财务一般要这个，但是该表数据不全！！！）保费规模看再保前，效益类指标计算结果通常会剔除再保影响车险：若险种代码指认为商业车险，则再保自留比例=0.9132;
