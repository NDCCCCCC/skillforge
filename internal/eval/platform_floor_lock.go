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

package eval

// PlatformFloorLock is the on-disk schema for the platform-floor frozen
// lock. It mirrors the fanout.lock.json rich-field shape used by Phase 13.
//
// This struct is the SINGLE point of truth for both:
//   - TestPlatformFloorLockSelfConsistent (verifies byte-identity by
//     re-marshalling the canonical body and comparing the hash)
//   - cmd/spike-platform-floor --fill-lock (writes metrics + operator
//     signature back into the lock)
//
// Both must serialize via the same field order to produce the same
// canonical-body SHA-256. The struct field order here is therefore
// load-bearing — do NOT reorder or rename fields without bumping
// platform-floor.lock.json's lock_sha256.
type PlatformFloorLock struct {
	LockID               string         `json:"lock_id"`
	ModelVersion         string         `json:"model_version"`
	Phase                int            `json:"phase"`
	PhaseSlug            string         `json:"phase_slug"`
	SchemaVersion        string         `json:"schema_version"`
	CreatedAt            string         `json:"created_at"`
	LockedBy             string         `json:"locked_by"`
	EvalSet              string         `json:"eval_set"`
	EvalSetSHA256        string         `json:"eval_set_sha256"`
	ModelVer             string         `json:"model_ver"`
	Seed                 int64          `json:"seed"`
	CaseCount            int            `json:"case_count"`
	CategoryCounts       map[string]int `json:"category_counts"`
	Thresholds           map[string]any `json:"thresholds"`
	PlatformFloorVersion string         `json:"platform_floor_version"`
	Metrics              map[string]any `json:"metrics"`
	OperatorSignedAt     string         `json:"operator_signed_at"`
	OperatorName         string         `json:"operator_name,omitempty"`
	LockSHA256           string         `json:"lock_sha256"`
}

// PlatformFloorLockCanonical is identical to PlatformFloorLock but
// excludes the lock_sha256 field. The canonical-body SHA-256 is
// computed by json.Marshal-ing this struct (field order is therefore
// load-bearing — matches PlatformFloorLock declaration order minus
// the trailing LockSHA256).
type PlatformFloorLockCanonical struct {
	LockID               string         `json:"lock_id"`
	ModelVersion         string         `json:"model_version"`
	Phase                int            `json:"phase"`
	PhaseSlug            string         `json:"phase_slug"`
	SchemaVersion        string         `json:"schema_version"`
	CreatedAt            string         `json:"created_at"`
	LockedBy             string         `json:"locked_by"`
	EvalSet              string         `json:"eval_set"`
	EvalSetSHA256        string         `json:"eval_set_sha256"`
	ModelVer             string         `json:"model_ver"`
	Seed                 int64          `json:"seed"`
	CaseCount            int            `json:"case_count"`
	CategoryCounts       map[string]int `json:"category_counts"`
	Thresholds           map[string]any `json:"thresholds"`
	PlatformFloorVersion string         `json:"platform_floor_version"`
	Metrics              map[string]any `json:"metrics"`
	OperatorSignedAt     string         `json:"operator_signed_at"`
	OperatorName         string         `json:"operator_name,omitempty"`
}