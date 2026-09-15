---
title: sql示例
source_idx: 042
section: SQL / 正文
---

## SQL / 正文

```sql
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
