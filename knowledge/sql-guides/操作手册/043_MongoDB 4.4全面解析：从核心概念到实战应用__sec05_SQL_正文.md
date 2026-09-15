---
title: MongoDB 4.4全面解析：从核心概念到实战应用
source_idx: 043
section: SQL / 正文
---

## SQL / 正文

```sql
```

数据库角色分类MongoDB提供细粒度的访问控制，主要角色包括：数据库用户角色：read、readWrite数据库管理角色：dbAdmin、dbOwner、userAdmin集群管理角色：clusterAdmin、clusterManager、clusterMonitor、hostManager备份恢复角色：backup、restore所有数据库角色：readAnyDatabase、readWriteAnyDatabase、userAdminAnyDatabase、dbAdminAnyDatabase超级用户角色：root五、MongoDB 4.4的基本使用数据库操作显示数据库列表：show dbs创建/切换数据库：use mydb当插入第一条数据时会自动创建数据库删除数据库：db.dropDatabase()集合操作创建集合：db.createCollection("users")或直接插入文档自动创建集合显示集合列表：show collections删除集合：db.users.drop()文档CRUD操作插入文档：db.users.insertOne({  name: "Alice",  age: 30,  email: "alice@example.com",  address: {    city: "Beijing",    s
