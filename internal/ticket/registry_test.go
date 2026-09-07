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

package ticket

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestNewProvider locks the config-driven registry matrix (TKT-01):
// "none" → zero-side-effect provider, "webhook" → outbound provider,
// unknown name → error listing the valid values, webhook without a URL →
// error. Bad config only ever surfaces as an error here — the fatal
// decision belongs to the assembly point (cmd/server/main.go).
func TestNewProvider(t *testing.T) {
	t.Run("none_returns_store_only_provider", func(t *testing.T) {
		p, err := NewProvider(ProviderNone, Config{})
		if err != nil {
			t.Fatalf("NewProvider(none) returned error: %v", err)
		}
		if p == nil {
			t.Fatal("NewProvider(none) returned nil provider")
		}
	})

	t.Run("none_dispatch_has_zero_network_side_effect", func(t *testing.T) {
		// Deliberately NO httptest server: any outbound side effect would
		// have nowhere legitimate to go. The none provider must succeed
		// with zero attempts and no external receipt.
		p, err := NewProvider(ProviderNone, Config{})
		if err != nil {
			t.Fatalf("NewProvider(none) returned error: %v", err)
		}
		res, err := p.Dispatch(context.Background(), Ticket{
			Number:      "TKT-1",
			SessionID:   "sess-abc",
			TurnID:      7,
			Description: "季度报销口径与差旅制度冲突",
			ChunkIDs:    []int64{11, 12},
			SkillID:     "compliance",
		})
		if err != nil {
			t.Fatalf("none Dispatch returned error: %v", err)
		}
		if res.Attempts != 0 {
			t.Errorf("none Dispatch Attempts = %d, want 0 (zero outbound traffic)", res.Attempts)
		}
		if res.ExternalRef != "" {
			t.Errorf("none Dispatch ExternalRef = %q, want empty (no external receipt)", res.ExternalRef)
		}
	})

	t.Run("webhook_with_url_returns_provider", func(t *testing.T) {
		p, err := NewProvider(ProviderWebhook, Config{WebhookURL: "http://127.0.0.1:1/hook"})
		if err != nil {
			t.Fatalf("NewProvider(webhook) returned error: %v", err)
		}
		if p == nil {
			t.Fatal("NewProvider(webhook) returned nil provider")
		}
	})

	t.Run("webhook_without_url_is_rejected", func(t *testing.T) {
		_, err := NewProvider(ProviderWebhook, Config{})
		if err == nil {
			t.Fatal("NewProvider(webhook) with empty WebhookURL: want error, got nil")
		}
		if !strings.Contains(err.Error(), "WebhookURL") {
			t.Errorf("error should name the missing knob, got: %v", err)
		}
	})

	t.Run("unknown_name_lists_valid_values", func(t *testing.T) {
		p, err := NewProvider("redis", Config{WebhookURL: "http://x"})
		if err == nil {
			t.Fatal("NewProvider(redis): want error, got nil")
		}
		if p != nil {
			t.Errorf("NewProvider(redis) should not return a provider, got %T", p)
		}
		for _, want := range []string{"redis", "none", "webhook"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q should mention %q", err, want)
			}
		}
	})
}

// TestProviderInterfaceIsNarrow locks the one-method discipline mirrored
// from retriever.IndexerBackend: the Provider surface is exactly Dispatch.
func TestProviderInterfaceIsNarrow(t *testing.T) {
	typ := reflect.TypeOf((*Provider)(nil)).Elem()
	if got := typ.NumMethod(); got != 1 {
		t.Fatalf("Provider has %d methods, want exactly 1 (Dispatch)", got)
	}
	if got := typ.Method(0).Name; got != "Dispatch" {
		t.Errorf("Provider sole method = %q, want Dispatch", got)
	}
}

// TestProviderNotWiredErrorConstant guards the 503 body contract the server
// layer consumes (29-06): non-empty and self-explanatory.
func TestProviderNotWiredErrorConstant(t *testing.T) {
	if ProviderNotWiredError == "" {
		t.Fatal("ProviderNotWiredError must not be empty (it is the 503 body)")
	}
	if !strings.Contains(ProviderNotWiredError, "not wired") {
		t.Errorf("ProviderNotWiredError = %q, want it to state the not-wired condition", ProviderNotWiredError)
	}
}
