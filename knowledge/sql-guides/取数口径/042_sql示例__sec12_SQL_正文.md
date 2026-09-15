---
title: sql示例
source_idx: 042
section: SQL / 正文
---

## SQL / 正文

```sql
-- `jeecg-boot`.vela_workflow_audit_record definition

CREATE TABLE `vela_workflow_audit_record` (
  `id` varchar(36) NOT NULL,
  `task_id` varchar(100) DEFAULT NULL COMMENT '任务id',
  `task_name` varchar(100) DEFAULT NULL COMMENT '任务名称',
  `task_des` text COMMENT '审批意见',
  `agree_flag` varchar(32) DEFAULT NULL COMMENT '是否同意',
  `task_type` varchar(32) DEFAULT NULL COMMENT '任务类型',
  `task_complete_time` varchar(32) DEFAULT NULL COMMENT '任务完成时间',
  `handle_org_code` varchar(32) DEFAULT NULL COMMENT '部门代码',
  `handle_name` varchar(255) DEFAULT NULL,
  `process_instance_name` varchar(255) DEFAULT NULL COMMENT '流程表单名称',
  `handle_by` varchar(32) DEFAULT NULL COMMENT '操作人',
  `business_key` varchar(32) DEFAULT NULL COMMENT '业务key',
  `created_time` varchar(32) DEFAULT NULL COMMENT '创建时间',
  `updated_time` varchar(32) DEFAULT NULL COMMENT '更新时间',
  `create_by` varchar(50) DEFAULT NULL COMMENT '创建人',
  `create_time` datetime DEFAULT NULL COMMENT '创建日期',
  `update_by` varchar(50) DEFAULT NULL COMMENT '更新人',
  `update_time` datetime DEFAULT NULL COMMENT '更新日期',
  `sys_org_code` varchar(64) DEFAULT NULL COMMENT '所属部门',
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_taskId` (`task_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='流程审批记录表';
```

建表语句转换成查询语句--正则表达式模式：varchar\(\d+\) (?:NOT NULL|DEFAULT NULL) COMMENT | (?:datetime|text|decimal\(\d+,\d+\)|bigint\(\d+\)|int\(\d+\)) (?:NOT NULL|DEFAULT NULL) COMMENT


--这个正则表达式会匹配所有字段定义中间的部分，包括：

--varchar类型：varchar(数字) NOT NULL COMMENT 或 varchar(数字) DEFAULT NULL COMMENT

--其他数据类型：datetime NOT NULL COMMENT、text DEFAULT NULL COMMENT、decimal(数字,数字) DEFAULT NULL COMMENT 等

--匹配示例：

varchar(36) NOT NULL COMMENT

varchar(60) DEFAULT NULL COMMENT

datetime DEFAULT NULL COMMENT

text COMMENT

decimal(15,2) DEFAULT NULL COMMENT

bigint(15) DEFAULT NULL COMMENT

int(11) DEFAULT NULL COMMENT

--这样您就可以找到所有字段定义中间的部分，然后手动替换成其他字符串。
