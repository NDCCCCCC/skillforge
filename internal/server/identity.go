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

// identity.go —— 最小身份层（2026-11-09）。
//
// 解决的问题：改造前系统里没有"用户"这个概念。store 层没有 user/tenant
// 列，ListSessions 无 caller 维度，于是形成一条无需任何凭证的完整越权链：
//
//	① GET /api/sessions          公开 → 全部 session_id + 用户首问文本
//	② GET /api/session/:id/approval  公开 → pending 审批行 + approval_id
//	③ POST /api/session/:id/approval/:approvalId  公开 → 裁决 HITL
//
// HITL 的全部安全价值在于"工具调用必须由人放行"。而放行权此前由
// session id 的保密性保证，session id 却由一个公开接口批量发放 ——
// 任何能访问端口的人都能自动批准任意 pending 工具调用（含 kpi_call
// 会执行 SQL 聚合）。
//
// 这里引入的不是认证系统，是**归属**：每个浏览器持有一个 secret，
// 请求通过 X-Owner-Token 携带。它只回答"这个会话是不是你的"，
// 不回答"你是谁"。这足以切断上面那条链，而不引入用户表、密码、
// 会话 cookie 的全部复杂度。
//
// 刻意不做的事（避免过度设计）：
//   - 不做登录/密码/角色（那需要用户表与迁移，属于商业化前置项）
//   - 不把 owner token 当凭证（它不签任何东西，不做授权判断）
//   - 不在日志里打印 token 值
package server

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/skill"
)

// ownerTokenHeader 是浏览器身份的请求头。前端 web/src/lib/owner-token.ts
// 是它的唯一写入点。
const ownerTokenHeader = "X-Owner-Token"

// maxOwnerTokenLen 限长，避免把任意长度的头写进 SQLite 主键索引。
const maxOwnerTokenLen = 128

// ownerTokenOf 读取调用方声明的归属 token。缺失或超长视为空（无归属）。
//
// 不做格式白名单：token 由前端生成（uuid / 32-hex），但后端只把它当作
// 不透明字符串做等值比较 —— 强制格式只会让未来换生成方式时静默失效。
func ownerTokenOf(c *app.RequestContext) string {
	got := string(c.Request.Header.Peek(ownerTokenHeader))
	if len(got) > maxOwnerTokenLen {
		return ""
	}
	return got
}

// claimSessionOwner 把当前请求的 owner 绑定到会话（首次写入生效）。
// 失败只记 WARN：归属记录失败不应该让一次正常对话失败 —— 代价是
// 该会话后续写操作会因"无归属"而要求 admin，属于 fail-closed 方向。
func (s *Deps) claimSessionOwner(ctx context.Context, sessionID string, c *app.RequestContext) {
	if s.Store == nil || sessionID == "" {
		return
	}
	owner := ownerTokenOf(c)
	if owner == "" {
		return
	}
	if err := s.Store.ClaimSessionOwner(ctx, sessionID, owner); err != nil {
		log.Printf("[WARN] server: claim session owner failed (sess=%s): %v (non-fatal; session stays unowned)", sessionID, err)
	}
}

// sessionOwnerMatches 判断调用方是否有权对该会话做写操作。
//
//	admin                          → 放行
//	会话归属非空且与调用方相等      → 放行
//	会话无归属（历史会话）          → 拒绝（要求 admin）
//
// 最后一条是刻意的 fail-closed：无法证明归属的会话不能被匿名裁决。
// 代价是改造前创建的历史会话，其审批必须由管理员操作。
func (s *Deps) sessionOwnerMatches(ctx context.Context, sessionID string, c *app.RequestContext) (bool, string) {
	if isAdminRequest(c) {
		return true, ""
	}
	if s.Store == nil {
		return false, "session store not initialized"
	}
	owner, err := s.Store.SessionOwner(ctx, sessionID)
	if err != nil {
		log.Printf("[WARN] server: session owner lookup failed (sess=%s): %v", sessionID, err)
		return false, "session ownership unavailable"
	}
	caller := ownerTokenOf(c)
	if owner == "" {
		return false, "session has no owner — admin token required"
	}
	if subtle.ConstantTimeCompare([]byte(owner), []byte(caller)) == 1 {
		return true, ""
	}
	return false, "session belongs to another owner"
}

// requireSessionOwner 是 sessionOwnerMatches 的 handler 前门。
// 失败写 403；调用方应 `if !s.requireSessionOwner(ctx, c, sessionID) { return }`。
func (s *Deps) requireSessionOwner(ctx context.Context, c *app.RequestContext, sessionID string) bool {
	ok, reason := s.sessionOwnerMatches(ctx, sessionID, c)
	if ok {
		return true
	}
	if reason == "" {
		reason = "forbidden"
	}
	c.JSON(http.StatusForbidden, map[string]string{"error": reason})
	return false
}

// knowledgeReadAllowed 判断调用方能否直读某个 tier 的内容。
//
// 检索路径的 tier 隔离由 retriever 层保证（PRE-similarity filter），但
// HTTP 直读路径不经过检索 —— 没有这道闸门，"检索不到"不等于"读不到"。
// chunk id 是递增整数、doc 的 file 参数可枚举，所以这不是理论风险：
// 改造前 GET /api/knowledge/chunk/7 就能从任意匿名客户端取走权威库原文。
//
// 放行条件（fail-closed）：
//   - admin token 有效 → 全部放行
//   - 否则必须声明 X-Skill-Name，且该 Skill 的 reads 解析出的 tiers
//     覆盖目标 tier
//   - 其它一律拒绝
//
// X-Skill-Name 是自述身份，这里依然不是认证 —— 它拦住的是"完全匿名的
// 批量枚举"，不是"冒充一个已知 Skill 名"。要真正解决冒充需要服务端可
// 验证的凭证（RBAC），那是独立的一步。这个取舍是有意为之：先把开放面
// 收成"必须声明身份"，再逐步上真凭证。
func (s *Deps) knowledgeReadAllowed(c *app.RequestContext, tier string) bool {
	if isAdminRequest(c) {
		return true
	}
	if tier == "" {
		return false
	}
	skillName := string(c.Request.Header.Peek("X-Skill-Name"))
	if skillName == "" {
		return false
	}
	// The Registry snapshot is the runtime source of truth (D5-01/D5-06)
	// and carries the hot-reloaded manifest. The Loader is a startup-
	// time scan only, so it is consulted as a fallback (tests build a
	// Loader without a Registry).
	if s.Registry != nil {
		if entry, ok := s.Registry.Snapshot()[skillName]; ok && entry != nil && entry.Manifest != nil {
			return tierAllowed(entry.Manifest, tier)
		}
		return false
	}
	if s.Loader == nil {
		return false
	}
	for _, m := range s.Loader.Skills() {
		if m.Name != skillName {
			continue
		}
		return tierAllowed(m, tier)
	}
	return false
}

// tierAllowed reports whether the Skill resolved the tier. A Skill with
// an EMPTY resolved-tier set reads nothing (default-deny) -- the same
// posture the retriever applies.
func tierAllowed(m *skill.Manifest, tier string) bool {
	for _, t := range m.ResolvedTiers() {
		if t == tier {
			return true
		}
	}
	return false
}
