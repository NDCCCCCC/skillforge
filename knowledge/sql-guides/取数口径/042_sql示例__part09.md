---
title: sql示例
source_idx: 042
split_part: 9/13
source_split: true
---

CREATE TABLE `hubei_tkzh_dxzbj_dlkycyrzr` (
  `id` varchar(36) NOT NULL COMMENT '自增主键ID',
  `s_bzsm` varchar(2400) DEFAULT NULL COMMENT '备注说明',
  `s_bf` decimal(30,2) DEFAULT NULL COMMENT '保费(元)',
  `s_mzbf` decimal(30,2) DEFAULT NULL COMMENT '每座保费(元)',
  `s_fl` decimal(30,2) DEFAULT NULL COMMENT '费率(‰)',
  `s_fjscry` decimal(30,2) DEFAULT NULL COMMENT '附加司乘人员(万元)',
  `s_xlmzpcxe` decimal(30,2) DEFAULT NULL COMMENT '行李/每座赔偿限额(万元)',
  `s_xlljpcxe` decimal(30,2) DEFAULT NULL COMMENT '行李/累计赔偿限额(万元)',
  `s_xlmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '行李/每次事故赔偿限额(万元)',
  `s_rsmzpcxe` decimal(30,2) DEFAULT NULL COMMENT '人身/每座赔偿限额(万元)',
  `s_rsmcsgpcxe` decimal(30,2) DEFAULT NULL COMMENT '人身/每次事故赔偿限额(万元)',
  `s_rsljpcxe` decimal(30,2) DEFAULT NULL COMMENT '人身/累计赔偿限额(万元)',
  `s_tbzzws` bigint(30) DEFAULT NULL COMMENT '投保总座位数',
  `s_ysbd` varchar(150) DEFAULT NULL COMMENT '运输标的',
  `s_clxz` varchar(90) DEFAULT NULL COMMENT '车辆性质',
  `s_xszcz` varchar(90) DEFAULT NULL COMMENT '行驶证车主',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` datetime DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请--道路客运承运人责任保险表';


-- `jeecg-boot`.hubei_tkzh_dxzbj_gzzrx definition
