---
title: 二层宽表存储过程
source_idx: 039
section: SQL / 正文
---

## SQL / 正文

```sql
  
	---车险加上生效日期、签发日期v_request_issue_t4

  insert into v_request_issue_t4
    select a.*
           ,  c.request_date
           --, (SELECT b.signature_date FROM ids_wuh.AUTO_AGREEMENT_REQUEST_T b WHERE b.policy_no=a.policy_no AND b.endorsement_no=a.endorsement_no) signature_date
           , c.requested_date
           , to_char(a.signature_date,'yyyymmdd')as business_date --, GREATEST(to_char(a.request_date,'yyyymmdd'),a.workdate_premium) business_date
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),1,4) as nf
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),5,2) as yf
           , SUBSTR(to_char(a.signature_date,'yyyymmdd'),7,2) as dd
           , c.request_reason
           , c.approval_document
           , b.CPICSCORE 
           , b.INDEPENDENTPRICERATE
           , b.LASTNCDRATE
           , b.NONCLAIMDISCOUNTRATE 
           , b.policy_costrate
           , b.PURERISK_PREMIUM
           , b.ACTUALDISCUTE_RATE
           , b.underwritingrate
           , b.channelrate
           , b.RULE_GROUP_NO
           , b.RULE_GROUP_NAME
           , b.TOTALCPICSCORE
           , b.TOTALECOMPENSATION_RATE
           , b.business_fee
           , b.changeable_feerate
           , b.ecompensation_rate
           , b.fixed_feerate
           , b.goalcompenstation_rate
           , b.lastyear_acctimes
    from v_request_issue_t3 a 
    LEFT JOIN ids_wuh.AUTO_NEW_PRODUCT_INFO_T b
    ON a.policy_no = b.policy_no AND a.endorsement_no = b.endorsement_no
    left join ids_wuh.AUTO_AGREEMENT_REQUEST_T c
    on a.policy_no =c.policy_no AND a.endorsement_no =c.endorsement_no;
  v_request_issue_t_num4 := sql%rowcount;
  if v_request_issue_t_num3 <> v_request_issue_t_num4 then
    RETURN;
  end if;
	

  ---合并非车车险签单明细v_request_issue_t5

	insert into v_request_issue_t5 SELECT * FROM v_request_issue_t2 UNION SELECT * FROM v_request_issue_t4;
	
	/*
	--request_date跨年的先把business_date设置为request_date
	--(但是business_date到底算到哪个月，需要核实)
	UPDATE v_request_issue_t5
  SET business_date = to_char(request_date,'yyyyMMdd'),
	    nf = to_char(request_date,'YYYY'),
		  yf = to_char(request_date,'MM'),
			dd = to_char(request_date,'dd')	
  WHERE to_char(request_date,'YYYY') < SUBSTR(workdate_request,1,4);
  
	*/

	
  --所有险种直接卡签发日期，business_date设置为signature_date
	/*UPDATE v_request_issue_t5
  SET business_date = to_char(signature_date,'yyyyMMdd'),
	    nf = to_char(signature_date,'YYYY'),
		  yf = to_char(signature_date,'MM'),
			dd = to_char(signature_date,'dd');	
```
