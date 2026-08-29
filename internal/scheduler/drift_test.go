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

package scheduler

import "testing"

func TestDetectDrift(t *testing.T) {
	tests := []struct {
		name      string
		prev      map[string]any
		curr      map[string]any
		threshold float64
		wantDrift bool
		wantErr   bool
	}{
		{
			name:      "nil prev returns no drift",
			prev:      nil,
			curr:      map[string]any{"kpi1": 10.0},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "empty prev returns no drift",
			prev:      map[string]any{},
			curr:      map[string]any{"kpi1": 10.0},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "empty curr returns error",
			prev:      map[string]any{"kpi1": 10.0},
			curr:      map[string]any{},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   true,
		},
		{
			name:      "no change returns no drift",
			prev:      map[string]any{"kpi1": 10.0},
			curr:      map[string]any{"kpi1": 10.0},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "small change below threshold returns no drift",
			prev:      map[string]any{"kpi1": 100.0},
			curr:      map[string]any{"kpi1": 101.0}, // 1% delta < 5%
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "large change above threshold returns drift",
			prev:      map[string]any{"kpi1": 100.0},
			curr:      map[string]any{"kpi1": 110.0}, // 10% delta > 5%
			threshold: 0.05,
			wantDrift: true,
			wantErr:   false,
		},
		{
			name:      "zero baseline skipped returns no drift",
			prev:      map[string]any{"kpi1": 0.0},
			curr:      map[string]any{"kpi1": 10.0},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "non-numeric value skipped returns no drift",
			prev:      map[string]any{"kpi1": "ten"},
			curr:      map[string]any{"kpi1": "twenty"},
			threshold: 0.05,
			wantDrift: false,
			wantErr:   false,
		},
		{
			name:      "multiple KPIs one exceeds threshold returns drift",
			prev:      map[string]any{"kpi1": 100.0, "kpi2": 50.0},
			curr:      map[string]any{"kpi1": 100.0, "kpi2": 60.0}, // 20% delta > 5%
			threshold: 0.05,
			wantDrift: true,
			wantErr:   false,
		},
		{
			name:      "integer types converted correctly",
			prev:      map[string]any{"kpi1": int(100)},
			curr:      map[string]any{"kpi1": int(110)},
			threshold: 0.05,
			wantDrift: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectDrift(tt.prev, tt.curr, tt.threshold)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectDrift() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantDrift {
				t.Errorf("DetectDrift() = %v, want %v", got, tt.wantDrift)
			}
		})
	}
}

func TestSerializeParseRoundTrip(t *testing.T) {
	original := map[string]any{
		"kpi1": float64(10.5),
		"kpi2": float64(20.0),
	}

	serialized, err := SerializeKPISnapshots(original)
	if err != nil {
		t.Fatalf("SerializeKPISnapshots() error = %v", err)
	}

	parsed, err := ParseKPISnapshots(serialized)
	if err != nil {
		t.Fatalf("ParseKPISnapshots() error = %v", err)
	}

	if parsed["kpi1"] != original["kpi1"] {
		t.Errorf("kpi1 round-trip: got %v, want %v", parsed["kpi1"], original["kpi1"])
	}
	if parsed["kpi2"] != original["kpi2"] {
		t.Errorf("kpi2 round-trip: got %v, want %v", parsed["kpi2"], original["kpi2"])
	}
}

func TestParseKPISnapshots_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantErr bool
	}{
		{"empty string returns nil", "", true, false},
		{"valid JSON object", `{"kpi1": 10.5}`, false, false},
		{"invalid JSON", `{invalid}`, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKPISnapshots(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKPISnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantNil && got != nil {
				t.Errorf("ParseKPISnapshots() = %v, want nil", got)
			}
		})
	}
}

func TestSerializeKPISnapshots_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"nil returns empty string", nil, ""},
		{"empty map returns empty JSON object", map[string]any{}, "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SerializeKPISnapshots(tt.m)
			if err != nil {
				t.Fatalf("SerializeKPISnapshots() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("SerializeKPISnapshots() = %q, want %q", got, tt.want)
			}
		})
	}
}
