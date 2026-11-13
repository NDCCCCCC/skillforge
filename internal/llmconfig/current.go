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
	"fmt"
	"os"
	"strings"
)

// DefaultGLMBaseURL 是 env preset 路径（无文件配置）的默认拨号端点，
// 与 cmd/server 的 defaultGLMBaseURL 逐字一致。导出它是为了让
// internal/llm 的共享上游工厂与这里的端点能力判定共用同一个事实来源，
// 避免三处字面量漂移。
const DefaultGLMBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"

// DefaultGLMModel 是 env preset 路径（无文件配置）的默认模型 id，
// 与 upstream.go / cmd/server 的 env 回落语义一致。导出它是为了让
// 「实际生效模型」与「面板报告模型」共用同一事实来源，避免字面量漂移。
const DefaultGLMModel = "glm-5.3-flash"

// CurrentVendor 返回文件 LLM 配置的 vendor ID；无配置或读取失败返回 ""。
// 读失败按未知厂商处理（保守：调用方沿用默认请求字段）——与 resolver 的
// fail-open-to-stub 姿态一致，这里绝不需要 Fatal。
//
// NOTE (CODE-AUDIT 2026-11-05 A-05/A-06): 这是"声明的 vendor"，不等于
// 实际拨号端点。请求字段策略请一律改用 EffectiveBaseURL() /
// SupportsThinkingDisabled()，不要再用本函数做能力判断。
func CurrentVendor() string {
	cfg, present, err := Load()
	if err != nil || !present {
		return ""
	}
	return cfg.Vendor
}

// EffectiveBaseURL 返回当前**实际会拨号**的 LLM 端点 —— 这是请求字段
// 能力判定的唯一事实来源（CODE-AUDIT 2026-11-05 A-05/A-06）：
//
//   - 文件配置存在：cfg.BaseURL；为空则回落 catalog plan 的 BaseURL
//     （与 llm.BuildUpstream / llmModelHandler 的回落语义一致）；
//   - 无文件配置：env preset 路径（LOCAL_BASE_URL 覆盖 GLM_BASE_URL，
//     再回落 DefaultGLMBaseURL）—— 补上 A-06 缺失的"env preset 也有
//     端点概念"，此前 CurrentVendor() 在这条路径返回 "" 导致 MiniMax
//     经 GLM_* 部署时收到 thinking:disabled → 全量 400/2013。
//
// 以"实际端点"而非"声明 vendor"为准，顺带修掉 A-05：vendor=zhipu 但
// base_url 覆盖到 api.minimaxi.com 时，不再误发 GLM 专属字段。
func EffectiveBaseURL() string {
	cfg, present, err := Load()
	if err == nil && present {
		if cfg.BaseURL != "" {
			return cfg.BaseURL
		}
		if p, ok := FindPlan(cfg.Vendor, cfg.Plan); ok {
			return p.BaseURL
		}
		return ""
	}
	// env preset：与 cmd/server 的 envOr 链逐字一致。
	if v := os.Getenv("LOCAL_BASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("GLM_BASE_URL"); v != "" {
		return v
	}
	return DefaultGLMBaseURL
}

// Effective 返回当前生效的 LLM 连接三元组 (baseURL, apiKey, model) ——
// 「页面上改了什么」与「基线跑的是什么」必须同一套，所以这是 eval 与
// 服务端共用的**单一事实来源**（261113-obm）。
//
// 解析顺序与 internal/llm.BuildUpstream **逐字一致**（PANEL_FILE >
// ENV_PRESET；llm 反向 import 本包，所以顺序在这里定义、那边照抄）：
//
//  1. 文件配置（~/.skillforge/llm.json，admin 面板保存）present 时：
//     baseURL = cfg.BaseURL，空则回落 catalog plan（FindPlan，A-04
//     parity）；apiKey = ResolveKey(cfg)；model = cfg.Model。
//  2. 无文件配置（或文件读失败 —— BuildUpstream 同样把 cfgErr 当作
//     无可用配置）回落 env preset：baseURL = LOCAL_BASE_URL >
//     GLM_BASE_URL > DefaultGLMBaseURL；apiKey = LOCAL_API_KEY >
//     GLM_API_KEY；model = LOCAL_MODEL > JUDGE_MODEL > DefaultGLMModel。
//     （model 链的中段是 JUDGE_MODEL —— 这是 eval 的 env 契约；服务端
//     BuildUpstream 的中段是 GLM_MODEL。除这一档外逐字一致。）
//
// 明确失败原则：文件配置存在但 key 解析失败（如 api_key_env 指向未
// 设置的变量）时返回 err，**不**静默降级到 env preset —— 页面明明配了
// 却拨到别处，比当场报错更难排查。
func Effective() (baseURL, apiKey, model string, err error) {
	cfg, present, loadErr := Load()
	if loadErr == nil && present {
		key, keyErr := ResolveKey(cfg)
		if keyErr != nil {
			return "", "", "", fmt.Errorf("llmconfig: Effective: %w", keyErr)
		}
		baseURL = cfg.BaseURL
		if baseURL == "" {
			if p, ok := FindPlan(cfg.Vendor, cfg.Plan); ok {
				baseURL = p.BaseURL
			}
		}
		return baseURL, key, cfg.Model, nil
	}
	// env preset：与 cmd/server 的 envOr 链逐字一致。
	if v := os.Getenv("LOCAL_BASE_URL"); v != "" {
		baseURL = v
	} else if v := os.Getenv("GLM_BASE_URL"); v != "" {
		baseURL = v
	} else {
		baseURL = DefaultGLMBaseURL
	}
	if v := os.Getenv("LOCAL_API_KEY"); v != "" {
		apiKey = v
	} else if v := os.Getenv("GLM_API_KEY"); v != "" {
		apiKey = v
	}
	if v := os.Getenv("LOCAL_MODEL"); v != "" {
		model = v
	} else if v := os.Getenv("JUDGE_MODEL"); v != "" {
		model = v
	} else {
		model = DefaultGLMModel
	}
	return baseURL, apiKey, model, nil
}

// SupportsThinkingDisabled 按实际拨号端点判定是否下发
// thinking:{type:disabled}（CODE-AUDIT 2026-11-05 A-10：由"minimax 单点
// 黑名单"反转为**能力白名单**）。
//
// 目前只有智谱 GLM 直连端点明确支持该参数。其余端点一律不下发：
//   - MiniMax M3.x 会因该字段返回 400/2013（requires adaptive thinking）；
//   - OpenAI / DeepSeek / Moonshot / SiliconFlow / Ollama 等 OpenAI 兼容
//     端点本就没有 thinking 参数，收到只是无收益的未知字段，一旦对方
//     启用严格 body 校验即复现同类 400。
//
// 返回 false 是安全默认：未知/新端点不会被悄悄塞进一个可能致命的字段。
func SupportsThinkingDisabled() bool {
	return strings.Contains(strings.ToLower(EffectiveBaseURL()), "open.bigmodel.cn")
}
