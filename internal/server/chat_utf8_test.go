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
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatHandlerRejectsNonUTF8Query guards the live-validated failure
// where a Windows shell running a legacy GBK codepage POSTs Chinese via
// curl. The mojibake bytes embed to a garbage vector that silently
// recalls the wrong chunk and emits a plausible-but-corrupted verdict
// (年假有几天 → anti_corruption_clause.md 0.31 instead of annual_leave.md
// 0.65). The UTF-8 validity guard in chatHandler MUST fail-fast with a
// 400 so the operator sees an explicit error, not a confidently-wrong
// verdict that looks like a RAG bug.
//
// We hand-build the JSON body: Go's json.Marshal validates/rejects
// non-UTF-8, so it cannot produce the exact bytes a misconfigured CLI
// sends. The GBK bytes for "年假有几天" are c4 ea bc d9 d3 d0 bc b8 cc ec.
func TestChatHandlerRejectsNonUTF8Query(t *testing.T) {
	deps := newTestDeps()
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body := []byte(`{"query":"`)
	// GBK "年假有几天" — invalid UTF-8 (c4 is a 2-byte UTF-8 lead but ea
	// is NOT a continuation byte, so the sequence is unparseable as UTF-8).
	body = append(body, 0xc4, 0xea, 0xbc, 0xd9, 0xd3, 0xd0, 0xbc, 0xb8, 0xcc, 0xec)
	body = append(body, `","skill":"compliance"}`...)

	resp, err := http.Post("http://"+addr+"/api/chat", "application/json", bytes.NewReader(body))
	require.NoError(t, err, "POST /api/chat")
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"non-UTF-8 query must be rejected with 400 (not silently embedded into a garbage vector); body=%q", string(b))
	assert.Contains(t, string(b), "UTF-8",
		"error must explain the UTF-8 requirement so the operator fixes the client encoding; body=%q", string(b))
}

// TestChatHandlerAcceptsUTF8Query is the positive control for the guard
// above: a well-formed UTF-8 Chinese query must NOT trip the guard and
// must proceed into the pipeline (here it reaches the 0-chunks refusal,
// proving it got past the UTF-8 check). Without this, a regression that
// over-rejects valid UTF-8 would silently break every Chinese query.
func TestChatHandlerAcceptsUTF8Query(t *testing.T) {
	deps := newTestDeps()
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := jsonMarshalUTF8(t, "年假有几天", "compliance")
	resp, err := http.Post("http://"+addr+"/api/chat", "application/json", bytes.NewReader(body))
	require.NoError(t, err, "POST /api/chat")
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")

	assert.NotContains(t, string(b), "UTF-8",
		"a valid UTF-8 Chinese query must NOT be rejected by the guard; body=%q", string(b))
}

// jsonMarshalUTF8 builds a UTF-8 JSON chat body without relying on
// encoding/json's package-level escape behavior (kept local so the test
// intent — "hand-built UTF-8 body" — reads next to the assertion).
func jsonMarshalUTF8(t *testing.T, query, skill string) ([]byte, error) {
	t.Helper()
	return []byte(`{"query":"` + query + `","skill":"` + skill + `"}`), nil
}
