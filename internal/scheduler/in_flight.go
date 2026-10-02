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
// (Phase 19 / Plan 19-01 / PORT-02/PORT-03).
package scheduler

import (
	"sync"
	"time"
)

// inFlight tracks currently-executing report runs by skill+period key.
// Used by PORT-07 concurrent guard: a second overlapping run is skipped.
type inFlight struct {
	mu sync.RWMutex
	m  map[string]time.Time // key = "skillID:period"
}

// tryAcquire returns true if the key was not already present (acquired the slot).
// Returns false if the key is already in the map (a run is in progress).
func (f *inFlight) tryAcquire(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[key]; ok {
		return false
	}
	f.m[key] = time.Now()
	return true
}

// release removes the key from the map, allowing a new run to proceed.
func (f *inFlight) release(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, key)
}

// skillPeriodKey returns the composite key for a (skillID, period) pair.
func skillPeriodKey(skillID, period string) string {
	return skillID + ":" + period
}
