---
title: sql示例
source_idx: 042
section: SQL / 正文
---

## SQL / 正文

```sql
-- `jeecg-boot`.hubei_tkzh_dxzbj_jzsgqyaq definition

CREATE TABLE `hubei_tkzh_dxzbj_jzsgqyaq` (
  `id` varchar(36) NOT NULL COMMENT '自增主键ID',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_flfy` decimal(30,2) DEFAULT NULL COMMENT '法律费用(万元)',
  `s_qxjyfy` decimal(30,2) DEFAULT NULL COMMENT '抢险救援费用(万元)',
  `s_gywgf` decimal(30,2) DEFAULT NULL COMMENT '雇员/误工费(人/天/元)',
  `s_gyylzrmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/医疗责任/每人每次事故赔偿限额(万元)',
  `s_gyylzrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/医疗责任/每次事故赔偿限额(万元)',
  `s_gyylzrljpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/医疗责任/累计赔偿限额(万元)',
  `s_gyswscmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/死亡伤残/每人每次事故赔偿限额(万元)',
  `s_gyswscmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/死亡伤残/每次事故赔偿限额(万元)',
  `s_gyswscljpcxe` decimal(30,2) DEFAULT NULL COMMENT '雇员/死亡伤残/累计赔偿限额(万元)',
  `s_szccsszrpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/财产损失责任赔偿限额(万元)',
  `s_szylzrmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/每人每次事故赔偿限额(万元)',
  `s_szylzrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/每次事故赔偿限额(万元)',
  `s_szylzrljpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/累计赔偿限额(万元)',
  `s_szswscmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/每人每次事故赔偿限额(万元)',
  `s_szswscmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/每次事故赔偿限额(万元)',
  `s_szswscljpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/累计赔偿限额(万元)',
  `s_gcdz` varchar(150) DEFAULT NULL COMMENT '工程地址',
  `s_gcmc` varchar(150) DEFAULT NULL COMMENT '工程名称',
  `s_gclx` varchar(90) DEFAULT NULL COMMENT '工程类型',
  `s_jzsgqyaqsczrbx` varchar(90) DEFAULT NULL COMMENT '建筑施工企业安全生产责任保险',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--建筑施工企业安全生产责任保险表';


-- `jeecg-boot`.hubei_tkzh_dxzbj_ywx definition

CREATE TABLE `hubei_tkzh_dxzbj_ywx` (
  `id` varchar(36) NOT NULL COMMENT '自增主键ID',
  `s_bfhj` decimal(30,2) DEFAULT NULL COMMENT '保费合计',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元/人)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_zyjt` decimal(30,2) DEFAULT NULL COMMENT '住院津贴(元/每天)',
  `s_ywyl` decimal(30,2) DEFAULT NULL COMMENT '意外医疗(万元)',
  `s_swsc` decimal(30,2) DEFAULT NULL COMMENT '死亡/伤残(万元)',
  `s_rs` bigint(30) DEFAULT NULL COMMENT '人数',
  `s_gzmx` varchar(90) DEFAULT NULL COMMENT '工种明细',
  `s_zylb` varchar(30) DEFAULT NULL COMMENT '职业类别',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--意外险表';
