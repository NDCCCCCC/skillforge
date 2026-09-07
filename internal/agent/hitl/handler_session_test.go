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

package hitl

// CR-04 (26-REVIEW) request-scoping tests.
//
// The handlers in hitlHandlers are package-global singletons while
// BuildToolsForSkill runs per HTTP request (each Hertz request has its
// own goroutine). Binding per-request state (tierEnum / defaultTier /
// sessionID / skillID / ruleMatcher) on the shared instance was both a
// Go data race and an audit-integrity defect: with two in-flight write
// requests the last WithContext won, so a memory_facts_write interrupt
// and its action_pending_approval audit row could be attributed to the
// WRONG session/skill. These tests pin the replacement contract:
// HandlerForSession returns a fresh copy per request and never touches
// the shared instance.

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
)

// TestHandlerForSessionRequestScoped — two calls with different session
// identities must yield two independent tools, each carrying its own
// identity, and the shared singleton must stay untouched.
func TestHandlerForSessionRequestScoped(t *testing.T) {
	reg := testKBReg()

	a, err := HandlerForSession(MemoryFactsTarget, reg, "sess-A", "skill-A", nil)
	require.NoError(t, err)
	b, err := HandlerForSession(MemoryFactsTarget, reg, "sess-B", "skill-B", nil)
	require.NoError(t, err)

	mftA, okA := a.(*MemoryFactsTool)
	require.True(t, okA, "memory:facts handler must be the bindable *MemoryFactsTool")
	mftB, okB := b.(*MemoryFactsTool)
	require.True(t, okB)

	assert.NotSame(t, mftA, mftB, "each request gets its own copy")
	assert.Equal(t, "sess-A", mftA.sessionID)
	assert.Equal(t, "skill-A", mftA.skillID)
	assert.Equal(t, "sess-B", mftB.sessionID)
	assert.Equal(t, "skill-B", mftB.skillID)

	// The shared singleton never carries request state.
	shared, ok := hitlHandlers[MemoryFactsTarget].(*MemoryFactsTool)
	require.True(t, ok)
	assert.Empty(t, shared.sessionID, "the global singleton must stay unbound (CR-04)")
	assert.Empty(t, shared.skillID, "the global singleton must stay unbound (CR-04)")

	// The copy publishes the registry vocabulary (T-26-06-2 preserved).
	info, err := a.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, MemoryFactsToolName, info.Name)
}

// TestHandlerForSessionNilRegistryFailsClosed — a nil registry (or one
// without an authority-role KB) refuses the clone: the fail-closed
// contract moves to the request path, it is not weakened by it.
func TestHandlerForSessionNilRegistryFailsClosed(t *testing.T) {
	_, err := HandlerForSession(MemoryFactsTarget, nil, "sess-A", "skill-A", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kb registry required")

	bad := kb.New([]kb.Entry{
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})
	_, err = HandlerForSession(MemoryFactsTarget, bad, "sess-A", "skill-A", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authority")
}

// TestHandlerForSessionUnregisteredTarget — a nil handler stays the
// caller's silent-skip case (nil, nil).
func TestHandlerForSessionUnregisteredTarget(t *testing.T) {
	h, err := HandlerForSession("skill_state", testKBReg(), "sess-A", "skill-A", nil)
	require.NoError(t, err)
	assert.Nil(t, h)
}

// TestHandlerForSessionConcurrentHammer is the race lock: concurrent
// requests with DISTINCT identities must each observe exactly their own
// identity in the tool they got back. Under the pre-fix shared-mutation
// posture this interleaving both races (visible under -race) and
// mis-attributes identities across goroutines.
func TestHandlerForSessionConcurrentHammer(t *testing.T) {
	const goroutines = 16
	const iterations = 50

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("sess-%02d", g)
			skillID := fmt.Sprintf("skill-%02d", g)
			for i := 0; i < iterations; i++ {
				h, err := HandlerForSession(MemoryFactsTarget, testKBReg(), sessionID, skillID, nil)
				if err != nil {
					errs <- fmt.Errorf("goroutine %s: %w", sessionID, err)
					return
				}
				mft, ok := h.(*MemoryFactsTool)
				if !ok {
					errs <- fmt.Errorf("goroutine %s: handler is %T", sessionID, h)
					return
				}
				// Identity must be exactly what THIS request asked for.
				if mft.sessionID != sessionID || mft.skillID != skillID {
					errs <- fmt.Errorf("cross-session mis-attribution: got (sess=%s, skill=%s), want (sess=%s, skill=%s)",
						mft.sessionID, mft.skillID, sessionID, skillID)
					return
				}
				// Info() reads tierEnum/defaultTier concurrently — with a
				// bound registry it must never fail.
				if _, err := h.Info(context.Background()); err != nil {
					errs <- fmt.Errorf("goroutine %s: Info: %w", sessionID, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
