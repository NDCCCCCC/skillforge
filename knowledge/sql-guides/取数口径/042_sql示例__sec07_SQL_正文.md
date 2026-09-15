---
title: sql示例
source_idx: 042
section: SQL / 正文
---

## SQL / 正文

```sql
-- `jeecg-boot`.hubei_tkzh_dxzbj_aqsczrx definition


```sql
CREATE TABLE `hubei_tkzh_dxzbj_aqsczrx` (
  `id` varchar(36) NOT NULL COMMENT 'ID',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_mrbf` decimal(30,2) DEFAULT NULL COMMENT '每人保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_flfy` decimal(30,2) DEFAULT NULL COMMENT '法律费用(万元)',
  `s_jdfy` decimal(30,2) DEFAULT NULL COMMENT '鉴定费用(万元)',
  `s_qxjyfy` decimal(30,2) DEFAULT NULL COMMENT '抢险救援费用(万元)',
  `s_szccssfy` decimal(30,2) DEFAULT NULL COMMENT '三者/财产损失费用(万元)',
  `s_szylzrmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/每人每次事故赔偿限额(万元)',
  `s_szylzrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/每次事故赔偿限额(万元)',
  `s_szylzrljpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/医疗责任/累计赔偿限额(万元)',
  `s_szswscmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/每人每次事故赔偿限额(万元)',
  `s_szswscmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/每次事故赔偿限额(万元)',
  `s_szswscljpcxe` decimal(30,2) DEFAULT NULL COMMENT '三者/死亡伤残/累计赔偿限额(万元)',
  `s_cyylzrmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/医疗责任/每人每次事故赔偿限额(万元)',
  `s_cyylzrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/医疗责任/每次事故赔偿限额(万元)',
  `s_cyylzrljpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/医疗责任/累计赔偿限额(万元)',
  `s_cyswscmrmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/死亡伤残/每人每次事故赔偿限额(万元)',
  `s_cyswscmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/死亡伤残/每次事故赔偿限额(万元)',
  `s_cyswscljpcxe` decimal(30,2) DEFAULT NULL COMMENT '从业/死亡伤残/累计赔偿限额(万元)',
  `s_rs` int(11) DEFAULT NULL COMMENT '人数',
  `s_gzxx` varchar(150) DEFAULT NULL COMMENT '工种信息',
  `s_bbxrsclbmc` varchar(90) DEFAULT NULL COMMENT '被保险人生产类别名称',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--安全生产责任险';


-- `jeecg-boot`.hubei_tkzh_dxzbj_ccx definition

CREATE TABLE `hubei_tkzh_dxzbj_ccx` (
  `id` varchar(36) NOT NULL COMMENT 'ID',
  `s_bdnr` varchar(2400) DEFAULT NULL COMMENT '标的内容',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_be` decimal(30,2) DEFAULT NULL COMMENT '保额(万元)',
  `s_bxjeqdfs` varchar(30) DEFAULT NULL COMMENT '保险金额确定方式',
  `s_bxjzqdfs` varchar(30) DEFAULT NULL COMMENT '保险价值确定方式',
  `s_bdlx` varchar(30) DEFAULT NULL COMMENT '标的类型',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`),
  KEY `idx_created_time` (`created_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--财产险';
