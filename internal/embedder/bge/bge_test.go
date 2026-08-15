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

package bge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// makeOpenAIShapeHandler returns an http.Handler that serves an OpenAI-shape
// /v1/embeddings response with a 1024-dim vector. Used by all three
// TLS / HTTP tests in this file; the only thing that varies is whether
// the test server is plain-HTTP or HTTPS-with-self-signed-cert.
func makeOpenAIShapeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build a 1024-entry float array; values are arbitrary — the
		// contract under test is the embedder's network layer, not the
		// semantic content of the vector.
		parts := make([]string, dimensions)
		for i := 0; i < dimensions; i++ {
			parts[i] = "0.1"
		}
		body := `{"data":[{"embedding":[` + strings.Join(parts, ",") + `]}]}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

// TestEmbed_TLS_InsecureSkipVerify_False — RED baseline. An httptest.NewTLSServer
// returns a self-signed certificate. With InsecureSkipVerify=false (the
// production default), the BGE-M3 embedder MUST reject the connection
// (tls: unknown certificate authority / x509 verification failure). This
// is the test that proves the default posture is safe.
func TestEmbed_TLS_InsecureSkipVerify_False(t *testing.T) {
	ts := httptest.NewTLSServer(makeOpenAIShapeHandler())
	defer ts.Close()

	b := New(Config{URL: ts.URL, InsecureSkipVerify: false})
	_, err := b.Embed(context.Background(), "hi")
	if err == nil {
		t.Fatalf("expected TLS verification error against httptest.NewTLSServer (self-signed), got nil")
	}
	// The exact wording varies across Go versions ("unknown certificate
	// authority" vs "x509: certificate signed by unknown authority"),
	// so we match on substrings known to appear in both.
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "x509") && !strings.Contains(low, "certificate") {
		t.Fatalf("expected error containing x509/certificate, got: %v", err)
	}
}

// TestEmbed_TLS_InsecureSkipVerify_True — GREEN gate. With
// InsecureSkipVerify=true (opt-in via BGE_M3_TLS_SKIP_VERIFY=true),
// the embedder MUST connect to the self-signed HTTPS endpoint and
// return a 1024-dim vector. This proves the operator-facing opt-in
// actually works for internal-network self-signed HTTPS deployments.
func TestEmbed_TLS_InsecureSkipVerify_True(t *testing.T) {
	ts := httptest.NewTLSServer(makeOpenAIShapeHandler())
	defer ts.Close()

	b := New(Config{URL: ts.URL, InsecureSkipVerify: true})
	vec, err := b.Embed(context.Background(), "hi")
	if err != nil {
		t.Fatalf("expected success against httptest.NewTLSServer with InsecureSkipVerify=true, got: %v", err)
	}
	if len(vec) != dimensions {
		t.Fatalf("expected 1024-dim vector, got len=%d", len(vec))
	}
}

// TestEmbed_HTTP_NoTLS_Regression — zero-regression guard. The
// common-case (plain HTTP, no TLS) embedder path MUST still work
// without any TLS configuration; the InsecureSkipVerify field is
// purely additive and must not perturb the existing HTTP path.
func TestEmbed_HTTP_NoTLS_Regression(t *testing.T) {
	ts := httptest.NewServer(makeOpenAIShapeHandler())
	defer ts.Close()

	b := New(Config{URL: ts.URL})
	vec, err := b.Embed(context.Background(), "hi")
	if err != nil {
		t.Fatalf("expected success against httptest.NewServer (plain HTTP), got: %v", err)
	}
	if len(vec) != dimensions {
		t.Fatalf("expected 1024-dim vector, got len=%d", len(vec))
	}
}
