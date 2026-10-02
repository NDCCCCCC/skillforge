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

// Package verdict — floor.go is the Phase 12 / Plan 12-01 library-only
// downgrade marker for the trust floor primitive.
//
// ApplyTrustFloor itself lives in internal/agent/trust_floor.go
// (Phase 11 / D3-01 / D-09 / KNOW-04). It is NOT in this package
// because it closes over the ChunkAccumulator state which is owned
// by the agent package (the accumulator lives across the full Run
// lifetime; verdict is a per-message envelope). The function is a
// pure read (returns *Refusal or nil) and produces no side effects.
//
// Post-Phase 12 the trust floor is consumed by
// runtime.ApplyTrustStage (Phase 12 / Plan 12-01) which inlines the
// 3-line floor logic to avoid the agent → runtime import cycle.
// The legacy ApplyTrustFloor remains the source of truth for
// internal/agent.Run's chatHandler-side invocation (Phase 11).
//
// This file is a documentation-only marker. It exists so the
// Phase 12 architectural boundary is visible in the verdict
// package's surface (the package-by-package "what lives where"
// grep) — anyone reading internal/verdict/floor.go learns that
// the trust floor primitive lives in internal/agent/trust_floor.go
// and that runtime.ApplyTrustStage is the unified exit point.
package verdict

// This file intentionally has no exported types or functions. Its
// sole purpose is to anchor the Phase 12 architectural boundary
// (P12-L1: verdict is library-only; runtime owns the trust stage)
// in a discoverable file under internal/verdict/.
//
// Future-proof: if a future Phase decides to move ApplyTrustFloor
// into this package (e.g. refactor ChunkAccumulator to be owned by
// verdict), the implementation lands here — the file exists to
// receive that migration.