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

// Package main — scaffold_stubs.go tracked the not-yet-implemented spike
// subcommands during Waves 0-2. As of Plan 01-04, ALL four subcommands
// have real implementations:
//
//   - runWire    (Plan 01-01) — cmd/spiked/wire.go
//   - runMilvus  (Plan 01-02) — cmd/spiked/milvus.go
//   - runSSE     (Plan 01-03) — cmd/spiked/sse.go
//   - runE2E     (Plan 01-04) — cmd/spiked/e2e.go
//
// This file is retained as a marker — Go's package system requires at
// least one .go file per package, but a package can have any number of
// files with documentation-only content. Keeping the file lets the
// "no stubs remain" acceptance check (grep `func runE2E` returns empty
// here) verify the scaffold transition is complete.
//
// If a future plan adds a new subcommand stub slot, add it here with a
// `func runX(ctx, args) error` placeholder returning nil, then replace
// with the real implementation in its own file.
package main
