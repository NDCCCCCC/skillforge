// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/document"
)

// ErrClass splits step failures by whether a retry can plausibly change
// the outcome (54-03 / Temporal's NonRetryableErrorTypes posture):
//
//   - ClassTransient: the failure was environmental (network blip, rate
//     limit, another pipeline holding a lock) — backoff and retry can
//     absorb it. The intranet Milvus/BGE pair has a documented flakiness
//     history, so transient is a WHITELIST: only recognized environmental
//     signatures retry.
//   - ClassDeterministic: the failure was in the payload itself (format,
//     password, schema) — retrying ten thousand times re-runs the same
//     error. DEFAULT-CLOSED: an unrecognized error classifies
//     deterministic, because an unknown failure that triggers a retry
//     storm is strictly worse than one that waits for a human.
type ErrClass int

const (
	ClassDeterministic ErrClass = iota
	ClassTransient
)

// String makes the classification prefix readable in step error rows
// ("[transient] embed failed: ..." — SC 3: the classification itself is
// the first fact an operator needs).
func (c ErrClass) String() string {
	if c == ClassTransient {
		return "transient"
	}
	return "deterministic"
}

// Prefix wraps a step error with its classification tag.
func (c ErrClass) Prefix(err error) string {
	return "[" + c.String() + "] " + err.Error()
}

// Classify maps a step error to its retry class. Every rule carries its
// rationale inline; the DEFAULT is deterministic (see ErrClass).
func Classify(err error) ErrClass {
	if err == nil {
		return ClassDeterministic
	}
	// --- transient whitelist ---
	if IsBackendUnavailable(err) {
		// Network-level backend failure (DNS/refused/unreachable/
		// deadline/net.Error timeout): the intranet Milvus/BGE pair's
		// documented flakiness class — backoff absorbs the jitter.
		return ClassTransient
	}
	msg := err.Error()
	for _, s := range []string{"429", "503", "rate limit", "too many requests", "temporarily unavailable"} {
		if strings.Contains(strings.ToLower(msg), s) {
			// Embedding-endpoint pressure signatures (BGE-M3 429/503).
			// Heuristic boundary, stated plainly: a payload whose text
			// legitimately contains "429" could false-positive — accepted,
			// because the cost of a wasted retry is one backoff cycle while
			// the cost of a misclassified transient is a dead-letter.
			return ClassTransient
		}
	}
	if errors.Is(err, errECLPipelineBusy) {
		// The ECL manager is running an admin pipeline — wait for it.
		return ClassTransient
	}
	// --- deterministic (payload/family errors) ---
	for _, e := range []error{
		document.ErrContentTypeMismatch,
		document.ErrUnknownFormat,
		document.ErrPasswordProtected,
	} {
		if errors.Is(err, e) {
			return ClassDeterministic
		}
	}
	if strings.Contains(msg, "unknown or deactivated") ||
		strings.Contains(msg, "slug deactivated during upload") {
		// Registry-gate failures: the payload targets a slug the operator
		// has turned off — retry cannot re-activate it.
		return ClassDeterministic
	}
	// DEFAULT: deterministic (Temporal posture — see ErrClass).
	return ClassDeterministic
}

// errECLPipelineBusy mirrors ecl.ErrPipelineBusy without importing the ecl
// package (the ingest package must not depend on the ECL manager — the ECL
// trigger seam is injected as a function). ecl.ErrPipelineBusy's message
// is the matching key; keep the two literals in sync (single grep away).
var errECLPipelineBusy = errors.New("ecl pipeline busy")

// retryLoop runs fn up to r.retryAttempts times for transient failures
// (exponential backoff 1s/4s/16s — ecl defaultBackoff's shape), and
// EXACTLY ONCE for deterministic ones. ctx.Done wins over sleep (the
// IN-06 precedent: a cancelled boot must not wait out a backoff).
func (r *Runner) retryLoop(ctx context.Context, fn func() error) error {
	var last error
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			base := r.retryBackoffBase
			if base <= 0 {
				base = time.Second
			}
			// Exponential shape 1s/4s/16s: base * 4^(attempt-1).
			delay := base << (2 * (attempt - 1))
			if delay <= 0 || delay > 30*time.Second {
				delay = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		last = fn()
		if last == nil {
			return nil
		}
		if Classify(last) == ClassDeterministic {
			return last // one shot: retrying cannot change the outcome
		}
		if attempt >= r.retryAttempts {
			return fmt.Errorf("%s (after %d transient retries)", last.Error(), attempt)
		}
	}
}
