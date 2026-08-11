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

package milvus

import (
	"context"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/skillforge/skillforge/internal/retriever"
)

// MilvusClient implements retriever.Retriever against a Milvus v2.4.2
// gRPC server. It owns the underlying milvus-sdk-go client and exposes
// the tier-filter-isolated surface the rest of SkillForge consumes.
type MilvusClient struct {
	mc  client.Client
	cfg Config
}

// New dials Milvus, runs a GetVersion health probe, and returns a
// connected MilvusClient. cfg.Addr is required; other zero-value fields
// are replaced with package defaults.
func New(ctx context.Context, cfg Config) (*MilvusClient, error) {
	cfg = cfg.withDefaults()
	mc, err := dialMilvus(ctx, cfg.Addr)
	if err != nil {
		return nil, err
	}
	return &MilvusClient{mc: mc, cfg: cfg}, nil
}

// Wrap wraps an already-connected milvus-sdk-go client into a
// MilvusClient so callers (tests, the legacy NewMilvusClient shim) can
// reuse an existing connection without re-dialing.
func Wrap(mc client.Client, cfg Config) *MilvusClient {
	return &MilvusClient{mc: mc, cfg: cfg.withDefaults()}
}

// ClientOf returns the underlying milvus-sdk-go client. Used by
// callers that need the raw SDK handle (the legacy NewMilvusClient
// shim, the integration test that calls DropCollection directly).
// Returns nil after Disconnect.
func (m *MilvusClient) ClientOf() client.Client {
	return m.mc
}

// Health probes the live connection (GetVersion).
func (m *MilvusClient) Health(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	if _, err := m.mc.GetVersion(probeCtx); err != nil {
		return err
	}
	return nil
}

// Disconnect closes the underlying client. The instance is unusable
// after this call.
func (m *MilvusClient) Disconnect() error {
	if m.mc == nil {
		return nil
	}
	return m.mc.Close()
}

// Ensure creates the collection + IVF_FLAT index if absent. Idempotent.
func (m *MilvusClient) Ensure(ctx context.Context) error {
	return EnsureCollection(ctx, m.mc, m.cfg)
}

// Insert seeds docs into the collection via the package-level helper.
func (m *MilvusClient) Insert(ctx context.Context, docs []retriever.Document) error {
	return SeedDocuments(ctx, m.mc, m.cfg, docs)
}