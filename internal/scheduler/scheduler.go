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
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
)

// Scheduler runs scheduled report regeneration jobs. It is a singleton
// owned by the server lifecycle: New returns a configured instance, Start
// registers all cron jobs from the Loader's Skills() and starts the cron
// runner, and Stop performs graceful shutdown.
type Scheduler struct {
	cron          *cron.Cron
	loader        *skill.Loader
	inFlight      *inFlight
	store         store.Store
	evalGate      *EvalGate
	kpiExecutor   *kpi.Executor
	ttlDetacher   Detacher
	ttlConfig     TTLCleanupConfig
	mu            sync.RWMutex
	stopped       bool
}

// New constructs a Scheduler with the given Loader.
func New(loader *skill.Loader) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger), cron.Recover(cron.DefaultLogger))),
		loader:   loader,
		inFlight: &inFlight{m: make(map[string]time.Time)},
	}
}

// SetStore injects the store backend.
func (s *Scheduler) SetStore(store store.Store) { s.store = store }

// SetEvalGate injects the eval gate.
func (s *Scheduler) SetEvalGate(gate *EvalGate) { s.evalGate = gate }

// SetKPIExecutor injects the KPI executor for snapshot capture.
func (s *Scheduler) SetKPIExecutor(exec *kpi.Executor) { s.kpiExecutor = exec }

// SetDetacher injects the AttachLRU detacher for TTL cleanup.
func (s *Scheduler) SetDetacher(d Detacher) { s.ttlDetacher = d }

// SetTTLCleanup configures the TTL cleanup job schedule.
// schedule is a cron expression (e.g. "0 2 * * *" = daily at 2am).
// A zero/empty schedule disables TTL cleanup.
func (s *Scheduler) SetTTLCleanup(cfg TTLCleanupConfig) { s.ttlConfig = cfg }

// EvalGate returns the scheduler's eval gate (needed by rerenderHandler).
func (s *Scheduler) EvalGate() *EvalGate { return s.evalGate }

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

	// Register TTL cleanup job if configured
	if s.ttlConfig.TTL > 0 {
		_, err := s.cron.AddFunc("0 2 * * *", func() {
			s.runTTLCleanup(ctx)
		})
		if err != nil {
			log.Printf("[WARN] scheduler: failed to register TTL cleanup cron: %v", err)
		} else {
			log.Printf("[OK] scheduler: TTL cleanup registered (TTL=%v)", s.ttlConfig.TTL)
		}
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

// runReportJob is called by cron on schedule. It delegates to RunReportJob.
func (s *Scheduler) runReportJob(ctx context.Context, m *skill.Manifest) {
	period := extractPeriodFromCron(m.ReportSchedule)
	input := ReportJobInput{
		SkillID:  m.Name,
		Period:   period,
		CronExpr: m.ReportSchedule,
		IsManual: false,
	}
	_ = RunReportJob(ctx, input, s.store, s.evalGate, s.kpiExecutor, s.loader)
}

// runTTLCleanup is called by cron on schedule. It delegates to RunTTLCleanup.
func (s *Scheduler) runTTLCleanup(ctx context.Context) {
	if s.ttlConfig.TTL <= 0 || s.store == nil {
		return
	}
	log.Printf("[INFO] scheduler: TTL cleanup starting (TTL=%v)", s.ttlConfig.TTL)
	if err := RunTTLCleanup(ctx, s.store, s.ttlDetacher, s.ttlConfig); err != nil {
		log.Printf("[ERROR] scheduler: TTL cleanup failed: %v", err)
	}
}

// extractPeriodFromCron derives the period string from a cron expression.
// Monthly cron (e.g. "0 9 1 * *") → "YYYY-MM"
// For v1, the period is derived from the cron expression.
// Returns current month as "YYYY-MM" as a sensible default.
func extractPeriodFromCron(cronExpr string) string {
	if cronExpr == "" {
		return ""
	}
	return time.Now().Format("2006-01")
}

// SkillPeriodKey exposes the helper for use in report_job.go.
func SkillPeriodKey(skillID, period string) string { return skillPeriodKey(skillID, period) }
