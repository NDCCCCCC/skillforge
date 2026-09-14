---
title: sql示例
source_idx: 042
split_part: 6/13
source_split: true
---

EFAULT NULL COMMENT '部门',
  `department_group_code` varchar(32) DEFAULT NULL COMMENT '机构',
  `created_by_name` varchar(32) DEFAULT NULL COMMENT '创建人姓名',
  `tyfjtk` text COMMENT '(通用)附加条款',
  `tytbyd` text COMMENT '(通用)特别约定',
  `tymp` text COMMENT '(通用)免赔',
  `tybzmx` text COMMENT '(通用)保障明细',
  `tybxqx` int(11) DEFAULT NULL COMMENT '(通用)保险期限',
  `tybxzq` datetime DEFAULT NULL COMMENT '(通用)保险止期',
  `tybxqq` datetime DEFAULT NULL COMMENT '(通用)保险起期',
  `styxx` text COMMENT '四、通用信息',
  `gcxxzmcqc` varchar(255) DEFAULT NULL COMMENT '(工程险)险种名称全称',
  `ccxxzmcqc` varchar(255) DEFAULT NULL COMMENT '(财产险)险种名称全称',
  `status` varchar(10) DEFAULT NULL COMMENT '状态',
  `tbrdz` varchar(255) DEFAULT NULL,
  `bbrdz` varchar(255) DEFAULT NULL,
  `jzsggczj` varchar(100) DEFAULT NULL COMMENT '(建筑施工)工程造价(元)',
  `bbrtyxyzdm` varchar(100) DEFAULT NULL,
  `jzgcyqxgcjs` varchar(100) DEFAULT NULL,
  `xzmcjzgcyqx` varchar(100) DEFAULT NULL COMMENT '险种名称-工程险',
  `updated_by` varchar(100) DEFAULT NULL,
  `updated_time` varchar(100) DEFAULT NULL,
  `gzygdw` varchar(255) DEFAULT NULL COMMENT '(雇主)用工单位',
  `tyhjzbf` decimal(15,2) DEFAULT NULL COMMENT '(通用)合计总保费',
  `tyxzmc` varchar(100) DEFAULT NULL COMMENT '(通用)险种名称',
  `zjxzttzjbx` varchar(100) DEFAULT NULL COMMENT '追加险种-团体重疾保险',
  `zjxztajjtzhywx` varchar(100) DEFAULT NULL COMMENT '追加险种-家庭综合意外险',
  `sjjgmczgs` varchar(100) DEFAULT NULL COMMENT '四级机构名称',
  `sjjgmc` varchar(100) DEFAULT NULL COMMENT '三级机构名称',
  `wcdyy` varchar(100) DEFAULT NULL COMMENT '未出单原因',
  `ycdbdh` varchar(100) DEFAULT NULL COMMENT '已出单保单号',
  `cdqkfk` varchar(100) DEFAULT NULL COMMENT '出单情况反馈',
  `yjzjxzmc` varchar(100) DEFAULT NULL COMMENT '一键追加险种名称',
  PRIMARY KEY (`id`),
  KEY `idx_business_key` (`business_key`),
  KEY `idx_created_time` (`created_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多险种报价申请';


-- `jeecg-boot`.hubei_tkzh_dxzbj_aqsczrx definition
