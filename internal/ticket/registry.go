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
	"fmt"
)

// Provider names accepted by NewProvider (SKILLFORGE_TICKET_PROVIDER).
const (
	// ProviderNone stores the ticket and performs zero outbound traffic:
	// Dispatch succeeds immediately with no side effect. This is the
	// default deployment posture — "仅落库不外发".
	ProviderNone = "none"
	// ProviderWebhook POSTs the ticket payload to Config.WebhookURL.
	ProviderWebhook = "webhook"
)

// Compile-time assertion: noneProvider satisfies Provider.
var _ Provider = noneProvider{}

// noneProvider is the store-only provider. Dispatch returns success with no
// side effect whatsoever — no network, no I/O. The server layer still
// persists the ticket row; the row's dispatch_status lands as "none" and no
// outbound call is ever attempted.
type noneProvider struct{}

// Dispatch implements Provider with zero side effects. It accepts a Ticket
// and returns DispatchResult{Attempts: 0} unconditionally: the ticket is
// considered delivered to its destination (the local store, handled by the
// caller) and nothing leaves the process.
func (noneProvider) Dispatch(_ context.Context, _ Ticket) (DispatchResult, error) {
	return DispatchResult{Attempts: 0}, nil
}

// NewProvider resolves a provider implementation from a configuration
// string. Valid names are "none" (store-only, zero outbound) and "webhook"
// (HTTP outbound to cfg.WebhookURL).
//
// Any other value is an error listing the valid names, and the webhook name
// without a WebhookURL is likewise an error — bad configuration only ever
// surfaces as an error from this package. The assembly point
// (cmd/server/main.go) decides the failure mode (log.Fatalf per D16-08-D);
// this package never reads env and never exits the process.
func NewProvider(name string, cfg Config) (Provider, error) {
	switch name {
	case ProviderNone:
		return noneProvider{}, nil
	case ProviderWebhook:
		if cfg.WebhookURL == "" {
			return nil, fmt.Errorf("ticket: provider %q requires a non-empty WebhookURL (set SKILLFORGE_TICKET_WEBHOOK_URL)", ProviderWebhook)
		}
		return newWebhookProvider(cfg), nil
	default:
		return nil, fmt.Errorf("ticket: unknown provider %q (valid values: %s, %s)", name, ProviderNone, ProviderWebhook)
	}
}
