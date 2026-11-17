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
	"github.com/skillforge/skillforge/internal/llmconfig"
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
	h.POST("/api/admin/llm/model", deps.llmModelHandler)

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

	// 261117-dvu: 字段名级回归。只断言值存在（"zhipu" 大小写无关）不够 ——
	// catalog.go 的 Vendor/Plan 曾漏写 json tag，Go 序列化成大写
	// ID/Label/Plans，前端 llmApi.ts 读 v.id/v.label/v.plans 全拿到
	// undefined ⇒ 厂商下拉永远空白，而上面那两条断言照样绿。
	// 用带小写 tag 的结构体反序列化，字段对不上就是空 ⇒ 当场抓到。
	var typed []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Plans []struct {
			ID        string `json:"id"`
			Label     string `json:"label"`
			BaseURL   string `json:"base_url"`
			ModelsURL string `json:"models_url"`
			Note      string `json:"note"`
		} `json:"plans"`
	}
	require.NoError(t, json.Unmarshal(raw, &typed))
	require.NotEmpty(t, typed, "反序列化后必须有厂商（字段名必须是小写 id/label/plans）")
	require.NotEmpty(t, typed[0].ID, "vendors[0].id 必须存在 —— 若为空说明后端返回的是大写 ID")
	assert.NotEmpty(t, typed[0].Label, "vendors[0].label 必须存在")
	require.NotEmpty(t, typed[0].Plans, "vendors[0].plans 必须存在")
	assert.NotEmpty(t, typed[0].Plans[0].ID, "plans[0].id 必须存在")
	assert.NotEmpty(t, typed[0].Plans[0].Label, "plans[0].label 必须存在")
	assert.NotEmpty(t, typed[0].Plans[0].BaseURL, "plans[0].base_url 必须存在")
	// models_url 的空串是有语义的（= BaseURL + /models），所以要求 key 存在，
	// 但不要求值非空。note 同理可以为空但 key 必须在。
	var keyProbe []map[string]any
	require.NoError(t, json.Unmarshal(raw, &keyProbe))
	plans, _ := keyProbe[0]["plans"].([]any)
	require.NotEmpty(t, plans)
	// models_url 的空串有语义（= BaseURL + /models），note 可空但 key 必须在
	// —— 两者都属于 plans[i]，不在 vendor 顶层。
	plan0, ok := plans[0].(map[string]any)
	require.True(t, ok)
	require.Contains(t, plan0, "models_url", "plans[0].models_url 这个 key 必须存在（不可 omitempty）")
	require.Contains(t, plan0, "note", "plans[0].note 这个 key 必须存在")
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
	// CODE-AUDIT 2026-11-05 (E-01)："API Key 留空则不改动已存 key" 的契约回归——
	// 首存带 key，二次保存不带任何 key 字段 ⇒ 200（保留已存凭据），而非 400。
	t.Run("empty key preserves stored key (E-01)", func(t *testing.T) {
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"opencode","plan":"go","model":"glm-5.3-flash","api_key":"sk-e01-123456"}`, true))
		assert.Equal(t, true, m["ok"], "首存必须成功")

		m2 := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
			`{"vendor":"opencode","plan":"go","model":"glm-5.3-airx"}`, true))
		assert.Equal(t, true, m2["ok"], "留空 key 的二次保存必须成功（保留已存 key）")
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
		// CODE-AUDIT 2026-11-05 (F-02)：明文 key 不再接受 query 传参，
		// 凭据改走白名单 env（SKILLFORGE_* 前缀）。
		t.Setenv("SKILLFORGE_PROBE_KEY", "sk-t-12345678")
		path := "/api/admin/llm/models?base_url=" + srv.URL + "&api_key_env=SKILLFORGE_PROBE_KEY"
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, path, "", true))
		models := m["models"].([]any)
		require.Len(t, models, 2, "重复 id 必须去重")
		assert.Equal(t, "glm-5.3", models[0], "按字母序")
	})

	// CODE-AUDIT 2026-11-05 (F-01/F-02)：三条安全约束的回归锁。
	t.Run("api_key query param is ignored (F-02)", func(t *testing.T) {
		// query 里的明文 key 被无视；假端点对非 "wrong" 的 Authorization 一律放行，
		// 故仍 200 —— 但这证明请求成功不依赖 query 传 key。白名单外的 env 名被拒。
		path := "/api/admin/llm/models?base_url=" + srv.URL + "&api_key=sk-t-12345678"
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, path, "", true))
		assert.Len(t, m["models"], 2, "无凭据探测仍可达（假端点不校验 Authorization）")
	})

	t.Run("api_key_env outside SKILLFORGE_* whitelist is not read (F-01)", func(t *testing.T) {
		t.Setenv("AWS_SECRET_ACCESS_KEY", "AKIA-should-never-leak")
		path := "/api/admin/llm/models?base_url=" + srv.URL + "&api_key_env=AWS_SECRET_ACCESS_KEY"
		invokeLLMAdmin(t, deps, http.MethodGet, path, "", true)
		// 断言方式：假端点会把 "Bearer wrong" 判 401，而 AWS key ≠ "wrong" ⇒ 放行；
		// 关键断言在服务端：resolveProbeKey 对非白名单名返回空。这里锁的是
		// 请求不被拒（匿名探测仍工作），真正的外泄防线由 envWhitelistedEnvName 单元测试锁。
	})

	t.Run("stored key does not follow a request-invented base_url (F-01/A-07)", func(t *testing.T) {
		// 先保存一份带 key 的配置（base 指向假端点），再以不同 base_url 发起
		// models 请求 ⇒ 已存 key 不得外带（resolveProbeKey 仅在 base 匹配时回落）。
		saveBody := `{"vendor":"custom","plan":"custom","base_url":"` + srv.URL + `","model":"m","api_key":"sk-stored-123456"}`
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config", saveBody, true))
		assert.Equal(t, true, m["ok"])

		other := newFakeOpenAI(t, `{"id":"cmpl-1","choices":[{"message":{"role":"assistant","content":"pong"}}]}`,
			`{"data":[{"id":"echo-key"}]}`)
		defer other.Close()
		// other 端点记录收到的 Authorization；这里只断言请求成功且不携带已存 key
		// ——由 resolveProbeKey 的 base 匹配约束保证（sk-stored 不会到达 other）。
		path := "/api/admin/llm/models?base_url=" + other.URL
		res := invokeLLMAdmin(t, deps, http.MethodGet, path, "", true)
		assert.Equal(t, 200, res.Result().StatusCode(), "匿名探测可达（未携带已存 key）")
	})
}

// TestLLMAdmin_EnvPresetConfigReportsEffectiveModel — 261113-lzz 缺陷 1：
// env preset 模式（无 ~/.skillforge/llm.json）下面板必须报告真实生效的
// 模型与端点。此前 env 分支 BaseURL/Model 无默认值，LOCAL_*/GLM_* 都未设
// 时报告 ""，而服务实际在跑 DefaultGLMModel —— 报告 ≠ 系统实际状态。
func TestLLMAdmin_EnvPresetConfigReportsEffectiveModel(t *testing.T) {
	// env preset 路径：llmTestDeps 已把 llm.json 指到空的临时目录（文件
	// 不存在 ⇒ Load present=false）；清空模型/端点变量，只留 key。
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("GLM_BASE_URL", "")
	t.Setenv("LOCAL_MODEL", "")
	t.Setenv("GLM_MODEL", "")
	t.Setenv("LOCAL_API_KEY", "")
	t.Setenv("GLM_API_KEY", "sk-env-preset-key-12345678")

	deps := llmTestDeps(t, nil)
	m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/config", "", true))
	assert.Equal(t, "env", m["source"])
	cfg, ok := m["config"].(map[string]any)
	require.True(t, ok, "env preset 必须有 config: %v", m)
	assert.Equal(t, llmconfig.DefaultGLMModel, cfg["model"],
		"面板报告的模型必须等于实际生效模型（与 BuildUpstream 同一回落链）")
	assert.Equal(t, llmconfig.DefaultGLMBaseURL, cfg["base_url"],
		"报告的端点必须等于实际拨号端点")

	// env 里显式设置模型时报告显式值（LOCAL_* 优先于 GLM_*）。
	t.Run("explicit LOCAL_MODEL wins", func(t *testing.T) {
		t.Setenv("LOCAL_MODEL", "my-model-override")
		m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet, "/api/admin/llm/config", "", true))
		cfg := m["config"].(map[string]any)
		assert.Equal(t, "my-model-override", cfg["model"])
	})
}

// TestLLMAdmin_EnvPresetModelsProbe — 261113-lzz 缺陷 2：env preset 模式
// 拉模型列表。此前 resolveProbeKey 只认白名单 env 名与已存文件配置，env
// preset 两者皆无 ⇒ 凭据退化为 "anonymous" ⇒ 真实网关拒绝 ⇒ 502。
// 假上游对 /models 强校验 Bearer key：env 凭据兜底不生效即 401 ⇒ 502，
// 测试失败 —— 200 即证明凭据来自 env 且随请求发出。
func TestLLMAdmin_EnvPresetModelsProbe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-env-preset-key-12345678" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"glm-5.3-flash"},{"id":"glm-5.3-airx"}]}`))
	})
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)

	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("GLM_BASE_URL", upstream.URL)
	t.Setenv("LOCAL_MODEL", "")
	t.Setenv("GLM_MODEL", "")
	t.Setenv("LOCAL_API_KEY", "")
	t.Setenv("GLM_API_KEY", "sk-env-preset-key-12345678")

	deps := llmTestDeps(t, nil)

	// base_url = env base ⇒ env 凭据可跟随（200 模型列表，非 502）。
	m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodGet,
		"/api/admin/llm/models?base_url="+upstream.URL, "", true))
	models := m["models"].([]any)
	require.Len(t, models, 2)
	assert.Equal(t, "glm-5.3-airx", models[0], "按字母序")
	assert.Equal(t, "glm-5.3-flash", models[1])

	// 安全约束回归（Task 3 兜底的另一半）：请求发明的 base（≠ env base）
	// 拿不到 env 凭据 —— env 的真实 key 绝不能被发往任意主机。
	//
	// 注意断言口径：后端在拿不到任何凭据时会发 "Bearer anonymous" 占位
	// （llm_admin.go:414 —— 部分网关允许无 key 列模型，如 ollama），那是
	// 「无凭据」的标记而非凭据泄漏。所以此处断言的是「env 真 key 未出现」，
	// 而不是「Authorization 头必须为空」。
	const envKey = "sk-env-preset-key-12345678"
	invented := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); strings.Contains(auth, envKey) {
			t.Errorf("env key leaked to a request-invented host: Authorization=%q", auth)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(invented.Close)
	res := invokeLLMAdmin(t, deps, http.MethodGet,
		"/api/admin/llm/models?base_url="+invented.URL, "", true)
	assert.Equal(t, http.StatusBadGateway, res.Result().StatusCode(),
		"发明端点拒收 anonymous ⇒ 502；env key 不得外带")
}

// TestLLMAdmin_ModelSwitch_EnvPreset — 261113-mkc：env preset 部署（无
// ~/.skillforge/llm.json）下聊天页模型选择器必须能切模型。此前无文件配置
// 一律 400（"save vendor/plan/api key first"），而部署常用 env preset
// （LOCAL_*/GLM_*）—— 面板 GET 已报告 env 形态（261113-lzz 缺陷 1），
// 切模型却仍是文件配置中心的盲区。
//
// 语义锁：模型写入进程环境 LOCAL_MODEL（BuildUpstream 的 env 回落链优先读
// 它 ⇒ 立即生效），端点/凭据继续用 env 既有值，文件配置不动；热重载后
// resolver 必须看到新模型；env key 只能以 masked 形态出现在响应里。
func TestLLMAdmin_ModelSwitch_EnvPreset(t *testing.T) {
	// env preset 前提：无文件配置（llmTestDeps 已把 SKILLFORGE_LLM_CONFIG
	// 指向不存在的临时路径）。相关变量全部先经 t.Setenv 注册（含空值），
	// handler 内部的 os.Setenv("LOCAL_MODEL", …) 才会在测试结束时被还原，
	// 不污染同包其他用例。
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("GLM_BASE_URL", "http://localhost:9999/v1")
	t.Setenv("LOCAL_MODEL", "")
	t.Setenv("GLM_MODEL", "")
	t.Setenv("LOCAL_API_KEY", "sk-env-preset-switch-123456")
	t.Setenv("GLM_API_KEY", "")

	// DynamicChatModel 构造需要一个 resolver；handler 走 applyLLMConfig() 后
	// 它会被真实 BuildUpstream 替换，所以这里注入的 fake 不参与断言。
	deps := llmTestDeps(t, func() (model.ToolCallingChatModel, error) {
		return &fakeLLMUp{}, nil
	})

	resp := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/model", `{"model":"my-model"}`, true)
	m := llmBody(t, resp) // 内置断言 200
	assert.Equal(t, true, m["ok"])
	assert.Equal(t, "my-model", m["model"], "响应必须回显切换后的模型")
	require.Contains(t, m, "available", "available 字段必须存在（值随 resolver 而定）")
	assert.Equal(t, true, m["available"])
	assert.Equal(t, "live", m["mode"])
	assert.NotEmpty(t, m["note"], "note 必须存在")
	assert.Contains(t, m["note"], "restart", "note 必须讲清进程重启即丢失（env 变量不持久化）")
	assert.Equal(t, "my-model", os.Getenv("LOCAL_MODEL"),
		"LOCAL_MODEL 必须被写入进程环境")

	// 「新模型真的进了上游构建路径」的可观测口径。
	//
	// 不能断言上面那个 fake resolver 看到了新模型：handler 的
	// applyLLMConfig() 会用真实 llm.BuildUpstream **替换**注入的 resolver，
	// fake resolver 在 handler 之后根本不会被调用（它只在 DynamicChatModel
	// 构造后的首次解析时跑一次，那次 LOCAL_MODEL 还是空）。
	//
	// 真正可观测的证据是 available==true —— 它来自 handler 末尾的
	// s.LLMDynamic.Available()，会强制走 resolve() → BuildUpstream，
	// 而 BuildUpstream 的 ENV_PRESET 分支正是读 LOCAL_MODEL 构造上游；
	// 模型名无效时它会返回 error，Available() 即为 false。
	assert.True(t, deps.LLMDynamic.Available(),
		"热重载后的上游必须能用新模型成功构建（Available() 触发真实 BuildUpstream）")

	// 安全约束：env key 绝不能以明文出现在任何响应字节里。
	raw := string(resp.Result().Body())
	assert.NotContains(t, raw, "sk-env-preset-switch-123456", "env key 明文不得出现在响应里")
	masked, _ := m["masked_key"].(string)
	assert.NotEmpty(t, masked, "masked_key 必须存在（env key 的 masked 形态）")
	assert.NotContains(t, masked, "123456", "masked key 绝不能含明文片段")
}

// TestLLMAdmin_ModelSwitch_NothingConfigured — 文件配置与 env preset 凭据
// 都不存在时保持 400，且错误文案必须把两条途径都说清（面板保存 / env 凭据）。
func TestLLMAdmin_ModelSwitch_NothingConfigured(t *testing.T) {
	t.Setenv("LOCAL_API_KEY", "")
	t.Setenv("GLM_API_KEY", "")

	deps := llmTestDeps(t, nil) // 无文件配置（空临时目录）
	r := invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/model", `{"model":"m"}`, true).Result()
	require.Equal(t, http.StatusBadRequest, r.StatusCode())
	var e graphResponseError
	require.NoError(t, json.Unmarshal(r.Body(), &e))
	assert.Contains(t, e.Error, "panel", "文案必须提到面板保存途径")
	assert.Contains(t, e.Error, "LOCAL_API_KEY", "文案必须提到 env preset 途径")
}

// TestLLMAdmin_ModelSwitch_FileConfig — 文件配置路径行为不变（261113-mkc
// 只在无文件配置时新增 env preset 分支）：Load → 改 Model → Save → 热重载，
// 新模型落盘；LOCAL_MODEL 不被触碰（那是 env 路径专属的写入）。
func TestLLMAdmin_ModelSwitch_FileConfig(t *testing.T) {
	t.Setenv("LOCAL_MODEL", "")
	deps := llmTestDeps(t, func() (model.ToolCallingChatModel, error) {
		if _, present, err := readSavedConfig(t); err == nil && present {
			return &fakeLLMUp{}, nil
		}
		return nil, nil
	})
	llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/config",
		`{"vendor":"opencode","plan":"go","model":"glm-5.3-flash","api_key":"sk-file-12345678"}`, true))

	m := llmBody(t, invokeLLMAdmin(t, deps, http.MethodPost, "/api/admin/llm/model", `{"model":"glm-5.3-airx"}`, true))
	assert.Equal(t, true, m["ok"])
	assert.Equal(t, "glm-5.3-airx", m["model"])
	assert.Equal(t, "live", m["mode"])

	b, err := os.ReadFile(os.Getenv("SKILLFORGE_LLM_CONFIG"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "glm-5.3-airx", "文件配置必须持久化新模型")
	assert.Equal(t, "", os.Getenv("LOCAL_MODEL"), "文件路径不得写 LOCAL_MODEL")
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
