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
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// platformPromptHandler is a RED stub for the PLAT-10 admin preview
// endpoint. It will be replaced by the real implementation in the next
// commit.
func (s *Deps) platformPromptHandler(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusNotImplemented, map[string]string{
		"error": "platform prompt preview not yet implemented",
	})
}
