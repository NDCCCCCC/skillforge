---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
    
  v_party_t_num := sql%rowcount;
  if v_MOTORISED_VEHICLE_T_num <> v_party_t_num then
    RETURN;
  end if;
	
	--新增保单��展表等字段

  insert into v_auto_agreement_t6
    select a.*
           , (select sum(b.TAX_AMOUNT) from ids_wuh.auto_premium_t b where b.policy_no=a.policy_no GROUP BY a.policy_no) tax_amount 
           , (SELECT b.auto_name FROM ids_wuh.AUTO_STORE_T b WHERE b.auto_code = a.selling_shop_code)  selling_shop_name
           , b.PRIVATECARSCORE 
           , b.PRIVATECARSPREADING 
           , b.SECURITY_FLAG 
           , b.partner_code
           , b.partner_name
           , b.tppartner_code 
           , b.DAMAGECARDEDUCTIBLES
           , b.DAMAGECARDEDUCTIBLESRATE 
           , t.CPICSCORE
           , t.INDEPENDENTPRICERATE
           , t.LASTNCDRATE
           , t.NONCLAIMDISCOUNTRATE
           , t.policy_costrate
           , t.PURERISK_PREMIUM
           , t.underwritingrate
           , t.CHANNELRATE
           , t.ACTUALDISCUTE_RATE
      from v_party_t a
      left join ids_wuh.AUTO_AGREEMENT_EXTEND_T b on a.policy_no = b.policy_no
      left join ids_wuh.AUTO_NEW_PRODUCT_INFO_T t on a.policy_no = t.policy_no and t.endorsement_no = '无';
  
  v_auto_agreement_t6_num := sql%rowcount;
  if v_auto_agreement_t6_num <> v_party_t_num then
    RETURN;
  end if;
```
