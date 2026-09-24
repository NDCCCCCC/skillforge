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

package ecl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// TestIN06ExtractBackoffRespectsCancellation pins the IN-06 fix:
// extractWithRetry's backoff sleep must yield to ctx.Done() promptly. The
// pre-fix code used plain time.Sleep, which blocked the cancellation path
// for the full backoff duration (up to 16s on the third attempt). With the
// fix, the loop uses select { ctx.Done() / time.After(d) } and exits with
// ctx.Err() as soon as the caller's context is cancelled.
//
// The test uses a long enough backoff (500ms) that the cancellation
// observably shortens the wait, and asserts both runtime < backoff duration
// and that the returned error is ctx.Err() (not the original failure
// error, which the loop would propagate if cancellation dropped).
func TestIN06ExtractBackoffRespectsCancellation(t *testing.T) {
	fs := newFakeFailureStore()

	// Extractor always fails → loop goes through attempts 1, 2, 3 with
	// backoff sleeps between them.
	ex := newFlakyExtractor(map[string]int{
		"001": 99, // force failure on every call
	})

	// Build manager with a long first-attempt backoff so cancellation
	// observably shortens the wait.
	m, _ := newTestManager(t, threeNoteKBDir(t), ex, fs, nil)
	m.backoff = []time.Duration{
		500 * time.Millisecond, // attempt 1 backoff (after first failure)
		500 * time.Millisecond,
		500 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel mid-backoff: 50ms is well under the 500ms backoff but long
	// enough to let the goroutine enter the select.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := m.extractWithRetry(ctx, Note{SourceIdx: "001"})
	elapsed := time.Since(start)

	require.Error(t, err)
	// IN-06 contract: backoff returned as soon as ctx was cancelled
	// (~50ms), not after the full 500ms sleep. Allow generous slack for
	// scheduler jitter on Windows but DO NOT drop below 50ms (the cancel
	// timing) — if the test ever takes the full 500ms+, the ctx-aware
	// select was reverted.
	assert.Less(t, elapsed, 450*time.Millisecond,
		"ctx-aware backoff must exit before full duration (took %s)", elapsed)
	assert.ErrorIs(t, err, context.Canceled,
		"extractWithRetry must propagate ctx.Err() (context.Canceled), not the original failure")
}

// TestIN06LoadBackoffRespectsCancellation pins the same fix for the
// loadWithRetry retry loop. kg-load failures (transient store apply errors)
// are the second of the three retry loops. The fn argument is what
// loadWithRetry invokes between backoff sleeps; always erroring it forces
// the loop through all 3 attempts with backoff between them.
func TestIN06LoadBackoffRespectsCancellation(t *testing.T) {
	fs := newFakeFailureStore()
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	m.backoff = []time.Duration{
		500 * time.Millisecond,
		500 * time.Millisecond,
		500 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// fn always fails → loadWithRetry retries up to maxAttempts.
	fn := func(_ context.Context) error {
		return errors.New("forced kg-load failure")
	}

	start := time.Now()
	err := m.loadWithRetry(ctx, Note{SourceIdx: "001"}, StageKGLoad, fn)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 450*time.Millisecond,
		"loadWithRetry ctx-aware backoff must exit before full duration (took %s)", elapsed)
	assert.ErrorIs(t, err, context.Canceled,
		"loadWithRetry must propagate ctx.Err() (context.Canceled)")
}

// TestIN06SeedBackoffRespectsCancellation pins the same fix for the
// seedWithRetry batch retry loop. milvus-load is the third retry loop
// (the only batch-level one).
func TestIN06SeedBackoffRespectsCancellation(t *testing.T) {
	fs := newFakeFailureStore()
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	m.backoff = []time.Duration{
		500 * time.Millisecond,
		500 * time.Millisecond,
		500 * time.Millisecond,
	}

	// seedFn always fails → batch retries up to maxAttempts.
	m.seedFn = func(_ context.Context, _ []retriever.Document) error {
		return errors.New("forced seed failure")
	}

	loaded := []loadedNote{{note: Note{SourceIdx: "001"}}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := m.seedWithRetry(ctx, loaded, nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 450*time.Millisecond,
		"seedWithRetry ctx-aware backoff must exit before full duration (took %s)", elapsed)
	// seedWithRetry returns ctx.Err() directly when cancelled mid-backoff.
	assert.ErrorIs(t, err, context.Canceled,
		"seedWithRetry must propagate ctx.Err() (context.Canceled)")
}
