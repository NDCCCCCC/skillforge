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

// IN-05 regression test — detached run goroutine panic recovery.
// Verifies that a panic inside the LLM adapter/splitter does NOT kill the
// server process, the running flag is cleared, and the stage returns to idle
// so the admin UI recovers without a restart.
package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

// panickingExtractor causes the pipeline goroutine to panic on the first
// ExtractNote call, simulating an LLM adapter or splitter crash.
type panickingExtractor struct {
	once sync.Once
}

func (p *panickingExtractor) ExtractNote(_ context.Context, _ ecl.Note) (ecl.Extraction, error) {
	var panicked interface{}
	p.once.Do(func() {
		panic("IN-05 panic simulation: LLM adapter crashed")
	})
	// If called again (shouldn't happen), return empty.
	return ecl.Extraction{}, nil
}

// fakeSeedForPanic is a no-op seed that records it was called.
type fakeSeedForPanic struct {
	called bool
	mu     sync.Mutex
}

func (f *fakeSeedForPanic) seed(_ context.Context, _ []retriever.Document) error {
	f.mu.Lock()
	f.called = true
	f.mu.Unlock()
	return nil
}

func (f *fakeSeedForPanic) wasCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.called
}

// fakeFailuresForPanic is a no-op ledger.
type fakeFailuresForPanic struct{}

func (fakeFailuresForPanic) UpsertFailure(_ ecl.Failure) error { return nil }
func (fakeFailuresForPanic) ListFailures() ([]ecl.Failure, error) {
	return []ecl.Failure{}, nil
}
func (fakeFailuresForPanic) GetFailure(_ string) (ecl.Failure, error) {
	return ecl.Failure{}, nil
}

// buildPanicManager creates a manager wired with a panicking extractor so
// that the run goroutine panics as soon as it enters the extract stage.
func buildPanicManager(t *testing.T) *ecl.Manager {
	t.Helper()
	seed := &fakeSeedForPanic{}
	fs := fakeFailuresForPanic{}
	kbDir := pipelineKBDir(t) // 3-note temp corpus (uses t.TempDir)
	mgr := ecl.NewManager("http://skillforge.dev/ns#", kbDir, store.New(),
		&panickingExtractor{}, fs, seed.seed)
	next := int64(1)
	mgr.SetIDAllocator(func() int64 {
		v := next
		next++
		return v
	})
	return mgr
}

// TestAdminPipelineRunGoroutineRecoversFromPanic (IN-05): a panic inside
// the detached run goroutine is recovered, the server process survives,
// the running flag is cleared (stage returns to idle), and the manager is
// claimable again.
func TestAdminPipelineRunGoroutineRecoversFromPanic(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")

	mgr := buildPanicManager(t)
	deps := &Deps{ECL: mgr}

	// Spin up a minimal Hertz server with just the run route.
	h := server.Default(server.WithHostPorts(":0"))
	h.POST("/api/admin/pipeline/run", deps.adminPipelineRunHandler)

	// Claim the manager first to start the run.
	if !mgr.CASRunning() {
		t.Fatal("CASRunning claim failed — manager should be free")
	}

	// Send the run request. The handler starts the goroutine and returns 200.
	// The goroutine will panic in extractWithRetry.
	resp := ut.PerformRequest(h.Engine, "POST", "/api/admin/pipeline/run", nil,
		ut.Header{Key: "X-Admin-Token", Value: "tok"})
	if resp.Code != 200 {
		t.Fatalf("run request: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Wait long enough for the panic to fire and RecoverFromPanic to run.
	// The panic fires during the first ExtractNote call inside the goroutine.
	time.Sleep(500 * time.Millisecond)

	// (a) The server process is still alive — t hasn't been fatal'd.
	// (b) The running flag is cleared: CASRunning now returns true.
	if mgr.CASRunning() {
		// Clean up the extra claim we just made.
		mgr.ReleaseRunning()
	} else {
		t.Error("(a/b) running flag was NOT cleared after panic — manager is stuck")
	}

	// (c) The stage is back to idle (RecoverFromPanic reset it).
	st := mgr.Status()
	if st.Stage != ecl.StageIdle {
		t.Errorf("(c) stage after panic = %q, want %q (admin UI must be operable)", st.Stage, ecl.StageIdle)
	}
	if st.Total != 0 || st.Done != 0 {
		t.Errorf("(c) counters after panic = (total=%d, done=%d), want all zero", st.Total, st.Done)
	}

	// (d) A new run can be claimed and reaches done (manager is fully healthy).
	if !mgr.CASRunning() {
		t.Fatal("second CASRunning claim failed after recovery")
	}
	go func() { _ = mgr.Run(context.Background()) }()
	st = awaitStage(t, mgr, ecl.StageDone)
	if st.Stage != ecl.StageDone {
		t.Errorf("(d) re-run stage = %q, want %q", st.Stage, ecl.StageDone)
	}
}
