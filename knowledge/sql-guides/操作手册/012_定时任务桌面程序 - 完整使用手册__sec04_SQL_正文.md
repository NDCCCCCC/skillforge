---
title: "定时任务桌面程序 - 完整使用手册"
source_idx: 012
section: SQL / 正文
---

## SQL / 正文



```sql
SELECT task_id, enabled, trigger_flag FROM monitor_config5.4 逐行通知 (row_notify)每行数据单独发送通知给对应接收人。配置项：接收人字段：指定哪列包含接收人（如 "email"）发送配置ID：使用哪个邮件/API配置发送类型："email" 或 "api"-- 示例：个人化薪资通知
SELECT name, amount, email FROM salary_report WHERE month = '2026-04'5.5 条件执行所有后置动作都支持条件（同模板语法）：条件说明status == "success"成功时触发status == "failed"失败时触发J_count > 0有数据时触发J_code != 0 && J_count > 100异常且超量时触发5.6 动态任务IDexecute_task / enable_task / disable_task 支持从输出动态读取任务ID：{[0].task_id} → 从第一行读取{[0].next_task_id} → 从自定义字段读取5.7 典型场景场景主任务后置动作条件异常告警SQL查异常数execute_taskJ_error_count > 0同步完成关闭监控执行同步脚本disable_taskstatus == "success"批量状态监控SQL查所有状态batch_process—个人化薪资通知SQL查薪资数据row_notify—任务链 A→B→C每步execute_taskstatus == "success"六、系统设置6.1 邮件配置 (SMTP)配置项说明SMTP 服务器如 smtp.qq.com、smtp.gmail.com端口587（TLS）或 465（SSL）使用 TLS启用加密连接用户名/密码邮箱登录凭据（QQ邮箱用授权码）常用配置：QQ邮箱：smtp.qq.com:587，QQ号 + 授权码163邮箱：smtp.163.com:465，用户名 + 授权码Gmail：smtp.gmail.com:587，应用专用密码6.2 API 配置 (Webhook)API 地址、请求方法（POST/PUT/PATCH）请求格式：JSON 或 Form自定义请求头和请求体模板6.3 数据库配置支持 MySQL、PostgreSQL、GaussDB、SQL Server、SQLite。配置主机、端口、数据库名、用户名密码。6.4 系统清理查看执行记录统计清理指定天数前的执行日志清理旧日志文件数据库碎片整理（VACUUM）七、高级功能7.1 重试机制设置失败后重试次数和间隔（秒），避免无限重试。7.2 测试执行保存前先测试，查看输出。JSON 键会显示在模板上方，点击可快速插入。7.3 自定义消息编辑勾选"允许编辑自定义消息"后，发送通知前可手动编辑消���内容。7.4 输出捕获勾选"自动捕获程序输出"后，标准输出和错误输出都会存储在执行记录中。八、常见问题问题解决方案任务没按时执行检查启用状态、Cron表达式、系统时间SQL执行失败检查数据库配置、SQL语法、执行权限邮件发送失败检查SMTP配置、授权码、网络、防火墙API通知失败检查API地址、请求格式、请求头网页截图不工作检查URL可访问性、超时时间、分辨率应用占用内存高定期清理旧执行记录和日志文件删除文件夹会级联删除所有子内容，不可恢复数据迁移备份 tasks.db + user.csv + bin/ 目录九、项目文件C:\Cron\├── tasks.db          # SQLite 数据库（所有数据）├── bin/              # 可执行文件目录├── logs/             # 运行日志 cron_YYYYMMDD.log└── user.csv          # 接收人（姓名,账号,邮箱）备份建议：定期备份 tasks.db 和 user.csv
```
