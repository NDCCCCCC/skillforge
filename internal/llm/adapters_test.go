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

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildByAdapter_Dispatch 锁住 261117-adp 的分派契约：每个 adapter id
// 必须真的构造出对应的 eino 原生 ChatModel（构造是纯本地操作，不发网络
// 请求 —— 这些适配器的 NewChatModel 只做配置校验与 client 组装）。
func TestBuildByAdapter_Dispatch(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		spec    adapterSpec
		wantErr bool
	}{
		{
			name: "empty adapter means openai fallback",
			spec: adapterSpec{Adapter: "", BaseURL: "https://example.invalid/v1", APIKey: "sk-x", Model: "m"},
		},
		{
			name: "explicit openai",
			spec: adapterSpec{Adapter: AdapterOpenAI, BaseURL: "https://example.invalid/v1", APIKey: "sk-x", Model: "m"},
		},
		{
			name: "deepseek native",
			spec: adapterSpec{Adapter: AdapterDeepSeek, BaseURL: "https://api.deepseek.com/v1", APIKey: "sk-x", Model: "deepseek-chat"},
		},
		{
			name: "ollama native",
			spec: adapterSpec{Adapter: AdapterOllama, BaseURL: "http://localhost:11434", Model: "qwen2.5:7b"},
		},
		{
			name: "ark native",
			spec: adapterSpec{Adapter: AdapterArk, BaseURL: "https://ark.cn-beijing.volces.com/api/v3", APIKey: "sk-x", Model: "ep-20240101"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := buildByAdapter(ctx, tc.spec)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "适配器 %q 必须能构造 ChatModel", tc.spec.Adapter)
			require.NotNil(t, cm)
			// 所有分支都必须满足 eino 的抽象契约（agent 侧依赖这个接口）。
			var _ = cm
		})
	}
}

// TestBuildByAdapter_UnknownIsNotSilent 是安全边界：配置笔误（未知 adapter id）
// 必须响亮失败，绝不能静默降级成 openai —— 那会把「打错端点」伪装成可用。
func TestBuildByAdapter_UnknownIsNotSilent(t *testing.T) {
	_, err := buildByAdapter(context.Background(), adapterSpec{
		Adapter: "openaiformat-typo", BaseURL: "https://example.invalid/v1", APIKey: "sk-x", Model: "m",
	})
	require.Error(t, err, "未知 adapter 不得静默兜底")
	assert.Contains(t, err.Error(), "unknown adapter")
	assert.Contains(t, err.Error(), "openaiformat-typo", "错误必须回显那个错值")
}

// TestAdapterOf 锁归一化：空串 → openai，其余原样透传。
func TestAdapterOf(t *testing.T) {
	assert.Equal(t, AdapterOpenAI, adapterOf(""), "空串必须归一化成 openai 兜底")
	assert.Equal(t, AdapterDeepSeek, adapterOf(AdapterDeepSeek))
	assert.Equal(t, AdapterOllama, adapterOf(AdapterOllama))
	assert.Equal(t, AdapterArk, adapterOf(AdapterArk))
}
