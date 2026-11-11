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
	"log"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/wire"
)

// streamTextSink 把 Eino ChatModel 流式产生的增量**直接写成 assistant 正文**。
//
// 为什么需要（2026-11-11 用户要求）：
// 取数通道实测 66 秒才出现第一个 text-delta —— streamOnce 早就逐 token
// 拿到了内容，但只把它们发成 text-delta-draft（喂给草稿卡），正文要等
// 整段生成完、解包、guard 校验之后才一次吐出。用户看到的是"等一分多钟，
// 答案突然整段出现"，而不是"模型在写"。
//
// 现在增量同时走两条路：draft（草稿卡，保持原样）+ 正文（本类型）。
// 底层用的就是 Eino 已有的 ChatModel.Stream + StreamReader.Recv ——
// 流式能力本来就是现成的，缺的只是把增量接到正文通道上。
//
// 三条保守规则（都不牺牲诚实性）：
//  1. 探针：前 64 个字符里第一个非空白字符若是 '{'，说明模型又把答案包成
//     JSON（OperationalSystemPrompt 明令禁止，但实测会犯）。这时**放弃直出**、
//     保留原有整段解包路径，避免把原始 JSON 吐进对话。
//  2. CR 归一：跨帧的 \r 用 pendingCR 挂起，绝不把 CR 漏给前端
//     （2026-11-11 实测：CR 会让 SQL 挤成一行、相邻 token 粘连成 "awhere"）。
//  3. 失败即降级：text-start/delta 发射失败只记日志并关闭直出，绝不打断
//     生成 —— 正文是展示层，不是信任边界。
type streamTextSink struct {
	c *app.RequestContext

	started bool
	failed  bool

	// 探针缓冲区：决定"这是正文"还是"这是一坨 JSON"。
	probe   strings.Builder
	decided bool
	sawJSON bool

	// pendingCR 处理跨帧的 \r（帧可能正好切在 \r\n 中间）。
	pendingCR bool

	// out 是**已经发给前端**的内容，用于让最终答案与用户看到的一致。
	out strings.Builder
}

// streamTextProbeLimit 是 JSON 探针的最大缓冲量。模型不听话时通常以
// `{"answer":` 或 `{ "answer"` 开头，64 字符足够判定。
const streamTextProbeLimit = 64

func newStreamTextSink(c *app.RequestContext) *streamTextSink {
	return &streamTextSink{c: c}
}

// Push 接收一个模型增量。nil 接收者是安全的（= 该通道不直出）。
func (s *streamTextSink) Push(delta string) {
	if s == nil || s.failed || s.sawJSON || delta == "" {
		return
	}
	if !s.decided {
		s.probe.WriteString(delta)
		trimmed := strings.TrimLeft(s.probe.String(), " \t\r\n\uFEFF")
		if trimmed == "" {
			// 还只是空白/换行 —— 继续等第一个真正的字符。
			if s.probe.Len() >= streamTextProbeLimit {
				s.decided = true
				s.flushProbe()
			}
			return
		}
		if trimmed[0] == '{' {
			// 模型把答案包成 JSON：直出会让用户先看到一坨原始 JSON，
			// 交给后面 ExtractJSON 的整段路径处理。
			s.sawJSON = true
			return
		}
		s.decided = true
		s.flushProbe()
		return
	}
	s.write(delta)
}

func (s *streamTextSink) flushProbe() {
	buffered := s.probe.String()
	s.probe.Reset()
	s.write(buffered)
}

// write 发一帧正文（惰性发 text-start）。
func (s *streamTextSink) write(chunk string) {
	chunk = s.normalize(chunk)
	if chunk == "" {
		return
	}
	if !s.started {
		if err := wire.EmitChunk(s.c, wire.UIMessageChunk{Type: "text-start", ID: "msg-1"}); err != nil {
			log.Printf("[WARN] server: emit text-start (streaming answer): %v (non-fatal)", err)
			s.failed = true
			return
		}
		s.started = true
		log.Printf("[INFO] server: streaming answer begins (first delta_bytes=%d)", len(chunk))
	}
	if err := wire.EmitChunk(s.c, wire.UIMessageChunk{Type: "text-delta", ID: "msg-1", Delta: chunk}); err != nil {
		log.Printf("[WARN] server: emit text-delta (streaming answer): %v (non-fatal)", err)
		s.failed = true
		return
	}
	s.out.WriteString(chunk)
}

func (s *streamTextSink) normalize(chunk string) string {
	if s.pendingCR {
		chunk = "\r" + chunk
		s.pendingCR = false
	}
	if strings.HasSuffix(chunk, "\r") {
		chunk = chunk[:len(chunk)-1]
		s.pendingCR = true
	}
	chunk = strings.ReplaceAll(chunk, "\r\n", "\n")
	chunk = strings.ReplaceAll(chunk, "\r", "\n")
	return chunk
}

// Append 在收尾阶段追加正文（目前用于 guard 拒绝时补"未通过校验"的
// 警告与知识库原文模板）。只在已经直出时有效 —— 没直出就交给整段路径。
func (s *streamTextSink) Append(text string) {
	if s == nil || !s.started || s.failed || text == "" {
		return
	}
	s.write(text)
}

// Flush 收尾：把挂起的 \r 落成 \n。
func (s *streamTextSink) Flush() {
	if s == nil || s.failed {
		return
	}
	if s.pendingCR {
		s.pendingCR = false
		s.write("\n")
	}
}

// Started 报告是否真的发过正文（决定下游是否跳过整段 text-delta）。
func (s *streamTextSink) Started() bool { return s != nil && s.started }

// SawJSON 报告探针是否判定为 JSON 包裹（此时走原有整段解包路径）。
func (s *streamTextSink) SawJSON() bool { return s != nil && s.sawJSON }

// Out 返回已经发给前端的正文（已做 CR 归一）。
func (s *streamTextSink) Out() string {
	if s == nil {
		return ""
	}
	return s.out.String()
}
