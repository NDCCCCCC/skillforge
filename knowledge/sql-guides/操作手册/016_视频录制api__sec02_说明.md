---
title: 视频录制api
source_idx: 016
section: 说明
---

## 说明

## 说明


API_key：rec_fd335d3da447babec0ec761a621a8ebd86e167faeaba61a72f3322b9f3a587edsm4秘钥：EDC6UNKa5JQUrBnBsmgRww==新建录制任务curl --location 'http://10.62.0.123:8080/api/v1/recordings/auto' \
--header 'Content-Type: application/json' \
--header 'Authorization: ••••••' \
--data '{
    "conference_number": "521270003",
    "end_time": "2026-04-08T07:00:00.000Z",
    "name": "test1111",
    "start_time": "2026-04-07T02:19:31.000Z"
}'
}'    "conference_number": 会议号    "name": 会议名称    "start_time": "2026-03-11T02:19:31.000Z"
    "end_time": "2026-03-29T07:00:00.000Z"


停止任务curl --location 'http://10.62.0.123:8080/api/v1/recordings/35/stop' \

--header 'Content-Type: application/json' \
--header 'Authorization: ••••••' \
--data '{
    "conference_number": "521270003",
    "end_time": "2026-03-29T07:00:00.000Z",
    "huawei_config_id": 1,
    "name": "test",
    "pre_join_minutes": 30,
    "record_delay_minutes": 0,
    "start_time": "2026-03-11T02:19:31.000Z"
}'系统对接认证方式系统支持两种认证方式，任选其一即可访问所��API接口。方式一：API Key 认证（推荐）通过 X-API-Key 请求头传递 API 密钥，无需登录获取 Token。1. 创建 API Key在管理后台「系统设置 → API密钥」页面创建，或通过管理员账号调用创建接口：curl -X POST 'http://10.62.0.123:8080/api/v1/apikeys' \  -H 'Authorization: Bearer <管理员token>' \  -H 'Content-Type: application/json' \  -d '{    "name": "对接系统A",    "expires_at": "2027-12-31T23:59:59Z",    "scope": ["recordings:read", "recordings:write"]  }'返回结果中的 key 字段即为 API Key（格式如 ak_xxxxxxxx），请妥善保存，仅展示一次。2. 使用 API Key 调用接口所有需要认证的接口都支持 API Key：# 新建录制任务curl -X POST 'http://10.62.0.123:8080/api/v1/recordings' \  -H 'X-API-Key: ak_你的密钥' \  -H 'Content-Type: application/json' \  -d '{    "conference_number": "521270003",    "name": "测试录制",    "start_time": "2026-04-07T02:00:00.000Z",    "end_time": "2026-04-07T04:00:00.000Z",    "huawei_config_id": 1,    "pre_join_minutes": 30,    "record_delay_minutes": 0  }'# 查询录制任务列表curl 'http://10.62.0.123:8080/api/v1/recordings' \  -H 'X-API-Key: ak_你的密钥'# 停止录制任务curl -X POST 'http://10.62.0.123:8080/api/v1/recordings/35/stop' \  -H 'X-API-Key: ak_你的密钥'3. API Key 说明格式：ak_ 前缀 + 随机字符串支持设置过期时间和权限范围（scope）支持设置 IP 白名单限制可在管理后台随时禁用或删除方式二：SM4-GCM Token 认证通过登录接口获取 SM4-GCM 加密的 Access Token，通过 Authorization: Bearer 请求头传递。1. Token 生成流程登录请求(用户名+密码)    ↓服务端构建 Claims 结构:    {        "uid": 用户ID,        "sub": 用户名,        "rid": 角色ID,        "perms": ["task:view", "task:create", ...],        "adm": 是否管理员,        "tt": "access",        "iat": 签发时间(unix),        "exp": 过期时间(unix),        "nbf": 生效时间(unix),        "iss": "record_v2"    }    ↓Claims JSON → SM4-GCM 加密 → Base64URL 编码 → Token 字符串SM4 密钥派生：从配置文件读取 sm4_secret 字符串计算 SHA256 哈希：sha256(sm4_secret)取哈希值的前 16 字节作为 SM4 密钥Token 加密过程：Claims 结构体序列化为 JSON生成 12 字节随机 Nonce使用 SM4-GCM 加密：SM4-GCM-Seal(nonce, plaintext)输出：nonce(12字节) + 密文 + authTag(16字节)Base64URL 编码（无 padding）得到最终 TokenToken 格式：base64url( nonce[12B] || SM4-GCM-密文 || authTag[16B] )2. 登录获取 Tokencurl -X POST 'http://10.62.0.123:8080/api/v1/auth/login' \  -H 'Content-Ty
