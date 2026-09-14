// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

// Regression test for filenameOf. Phase 999.2 introduced nested KB
// directories; filenameOf strips the tier prefix but must PRESERVE any
// subdirectory path so the graph visually reflects the operator's
// organization.
//
// This test pins the strings.Cut direction (before vs after separator)
// so the bug "rest, _, ok := strings.Cut(...)" (which returns before,
// giving the tier slug instead of the subdir path) cannot regress.

package knowledge

import "testing"

func TestFilenameOf_PreservesSubdirectory(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "flat file in tier",
			source: "authority/annual_leave.md",
			want:   "annual_leave.md",
		},
		{
			name:   "nested subdirectory preserved (Phase 999.2)",
			source: "sql-guides/汽车险/004_xxx.md",
			want:   "汽车险/004_xxx.md",
		},
		{
			name:   "deeply nested subdirectory preserved",
			source: "sql-guides/财务对账/013_财务-政保入账清单.md",
			want:   "财务对账/013_财务-政保入账清单.md",
		},
		{
			name:   "deeply nested multi-level",
			source: "policies/hr/onboarding/v2/leave.md",
			want:   "hr/onboarding/v2/leave.md",
		},
		{
			name:   "no separator — return as-is",
			source: "plain.md",
			want:   "plain.md",
		},
		{
			name:   "empty string — return as-is",
			source: "",
			want:   "",
		},
		{
			name:   "trailing slash — return empty after",
			source: "tier/",
			want:   "",
		},
		{
			name:   "part-file with subdir prefix",
			source: "sql-guides/操作手册/006_xxx__part12.md",
			want:   "操作手册/006_xxx__part12.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filenameOf(tc.source)
			if got != tc.want {
				t.Errorf("filenameOf(%q) = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}
