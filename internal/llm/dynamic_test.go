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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUp 是最小上游替身：记录调用次数，Stream 返回单 chunk 流。
type fakeUp struct {
	calls    atomic.Int64
	tools    atomic.Int64 // WithTools 次数
	lastTool []*schema.ToolInfo
	fail     error
}

func (f *fakeUp) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	f.calls.Add(1)
	if f.fail != nil {
		return nil, f.fail
	}
	return &schema.Message{Content: "ok"}, nil
}

func (f *fakeUp) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	f.calls.Add(1)
	if f.fail != nil {
		return nil, f.fail
	}
	s := schema.StreamReaderFromArray([]*schema.Message{{Content: "ok"}})
	return s, nil
}

func (f *fakeUp) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	f.tools.Add(1)
	f.lastTool = tools
	return f, nil
}

func msgs() []*schema.Message {
	return []*schema.Message{{Role: schema.User, Content: "hi"}}
}

func TestDynamic_DelegatesToCurrentUpstream(t *testing.T) {
	a, b := &fakeUp{}, &fakeUp{}
	cur := a
	d := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return cur, nil })

	_, err := d.Generate(context.Background(), msgs())
	require.NoError(t, err)
	assert.EqualValues(t, 1, a.calls.Load())

	cur = b // 面板切换：同一 resolver 槽换了上游
	_, err = d.Generate(context.Background(), msgs())
	require.NoError(t, err)
	assert.EqualValues(t, 1, b.calls.Load(), "切换后必须走新上游")

	sr, err := d.Stream(context.Background(), msgs())
	require.NoError(t, err)
	_, err = sr.Recv()
	require.NoError(t, err)
	sr.Close()
	assert.EqualValues(t, 2, b.calls.Load(), "Stream 也应委托到 b（Generate 1 次 + Stream 1 次）")
}

func TestDynamic_AvailableAndNoUpstream(t *testing.T) {
	d := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return nil, nil })
	assert.False(t, d.Available())
	_, err := d.Generate(context.Background(), msgs())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoUpstream), "错误必须 errors.Is ErrNoUpstream，got %v", err)

	d2 := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return nil, fmt.Errorf("boom") })
	assert.False(t, d2.Available())
	_, err = d2.Generate(context.Background(), msgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom", "原错误信息必须透传")

	up := &fakeUp{}
	d3 := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return up, nil })
	assert.True(t, d3.Available())
}

func TestDynamic_WithToolsLazyRebind(t *testing.T) {
	a, b := &fakeUp{}, &fakeUp{}
	cur := a
	d := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return cur, nil })

	tools := []*schema.ToolInfo{}
	bound, err := d.WithTools(tools)
	require.NoError(t, err)
	assert.EqualValues(t, 0, a.tools.Load(), "WithTools 必须惰性：绑定时不触上游")

	_, err = bound.Generate(context.Background(), msgs())
	require.NoError(t, err)
	assert.EqualValues(t, 1, a.tools.Load(), "首次调用时才对当前上游 WithTools")

	// 面板切换后，同一 bound 必须重绑到新上游（D-52-03 核心承诺）。
	cur = b
	_, err = bound.Generate(context.Background(), msgs())
	require.NoError(t, err)
	assert.EqualValues(t, 1, b.tools.Load(), "切换后 bound 调用必须重绑到 b")
	assert.Equal(t, tools, b.lastTool)
}

func TestDynamic_SetResolverHotSwap(t *testing.T) {
	a := &fakeUp{}
	d := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) { return a, nil })
	require.True(t, d.Available())

	d.SetResolver(nil)
	assert.False(t, d.Available(), "SetResolver(nil) 后必须立即不可用")
	_, err := d.Generate(context.Background(), msgs())
	require.Error(t, err)

	b := &fakeUp{}
	d.SetResolver(func() (model.ToolCallingChatModel, error) { return b, nil })
	assert.True(t, d.Available(), "换回有上游的 resolver 后立即可用（stub→live 热切换）")
}

func TestDynamic_ConcurrentSmoke(t *testing.T) {
	// -race 下的并发 smoke：面板热切换与请求并发不 data race。
	a, b := &fakeUp{}, &fakeUp{}
	var cur atomic.Pointer[fakeUp]
	cur.Store(a)
	d := NewDynamicChatModel(func() (model.ToolCallingChatModel, error) {
		return cur.Load(), nil
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() { // 模拟面板切换
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if i%2 == 0 {
				cur.Store(b)
			} else {
				cur.Store(a)
			}
			if i%50 == 0 {
				select {
				case <-stop:
					return
				default:
				}
			}
		}
		close(stop)
	}()
	wg.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = d.Generate(context.Background(), msgs()) // 错误可接受（切换瞬间无上游不存在——此处 resolver 恒有上游）
			}
		}()
	}
	wg.Wait()
}
