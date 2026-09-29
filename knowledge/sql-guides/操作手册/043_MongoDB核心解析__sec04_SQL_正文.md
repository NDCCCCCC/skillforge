---
title: MongoDB 4.4全面解析：从核心概念到实战应用
source_idx: 043
section: SQL / 正文
---

## SQL / 正文

```sql
## SQL / 正文




ALTER TABLE操作，可能造成停机扩展能力：MongoDB原生支持水平扩展(分片)，可以轻松通过添加更多节点来增加存储容量和吞吐量MariaDB主要依赖垂直扩展(更强大的服务器)，虽然可以通过分片或集群实现水平扩展，但配置复杂查询能力：MongoDB提供丰富的查询操作符和强大的聚合管道，适合处理复杂的数据分析任务MariaDB的SQL支持更成熟，特别���长复杂关联查询和报表生成事务处理：MongoDB 4.4支持跨分片的ACID事务，但相比MariaDB这类关系型数据库，事务性能仍有差距MariaDB的事务处理更成熟，适合需要高频、复杂事务的系统开发效率：MongoDB的文档模型与编程语言中的对象(如JSON)映射自然，减少ORM阻抗失配，提高开发效率MariaDB需要对象-关系映射(ORM)或手动编写SQL语句选择建议：选择MongoDB当您需要：处理半结构化数据、快速迭代开发、水平扩展能力、高吞吐量读写、灵活的数据模型选择MariaDB当您需要：严格的数据一致性、复杂事务处理、复杂SQL查询、成熟的生态系统四、MongoDB 4.4的安装与配置MongoDB 4.4支持多种操作系统环境，包括Windows、Linux等。以下是主要平台的安装方法：Windows安装下载安装包：从MongoDB官网下载Windows版的MongoDB 4.4.0安装包(mongodb-windows-x86_64-4.4.0-signed.msi)运行安装程序：双击MSI文件启动安装向导接受许可协议选择安装类型："默认"或"自定义"指定数据文件、日志文件和可执行文件的安装位置设置MongoDB作为Windows服务运行，配置启动类型和服务名验证安装：访问默认端口(localhost:27017)验证是否安装成功使用MongoDB Compass GUI工具测试连接Linux安装(CentOS/RHEL示例)添加MongoDB存储库：vi /etc/yum.repos.d/mongodb-org-4.4.repo添加以下内容：[mongodb-org-4.4]name=MongoDB Repositorybaseurl= https://repo.mongodb.org/yum/redhat/$releasever/mongodb-org/4.4/x86_64/ gpgcheck=1enabled=1gpgkey= https://www.mongodb.org/static/pgp/server-4.4.asc 安装MongoDB：sudo yum install mongodb-org启动服务：sudo systemctl start mongod    # 启动sudo systemctl enable mongod   # 设置开机自启动连接MongoDB：mongo    # 启动mongo shell配置安全认证创建管理员账户：use admindb.createUser({  user: "admin",  pwd: "password123",  roles: [{ role: "root", db: "admin" }]})启用认证：编辑配置文件/etc/mongod.conf(Linux)或C:\Program Files\MongoDB\Server\4.4\bin\mongod.cfg(Windows)添加/修改： security:  authorization: enabled注意：authorization:后必须有一个空格重启服务：sudo systemctl restart mongod使用认证连接：mongo -u admin -p password123 --authenticationDatabase admin或使用连接字符串：const url = 'mongodb://admin:password123@localhost:27017/';
