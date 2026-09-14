---
title: sql示例
source_idx: 042
split_part: 10/13
source_split: true
---

CREATE TABLE `hubei_tkzh_dxzbj_gzzrx` (
  `s_xh` bigint(15) DEFAULT NULL COMMENT '序号',
  `s_bfhj` decimal(15,2) DEFAULT NULL COMMENT '保费合计(元)',
  `s_bf` decimal(15,2) DEFAULT NULL COMMENT '保费(元/人)',
  `s_fl` decimal(15,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_wgf` decimal(15,2) DEFAULT NULL COMMENT '误工费(元/每天)',
  `s_gsyl` decimal(15,2) DEFAULT NULL COMMENT '工伤医疗',
  `s_swsc` decimal(15,2) DEFAULT NULL COMMENT '死亡/伤残(万)',
  `s_rs` bigint(15) DEFAULT NULL COMMENT '人数',
  `s_sfgs` varchar(30) DEFAULT NULL COMMENT '是否工伤',
  `s_gzmx` varchar(180) DEFAULT NULL COMMENT '工种明细',
  `s_zylb` varchar(30) DEFAULT NULL COMMENT '职业类别',
  `instance_id` varchar(60) DEFAULT NULL COMMENT '流程实例编号',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  `id` varchar(100) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请-雇主责任险';


-- `jeecg-boot`.hubei_tkzh_dxzbj_gzzrx2 definition

CREATE TABLE `hubei_tkzh_dxzbj_gzzrx2` (
  `id` varchar(36) NOT NULL COMMENT 'ID',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_mrrsshmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每人人身伤害每次事故赔偿限额',
  `s_rsshccssmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '人身伤害财产损失每次事故赔偿限额',
  `s_mrccmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每人财产每次事故赔偿限额',
  `s_ccccssmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '财产财产损失每次事故赔偿限额',
  `s_mcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '每次事故赔偿限额',
  `s_ljpcxe` decimal(30,2) DEFAULT NULL COMMENT '累计赔偿限额',
  `s_mj` decimal(30,2) DEFAULT NULL COMMENT '面积(m²)',
  `s_bxbd` varchar(150) DEFAULT NULL COMMENT '保险标的',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--公众责任险表2';


-- `jeecg-boot`.hubei_tkzh_dxzbj_jgx definition
