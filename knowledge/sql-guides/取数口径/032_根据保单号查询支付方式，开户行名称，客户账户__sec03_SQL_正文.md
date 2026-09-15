---
title: 根据保单号查询支付方式，开户行名称，客户账户
source_idx: 032
section: SQL / 正文
---

## SQL / 正文

```sql
UHL60I6921B000005W','AWUHL60I6921B000006J','AWUHL60I6921B000007F','AWUHL60I6921B000008Y','AWUHL60I6921B000009S','AWUHL60I6921B000010I','AWUHL60I6921B000011K','AWUHL60I6921B000012V','AWUHL60I6921B000013Z','AWUHL60I6921B000013Z','AWUHL60I6921B000014V','AWUHL60I6921B000015I','AWUHL60I6921B000016O','AWUHL60I6921B000017N','AWUHL60I6921B000018H','AWUHL60I6921B000019S','AWUHL60I6921B000020V','AWUHL60I6921B000021L','AWUHL60I6921B000022M','AWUHL60I6921B000023I','AWUHL60I6921B000024V','AWUHL60I6921B000025C','AWUHL60I6921B000026O','AWUHL60I6921B000027N','AWUHL60I6921B000028R','AWUHL60I6921B000029V','AWUHL60I6921B000030N','AWUHL60I6921B000031H','AWUHL60I6921B000032M','AWUHL60I6921B000033Q','AWUHL60I6921B000034B','AWUHL60I6921B000035T','AWUHL60I6921B000036K','AWUHL60I6921B000036K','AWUHL60I6921B000037Q','AWUHL60I6921QAAAAA1O','AWUHL60I6921QAAAAA2O','AWUHLFCI6921QAAAAA1F')
group by b.deal_date,b.payment_kind, b.payment_sequence, b.policy_no

select a.department_group_name,a.policy_no,a.caliberof_new_channels_name,a.caliberofnewchannels_2level,a.total_premium,a.tax_vehicle_amount,a.tax_amount,a.signature_date,
case when b.pay_way='07' then '快钱'
when b.pay_way='09' then '电销通联支付'
when b.pay_way='1' then '支票'
when b.pay_way='2' then '划卡'
when b.pay_way='3' then '银行柜面现金'
when b.pay_way='4' then '现金'
when b.pay_way='5' then '网银转帐'
when b.pay_way='6' then '刷卡'
when b.pay_way='7' then '现金结算'
when b.pay_way='8' then '网上银行'
when b.pay_way='9' then '电话银行 '
when b.pay_way='10' then ' 柜台转账 '
when b.pay_way='11' then '实时代扣 '
when b.pay_way='12' then '支付宝'
when b.pay_way='3A' then ' 集中支付 '
when b.pay_way='chinapay' then '银联电子支付'
when b.pay_way='weixin' then '微信支付'
when b.pay_way='SAICPAY' then '上汽支付'
when b.pay_way='mpay' then '聚合支付'
when b.pay_way='TENPAY' then '财付通'
else '未知' end pay_way,
case when b.paytype='01' then 'POS机刷卡'
when b.paytype='02' then '支付宝'
when b.paytype='03' then '微信'
when b.paytype='04' then '个人转账'
when b.paytype='05' then '银行卡代扣'
when b.paytype='06' then '快捷支付（在线支付）'
when b.paytype='07' then '现金'
when b.paytype='08' then '支票'
when b.paytype='09' then '企业转账'
when b.paytype='10' then '汇票'
when b.paytype='11' then '快钱账户'
when b.paytype='12' then '京东支付'
when b.paytype='99' then '其他'
else '未知' end paytype,a.selling_shop_code,a.operator_name
from hlj.dl_auto_policy_t a,ids.auto_fee_record_t b,ids.auto_fee_check_info_t c
where a.policy_no=b.business_no
and b.pay_app_no = c.pay_app_no
and c.status = '1'
and c.fee_check_info_id is not null
and trunc(a.signature_date) between to_date('20230101','yyyymmdd') and to_date('20231029','yyyymmdd')
```
