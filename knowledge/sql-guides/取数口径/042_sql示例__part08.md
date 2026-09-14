---
title: sql示例
source_idx: 042
split_part: 8/13
source_split: true
---

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


-- `jeecg-boot`.hubei_tkzh_dxzbj_cpzrx definition

CREATE TABLE `hubei_tkzh_dxzbj_cpzrx` (
  `id` varchar(36) NOT NULL COMMENT 'ID',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_mrrsshmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每人人身伤害每次事故赔偿限额(万元)',
  `s_rsshccssmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '人身伤害财产损失每次事故赔偿限额(万元)',
  `s_mrccmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每人财产每次事故赔偿限额(万元)',
  `s_ccssmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '财产损失每次事故赔偿限额(万元)',
  `s_mcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每次事故赔偿限额(万元)',
  `s_ljpcxe` decimal(30,2) DEFAULT NULL COMMENT '累计赔偿限额(万元)',
  `s_xse` decimal(30,2) DEFAULT NULL COMMENT '销售额(万元)',
  `s_xsqy` varchar(150) DEFAULT NULL COMMENT '销售区域',
  `s_bz` varchar(150) DEFAULT NULL COMMENT '币种',
  `s_cpmc` varchar(150) DEFAULT NULL COMMENT '产品名称',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--产品责任险表';


-- `jeecg-boot`.hubei_tkzh_dxzbj_dlkycyrzr definition
