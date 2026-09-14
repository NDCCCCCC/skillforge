---
title: sql示例
source_idx: 042
split_part: 11/13
source_split: true
---

CREATE TABLE `hubei_tkzh_dxzbj_jgx` (
  `id` varchar(36) NOT NULL COMMENT '自增主键ID',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_zyjt` decimal(30,2) DEFAULT NULL COMMENT '住院津贴(元/每天)',
  `s_ywyl` decimal(30,2) DEFAULT NULL COMMENT '意��医疗(万元)',
  `s_swsc` decimal(30,2) DEFAULT NULL COMMENT '死亡/伤残(万元)',
  `s_gczj` varchar(2400) DEFAULT NULL COMMENT '工程造价',
  `s_gcmc` varchar(150) DEFAULT NULL COMMENT '工程名称',
  `s_gcdz` varchar(150) DEFAULT NULL COMMENT '工程地址',
  `s_jjfs` varchar(30) DEFAULT NULL COMMENT '计价方式',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--建工险';


-- `jeecg-boot`.hubei_tkzh_dxzbj_jqshx definition

CREATE TABLE `hubei_tkzh_dxzbj_jqshx` (
  `id` varchar(36) NOT NULL COMMENT 'id',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` varchar(30) DEFAULT NULL COMMENT '费率',
  `s_be` decimal(30,2) DEFAULT NULL COMMENT '保额(万元)',
  `s_sblx` varchar(30) DEFAULT NULL COMMENT '设备类型',
  `s_sbysynx` varchar(90) DEFAULT NULL COMMENT '设备已使用年限',
  `s_bxbd` varchar(150) DEFAULT NULL COMMENT '保险标的',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`),
  KEY `idx_created_time` (`created_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--机器损坏险';


-- `jeecg-boot`.hubei_tkzh_dxzbj_jzgcyqx definition
