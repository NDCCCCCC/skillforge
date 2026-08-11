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

// Package main implements the `spiked` CLI entry point for the SkillForge
// integration spike. It dispatches to one of four subcommands (wire, sse,
// milvus, e2e), each of which validates a distinct wire contract that Phase 2+
// code will rely on. See 01-PLAN.md and 01-VALIDATION.md for the spike
// contract definitions.
package main

import (
	"context"
	"fmt"
	"os"
)

const usage = `usage: spiked <subcommand> [flags]
  subcommands: wire, sse, milvus, e2e`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	ctx := context.Background()
	sub := os.Args[1]
	args := os.Args[2:]

	var err error
	switch sub {
	case "wire":
		err = runWire(ctx, args)
	case "milvus":
		err = runMilvus(ctx, args)
	case "sse":
		err = runSSE(ctx, args)
	case "e2e":
		err = runE2E(ctx, args)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n%s\n", sub, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", sub, err)
		os.Exit(1)
	}
}
