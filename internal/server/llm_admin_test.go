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

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/llm"
)

const llmAdminToken = "llm-admin-test-token"

// invokeLLMAdmin 对一个只挂了 llm admin 路由的引擎发请求（照
// admin_audit_test.go 的 invoke 惯例）。
func invokeLLMAdmin(t *testing.T, deps *Deps, method, path, body string, withToken bool) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/llm/providers", deps.llmProvidersHandler)
	h.GET("/api/admin/llm/config", deps.llmConfigHandler)
	h.POST("/api/admin/llm/config", deps.llmSaveHandler)
	h.POST("/api/admin/llm/test", deps.llmTestHandler)
	h.GET("/api/admin/llm/models", deps.llmModelsHandler)

	var headers []ut.Header
	if withToken {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: llmAdminToken})
	}
	if body == "" {
		return ut.PerformRequest(h.Engine, method, path, nil, headers...)
	}
	return ut.PerformRequest(h.Engine, method, path, &ut.Body{Body: strings.NewReader(body), Len: len(body)},
		append(headers, ut.Header{Key: "Content-Type", Value: "application/json"})...)
}

// llmTestDeps 组装带 LLMDynamic 的 Deps，并把 llm.json 指到测试目录。
func llmTestDeps(t *testing.T, resolver func() (model.ToolCallingChatModel, error)) *Deps {
	t.Helper()
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "llm.json"))
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", llmAdminToken)
	return &Deps{LLMDynamic: llm.NewDynamicChatModel(resolver)}
}

func llmBody(t *testing.T, resp *ut.ResponseRecorder) map[string]any {
	t.Helper()
	r := resp.Result()
	require.Equal(t, http.StatusOK, r.StatusCode(), "body: %s", string(r.Body()))
	var m map[string]any
	require.NoError(t, json.Unmarshal(r.Body(), &m))
	return m
}

func TestLLMAdmin_ProvidersCatalog(t *testing.T) {
	deps := llmTestDeps(t, nil)

	// 无 token ⇒ 403 先于一切（T-52-02）。
	r := invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/providers", "", false).Result()
	assert.Equal(t, http.StatusForbidden, r.StatusCode())

	m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/providers", "", true))
	vendors, ok := m["vendors"].([]any)
	require.True(t, ok, "vendors 必须是数组")
	assert.NotEmpty(t, vendors)
	// 用户点名的两个厂商必须在目录里。
	raw, _ := json.Marshal(vendors)
	assert.Contains(t, string(raw), `"opencode"`)
	assert.Contains(t, string(raw), `"zhipu"`)
}

func TestLLMAdmin_ConfigMaskedAndSources(t *testing.T) {
	t.Run("no file no env = source none + stub", func(t *testing.T) {
		// 隔离宿主可能导出的 LOCAL_API_KEY / GLM_API_KEY（env 回落断言的前提）。
		os.Unsetenv("LOCAL_API_KEY")
		os.Unsetenv("GLM_API_KEY")
		deps := llmTestDeps(t, func() (model.ToolCallingChatModel, error) { return nil, nil })
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/config", "", true))
		assert.Equal(t, "none", m["source"])
		assert.Equal(t, "stub", m["mode"])
		assert.Equal(t, false, m["available"])
	})

	t.Run("env fallback masked", func(t *testing.T) {
		os.Unsetenv("LOCAL_API_KEY")
		t.Setenv("LOCAL_API_KEY", "sk-env-secret-12345678")
		t.Setenv("LOCAL_BASE_URL", "http://localhost:9999/v1")
		t.Setenv("LOCAL_MODEL", "test-model")
		deps := llmTestDeps(t, func() (model.ToolCallingChatModel, error) { return nil, nil })
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/config", "", true))
		assert.Equal(t, "env", m["source"])
		cfg, ok := m["config"].(map[string]any)
		require.True(t, ok, "config 必须存在: %v", m)
		assert.NotContains(t, cfg["masked_key"], "secret", "masked key 绝不能含明文片段")
	})

	t.Run("file source masked + live mode", func(t *testing.T) {
		up := &fakeLLMUp{}
		deps := llmTestDeps(t, func() (model.ToolCallingChatModel, error) { return up, nil })
		body := `{"vendor":"opencode","plan":"go","model":"glm-5.3-flash","api_key":"sk-file-secret-98765432"}`
		llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config", body, true))

		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/config", "", true))
		assert.Equal(t, "file", m["source"])
		assert.Equal(t, "live", m["mode"])
		cfg := m["config"].(map[string]any)
		assert.NotContains(t, cfg["masked_key"], "secret")
		// 保存路径落盘且内容含明文（本机 0600 文件，T-52-01 的另一半）。
		path := os.Getenv("SKILLFORGE_LLM_CONFIG")
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(b), "sk-file-secret-98765432")
	})
}

func TestLLMAdmin_SaveHotReloads(t *testing.T) {
	// resolver 模拟"每次 Load 文件"的生产形态：文件出现 ⇒ 上游出现。
	deps := llmTestDeps(t, nil)
	dyn := deps.LLMDynamic
	dyn.SetResolver(func() (model.ToolCallingChatModel, error) {
		cfg, present, err := readSavedConfig(t)
		if err != nil || !present {
			return nil, err
		}
		if cfg.Vendor == "opencode" {
			return &fakeLLMUp{}, nil
		}
		return nil, nil
	})
	assert.False(t, dyn.Available(), "保存前应为 stub")

	body := `{"vendor":"opencode","plan":"go","model":"glm-5.3-flash","api_key":"sk-x-123456"}`
	m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config", body, true))
	assert.Equal(t, true, m["ok"])
	assert.Equal(t, true, m["available"])
	assert.Equal(t, "live", m["mode"], "保存后必须立即热重载到 live（无需重启）")
}

func TestLLMAdmin_SaveValidation(t *testing.T) {
	deps := llmTestDeps(t, nil)

	t.Run("unknown vendor 400", func(t *testing.T) {
		r := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"nope","plan":"x","model":"m","api_key":"k12345678"}`, true).Result()
		assert.Equal(t, http.StatusBadRequest, r.StatusCode())
	})
	t.Run("custom without base_url 400", func(t *testing.T) {
		r := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"custom","plan":"custom","model":"m","api_key":"k12345678"}`, true).Result()
		assert.Equal(t, http.StatusBadRequest, r.StatusCode())
	})
	t.Run("missing key 400", func(t *testing.T) {
		r := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"opencode","plan":"go","model":"glm-5.3-flash"}`, true).Result()
		assert.Equal(t, http.StatusBadRequest, r.StatusCode())
	})
	t.Run("no token 403 even before validation", func(t *testing.T) {
		r := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"nope"}`, false).Result()
		assert.Equal(t, http.StatusForbidden, r.StatusCode())
	})
}

func TestLLMAdmin_TestAndModels(t *testing.T) {
	// httptest 假 OpenAI 端点：/chat/completions 200、/models 200。
	srv := newFakeOpenAI(t, `{"id":"cmpl-1","choices":[{"message":{"role":"assistant","content":"pong"}}]}`,
		`{"data":[{"id":"glm-5.3-flash"},{"id":"glm-5.3"},{"id":"glm-5.3-flash"}]}`)
	defer srv.Close()

	deps := llmTestDeps(t, nil)
	body := `{"vendor":"opencode","plan":"go","base_url":"` + srv.URL + `","model":"glm-5.3-flash","api_key":"sk-t-12345678"}`

	t.Run("test ok", func(t *testing.T) {
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/test", body, true))
		assert.Equal(t, true, m["ok"])
	})

	t.Run("test 401 classified", func(t *testing.T) {
		bad := `{"vendor":"opencode","plan":"go","base_url":"` + srv.URL + `","model":"glm-5.3-flash","api_key":"wrong"}`
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/test", bad, true))
		assert.Equal(t, false, m["ok"])
		assert.Contains(t, m["error"], "key")
	})

	t.Run("models listed and deduped", func(t *testing.T) {
		path := "/api/admin/llm/models?base_url=" + srv.URL + "&api_key=sk-t-12345678"
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, path, "", true))
		models := m["models"].([]any)
		require.Len(t, models, 2, "重复 id 必须去重")
		assert.Equal(t, "glm-5.3", models[0], "按字母序")
	})
}

func TestLLMAdmin_NilPanel503(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", llmAdminToken)
	deps := &Deps{} // LLMDynamic == nil
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/llm/config"},
		{http.MethodPost, "/api/admin/llm/config"},
		{http.MethodPost, "/api/admin/llm/test"},
		{http.MethodGet, "/api/admin/llm/models"},
	} {
		r := invokeLLMAdmin(t, deps, tc.method, tc.path, `{}`, true).Result()
		assert.Equal(t, http.StatusServiceUnavailable, r.StatusCode(), tc.path)
	}
	// providers 是纯静态目录，不依赖装配 ⇒ 200（token 仍必须先过）。
	r := invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/providers", "", true).Result()
	assert.Equal(t, http.StatusOK, r.StatusCode())
}

// TestChatModelAvailable_DynamicSemantics 锁住 nil 语义的分派规则
// （49-02 stub 语义在 Phase 52 之后不变）。
func TestChatModelAvailable_DynamicSemantics(t *testing.T) {
	assert.False(t, ChatModelAvailable(nil), "nil 永远不可用（测试替身语义）")
	assert.False(t, ChatModelAvailable(llm.NewDynamicChatModel(nil)), "空 resolver = stub")
	assert.True(t, ChatModelAvailable(llm.NewDynamicChatModel(func() (model.ToolCallingChatModel, error) {
		return &fakeLLMUp{}, nil
	})), "有上游 = live")
	assert.True(t, ChatModelAvailable(&fakeLLMUp{}), "非 Dynamic 的实现（测试替身）非 nil 即可用")
}

func TestRedline_SkipsOnEmptyDynamic(t *testing.T) {
	// 49-02 的 redline 语义在 Phase 52 下保持：Dynamic 无上游 ⇒ 红线跳过。
	deps := &Deps{ChatModel: llm.NewDynamicChatModel(nil)}
	assert.False(t, ChatModelAvailable(deps.ChatModel))
	assert.True(t, strings.Contains("stub", "stub")) // 占位：真实跳过行为由既有 redline 测试覆盖
}

// newFakeOpenAI 起一个最小 OpenAI 兼容假端点：/chat/completions 返回
// chatBody；/models 返回 modelsBody；Authorization 头为 "wrong" 时
// chat 返回 401（错误分类测试用）。
func newFakeOpenAI(t *testing.T, chatBody, modelsBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer wrong" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(chatBody))
	})
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(modelsBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// readSavedConfig 读 llmTestDeps 指定的 llm.json（生产 resolver 形态的
// 测试替身）。
func readSavedConfig(t *testing.T) (cfg struct {
	Vendor string `json:"vendor"`
	Plan   string `json:"plan"`
}, present bool, err error) {
	t.Helper()
	b, err := os.ReadFile(os.Getenv("SKILLFORGE_LLM_CONFIG"))
	if err != nil {
		return cfg, false, nil // not present
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, false, err
	}
	return cfg, true, nil
}

// fakeLLMUp 是 server 侧的最小上游替身（与 internal/llm 测试里的独立）。
type fakeLLMUp struct{}

func (f *fakeLLMUp) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Content: "ok"}, nil
}

func (f *fakeLLMUp) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{{Content: "ok"}}), nil
}

func (f *fakeLLMUp) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}
