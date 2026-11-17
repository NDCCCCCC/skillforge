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

package llm

// 261117-adp — 原生适配器分派（用户决策：能原生就原生，其余 OpenAI 兜底）。
//
// 背景：本项目此前只引入了 eino-ext 的 openai 一个适配器，所有厂商（包括
// DeepSeek、Ollama）都走 OpenAI 兼容协议。OpenAI 兼容确实覆盖面广，但它是
// **最低公约数**：每家非标准字段（智谱 thinking、Ollama think、DeepSeek 的
// reasoning_content 提取）都得靠 extrafields.go 手写补丁，补丁与厂商行为
// 一漂移就是线上 bug —— 用户原话「有原生的不用，所以才出一堆bug」。
//
// 现在按 Plan.Adapter 分派：
//   - "deepseek" → eino-ext/components/model/deepseek（原生 DeepSeek 协议）
//   - "ollama"   → eino-ext/components/model/ollama（原生 Ollama 协议）
//   - "ark"      → eino-ext/components/model/ark（火山方舟）
//   - ""/"openai"→ eino-ext/components/model/openai（**兜底**：所有 OpenAI
//                   兼容网关，含自定义第三方端点）
//
// 版本约束：deepseek@v0.1.7 / ollama@v0.1.9 要求 eino ≥ v0.7.13，
// ark@v0.1.60 同样（ark v0.1.7x 要求 eino v0.9.22，会把项目从 v0.9.14
// 拽上去 —— 因此锁在 v0.1.60，升级 eino 时才一并提升）。
//
// 为什么兜底留在 OpenAI 而不是"每个厂商写一个 else if"：自定义第三方端点
// （catalog 的 custom vendor）本质上就是「任意 OpenAI 兼容网关」的入口，
// 它必须永远可用 —— 那是用户的逃生舱。

import (
	"context"
	"fmt"

	einoark "github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// AdapterOpenAI 是兜底适配器 id（也是零值语义：空串按 openai 处理）。
const AdapterOpenAI = "openai"

// AdapterDeepSeek / AdapterOllama / AdapterArk 是 eino 原生适配器 id。
const (
	AdapterDeepSeek = "deepseek"
	AdapterOllama   = "ollama"
	AdapterArk      = "ark"
)

// adapterSpec 是一次模型构造的完整输入。字段与各适配器配置的并集对齐，
// 由 buildByAdapter 按适配器类型选择性使用。
type adapterSpec struct {
	Adapter string
	BaseURL string
	APIKey  string
	Model   string
}

// buildByAdapter 按 spec.Adapter 选择 eino 原生适配器构造 ChatModel。
//
// 所有分支返回同一个 eino 抽象（*XxxChatModel 都实现
// model.ToolCallingChatModel），因此调用方无需感知适配器差异。
//
// 未知的 adapter id **不**静默降级到 openai —— 那会把一个配置笔误变成
// 「看起来能用但打错端点」的静默故障；这里返回明确错误，让面板的测试
// 按钮把问题说清楚。
func buildByAdapter(ctx context.Context, spec adapterSpec) (model.ToolCallingChatModel, error) {
	switch spec.Adapter {
	case "", AdapterOpenAI:
		cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			BaseURL: spec.BaseURL,
			APIKey:  spec.APIKey,
			Model:   spec.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("openai adapter: %w", err)
		}
		return cm, nil

	case AdapterDeepSeek:
		// DeepSeek 原生适配器自己处理 reasoning_content（deepseek-reasoner 的
		// 思维链字段）与 tool_calls 的格式差异 —— 这正是走 openai 兼容路径
		// 时需要手写补丁的地方。APIKey 必填，BaseURL 可覆盖（自建代理）。
		cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			BaseURL: spec.BaseURL,
			APIKey:  spec.APIKey,
			Model:   spec.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("deepseek adapter: %w", err)
		}
		return cm, nil

	case AdapterOllama:
		// Ollama 原生适配器用 /api/chat 而非 OpenAI 兼容端点，其 HTTPClient
		// 超时与 keep_alive 语义也不同。BaseURL 应是 http://host:11434（不带
		// /v1）；catalog 里的 ollama plan 为此单独标注。
		cm, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: spec.BaseURL,
			Model:   spec.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("ollama adapter: %w", err)
		}
		return cm, nil

	case AdapterArk:
		// 火山方舟：Model 是方舟上的 endpoint id，不是模型名。APIKey 形态
		// 与智谱不同，因此这一个必须用原生适配器，不能走 OpenAI 兼容。
		cm, err := einoark.NewChatModel(ctx, &einoark.ChatModelConfig{
			BaseURL: spec.BaseURL,
			APIKey:  spec.APIKey,
			Model:   spec.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("ark adapter: %w", err)
		}
		return cm, nil

	default:
		return nil, fmt.Errorf("llm: unknown adapter %q (supported: %q, %q, %q, %q)",
			spec.Adapter, AdapterOpenAI, AdapterDeepSeek, AdapterOllama, AdapterArk)
	}
}

// adapterOf 把 Plan.Adapter 归一化成具体 id（空串 = openai 兜底）。
func adapterOf(planAdapter string) string {
	if planAdapter == "" {
		return AdapterOpenAI
	}
	return planAdapter
}
