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

package agenttools

import (
	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/agent/hitl"
)

// BuildReportPublishTool returns the HITL report_publish Tool registered
// in the hitl registry. It is added to the data-channel tool set when
// the Skill declares writes: ["report:publish"] (D18-A4).
func BuildReportPublishTool() tool.InvokableTool {
	h := hitl.LookupHandler("report:publish")
	if h == nil {
		panic("agenttools.BuildReportPublishTool: report:publish handler not found in hitl registry")
	}
	return h
}
