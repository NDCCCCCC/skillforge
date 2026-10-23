// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package llmconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---- catalog.go ----

func TestCatalog(t *testing.T) {
	vendors := Catalog()
	if len(vendors) == 0 {
		t.Fatal("目录为空")
	}

	// FindPlan 命中 zhipu coding-plan
	p, ok := FindPlan("zhipu", "coding-plan")
	if !ok {
		t.Fatal("FindPlan 未命中 zhipu coding-plan")
	}
	if p.BaseURL != "https://open.bigmodel.cn/api/coding/paas/v4" {
		t.Fatalf("zhipu coding-plan BaseURL = %q", p.BaseURL)
	}

	// FindPlan 命中 opencode zen，且 ModelsURL 固定
	p, ok = FindPlan("opencode", "zen")
	if !ok {
		t.Fatal("FindPlan 未命中 opencode zen")
	}
	if p.BaseURL != "https://opencode.ai/inference/openai/v1" {
		t.Fatalf("opencode zen BaseURL = %q", p.BaseURL)
	}
	if p.ModelsURL != "https://opencode.ai/inference/v1/models" {
		t.Fatalf("opencode zen ModelsURL = %q", p.ModelsURL)
	}

	// custom 的 BaseURL 必须为空（由用户填写）
	p, ok = FindPlan("custom", "custom")
	if !ok {
		t.Fatal("FindPlan 未命中 custom")
	}
	if p.BaseURL != "" {
		t.Fatalf("custom BaseURL 应为空，got %q", p.BaseURL)
	}

	// 未命中路径
	if _, ok := FindPlan("nope", "nada"); ok {
		t.Fatal("FindPlan 不应命中不存在的 vendor/plan")
	}
}

// ---- store.go ----

// newTestStoreEnv 把 SKILLFORGE_LLM_CONFIG 指到 t.TempDir 下的文件，
// 返回清理函数。
func newTestStoreEnv(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llm.json")
	t.Setenv("SKILLFORGE_LLM_CONFIG", path)
	return path
}

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	path := newTestStoreEnv(t)

	in := Config{Vendor: "zhipu", Plan: "coding-plan", Model: "glm-5.3-flash", APIKey: "sk-test-1234567890"}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, present, err := Load()
	if err != nil || !present {
		t.Fatalf("Load: present=%v err=%v", present, err)
	}
	if out.Vendor != in.Vendor || out.Plan != in.Plan || out.Model != in.Model || out.APIKey != in.APIKey {
		t.Fatalf("往返不等: %+v vs %+v", out, in)
	}
	if _, err := time.Parse(time.RFC3339, out.UpdatedAt); err != nil {
		t.Fatalf("UpdatedAt 非 RFC3339: %q", out.UpdatedAt)
	}

	// 权限断言只在非 Windows 下跑（Windows chmod 是 no-op）
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Fatalf("文件权限 = %o， want 600", perm)
		}
	}
}

func TestStoreLoadAbsent(t *testing.T) {
	newTestStoreEnv(t)
	_, present, err := Load()
	if err != nil {
		t.Fatalf("不存在的文件不应报错: %v", err)
	}
	if present {
		t.Fatal("不存在的文件 present 应为 false")
	}
}

func TestStoreValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"合法", Config{Vendor: "openai", Plan: "standard", Model: "gpt-4o", APIKey: "k12345678"}, false},
		{"合法 env 间接 key", Config{Vendor: "openai", Plan: "standard", Model: "gpt-4o", APIKeyEnv: "MY_KEY"}, false},
		{"custom 缺 base_url", Config{Vendor: "custom", Plan: "custom", Model: "m", APIKey: "k12345678"}, true},
		{"custom 带 base_url", Config{Vendor: "custom", Plan: "custom", Model: "m", BaseURL: "https://x/v1", APIKey: "k12345678"}, false},
		{"缺 vendor", Config{Model: "m", APIKey: "k12345678"}, true},
		{"缺 model", Config{Vendor: "openai", Plan: "standard", APIKey: "k12345678"}, true},
		{"缺 key（明文与 env 都空）", Config{Vendor: "openai", Plan: "standard", Model: "m"}, true},
		{"未知 vendor", Config{Vendor: "nope", Plan: "x", Model: "m", APIKey: "k12345678"}, true},
		{"已知 vendor+plan", Config{Vendor: "opencode", Plan: "go", Model: "m", APIKey: "k12345678"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestStoreResolveKeyEnvPriority(t *testing.T) {
	t.Setenv("SKILLFORGE_TEST_KEY_ENV", "env-key-value")

	// env 优先于明文
	c := Config{APIKeyEnv: "SKILLFORGE_TEST_KEY_ENV", APIKey: "plain-key"}
	got, err := ResolveKey(c)
	if err != nil || got != "env-key-value" {
		t.Fatalf("env 应优先: got=%q err=%v", got, err)
	}

	// env 名存在但值为空 ⇒ 报错
	t.Setenv("SKILLFORGE_TEST_KEY_EMPTY", "")
	c = Config{APIKeyEnv: "SKILLFORGE_TEST_KEY_EMPTY"}
	if _, err := ResolveKey(c); err == nil {
		t.Fatal("env 值为空应报错")
	}

	// 无 env ⇒ 明文
	c = Config{APIKey: "plain-key"}
	got, err = ResolveKey(c)
	if err != nil || got != "plain-key" {
		t.Fatalf("明文回退: got=%q err=%v", got, err)
	}
}

func TestStoreMask(t *testing.T) {
	if got := Mask("short"); got != "****" {
		t.Fatalf("Mask(8字符内) = %q", got)
	}
	if got := Mask("12345678"); got != "****" {
		t.Fatalf("Mask(恰好8) = %q", got)
	}
	if got := Mask("123456789"); got != "…6789" {
		t.Fatalf("Mask(9) = %q", got)
	}
	if got := Mask("sk-abcdef1234567890"); got != "…7890" {
		t.Fatalf("Mask(长) = %q", got)
	}
}

// ---- probe.go: TestKey 错误分类 ----

func TestKeyClassification(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantOK   bool
		wantSub  string // Error 应包含的子串；空 = 无 Error
	}{
		{"401", http.StatusUnauthorized, `{"error":"bad key"}`, false, "key 被拒绝"},
		{"403", http.StatusForbidden, `{"error":"forbidden"}`, false, "key 被拒绝"},
		{"404", http.StatusNotFound, "not found", false, "URL 不存在"},
		{"429", http.StatusTooManyRequests, `{"error":"rate"}`, false, "限流"},
		{"500", http.StatusInternalServerError, "boom", false, "上游错误"},
		{"503", http.StatusServiceUnavailable, "boom", false, "上游错误"},
		{"2xx 正常响应", http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"pong"}}]}`, true, ""},
		{"2xx 错误信封", http.StatusOK, `{"error":{"message":"quota"}}`, false, "上游返回错误信封"},
		{"2xx 错误信封字符串", http.StatusOK, `{"error":"upstream exploded"}`, false, "上游返回错误信封"},
		{"2xx 非JSON", http.StatusOK, `<html>ok</html>`, false, "不是合法 JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			res := TestKey(context.Background(), srv.URL, "sk-test", "test-model")
			if res.OK != tc.wantOK {
				t.Fatalf("OK = %v, want %v (error=%q)", res.OK, tc.wantOK, res.Error)
			}
			if tc.wantSub == "" {
				if res.Error != "" {
					t.Fatalf("Error 应为空, got %q", res.Error)
				}
			} else if !strings.Contains(res.Error, tc.wantSub) {
				t.Fatalf("Error = %q, 应包含 %q", res.Error, tc.wantSub)
			}
		})
	}
}

// TestKeyRequestShape 验证请求路径与 body 形态。
func TestKeyRequestShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-abc" {
			t.Errorf("Authorization = %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"].(float64) != 1 {
			t.Errorf("max_tokens = %v", body["max_tokens"])
		}
		if body["stream"] != false {
			t.Errorf("stream = %v", body["stream"])
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer srv.Close()

	res := TestKey(context.Background(), srv.URL+"/v1", "sk-abc", "m1")
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if res.LatencyMS < 0 {
		t.Fatalf("LatencyMS = %d", res.LatencyMS)
	}
}

// TestKeyTimeout 验证超时分类。handler 睡 200ms，context 50ms 到期。
func TestKeyTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res := TestKey(ctx, srv.URL, "k", "m")
	if res.OK {
		t.Fatal("超时不应 OK")
	}
	if !strings.Contains(res.Error, "连接超时") {
		t.Fatalf("Error = %q, 应为 连接超时", res.Error)
	}
}

// TestKeyNetworkError 验证连接拒绝保持原始错误文本。
func TestKeyNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 立即关闭 ⇒ 下一步连接被拒绝

	res := TestKey(context.Background(), url, "k", "m")
	if res.OK {
		t.Fatal("连接失败不应 OK")
	}
	if res.Error == "" || strings.Contains(res.Error, "连接超时") {
		t.Fatalf("应保留原始网络错误, got %q", res.Error)
	}
}

// ---- probe.go: ListModels 四种形状 ----

func TestListModelsShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"data 数组", `{"data":[{"id":"b","object":"model"},{"id":"a"}]}`, []string{"a", "b"}},
		{"models 数组", `{"models":[{"id":"y"},{"id":"x"}]}`, []string{"x", "y"}},
		{"纯字符串数组", `["m2","m1","m2"]`, []string{"m1", "m2"}},
		{"纯对象数组", `[{"id":"n"},{"id":"m"}]`, []string{"m", "n"}},
		{"空 id 跳过", `{"data":[{"id":""},{"id":"k"}]}`, []string{"k"}},
		{"排序去重", `{"data":[{"id":"b"},{"id":"a"},{"id":"b"},{"id":"a"}]}`, []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer sk-lm" {
					t.Errorf("Authorization = %q", got)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			got, err := ListModels(context.Background(), srv.URL, "", "sk-lm")
			if err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestListModelsFallbackURL 验证 modelsURL 为空时回退 baseURL+"/models"。
func TestListModelsFallbackURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m"}]}`))
	}))
	defer srv.Close()

	got, err := ListModels(context.Background(), "", srv.URL+"/v1", "")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(got) != 1 || got[0] != "m" {
		t.Fatalf("got %v", got)
	}
}

// TestListModelsErrorStatus 验证非 2xx 报错且包含状态码。
func TestListModelsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := ListModels(context.Background(), srv.URL, "", "k"); err == nil {
		t.Fatal("非 2xx 应报错")
	} else if !strings.Contains(err.Error(), "403") {
		t.Fatalf("错误应含状态码: %v", err)
	}
}

// TestListModelsUnknownShape 验证无法识别的形状报错。
func TestListModelsUnknownShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"weird":{"nested":true}}`))
	}))
	defer srv.Close()

	if _, err := ListModels(context.Background(), srv.URL, "", "k"); err == nil {
		t.Fatal("未知形状应报错")
	}
}
