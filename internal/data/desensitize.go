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

package data

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
)

// KMinDefault is the Phase 15 GLOBAL default for the k-anonymity query
// floor (D15-D3). A dataset may override it per-import via the rule
// file's k_min field (0 = use this default; the override is stored in
// the datasets registry, Phase 16 reads it).
//
// The literal is the call-site source-grep target
// (scripts/verify-phase-15.sh Stage 9 — fanout_runtime.go:63 model):
// every change is an explicit code edit + assertion update, NOT a
// config knob. D15-D3 / 15-RESEARCH Pitfall 11 (k_min vs bucket join:
// values below k_min are suppressed at query time by the query-side
// defense line — v1 ships the constant, Phase 16 enforces it).
const KMinDefault = 5

// DeriveDatasetSalt derives the per-dataset HMAC key from the site salt:
//
//	datasetSalt = HMAC-SHA256(key=siteSalt, msg="dsalt:"+dataset)
//
// Fail-closed: an empty siteSalt is an ERROR, mirroring the
// isAdminRequest unset-env semantics — an unconfigured site must never
// import data under a degenerate salt. The caller supplies the site
// salt from SKILLFORGE_DATA_SALT at the import boundary (15-06).
//
// The salt is per-DATASET (logical name), NEVER per-(dataset, period)
// or per-batch — the same customer hashes identically across monthly
// batches, which is the precondition for period-over-period joins
// (Pitfall 4). datasetSalt is NOT persisted: the derivation is
// deterministic, so no registry column and no migration exist
// (15-RESEARCH Pattern 2).
//
// Operational warning: rotating SKILLFORGE_DATA_SALT changes every
// derived salt — existing stored hashes stop joining with new imports
// and a full re-import is required. Documented in datarules/README.md.
func DeriveDatasetSalt(siteSalt, dataset string) ([]byte, error) {
	if siteSalt == "" {
		return nil, fmtError("DeriveDatasetSalt", errors.New(
			"site salt is empty — set SKILLFORGE_DATA_SALT (fail-closed)"))
	}
	mac := hmac.New(sha256.New, []byte(siteSalt))
	mac.Write([]byte("dsalt:" + dataset))
	return mac.Sum(nil), nil
}

// L1Hash computes the L1 deterministic pseudonymization hash for one
// (dataset, column, normalizedValue) triple:
//
//	HMAC-SHA256(key=datasetSalt, msg="field:"+column+"\x00"+normalizedValue)
//
// Construction is LOCKED (STACK-VERIFICATION §3.2):
//   - The input set deliberately contains NO period parameter — see
//     DeriveDatasetSalt (Pitfall 4, TestHashStableAcrossPeriods).
//   - The "field:" domain tag + 0x00 separator give field-domain
//     separation: the same value in two columns never collides, and a
//     column name can never splice into the value (the 0x00 byte is
//     illegal in normalized text input by construction of the decoders).
//   - Hand-rolled hash(salt‖value) concatenation is FORBIDDEN
//     (length-extension / domain-confusion surface — Don't Hand-Roll
//     table, 15-RESEARCH).
//   - Cross-dataset separation comes from the per-dataset salt: the
//     same (column, value) under a different dataset hashes differently
//     — the cryptographic landing of data iron-rule ④ (拒绝跨数据集
//     推断个体). This is a feature, not a defect.
//
// Output: full 64-char lowercase hex. Storage does NOT truncate —
// truncation lowers collision resistance; preview-side truncation is a
// display-layer concern only.
func L1Hash(datasetSalt []byte, column, normalizedValue string) string {
	mac := hmac.New(sha256.New, datasetSalt)
	mac.Write([]byte("field:"))
	mac.Write([]byte(column))
	mac.Write([]byte{0})
	mac.Write([]byte(normalizedValue))
	return hex.EncodeToString(mac.Sum(nil))
}

// FixedBucket maps a numeric value into its fixed-width bucket floor
// (L2 / D15-D1):
//
//	bucket = math.Floor(value/width) * width
//
// Boundary contract (TestBucketBoundaryStableAcrossPeriods): the bucket
// depends ONLY on (value, width) — no period, no batch, no quantile.
// Fixed boundaries are what make month-over-month KPI trends
// comparable; quantile buckets would drift with each batch's
// distribution and were explicitly rejected (D15-D1).
//
// width MUST be > 0. The guard lives at rule-load time: rules.go
// ValidateRules rejects bucket_fixed with width <= 0, so this function
// is a panic-free direct computation and does not re-validate (the
// rules layer owns the width contract — see the ValidateRules godoc).
func FixedBucket(value, width float64) float64 {
	return math.Floor(value/width) * width
}

// fmtError wraps err with the data-package prefix ("data.%s: %w" —
// Shared Pattern F; mirrors duckdb/sqlite fmtError forms).
func fmtError(fn string, err error) error {
	return fmt.Errorf("data.%s: %w", fn, err)
}
