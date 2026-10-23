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

// Phase 52 (llm-provider-panel): 委托式 chat model —— 管理面板保存配置后
// 无需重启即可切换上游（用户决策 2026-10-23「立即生效（热重载）」）。
//
// 设计要点：
//   - DynamicChatModel 持有一个 UpstreamResolver（每次 Generate/Stream 调用
//     时解析"当前上游"），而不是持有一个固定的上游实例。resolver 必须快速
//     （读本地文件/原子指针），不得在持锁状态下做网络 IO。
//   - stub 模式 = resolver 返回 nil 上游。Deps.ChatModel 仍然非 nil（委托式
//     包装恒在），"是否存在真上游"由 Available() 表达；internal/server 的
//     ChatModelAvailable helper 负责把这一点接到 49-02 的 nil 语义上
//     （redline skip、approval/plan 503）。
//   - WithTools 不立即绑定：返回一个 boundDynamic，在每次调用时解析**当前**
//     上游并现做 upstream.WithTools(tools)。若立即绑定，面板切换上游后
//     ReAct agent 仍会打到旧厂商——违背热重载承诺。
package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// UpstreamResolver 返回当前应使用的上游模型。返回 (nil, nil) 或 (nil, err)
// 都表示"当前没有可用上游"（stub 模式）；err 会原样透传给调用方（%w 包装）。
type UpstreamResolver func() (model.ToolCallingChatModel, error)

// ErrNoUpstream 是无可用上游时 Generate/Stream 返回的错误语义（包装后透传）。
var ErrNoUpstream = fmt.Errorf("no LLM upstream configured (stub mode) — configure one via POST /api/admin/llm/config")

// DynamicChatModel 把"上游是谁"推迟到每次调用时解析（D-52-03 热重载）。
// 零值不可用；用 NewDynamicChatModel 构造。
type DynamicChatModel struct {
	mu  sync.RWMutex
	res UpstreamResolver
}

// NewDynamicChatModel 构造委托式模型。res 为 nil 时行为等同"永无上游"。
func NewDynamicChatModel(res UpstreamResolver) *DynamicChatModel {
	return &DynamicChatModel{res: res}
}

// SetResolver 热切换 resolver（面板保存配置后调用）。进行中的旧调用不受
// 影响；新调用立即走新 resolver。
func (d *DynamicChatModel) SetResolver(res UpstreamResolver) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.res = res
}

func (d *DynamicChatModel) resolver() UpstreamResolver {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.res
}

// resolve 解析当前上游；(nil, err) 归一化为 ErrNoUpstream 包装。
func (d *DynamicChatModel) resolve() (model.ToolCallingChatModel, error) {
	res := d.resolver()
	if res == nil {
		return nil, fmt.Errorf("%w: no resolver installed", ErrNoUpstream)
	}
	up, err := res()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoUpstream, err)
	}
	if up == nil {
		return nil, ErrNoUpstream
	}
	return up, nil
}

// Available 报告当前是否存在可用上游（面板状态卡 / ChatModelAvailable 的依据）。
// 只做解析，不发起任何模型请求。
func (d *DynamicChatModel) Available() bool {
	up, err := d.resolve()
	return err == nil && up != nil
}

// Generate implements model.BaseChatModel —— 解析当前上游并委托。
func (d *DynamicChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	up, err := d.resolve()
	if err != nil {
		return nil, err
	}
	return up.Generate(ctx, input, opts...)
}

// Stream implements model.BaseChatModel —— 解析当前上游并委托。
func (d *DynamicChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	up, err := d.resolve()
	if err != nil {
		return nil, err
	}
	return up.Stream(ctx, input, opts...)
}

// WithTools implements model.ToolCallingChatModel —— 惰性重绑（D-52-03）。
// 返回的 boundDynamic 在每次调用时用**当前**上游现做 WithTools，因此面板
// 切换上游后，ReAct agent 手里的绑定模型自动跟随新上游。
func (d *DynamicChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &boundDynamic{parent: d, tools: tools}, nil
}

// PipelineDynamic 把 verdict 流式接缝（model.BaseChatModel）接到面板配置上：
// 上游可用 ⇒ 真模型；不可用（stub 模式）⇒ 回落 stub。stubchat extractor 自己
// 给 Verdict 打 mode="stub"，因此从 stub 切到真模型时标注自动消失（D-08）。
type PipelineDynamic struct {
	dyn  *DynamicChatModel
	fall model.BaseChatModel
}

// NewPipelineDynamic 组装流式接缝的动态路由。dyn 为空时恒走 fallback。
func NewPipelineDynamic(dyn *DynamicChatModel, fallback model.BaseChatModel) *PipelineDynamic {
	return &PipelineDynamic{dyn: dyn, fall: fallback}
}

// Generate implements model.BaseChatModel.
func (p *PipelineDynamic) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if p.dyn != nil {
		if up, err := p.dyn.resolve(); err == nil && up != nil {
			return up.Generate(ctx, input, opts...)
		}
	}
	return p.fall.Generate(ctx, input, opts...)
}

// Stream implements model.BaseChatModel.
func (p *PipelineDynamic) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if p.dyn != nil {
		if up, err := p.dyn.resolve(); err == nil && up != nil {
			return up.Stream(ctx, input, opts...)
		}
	}
	return p.fall.Stream(ctx, input, opts...)
}


// boundDynamic 是 WithTools 的惰性绑定形态：不持有具体上游，只持有
// 父 DynamicChatModel 与工具集。
type boundDynamic struct {
	parent *DynamicChatModel
	tools  []*schema.ToolInfo
}

func (b *boundDynamic) resolve() (model.ToolCallingChatModel, error) {
	up, err := b.parent.resolve()
	if err != nil {
		return nil, err
	}
	bound, err := up.WithTools(b.tools)
	if err != nil {
		return nil, fmt.Errorf("bind tools on current upstream: %w", err)
	}
	return bound, nil
}

// Generate implements model.BaseChatModel.
func (b *boundDynamic) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	up, err := b.resolve()
	if err != nil {
		return nil, err
	}
	return up.Generate(ctx, input, opts...)
}

// Stream implements model.BaseChatModel.
func (b *boundDynamic) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	up, err := b.resolve()
	if err != nil {
		return nil, err
	}
	return up.Stream(ctx, input, opts...)
}

// WithTools implements model.ToolCallingChatModel —— 绑定可以叠加：以当前
// 上游解析结果再套一层新工具集（丢弃旧工具集，与 upstream.WithTools 的
// 语义一致：调用方显式给出本次需要的完整工具集）。
func (b *boundDynamic) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &boundDynamic{parent: b.parent, tools: tools}, nil
}
