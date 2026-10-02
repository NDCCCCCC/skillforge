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

// Package server — runner_chat.go is Phase 11 / Plan 11-01's minimal
// entry-point seam for the Runner-path chatHandler.
//
// Why this file exists:
//
//   - The legacy route registration in server.New is
//     h.POST(\"/api/chat\", deps.chatHandler) — a method-value on
//     *Deps. The new Runner-path chatHandler lives in
//     internal/server/runner_setup.go as (*Deps).ChatHandler.
//   - To flip the route binding without touching server.New in
//     11-01 (we want minimal blast radius — Plan 11-04 is the
//     proper flip plan), the runnerChatHandler helper below is
//     the entry seam. server.New can swap the binding by changing
//     h.POST(\"/api/chat\", deps.chatHandler) to
//     h.POST(\"/api/chat\", deps.runnerChatHandler) in one line.
//   - For 11-01 the seam stays inactive (server.New still binds
//     deps.chatHandler). 11-04 activates it by editing the binding.
//
// This file's chatRequest / lastUserQuery / defaultSkill imports are
// shared with chat.go (lifted from there); the helpers themselves stay
// in chat.go (no duplication, no drift risk).
package server

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// runnerChatHandler is the entry seam for the Runner-path chatHandler
// (Plan 11-01). It is a thin one-line delegation to (*Deps).ChatHandler
// — the seam exists so server.New can flip the route binding by
// changing one method-value (deps.chatHandler → deps.runnerChatHandler)
// without touching either chat.go or runner_setup.go.
//
// 11-01: server.New keeps the legacy deps.chatHandler binding. 11-04
// flips the binding to deps.runnerChatHandler.
func (s *Deps) runnerChatHandler(ctx context.Context, c *app.RequestContext) {
	s.ChatHandler(ctx, c)
}
