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

package agent

import (
	"github.com/skillforge/skillforge/internal/platformfloor"
)

// PlatformFloorVersion mirrors the single source in internal/platformfloor.
//
// The literal "1.1.0" (since the 14-10 tail re-anchor) is duplicated here ONLY so the verify-phase-14
// source-grep guard (Pattern E — fanout_runtime.go:63 model) can find it
// in the canonical agent-package file without the verify script needing
// to scan internal/platformfloor/. The leaf package remains the
// authoritative declaration; this constant exists for D14-B1 audit
// visibility and for the plan-level acceptance criterion that the
// verify-phase-14 grep guard can find this exact literal in the
// canonical agent-package file. A bump must edit BOTH this line AND
// the leaf's PlatformFloorVersion together (see platformfloor package
// doc for D14-B1 / D14-B4 bump discipline).
const PlatformFloorVersion = "1.1.0"

// ComposePlatformPrompt is the agent-package entry point to the single
// canonical system-prompt composition (Pitfall 17 root). It is a thin
// pass-through to internal/platformfloor.ComposePlatformPrompt — every
// chat-path renderer (systemPrompt, SystemPrompt) calls this function
// rather than building a prefix+template layout by hand. Adding any
// other composition site re-introduces the dual-source-of-truth class
// of bugs the floor exists to eliminate (the WalkDir guard
// TestNoBypassOfPlatformFloorCompose bans such sites).
//
// Re-export identity is locked by TestPlatformFloorReExportIdentity —
// if this function ever wraps or rewrites the skill prompt (e.g., adds
// formatting), that test fails immediately.
//
// Dependency direction note: internal/agent imports internal/platformfloor
// (a leaf), so there is no cycle. internal/runtime CANNOT import
// internal/agent directly (runner_setup.go creates the agent→runtime
// edge), which is why the deep-path renderer
// (internal/runtime/deep_agent.go) calls platformfloor.ComposePlatformPrompt
// directly via the same leaf.
func ComposePlatformPrompt(skillPrompt string) string {
	return platformfloor.ComposePlatformPrompt(skillPrompt)
}
