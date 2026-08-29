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

// Package scheduler implements the cron-driven report regeneration service
// (Phase 19 / Plan 19-02).
package scheduler

import (
	"encoding/json"
	"fmt"
	"math"
)

// DetectDrift compares previous and current KPI snapshots and returns true if
// any KPI value differs by more than the configured threshold. The prev map
// may be nil (first run — no drift possible).
func DetectDrift(prev, curr map[string]any, threshold float64) (bool, error) {
	if len(prev) == 0 {
		return false, nil // first run — no drift possible
	}
	if len(curr) == 0 {
		return false, fmt.Errorf("current snapshot is empty")
	}
	for kpi, prevVal := range prev {
		currVal, ok := curr[kpi]
		if !ok {
			// KPI present in previous but not current — not a drift, just a new KPI set
			continue
		}
		prevF, ok1 := toFloat64(prevVal)
		currF, ok2 := toFloat64(currVal)
		if !ok1 || !ok2 {
			// Non-numeric values — skip
			continue
		}
		if prevF == 0 {
			// Cannot compute relative delta for zero baseline — skip
			continue
		}
		delta := math.Abs((currF - prevF) / prevF)
		if delta > threshold {
			return true, nil
		}
	}
	return false, nil
}

// toFloat64 converts a numeric any to float64. Returns ok=false if the value
// is not a numeric type (int, float, uint).
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint32:
		return float64(n), true
	default:
		return 0, false
	}
}

// ParseKPISnapshots parses a JSON KPI snapshot string into a map.
// Used by the store layer to deserialize report_runs.kpi_snapshots.
func ParseKPISnapshots(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("parse kpi snapshots: %w", err)
	}
	return m, nil
}

// SerializeKPISnapshots serializes a KPI snapshot map to JSON.
func SerializeKPISnapshots(m map[string]any) (string, error) {
	if m == nil {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("serialize kpi snapshots: %w", err)
	}
	return string(b), nil
}
