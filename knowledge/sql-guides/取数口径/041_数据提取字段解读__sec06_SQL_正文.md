---
title: 数据提取字段解读
source_idx: 041
section: SQL / 正文
---

## SQL / 正文

```sql
第一受益人NONAUTO_BENEFICIARY_T非车通用人员信息表（投被保、受益、雇员以外的人员信息表：被监护人）NONAUTO_PEOPLE_T签单保费 = 原保费 * 实际折扣率=> （premium_amount）TOTAL_PREMIUM = YEAR_PREMIUM * POLICY_ADJUST_RATIO商改前：实际折扣率 = 无赔款优待浮动系数（NCD系数） * 渠道系数 * 自主核保系数AUTO_NEW_PRODUCT_INFO_T：车险商改新产品表ACTUALDISCUTE_RATE：实际折扣率NONCLAIMDISCOUNTRATE：NCD系数（保单表的policy_adjust_ratio）CHANNELRATE：渠道系数UNDERWRITINGRATE：自主核保系数商改后：实际折扣率 = 无赔款优待浮动系数（NCD系数） * 自主定价系数 * 交通违法系数用自主定价系数替代渠道系数和自主核保系数，交通违法系数目前湖北还没有用，全部默认是1跟单费用率：跟单= 手续费+佣金（一般没加）+绩效+业务(SELECT SUM(DEFERCOMMISSIONRATE+DEFER_PERFORMANCE+DELAY_BUSINESS_RATE) from AUTO_NEW_PRODUCT_INFO_T@ORA_241 t2WHERE t1.policy_no=t2.policy_noGROUP BY t1.policy_no) 跟单费用率==总费用=业务费+绩效+手续费(佣金)+增值服务费（代理是人叫佣金，代理是公司叫手续费）==费用版块率值的计算公式综合费用率=(手续费及佣金+税金及附加+业务管理费+资产减值损失+分保费用支出-摊回分保费用)/已赚保费简单费用率=(手续费及佣金+税金及附加+业务管理费+资产减值损失)/入账保费综合费用与简单费用的区别在于(1)简单费用不考虑再保相关费用的影响、(2)分母不同，入账和已赚手续费率=手续费及佣金/入账保费业务及管理费率=业务及管理费/入账保费统筹费用率=统筹业务费/入账保费移动看板采用最新的部门组分类：除了汇总，其他一般用内连接

```sql
with v_case_1 as (/* * 原模板数据集逻辑  * SELECT * FROM ... */)select a.department_group_code AS 帆软部门代码,a.department_group_name AS 帆软部门组名称,a.department_group_name_new AS 部门组,b.*from fr_bmz_t a,v_case_1 bwhere a.department_group_code=b.department_group_code(+)移动看板对应明细报表：返璞归真
select * from ... where workdate = '${workdate}'and 部门组名称 = (select department_group_name from fr_bmz_t b where b.department_group_name_new like '%${部门组}%')根据保单号查支付申请号（用于财猫订单批量匹配）非车见费出单信息表：nonauto_payinfo_t.pay_appno <- 源表 pa_payment_request_info.pay_appno缴费计划表：money_in_scheduler_t.payment_request_no <- 源表 pa_pay_request_detail.payment_request_no其他小知识车辆置换险一般是4S店找我们买的保险(被保人是4S店)，如果客户刚提的新车出问题就需要换新车，主要承担这部分风险责任，一般保费低理赔很少。客户买车之后才会买这个保险，而我们为了录单方便或者冲开门红业绩，会进行批量倒签单，而投保单信息是很早就发给我们了的监护人责任险是给监护人买的保险，如果孩子在学校打架需要赔钱，就可以分担监护人的责任风险非机动车第三者责任保险是指投保对象为残疾人机动轮椅车、电动自行车、电瓶车等卡单业务（非车险）很老的业务但现在还有，基本上都是企财险。例如业务员一次性买100张卡，每张卡50元，然后把卡一张一张卖给客户，卡激活后之前的大保单会批退50元，同时生成一张50元的小保单。被保人是"卡单业务"的就是大保单nonauto_agreement_t.is_policy_card = 1 就是卡单业务LS+5位是兰州陕西军区等车牌、LS+VIN后6位是临牌新车先买保险后上牌，出来行驶证应该马上给他批改的，但一般店里专员懒，都是出险才批改

```
