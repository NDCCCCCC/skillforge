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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnvIntOr — 261117-n96：SKILLFORGE_ANSWER_MAX_TOKENS 的解析旋钮。
// 锁三件事：未设回默认、合法值生效、非法/负值回默认（不静默采用）。
func TestEnvIntOr(t *testing.T) {
	const def = 8192

	t.Run("unset falls back to default", func(t *testing.T) {
		t.Setenv("TEST_INT_KNOB", "")
		assert.Equal(t, def, envIntOr("TEST_INT_KNOB", def))
	})

	t.Run("valid value is honored", func(t *testing.T) {
		t.Setenv("TEST_INT_KNOB", "16384")
		assert.Equal(t, 16384, envIntOr("TEST_INT_KNOB", def))
	})

	t.Run("non-numeric falls back", func(t *testing.T) {
		t.Setenv("TEST_INT_KNOB", "abc")
		assert.Equal(t, def, envIntOr("TEST_INT_KNOB", def))
	})

	t.Run("negative and zero fall back", func(t *testing.T) {
		t.Setenv("TEST_INT_KNOB", "-5")
		assert.Equal(t, def, envIntOr("TEST_INT_KNOB", def))
		t.Setenv("TEST_INT_KNOB", "0")
		assert.Equal(t, def, envIntOr("TEST_INT_KNOB", def))
	})

	t.Run("surrounding spaces are trimmed", func(t *testing.T) {
		t.Setenv("TEST_INT_KNOB", " 4096 ")
		assert.Equal(t, 4096, envIntOr("TEST_INT_KNOB", def))
	})
}
