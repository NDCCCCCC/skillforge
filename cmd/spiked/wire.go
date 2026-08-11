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

// Package main — wire subcommand stub. Task 2 of plan 01-01 overwrites this
// file with the real implementation: a dynamic-port mock OpenAI server + Eino
// ChatModel round-trip that proves `temperature`, `response_format`, and
// `thinking` extras reach the wire.
package main

import (
	"context"
	"fmt"
)

// runWire is a placeholder for the wire-format regression test (SC-1 layer a).
// The real implementation in Task 2 starts an in-process mock OpenAI server,
// configures an Eino ChatModel pointing at the mock, and asserts the
// temperature/response_format/thinking extras serialize correctly.
func runWire(ctx context.Context, args []string) error {
	fmt.Println("[WIP] wire: implementation in Task 2")
	return nil
}
