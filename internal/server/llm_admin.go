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

// Phase 52 (llm-provider-panel) — LLM provider 管理端点。
//
// Wire contract（全部 X-Admin-Token 门控，403 先于任何 503/400）：
//
//	GET  /api/admin/llm/providers          → 200 {vendors: [...]}（静态目录）
//	GET  /api/admin/llm/config             → 200 {source, config(masked), mode, available}
//	POST /api/admin/llm/config             → 200 {ok, available, masked_key} | 400 {error} | 503
//	POST /api/admin/llm/test               → 200 TestResult | 400 {error} | 503
//	GET  /api/admin/llm/models?vendor=&plan=&base_url=&api_key_env=
//	                                       → 200 {models} | 400 {error} | 503
//	POST /api/admin/llm/model {model}      → 200 {ok, model, available, masked_key}
//	                                       | 400 {error} | 503 —— 只切模型,
//	                                       密钥不出服务端(聊天页选择器用)。
//
// 关键语义（D-52 系列）：
//   - GET 永不返回明文 key（T-52-01：llmconfig.Mask）。
//   - POST 保存后立即热重载（LLMDynamic.SetResolver），无需重启。
//   - Deps.LLMDynamic == nil ⇒ 面板功能未装配，端点 503（照 Wizard/Knowledge
//     的 nil-posture 惯例）。
//   - 保存的配置持久化在 ~/.skillforge/llm.json（0600，不入仓库）。
package server

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/llm"
	"github.com/skillforge/skillforge/internal/llmconfig"
)

// envOr 是 cmd/server/main.go 同名 helper 的 server 侧副本（该 helper 在
// main 包里，不可导入）；Phase 52 的 env 回落报告使用。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// llmConfigResponse 是 GET /api/admin/llm/config 的响应体。
// Key 永远以 masked 形态返回（T-52-01）。
type llmConfigResponse struct {
	Source    string           `json:"source"` // "file" | "env" | "none"
	Config    *llmConfigPublic `json:"config,omitempty"`
	Mode      string           `json:"mode"`     // "live" | "stub"
	Available bool             `json:"available"`
}

// llmConfigPublic 是对外可见的配置形态（key masked）。
type llmConfigPublic struct {
	Vendor    string `json:"vendor"`
	Plan      string `json:"plan"`
	BaseURL   string `json:"base_url,omitempty"`
	Model     string `json:"model"`
	MaskedKey string `json:"masked_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// llmSaveRequest 是 POST /api/admin/llm/config 的请求体。
// APIKey 与 APIKeyEnv 二选一；都为空 = 错误（新配置必须有 key）。
type llmSaveRequest struct {
	Vendor    string `json:"vendor"`
	Plan      string `json:"plan"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	APIKeyEnv string `json:"api_key_env"`
}

// llmTestRequest 是 POST /api/admin/llm/test 的请求体；全部字段可选——
// 省略时用已保存的文件配置（没有则 400）。
type llmTestRequest struct {
	Vendor    string `json:"vendor"`
	Plan      string `json:"plan"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	APIKeyEnv string `json:"api_key_env"`
}

// llmProvidersHandler implements GET /api/admin/llm/providers.
func (s *Deps) llmProvidersHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, map[string]any{"vendors": llmconfig.Catalog()})
}

// llmConfigHandler implements GET /api/admin/llm/config.
func (s *Deps) llmConfigHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.LLMDynamic == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "llm provider panel not initialized"})
		return
	}

	resp := llmConfigResponse{Mode: "stub"}
	// GET 不得改变系统状态，但这里的语义是"报告当前生效模式"——生产里
	// LLMDynamic 即 Deps.ChatModel（同一指针），两处读取等价；测试里 Deps
	// 常只装 LLMDynamic（不装 ChatModel），所以直接问 LLMDynamic。
	resp.Available = s.LLMDynamic.Available()
	if resp.Available {
		resp.Mode = "live"
	}

	cfg, present, err := llmconfig.Load()
	switch {
	case err != nil:
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "llm config unreadable: " + err.Error()})
		return
	case present:
		resp.Source = "file"
		resp.Config = publicConfig(cfg)
	default:
		// 回落 env（LOCAL_*/GLM_*）：只报告事实，不回写文件。
		key := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))
		if key != "" {
			resp.Source = "env"
			resp.Config = &llmConfigPublic{
				Vendor:    "env",
				Plan:      "env",
				BaseURL:   envOr("LOCAL_BASE_URL", envOr("GLM_BASE_URL", "")),
				Model:     envOr("LOCAL_MODEL", envOr("GLM_MODEL", "")),
				MaskedKey: llmconfig.Mask(key),
			}
		} else {
			resp.Source = "none"
		}
	}
	c.JSON(http.StatusOK, resp)
}

// publicConfig 把内部配置转成对外形态（key masked）。
func publicConfig(cfg llmconfig.Config) *llmConfigPublic {
	p := &llmConfigPublic{
		Vendor:    cfg.Vendor,
		Plan:      cfg.Plan,
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		APIKeyEnv: cfg.APIKeyEnv,
		UpdatedAt: cfg.UpdatedAt,
	}
	if cfg.APIKey != "" {
		p.MaskedKey = llmconfig.Mask(cfg.APIKey)
	}
	return p
}

// llmSaveHandler implements POST /api/admin/llm/config —— 保存 + 立即热重载。
func (s *Deps) llmSaveHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.LLMDynamic == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "llm provider panel not initialized"})
		return
	}
	var req llmSaveRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid JSON body"})
		return
	}
	cfg := llmconfig.Config{
		Vendor:    req.Vendor,
		Plan:      req.Plan,
		BaseURL:   req.BaseURL,
		Model:     req.Model,
		APIKey:    req.APIKey,
		APIKeyEnv: req.APIKeyEnv,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	// CODE-AUDIT 2026-11-05 (E-01): the panel UI promises "API Key 留空则不改动
	// 已存 key" — honor it. The GET response never echoes the plaintext key
	// back, so a panel round-trip (load → tweak model → save) arrives with
	// both key fields empty; before this fix Validate() 400'd and the ONLY
	// workaround was re-entering the plaintext key (which fetchLLMModels
	// then leaked into the URL). Preserve the stored credential when the
	// request omits both; a first-ever save with no key still 400s via
	// Validate below.
	if req.APIKey == "" && req.APIKeyEnv == "" {
		if stored, present, err := llmconfig.Load(); err == nil && present {
			cfg.APIKey = stored.APIKey
			cfg.APIKeyEnv = stored.APIKeyEnv
		}
	}
	if err := cfg.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}
	if err := llmconfig.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "save failed: " + err.Error()})
		return
	}

	// 立即热重载：换 resolver，新请求即走新上游（无需重启）。
	s.applyLLMConfig()

	ok := s.LLMDynamic.Available()
	c.JSON(http.StatusOK, map[string]any{
		"ok":         true,
		"available":  ok,
		"mode":       map[bool]string{true: "live", false: "stub"}[ok],
		"masked_key": publicConfig(cfg).MaskedKey,
		"note":       "hot-reloaded; no server restart needed",
	})
}

// applyLLMConfig 用"每次调用重新 Load 文件"的 resolver 热装配 LLMDynamic。
// 文件被手工编辑也生效；Load 是本地小文件读取，代价可接受（T-52-05）。
// 构造失败（key 被撤、URL 坏）⇒ resolver 返回错误 ⇒ Available()=false，
// 管道回落 stub —— 服务不死（Phase 52 决策：启动/运行都不因坏上游 Fatalf）。
func (s *Deps) applyLLMConfig() {
	if s.LLMDynamic == nil {
		return
	}
	s.LLMDynamic.SetResolver(func() (model.ToolCallingChatModel, error) {
		cfg, present, err := llmconfig.Load()
		if err != nil {
			return nil, err
		}
		if !present {
			return nil, errNoFileConfig
		}
		key, err := llmconfig.ResolveKey(cfg)
		if err != nil {
			return nil, err
		}
		base := cfg.BaseURL
		if base == "" {
			plan, ok := llmconfig.FindPlan(cfg.Vendor, cfg.Plan)
			if !ok {
				return nil, errUnknownVendorPlan
			}
			base = plan.BaseURL
		}
		cm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
			BaseURL: base,
			APIKey:  key,
			Model:   cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return cm, nil
	})
}

// errNoFileConfig / errUnknownVendorPlan 是 resolver 的内部哨兵
// （Available()=false 的两条主因，面板上可据此提示）。
var (
	errNoFileConfig      = errStr("no llm config file (stub mode)")
	errUnknownVendorPlan = errStr("unknown vendor/plan in llm config")
)

type errStr string

func (e errStr) Error() string { return string(e) }

// llmTestHandler implements POST /api/admin/llm/test.
func (s *Deps) llmTestHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.LLMDynamic == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "llm provider panel not initialized"})
		return
	}
	var req llmTestRequest
	_ = c.BindAndValidate(&req) // 空 body 合法 ⇒ 用已存配置

	base, key, modelName, err := s.resolveTestTarget(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, llmconfig.TestKey(ctx, base, key, modelName))
}

// resolveTestTarget 把测试请求解析成 (baseURL, key, model)：
// 请求字段优先；缺失字段从已存文件配置 / 目录 plan 补齐。
//
// CODE-AUDIT 2026-11-05 (F-01/A-07)：三个安全约束——
//  1. URL 来源：catalog 厂商的 base_url 仅在与目录值一致时被采纳（SSRF 加固）；
//  2. api_key_env 仅接受 SKILLFORGE_* 白名单（防任意环境变量外泄）；
//  3. 已存配置的 key 仅在最终 base_url 与已存配置一致时回落（防把 key 发往任意主机）。
func (s *Deps) resolveTestTarget(req llmTestRequest) (string, string, string, error) {
	base := req.BaseURL
	modelName := req.Model
	vendor, plan := req.Vendor, req.Plan

	if base == "" || modelName == "" {
		if cfg, present, err := llmconfig.Load(); err == nil && present {
			if base == "" {
				base = cfg.BaseURL
			}
			if modelName == "" {
				modelName = cfg.Model
			}
			if vendor == "" {
				vendor = cfg.Vendor
			}
			if plan == "" {
				plan = cfg.Plan
			}
		}
	}
	if base == "" {
		p, ok := llmconfig.FindPlan(vendor, plan)
		if !ok {
			return "", "", "", errStr("unknown vendor/plan and no base_url given")
		}
		base = p.BaseURL
	}
	if modelName == "" {
		return "", "", "", errStr("model is required for a connectivity test")
	}

	key := req.APIKey // 调用方显式提供的 key 属于其自身凭据，直接放行（端点自选）
	if key == "" && req.APIKeyEnv != "" && envWhitelistedEnvName(req.APIKeyEnv) {
		key = os.Getenv(req.APIKeyEnv)
	}
	if key == "" {
		// 依赖服务端持有的凭据（env / 已存配置）时，base 必须先过 URL 来源约束
		// ——否则"把服务端 key 发往任意主机"的通道在这里重开。
		if req.BaseURL != "" {
			constrained, _ := resolveProbeURLs(vendor, plan, req.BaseURL, "")
			base = constrained
		}
		key = resolveProbeKey(base, "")
	}
	if key == "" {
		return "", "", "", errStr("api_key required (or save a config first)")
	}
	return base, key, modelName, nil
}

// envWhitelistedEnvName — CODE-AUDIT 2026-11-05 (F-01): api_key_env values
// are restricted to SKILLFORGE_* names. Without this, a stolen admin token
// could read ANY process env var (cloud credentials, DATABASE_URL) and have
// the server send it as a Bearer header to a caller-chosen URL.
func envWhitelistedEnvName(name string) bool {
	return strings.HasPrefix(name, "SKILLFORGE_")
}

// resolveProbeURLs applies the F-01/A-07 URL-source constraint: for catalog
// vendors the request-supplied base_url/models_url are honored ONLY when
// they byte-match the catalog plan values — otherwise the catalog values
// win. A stolen admin token can therefore no longer point the server-side
// probe at an arbitrary host while a real key is attached. vendor=custom
// (and unknown vendors) keep the request/saved values: custom endpoints are
// operator-registered on purpose (e.g. ollama on localhost).
func resolveProbeURLs(vendor, plan, reqBase, reqModelsURL string) (baseURL, modelsURL string) {
	p, ok := llmconfig.FindPlan(vendor, plan)
	if !ok {
		return reqBase, reqModelsURL
	}
	if reqBase == p.BaseURL {
		baseURL = reqBase
	} else {
		baseURL = p.BaseURL
	}
	if reqModelsURL == p.ModelsURL {
		modelsURL = reqModelsURL
	} else {
		modelsURL = p.ModelsURL
	}
	return baseURL, modelsURL
}

// resolveProbeKey resolves the probe credential under the F-01/A-07 key-source
// constraints: (1) api_key_env only via the SKILLFORGE_* whitelist; (2) the
// stored file config's key may only follow a base URL the admin themselves
// registered (request-invented URLs get no stored key — this closes the
// "send the stored key to any host" and "echo it back via the response"
// read-back channel).
func resolveProbeKey(baseURL, apiKeyEnv string) string {
	if apiKeyEnv != "" && envWhitelistedEnvName(apiKeyEnv) {
		if v := os.Getenv(apiKeyEnv); v != "" {
			return v
		}
	}
	if cfg, present, err := llmconfig.Load(); err == nil && present {
		storedBase := cfg.BaseURL
		if storedBase == "" {
			if p, ok := llmconfig.FindPlan(cfg.Vendor, cfg.Plan); ok {
				storedBase = p.BaseURL
			}
		}
		if baseURL != "" && baseURL == storedBase {
			key, _ := llmconfig.ResolveKey(cfg)
			return key
		}
	}
	return ""
}

// llmModelsHandler implements GET /api/admin/llm/models.
//
// CODE-AUDIT 2026-11-05 (F-02): the api_key query parameter is GONE — keys
// travel via api_key_env (whitelisted) or the server-stored config only.
func (s *Deps) llmModelsHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.LLMDynamic == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "llm provider panel not initialized"})
		return
	}
	vendor := string(c.Query("vendor"))
	plan := string(c.Query("plan"))
	baseURL, modelsURL := resolveProbeURLs(vendor, plan,
		string(c.Query("base_url")), string(c.Query("models_url")))
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "base_url required (pick a vendor/plan or pass base_url)"})
		return
	}
	apiKey := resolveProbeKey(baseURL, string(c.Query("api_key_env")))
	if apiKey == "" {
		// 部分网关允许无 key 列模型（如 ollama）；不强制。
		apiKey = "anonymous"
	}
	models, err := llmconfig.ListModels(ctx, modelsURL, baseURL, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, graphResponseError{Error: "model list failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, map[string]any{"models": models})
}

// llmModelSwitchRequest is the POST /api/admin/llm/model body. Key is
// deliberately absent — the chat-page model picker must never see or
// resend it; the server-side file config (~/.skillforge/llm.json, 0600)
// already holds it.
type llmModelSwitchRequest struct {
	Model string `json:"model"`
}

// llmModelHandler implements POST /api/admin/llm/model —— 只切模型:
// Load 现有文件配置 → 改 Model → Save → 立即热重载。密钥永不出服务端,
// 前端只需 Admin Token。尚未保存过完整配置(无 file)时 400 —— 让用户
// 先去 admin LLM 面板保存一次(Vendor/Plan/BaseURL/key)。
func (s *Deps) llmModelHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.LLMDynamic == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "llm provider panel not initialized"})
		return
	}
	var req llmModelSwitchRequest
	if err := c.BindAndValidate(&req); err != nil || strings.TrimSpace(req.Model) == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid JSON body: model is required"})
		return
	}
	cfg, present, err := llmconfig.Load()
	if err != nil || !present || cfg.BaseURL == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "no saved llm config — save vendor/base_url/api key once in the admin LLM panel first"})
		return
	}
	cfg.Model = strings.TrimSpace(req.Model)
	if err := cfg.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}
	if err := llmconfig.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "save failed: " + err.Error()})
		return
	}
	s.applyLLMConfig()
	ok := s.LLMDynamic.Available()
	c.JSON(http.StatusOK, map[string]any{
		"ok":         true,
		"model":      cfg.Model,
		"available":  ok,
		"mode":       map[bool]string{true: "live", false: "stub"}[ok],
		"masked_key": publicConfig(cfg).MaskedKey,
		"note":       "hot-reloaded; no server restart needed",
	})
}

// 确保接口实现（编译期护栏）。
var (
	_ model.ToolCallingChatModel = (*llm.DynamicChatModel)(nil)
)
