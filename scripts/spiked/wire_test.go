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

// Package main — wire subcommand tests (SC-1 layer a).
//
// Two tests validate the wire-format contract end-to-end:
//   - TestSpikeWireMock: end-to-end via runWire() (parses SKILL.md, calls
//     Generate, asserts success).
//   - TestMockServerReceivesWireExtras: lower-level — spins up the mock
//     server directly, makes a Generate call with the 3 extras, asserts
//     the captured body contains all 3 substrings (temperature, json_object,
//     disabled).
//
// Both run in < 5s wall-clock (no external network, dynamic port, no sleep).
package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpikeWireMock validates the end-to-end runWire() flow. It calls the
// real runWire (no flag parsing) against the in-process mock server. The
// runWire function must:
//   - start the mock on a dynamic port
//   - parse testdata/skills/authority/SKILL.md (temperature: 0)
//   - invoke Eino ChatModel.Generate with the 3 extras
//   - assert all 3 extras reached the mock
func TestSpikeWireMock(t *testing.T) {
	// Run with a generous timeout; the inner startMockServer uses a 1ms
	// grace period so this should be sub-second in practice.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runWire(ctx, []string{})
	assert.NoError(t, err, "runWire end-to-end should succeed against mock")
}

// TestMockServerReceivesWireExtras is a lower-level test that:
//   - spins up the mock with a captured body buffer
//   - configures an Eino ChatModel with the same 3 extras runWire uses
//   - invokes Generate directly (no SKILL.md parse — focus on wire format)
//   - asserts the captured body contains "temperature", "json_object",
//     and "disabled" substrings
//
// This test is intentionally redundant with TestSpikeWireMock so a failure
// here pinpoints "wire format broken" vs "SKILL.md parse broken".
func TestMockServerReceivesWireExtras(t *testing.T) {
	var captured bytes.Buffer
	ctx := context.Background()

	port, shutdown, err := startMockServer(&captured)
	require.NoError(t, err, "startMockServer")
	defer func() { _ = shutdown() }()

	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: "http://127.0.0.1:" + intToStr(port) + "/v1",
		APIKey:  "mock-key", //nolint:gosec // test fixture
		Model:   "test-model",
	})
	require.NoError(t, err, "NewChatModel")

	messages := []*schema.Message{
		schema.UserMessage("test query"),
	}

	_, err = cm.Generate(ctx, messages,
		model.WithTemperature(0),
		model.WithMaxTokens(400),
		openai.WithExtraFields(map[string]any{
			"response_format": map[string]string{"type": "json_object"},
			"thinking":        map[string]string{"type": "disabled"},
		}),
	)
	require.NoError(t, err, "Generate")

	body := captured.String()
	assert.True(t, captured.Len() > 0, "captured body must be non-empty (defensive)")
	assert.True(t, strings.Contains(body, "temperature"),
		"captured body must contain temperature field; got: %s", body)
	assert.True(t, strings.Contains(body, "json_object"),
		"captured body must contain response_format.type=json_object; got: %s", body)
	assert.True(t, strings.Contains(body, "disabled"),
		"captured body must contain thinking.type=disabled; got: %s", body)
}

// intToStr is a tiny stdlib-free helper to avoid pulling fmt into the hot
// test path; it keeps the test readable without an extra import block.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
