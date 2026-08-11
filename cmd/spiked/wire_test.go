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

// Wire subcommand tests. The real assertions (TestSpikeWireMock +
// TestMockServerReceivesWireExtras) are written in Task 2 of plan 01-01.
// This file exists so `go test ./cmd/spiked` compiles during Wave 0 and
// the router test path is exercised end-to-end.
package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSpikeWireStub is a placeholder for the real wire test (Task 2).
// It exercises the router + stub and proves the CLI is wired together.
func TestSpikeWireStub(t *testing.T) {
	err := runWire(context.Background(), []string{})
	assert.NoError(t, err, "runWire stub should return nil in Wave 0")
}
