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

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/skillforge/skillforge/internal/skill"
)

// Scheduler runs scheduled report regeneration jobs. It is a singleton
// owned by the server lifecycle: New returns a configured instance, Start
// registers all cron jobs from the Loader's Skills() and starts the cron
// runner, and Stop performs graceful shutdown.
type Scheduler struct {
	cron     *cron.Cron
	loader   *skill.Loader
	inFlight *inFlight
	mu       sync.RWMutex
	stopped  bool
}

// New constructs a Scheduler with the given Loader and in-flight tracker.
func New(loader *skill.Loader) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger), cron.Recover(cron.DefaultLogger))),
		loader:   loader,
		inFlight: &inFlight{m: make(map[string]time.Time)},
	}
}

// Start registers cron jobs for all Skills that have a non-empty ReportSchedule
// and starts the cron runner. Idempotent: calling Start twice is a no-op after the first.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return fmt.Errorf("scheduler: already stopped")
	}
	for _, m := range s.loader.Skills() {
		if m.ReportSchedule == "" {
			continue
		}
		// Capture m in closure for cron
		manifest := m
		_, err := s.cron.AddFunc(m.ReportSchedule, func() {
			s.runReportJob(ctx, manifest)
		})
		if err != nil {
			log.Printf("[WARN] scheduler: failed to register cron for skill %q: %v", manifest.Name, err)
			continue
		}
		log.Printf("[OK] scheduler: registered %s for skill %s", manifest.ReportSchedule, manifest.Name)
	}
	s.cron.Start()
	log.Printf("[OK] scheduler: started")
	return nil
}

// Stop performs graceful shutdown: stops accepting new jobs and waits for
// running jobs to complete (via the context timeout).
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.mu.Unlock()
	defer s.cron.Stop()
	// cron.Stop() waits for running jobs; give it 30s
	dl, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	s.cron.Stop()
	<-dl.Done()
	log.Printf("[OK] scheduler: stopped")
	return nil
}

// runReportJob is the cron job body. It checks eval gate, acquires in-flight slot,
// then triggers the report run. This is the skeleton — the eval gate and
// full run logic are in 19-02-PLAN.
func (s *Scheduler) runReportJob(ctx context.Context, m *skill.Manifest) {
	key := skillPeriodKey(m.Name, "") // period resolved from manifest or cron
	if !s.inFlight.tryAcquire(key) {
		log.Printf("[INFO] scheduler: skipping %s (already in flight)", key)
		// TODO(19-02): write skipped_duplicate audit row
		return
	}
	defer s.inFlight.release(key)
	log.Printf("[INFO] scheduler: firing report job for skill=%s", m.Name)
	// TODO(19-02): eval gate check, full run
}

// SkillPeriodKey exposes the helper for use in report_job.go.
func SkillPeriodKey(skillID, period string) string { return skillPeriodKey(skillID, period) }
