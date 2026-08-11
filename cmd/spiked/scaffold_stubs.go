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

// Package main — scaffold stubs for spike subcommands that are not yet
// implemented. Each stub returns nil (not an error) so the router
// can validate end-to-end dispatch during Wave 0. Plan 04
// replaces the remaining stub with a real implementation.
//
// Note: runMilvus (Plan 02) is implemented in milvus.go.
// Note: runSSE (Plan 03) is implemented in sse.go.
package main

import (
	"context"
	"fmt"
)

// runE2E is a placeholder for the full E2E pipeline trace spike (Plan 04).
// Real implementation runs 5 compliance queries through skill parse →
// Eino → GLM-5.2 → SSE → mock client → Milvus → BGE-M3 → tier filter.
func runE2E(ctx context.Context, args []string) error {
	fmt.Println("[STUB] e2e: not yet implemented (Plan 04)")
	return nil
}
