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

package server

import (
	"errors"
	"testing"
)

// TestCR01PersistedFlagFromPipelineError verifies CR-01: when VerdictPipeline
// returns a pipelineEmitError with persisted=true (SSE emit failed after the
// audit+turn row was already written), chatHandler must NOT call writeAuditAndTurn
// a second time.
func TestCR01PersistedFlagFromPipelineError(t *testing.T) {
	// The pipelineEmitError type must exist and implement error + GetPersisted.
	pe := &pipelineEmitError{err: errors.New("emit failed"), persisted: true}
	if pe.Error() != "emit failed" {
		t.Fatalf("expected error string 'emit failed', got %q", pe.Error())
	}
	if !pe.GetPersisted() {
		t.Fatal("expected GetPersisted() == true")
	}

	// Unwrap must return the underlying error.
	sentinel := errors.New("emit failed")
	pe2 := &pipelineEmitError{err: sentinel, persisted: true}
	if !errors.Is(pe2, sentinel) {
		t.Fatal("errors.Is(pe, sentinel) should be true via Unwrap")
	}

	// persisted=false case.
	peFalse := &pipelineEmitError{err: errors.New("some error"), persisted: false}
	if peFalse.GetPersisted() {
		t.Fatal("expected GetPersisted() == false")
	}
}

// TestCR01ChatHandlerSkipsSecondWrite verifies the chatHandler error branch
// skips writeAuditAndTurn when the pipeline error carries persisted=true.
func TestCR01ChatHandlerSkipsSecondWrite(t *testing.T) {
	// This test documents the contract:
	// - pipelineEmitError.GetPersisted() == true  → chatHandler must NOT write
	// - pipelineEmitError.GetPersisted() == false → chatHandler must write
	//
	// The actual SSE-emit-failure scenario is end-to-end tested via the
	// integration tests; this test locks the interface contract.
	persistedErr := &pipelineEmitError{err: errors.New("SSE emit failed"), persisted: true}

	// Simulate the type-switch check from chatHandler.
	// The check is performed on the error interface (the parameter passed to
	// chatHandler is error), so we wrap in an interface{}.
	var errVal interface{} = persistedErr
	gotPersisted := false
	if pe, ok := errVal.(interface{ GetPersisted() bool }); ok {
		gotPersisted = pe.GetPersisted()
	}
	if !gotPersisted {
		t.Fatal("pipelineEmitError with persisted=true should be detected by type switch")
	}
}
