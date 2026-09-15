---
title: 视频录制api
source_idx: 016
section: 说明
---

## 说明

pe: application/json' \  -d '{"username": "api", "password": "Hubei@1992"}'返回结果：{    "code": 0,    "message": "success",    "data": {        "access_token": "xJ3k...（SM4-GCM加密的Token字符串）",        "refresh_token": "mN7p...（用于刷新的Token）",        "expires_in": 7200,        "user": {            "id": 2,            "username": "api",            "role_name": "operator",            "is_admin": false,            "permissions": ["task:view", "task:create", "task:start", "task:stop"]        }    }}access_token：有效期 2 小时refresh_token：有效期 7 天，用于刷新 access_tokenexpires_in：access_token 有效秒数3. 使用 Token 调用接口# 新建录制任务curl -X POST 'http://10.62.0.123:8080/api/v1/recordings' \  -H 'Authorization: Bearer <access_token>' \  -H 'Content-Type: application/json' \  -d '{    "conference_number": "521270003",    "name": "测试录制",    "start_time": "2026-04-07T02:00:00.000Z",    "end_time": "2026-04-07T04:00:00.000Z",    "huawei_config_id": 1,    "pre_join_minutes": 30  }'# 查询录制任务列表curl 'http://10.62.0.123:8080/api/v1/recordings' \  -H 'Authorization: Bearer <access_token>'# 停止录制任务curl -X POST 'http://10.62.0.123:8080/api/v1/recordings/35/stop' \  -H 'Authorization: Bearer <access_token>'4. Token 刷新access_token 过期后，使用 refresh_token 获取新的 Token 对：curl -X POST 'http://10.62.0.123:8080/api/v1/auth/refresh' \  -H 'Content-Type: application/json' \  -d '{"refresh_token": "<refresh_token>"}'返回新的 access_token 和 refresh_token，旧的 refresh_token 立即失效。5. 外部系统自行生成 Token（高级）如果外部系统需要自行生成 Token（不通过登录接口），需满足以下条件：必需信息：服务端配置的 sm4_secret 值（需从管理员获取）Claims 结构定义Go 生成示例：// test_sm4_token.go — 外部系统 SM4-GCM Token 生成与 API 测试
//
// 模拟外部系统使用 sm4_secret 自行生成 Token，然后调用服务端 API。
//
// 用法:
//   go run scripts/test_sm4_token.go
package main

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm4"
)

const (
	sm4Secret  = "EDC6UNKa5JQUrBnBsmgRww=="
	baseURL    = "http://10.62.0.123:8080"
	userID     = 2   // api 用户
	username   = "api"
	roleID     = 4   // api_client 角色
	tokenType  = "access"
	expireHour = 2
)

type Claims struct {
	UserID      uint     `json:"uid"`
	Username    string   `json:"sub"`
	RoleID      uint     `json:"rid"`
	Permissions []string `json:"perms"`
	IsAdmin     bool     `json:"adm"`
	TokenType   string   `json:"tt"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
	NotBefore   int64    `json:"nbf"`
	Issuer      string   `json:"iss"`
}

func deriveSM4Key(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:16]
}

func generateToken(secret string, claims *Claims) (string, error) {
	key := deriveSM4Key(secret)
	block, err := sm4.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("sm4.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher.NewGCM: %w", err)
	}

	plaintext, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  外部系统 SM4-GCM Token 生成与 API 测试")
	fmt.Println("=

=======================================")
	fmt.Println()

	// --- Step 1: 生成 Token ---
	now := time.Now()
	claims := &Claims{
		UserID:      userID,
		Username:    username,
