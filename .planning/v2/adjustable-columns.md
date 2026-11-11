# 可调槽位白名单候选（语料统计）

> 本文件由 `go run ./cmd/sqlguard-columns` 生成，是**候选清单**，不是授权。
> 把某一列加进 `internal/sqlguard.AdjustableColumns` 属于扩大授权面，
> 必须人工确认（PR 审核）——见 `.planning/v2/DESIGN.md` §8.1。

- 扫描目录：`knowledge/sql-guides`
- Markdown 文件：202；可解析 SQL 块：124；解析失败跳过：0
- 不同谓词列：61（其中已在白名单：11）

| 列 | 谓词数 | 带字面量 | 值类型 | 已在白名单 | 语料里的值样例 |
|---|---:|---:|---|---|---|
| `xzdl` | 11 | 11 | string | — | `'农险'` / `'农险' ; select*from ids_wuh . dwd_ag_cbfhqd_t t w…` / `'非车险'` |
| `inception_date` | 10 | 10 | date, string | ✅ | `'2025' 第二种：select t.policy_no 保单号, t.policy_appl…` / `to_date ('2024-01-01 00:00:00', 'yyyy-mm-dd hh24…` / `to_date ('20240101 00:00:00', 'yyyymmdd hh24:mi:…` |
| `lines` | 8 | 8 | string | — | `'渠道管理部' AND label_name<>'专属险种' AND xzhdlmc='十、短期…` |
| `department_group_name` | 7 | 7 | list, string | ✅ | `'业务'` / `'襄阳中心支公司'` / `'随州中心支公司' ;` |
| `product_code` | 6 | 6 | list | ✅ | `('23XK9800', '23C49800') ; drop table tmp_ycb_00…` / `('24489800', '1107A000'` / `('24489800', '1107DS00', '1107IX00'` |
| `section_office_code` | 6 | 6 | list | — | `('OVD', 'CCI') ;` / `('OVD', 'CCI') ; update etl_NONAUTO_POLICY_T set…` / `('OVD', 'CCI') ; update etl_policy_request_issue…` |
| `policy_no` | 6 | 5 | list, string | ✅ | `'AWUHBSGJ0125QAAAAA2C' SELECT t.policy_no 保单号, t…` / `('AWUH00HE0618B000007R', 'AWUH00HE0618B000023V',…` / `('AWUHH631EY25FN00000D', 'AWUHH631EY25FN00000E',…` |
| `department_code` | 4 | 4 | list, string | — | `'06X'` / `('050', '06X', 'C19'` |
| `product_name` | 4 | 4 | list, string | ✅ | `'%意外伤害医疗%' select product_code, product_name, po…` / `'机动车交通事故责任强制保险条款（2020版）'` / `'雇主责任险'` |
| `blanket_customer_code` | 3 | 3 | string | — | `'%DST00%'` / `'DST00'` |
| `nf` | 3 | 3 | string | — | `'2020'` / `'2025'` / `'2026'` |
| `qsh` | 3 | 3 | string | — | `'08'` / `'12'` |
| `agent_code` | 3 | 3 | list, string | ✅ | `'100EI7'` / `'100EI7' 关于提取武汉中心支公司团客全险种承保理赔数据的需求具体提取内容如下：1、时间：…` / `('431', '100AG2'` |
| `class_name` | 3 | 3 | string | — | `'团客'` / `'团客'),` |
| `dd` | 6 | 2 | number | — | `20260101` / `20260228` |
| `estimate_date` | 3 | 2 | string | — | `'20210331' ; DROP TABLE TMP_WANG_0022 PURGE ; CR…` / `'20260131' ; drop table tmp_ycb_003 ;` |
| `premium_amount` | 2 | 2 | number | — | `10000` / `10000 DELETE FROM FGS_FG_FXQ_Result_CB WHERE wor…` |
| `plate_number` | 2 | 2 | list | — | `('鄂AA16007', '鄂W675E6', 'LS029799'` / `('鄂AW1111', '鄂AS0R15', '鄂AFC8383', '鄂AU43V7', '鄂…` |
| `department_name` | 2 | 2 | string | ✅ | `'公安支公司'` / `'蕲春支公司'` |
| `yf` | 2 | 2 | string | — | `'06' select a.product_code, a.product_name, sum …` / `'6' select a.product_code, a.product_name, a.pol…` |
| `issue_date` | 2 | 2 | date | ✅ | `TO_DATE ('2025-01-01 00:00:00', 'yyyy-mm-dd hh24…` / `to_date ('2026-01-01 00:00:00', 'YYYY-MM-DD HH24…` |
| `endorsement_no` | 2 | 2 | string | — | `'无'` / `'无' ;` |
| `section_office_name` | 2 | 2 | list, string | ✅ | `'%营销服务部%'` / `('竹溪支公司', '远安支公司'` |
| `department_group_code` | 2 | 1 | string | ✅ | `'420100'` |
| `赔付金额` | 1 | 1 | number | — | `0 ; drop table tmp_ycb_001 ; create table tmp_yc…` |
| `operator_code` | 1 | 1 | list | ✅ | `('015188', '003097'` |
| `pending_type` | 1 | 1 | list | — | `('386', '396'` |
| `pending_cause` | 1 | 1 | string | — | `'1'` |
| `standard_credential_type` | 1 | 1 | string | — | `'111'` |
| `dept_code` | 1 | 1 | list | — | `('050', '06X', 'C19'` |
| `endorseno` | 1 | 1 | null | — | `null 方案二 select a.policy_no 保单号, a.product_name …` |
| `is_lastest` | 1 | 1 | string | — | `'1' 学平险相关被保人清单：human_body_t学校班级信息：nonauto_plc_el…` |
| `posted_date` | 1 | 1 | date | — | `to_date ('2025-12-31 23:59:59', 'yyyy-mm-dd hh24…` |
| `rzbf_notax` | 1 | 1 | number | — | `0 ; update tmp_ycb_002 a set 部门组名称='业务', 部门代码=(s…` |
| `client_name` | 1 | 1 | string | — | `'%国任财产保险股份有限公司武汉中心支公司%' select t.nf 年, t.qsh 月, …` |
| `table_name` | 1 | 1 | string | — | `'pro_qdrbt0_bt01_gfc_and_xny' ; IF v_count=0 THE…` |
| `commissiontype` | 1 | 1 | list | — | `('0', '1'` |
| `vin` | 1 | 1 | list | — | `('LM8F7D79XNA035265', 'LSVZM60T3N2093033'` |
| `table_schema` | 1 | 1 | string | — | `'wuh_dw'` |
| `indemnity_no` | 1 | 1 | list | — | `(SELECT indemnity_no FROM fr_fgs_cw_gbth_delete_…` |
| `month` | 1 | 1 | string | — | `'2026-04' 5.5 条件执行所有后置动作都支持条件（同模板语法）：条件说明status=…` |
| `partner_name` | 1 | 1 | string | — | `'%官微%' or b.partner_name like '%码上保%'` |
| `inception_year` | 1 | 1 | string | — | `'2025'` |
| `stats_yearmonth` | 1 | 1 | string | — | `'' select org_info 机构, sum (corporate_income_tax…` |
| `role` | 1 | 1 | string | — | `'1'` |
| `totallossreductionamount` | 1 | 1 | number | — | `0` |
| `kq_new` | 1 | 1 | list | — | `('政保客户', '0', '政保健康'` |
| `itemcode` | 1 | 1 | string | — | `'LHEL033C'` |
| `rn` | 1 | 1 | number | — | `1 `` |
| `lastendcasetime` | 1 | 1 | date | — | `TO_DATE ('20241201 00:00:00', 'YYYYMMDD hh24:mi:…` |
| `signature_date` | 4 | 0 | — | — | — |
| `polno` | 3 | 0 | — | — | — |
| `保单号` | 2 | 0 | — | — | — |
| `business_date` | 2 | 0 | — | — | — |
| `sign_year` | 2 | 0 | — | — | — |
| `claim_folder_no` | 1 | 0 | — | — | — |
| `评估时点` | 1 | 0 | — | — | — |
| `lastquotationdate` | 1 | 0 | — | — | — |
| `评估时点1` | 1 | 0 | — | — | — |
| `workdate` | 1 | 0 | — | — | — |
| `sequenceno` | 1 | 0 | — | — | — |

## 怎么用

1. 挑出「带字面量」且尚未在白名单里的列，判断它是否属于 机构/部门/人员、险种、时间 这三类业务槽位。
2. 是 → 在 `internal/sqlguard/verify.go` 的 `AdjustableColumns` 里加一行，并在 commit message 说明语料依据。
3. 不是（例如 `status`、`internal_flag` 这种口径字段）→ 保持不在白名单，R4 会把它记为告警。

## 白名单里有、但本次语料里没出现过的列

可能来自人工判断或别的语料来源；保留还是清理需要人工确认。

- `agent_name`
- `application_date`
- `kq`
- `operator_name`
- `planned_end_date`
- `policy_application_no`
