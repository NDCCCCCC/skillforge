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
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/llm"
)

// ChatModelAvailable 报告"当前是否存在可用的真 LLM 上游"（Phase 52）。
//
// 这是 49-02 nil 语义的热重载版本。49-02 的约定是 `s.ChatModel == nil`
// 表示 stub 模式（redline 跳过、approval/plan 503）；Phase 52 之后生产
// 的 Deps.ChatModel 是一个 *llm.DynamicChatModel（永不为 nil，否则无法
// 热切换），"是否有真上游"改由 Available() 表达。因此这里的判定分派：
//
//   - cm == nil                       → false（测试里的既有 stub 语义）
//   - cm 是 *llm.DynamicChatModel     → Available()（upstream 槽实时状态）
//   - 其它实现（测试替身/直连模型）   → 非 nil 即可用
//
// 测试替身的行为与 Phase 49 完全一致——它们不需要感知 Phase 52。
func ChatModelAvailable(cm model.ToolCallingChatModel) bool {
	if cm == nil {
		return false
	}
	if dyn, ok := cm.(*llm.DynamicChatModel); ok {
		return dyn.Available()
	}
	return true
}
