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

// Package main is the spike-only binary for the Phase 9 history-injection
// trust-floor spike (GEVL-08). Plan 09-03 ships the parsing layer
// (spike_row.go) and the runner (runner.go) plus this binary entry
// point. Plan 09-04 extends --history-length=long and Plan 09-05
// extends --summarize; both Plan 09-03 paths deliberately stub with a
// prefix-tagged error message so the operator can grep to the right
// successor plan.
//
// THIS FILE IS A STUB during Task 1 of Plan 09-03 — it exists ONLY so
// `go build ./cmd/spike-history` succeeds while spike_row.go is the
// only production file. Task 3 of Plan 09-03 replaces the stub with
// the real flag parsing + bootstrap + main loop.
package main

import (
	"fmt"
	"os"
)

// main is a deliberate placeholder. The real implementation is added
// in Task 3 of Plan 09-03 (flag parsing, bootstrap selection, main
// loop, dry-run smoke, long/summarize stub gates).
func main() {
	fmt.Fprintln(os.Stderr, "cmd/spike-history: stub — replaced by Task 3 of Plan 09-03")
	os.Exit(0)
}