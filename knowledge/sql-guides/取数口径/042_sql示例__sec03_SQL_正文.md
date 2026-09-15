---
title: sql示例
source_idx: 042
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文



CREATE TABLE `hubei_tkzh_dxzbjsq` (
  `id` varchar(36) NOT NULL COMMENT 'id',
  `business_key` varchar(60) DEFAULT NULL COMMENT '流水号',
  `created_by` varchar(60) DEFAULT NULL COMMENT '创建人',
  `created_time` varchar(60) DEFAULT NULL COMMENT '创建时间',
  `jbxx` varchar(60) DEFAULT NULL COMMENT '一、基本信息：',
  `jgmc` varchar(100) DEFAULT NULL COMMENT '机构名称',
  `jbxm` varchar(300) DEFAULT NULL COMMENT '经办姓名',
  `kqbm` varchar(60) DEFAULT NULL COMMENT '客群部门',
  `qdlx` varchar(60) DEFAULT NULL COMMENT '渠道类型',
  `qdmc` varchar(300) DEFAULT NULL COMMENT '渠道名称',
  `hbr` varchar(500) DEFAULT NULL COMMENT '核保人',
  `sbrq` datetime DEFAULT NULL COMMENT '申报日期',
  `khxx` varchar(60) DEFAULT NULL COMMENT '二、客户信息：',
  `tbrmc` varchar(300) DEFAULT NULL COMMENT '投保人名称',
  `tbrtyxyzdm` varchar(180) DEFAULT NULL COMMENT '投保人统一信用证代码',
  `hylbtbr` text COMMENT '行业类别(投保人)',
  `qthy` varchar(360) DEFAULT NULL COMMENT '其他行业(投保人)',
  `bbxr` varchar(180) DEFAULT NULL COMMENT '被保险人',
  `bbrtxyzdm` varchar(180) DEFAULT NULL COMMENT '被保人统一信用证代码',
  `hylbbbr` text COMMENT '行业类别(被保人)',
  `xbhxb` varchar(120) DEFAULT NULL COMMENT '新保或续保',
  `xbdhrxb` varchar(360) DEFAULT NULL COMMENT '续保单号(如续保)',
  `khjnpfqkrxb` text COMMENT '客户近年赔付情况(如新保)',
  `xzxx` varchar(60) DEFAULT NULL COMMENT '三、险种信息：',
  `xzdl` varchar(120) DEFAULT NULL COMMENT '险种大类',
  `xzmcqc` varchar(240) DEFAULT NULL COMMENT '险种名称(全称)',
  `xzdlqyccbx` varchar(60) DEFAULT NULL COMMENT '险种大类-企业财产保险',
  `xzdljtccbx` varchar(60) DEFAULT NULL COMMENT '险种大类-家庭财产保险',
  `xzdlgcbx` varchar(60) DEFAULT NULL COMMENT '险种大类-工程保险',
  `xzdlzrbx` varchar(60) DEFAULT NULL COMMENT '险种大类-责任保险',
  `xzdlrsywbx` varchar(60) DEFAULT NULL COMMENT '险种大类-人身意外保险',
  `xzdlhwysbx` varchar(60) DEFAULT NULL COMMENT '险种大类-货物运输保险',
  `xzdlcbbx` varchar(60) DEFAULT NULL COMMENT '险种大类-船舶保险',
  `xzdltsfxbx` varchar(60) DEFAULT NULL COMMENT '险种大类-特殊风险保险',
  `xzdldqjkbx` varchar(60) DEFAULT NULL COMMENT '险种大类-短期健康保险',
  `xzdlxybx` varchar(60) DEFAULT NULL COMMENT '险种大类-信用保险',
  `xzdlbzbx` varchar(60) DEFAULT NULL COMMENT '险种大类-保证保险',
  `xzmccaqsczrx` varchar(15) DEFAULT NULL COMMENT '险种名称-C安全生产责任险',
  `xzmcdjzsgqyaqsczrbx` varchar(15) DEFAULT NULL COMMENT '险种名称-D建筑施工企业安全生产责任保险',
  `xzmcedlkycyrzrbx` varchar(15) DEFAULT NULL COMMENT '险种名称-E道路客运承运人责任保险',
  `xzmcfcpzrx` varchar(15) DEFAULT NULL COMMENT '险种名称-F产品责任险',
  `xzmcaywx` varchar(15) DEFAULT NULL COMMENT '险种名称-A意外险',
  `xzmcbjgx` varchar(15) DEFAULT NULL COMMENT '险种名称-B建工险',
  `xzmcaccx` varchar(15) DEFAULT NULL COMMENT '险种名称-A财产险',
  `xzmcbjqshx` varchar(15) DEFAULT NULL COMMENT '险种名称-B机器损坏险',
  `xzmcbgzzrx` varchar(15) DEFAULT NULL COMMENT '险种名称-B公众责任险',
  `xzmcgzzrx` varchar(30) DEFAULT NULL COMMENT '险种名称-A雇主责任险',
  `xzjzgcyqx` varchar(20) DEFAULT NULL COMMENT '险种名称-建筑工程一切险',
  `jzgcyqxgcmc` varchar(300) DEFAULT NULL COMMENT '(建筑工程一切险)工程名称',
  `jzgcyqxxmlx` varchar(600) DEFAULT NULL COMMENT '(建筑工程一切险)项目类型',
  `jzgcyqxbzmxjbf` text COMMENT '(建筑工程一切险)保障明细及保费',
  `jzgcyqxgcqq` datetime DEFAULT NULL COMM

ENT '(建筑工程一切险)工程起期',
  `jzgcyqxgczq` datetime DEFAULT NULL COMMENT '(建筑工程一切险)工程止期',
