---
title: 视频录制api
source_idx: 016
split_part: 4/5
source_split: true
---

=======================================")
	fmt.Println()

	// --- Step 1: 生成 Token ---
	now := time.Now()
	claims := &Claims{
		UserID:      userID,
		Username:    username,
		RoleID:      roleID,
		Permissions: []string{"tasks:view", "tasks:create", "tasks:start", "tasks:stop"},
		IsAdmin:     false,
		TokenType:   tokenType,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(time.Duration(expireHour) * time.Hour).Unix(),
		NotBefore:   now.Unix(),
		Issuer:      "record_v2",
	}

	token, err := generateToken(sm4Secret, claims)
	if err != nil {
		fmt.Printf("❌ Token 生成失败: %v\n", err)
		return
	}

	fmt.Printf("✓ Token 生成成功 (长度: %d)\n", len(token))
	fmt.Printf("  sm4_secret: %s\n", sm4Secret)
	fmt.Printf("  user: %s (id=%d, role_id=%d)\n", username, userID, roleID)
	fmt.Printf("  有效期: %d小时 (exp=%d)\n", expireHour, claims.ExpiresAt)
	fmt.Println()

	// --- Step 2: 用生成的 Token 请求 API ---
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"获取当前用户", "GET", "/api/v1/auth/me"},
		{"获取录制任务列表", "GET", "/api/v1/recordings"},
		{"获取用户列表", "GET", "/api/v1/users"},
		{"获取权限列表", "GET", "/api/v1/permissions"},
		{"系统统计", "GET", "/api/v1/system/stats"},
	}

	passCount := 0
	for _, tc := range tests {
		url := baseURL + tc.path
		req, _ := http.NewRequest(tc.method, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("❌ %s: 请求失败 - %v\n", tc.name, err)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// 解析响应码
		var result struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		json.Unmarshal(body, &result)

		if result.Code == 0 {
			fmt.Printf("✓ %s: 成功 (HTTP %d, code=%d)\n", tc.name, resp.StatusCode, result.Code)
			passCount++
		} else {
			fmt.Printf("❌ %s: 失败 (HTTP %d, code=%d, msg=%s)\n", tc.name, resp.StatusCode, result.Code, result.Message)
		}

		// 显示部分响应内容
		preview := string(body)
		if len(preview) > 150 {
			preview = preview[:150] + "..."
		}
		fmt.Printf("  响应: %s\n", preview)
		fmt.Println()
	}

	// --- Step 3: 对比测试 — 登录获取的 Token ---
	fmt.Println("--- 对比测试: 通过登录接口获取的 Token ---")
	loginBody := `{"username":"admin","password":"admin123"}`
	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		fmt.Printf("❌ 登录失败: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var loginResult struct {
			Code int `json:"code"`
			Data struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int64  `json:"expires_in"`
			} `json:"data"`
		}
		json.Unmarshal(body, &loginResult)
		if loginResult.Code == 0 {
			fmt.Printf("✓ 登录 Token 获取成功 (长度: %d, expires_in: %ds)\n",
				len(loginResult.Data.AccessToken), loginResult.Data.ExpiresIn)
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  测试完成: %d/%d 通过\n", passCount, len(tests))
	fmt.Println("========================================")
}
6. 自行生成 Token 注意事项sm4_secret 必须与服务端 config.yaml 中的值完全一致SM4 密钥固定不变（除非管理员手动更改配置）Claims 中 tt 字段必须为 "access"exp 过期时间不要设置过长（建议 2 小时）permissions 需与用户实���权限一致，否则可能被拦截两种方式对比特性API KeySM4-GCM Token认证方式X-API-Key 请求头Authorization: Bearer 请求头获取方式管理后台创建调用登录接口或自行生成有效期可自定义（支持永