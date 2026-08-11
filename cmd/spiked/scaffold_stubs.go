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
// can validate end-to-end dispatch during Wave 0. Plan 02/03/04
// replace these stubs with real implementations.
package main

import (
	"context"
	"fmt"
)

// runMilvus is a placeholder for the Milvus tier-filter spike (Plan 02).
// Real implementation seeds 4 tiers + asserts query with `tier in [...]`
// returns zero cross-tier leaks.
func runMilvus(ctx context.Context, args []string) error {
	fmt.Println("[STUB] milvus: not yet implemented (Plan 02)")
	return nil
}

// runSSE is a placeholder for the Hertz SSE → Vercel AI SDK v5 spike (Plan 03).
// Real implementation emits text/event-stream and asserts UIMessageChunk
// alignment end-to-end.
func runSSE(ctx context.Context, args []string) error {
	fmt.Println("[STUB] sse: not yet implemented (Plan 03)")
	return nil
}

// runE2E is a placeholder for the full E2E pipeline trace spike (Plan 04).
// Real implementation runs 5 compliance queries through skill parse →
// Eino → GLM-5.2 → SSE → mock client → Milvus → BGE-M3 → tier filter.
func runE2E(ctx context.Context, args []string) error {
	fmt.Println("[STUB] e2e: not yet implemented (Plan 04)")
	return nil
}
