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

// Package llmconfig 是 Phase 52 的 LLM provider 配置域：厂商/套餐目录
// （catalog.go）、本机配置持久化（store.go）、连通性测试与模型列表
// （probe.go）。只供 admin 面板与装配层使用，不进入业务请求路径。
package llmconfig

// Plan 是一个厂商下的一种套餐/接入方式。
type Plan struct {
	ID        string // "coding-plan", "go", "zen", ...
	Label     string // 人类可读名称
	BaseURL   string // OpenAI 兼容 chat/completions base；custom 时为空串由用户填
	ModelsURL string // 模型列表端点；空 = BaseURL + "/models"
	Note      string // 一句话说明（计费/隐私/前置条件）
}

// Vendor 是一个 LLM 厂商，含其全部可选套餐。
type Vendor struct {
	ID    string // "zhipu", "opencode", "openai", "deepseek", ...
	Label string
	Plans []Plan
}

// Catalog 返回内置厂商/套餐目录。每次调用返回全新切片，调用方可自由
// 修改而不影响后续调用（避免面板并发读写共享目录时踩脏数据）。
func Catalog() []Vendor {
	return []Vendor{
		{
			ID:    "zhipu",
			Label: "智谱 GLM",
			Plans: []Plan{
				{
					ID:      "coding-plan",
					Label:   "Coding Plan（包月）",
					BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
					Note:    "GLM Coding Plan 包月套餐；计费按订阅额度，不按 token",
				},
				{
					ID:      "standard",
					Label:   "标准 API（按量）",
					BaseURL: "https://open.bigmodel.cn/api/paas/v4",
					Note:    "标准开放平台 API，按 token 计费",
				},
			},
		},
		{
			ID:    "opencode",
			Label: "OpenCode",
			Plans: []Plan{
				{
					ID:        "go",
					Label:     "Go Plan（$10/月）",
					BaseURL:   "https://opencode.ai/inference/openai/v1",
					ModelsURL: "https://opencode.ai/inference/v1/models",
					Note:      "Console 订阅 $10/月；免费档模型为 *-free 后缀（如 grok-code-free）",
				},
				{
					ID:        "go-plus",
					Label:     "Go Plus（$40/月）",
					BaseURL:   "https://opencode.ai/inference/openai/v1",
					ModelsURL: "https://opencode.ai/inference/v1/models",
					Note:      "Console 订阅 $40/月，额度更高；免费档模型为 *-free 后缀",
				},
				{
					ID:        "zen",
					Label:     "Zen（按量）",
					BaseURL:   "https://opencode.ai/inference/openai/v1",
					ModelsURL: "https://opencode.ai/inference/v1/models",
					Note:      "Console 按量计费；免费档模型为 *-free 后缀",
				},
			},
		},
		{
			ID:    "openai",
			Label: "OpenAI",
			Plans: []Plan{
				{
					ID:      "standard",
					Label:   "官方 API",
					BaseURL: "https://api.openai.com/v1",
					Note:    "OpenAI 官方 API，按 token 计费",
				},
			},
		},
		{
			ID:    "deepseek",
			Label: "DeepSeek",
			Plans: []Plan{
				{
					ID:      "standard",
					Label:   "官方 API",
					BaseURL: "https://api.deepseek.com/v1",
					Note:    "DeepSeek 开放平台，按 token 计费",
				},
			},
		},
		{
			ID:    "moonshot",
			Label: "Moonshot (Kimi)",
			Plans: []Plan{
				{
					ID:      "standard",
					Label:   "官方 API",
					BaseURL: "https://api.moonshot.cn/v1",
					Note:    "月之暗面开放平台，按 token 计费",
				},
			},
		},
		{
			ID:    "siliconflow",
			Label: "SiliconFlow（硅基流动）",
			Plans: []Plan{
				{
					ID:      "standard",
					Label:   "官方 API",
					BaseURL: "https://api.siliconflow.cn/v1",
					Note:    "硅基流动聚合平台，按 token 计费",
				},
			},
		},
		{
			ID:    "ollama",
			Label: "Ollama（本地）",
			Plans: []Plan{
				{
					ID:      "local",
					Label:   "本地服务",
					BaseURL: "http://localhost:11434/v1",
					Note:    "需先在本机运行 ollama serve；模型在本地推理，不出网",
				},
			},
		},
		{
			ID:    "custom",
			Label: "自定义第三方",
			Plans: []Plan{
				{
					ID:      "custom",
					Label:   "自定义 OpenAI 兼容端点",
					BaseURL: "", // 由用户在面板里填写（Plan.BaseURL 为空 ⇒ 前端必填 base_url）
					Note:    "任意 OpenAI 兼容网关；base_url 需填到 /v1 一级",
				},
			},
		},
	}
}

// FindPlan 按 vendor/plan 双 ID 在目录中查找套餐；找不到返回 false。
func FindPlan(vendorID, planID string) (Plan, bool) {
	for _, v := range Catalog() {
		if v.ID != vendorID {
			continue
		}
		for _, p := range v.Plans {
			if p.ID == planID {
				return p, true
			}
		}
	}
	return Plan{}, false
}
