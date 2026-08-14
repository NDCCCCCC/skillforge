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

// Package main — wire subcommand (SC-1 layer a).
//
// Runs an in-process mock OpenAI server on a dynamic port, configures an
// Eino ChatModel pointing at the mock, invokes Generate() with the
// temperature / response_format / thinking extras, then asserts the
// captured request body contains all three extras. This proves Eino v0.7.7
// + eino-ext openai v0.1.5 serialize the extras correctly to the wire
// without burning any real API quota (per D-02).
//
// Pattern adapted from `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go`
// (spike 001). Key adaptations:
//   - Dynamic port allocation via net.Listen("tcp", "127.0.0.1:0")
//     (avoids Pitfall 7 — a hardcoded port may collide on shared hosts).
//   - Captured request body passed in by caller (avoids package-level globals).
//   - response_format + thinking extras layered on top of temperature
//     (proves openai.WithExtraFields map serializes correctly).
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)

// SKILLFrontmatter mirrors the SKILL.md v1.1 7-field schema (subset
// needed by wire spike: name, description, agent, reads, writes, temperature).
type SKILLFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Agent       string   `yaml:"agent"`
	Reads       []string `yaml:"reads"`
	Writes      []string `yaml:"writes"`
	Temperature float32  `yaml:"temperature"`
}

// findProjectRoot walks up from the current working directory looking for
// go.mod. Returns the directory containing go.mod, or an error if not
// found within 10 levels. Used to make fixture paths robust regardless of
// where the binary or test was launched from (cmd/spiked/ vs project root).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found within 10 levels of CWD")
}

// parseSKILL extracts YAML frontmatter between `---` markers. Pattern from
// spike 001 (lines 35-55). The path is resolved relative to the project
// root (found by walking up from CWD to go.mod) so callers can use a
// project-relative path like "testdata/skills/authority/SKILL.md".
func parseSKILL(path string) (*SKILLFrontmatter, error) {
	if !filepath.IsAbs(path) {
		root, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("resolve project root: %w", err)
		}
		path = filepath.Join(root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("no YAML frontmatter in %s", path)
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated frontmatter in %s", path)
	}
	yamlStr := rest[:end]
	var fm SKILLFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
	}
	return &fm, nil
}

// startMockServer boots an OpenAI-compatible /v1/chat/completions handler
// on a dynamic port. The captured request body is appended to capturedBody
// so the caller can assert the wire format. Returns the port, a shutdown
// closure, and any startup error.
//
// Adapted from spike 001 (lines 59-77) with the following changes:
//   - Port is dynamic (net.Listen on :0) — no hardcoded port.
//   - capturedBody is passed in, not a package-level global.
//   - shutdown is a closure (defer-friendly) that returns the error
//     from http.Server.Shutdown.
//   - No time.Sleep race — server is ready when ListenAndServe returns
//     (the listener accepts connections as soon as net.Listen succeeds).
func startMockServer(capturedBody *bytes.Buffer) (port int, shutdown func() error, err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody.Write(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Fixed response from spike 001 (line 69) — no reasoning_content,
		// finish_reason=stop. Sufficient for wire-format validation.
		w.Write([]byte(`{"id":"t","object":"chat.completion","created":0,"model":"x","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	})

	// Bind to a free port on loopback. Port 0 = "let kernel pick".
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("mock listen: %w", err)
	}
	srv := &http.Server{Handler: mux}

	// Serve in a goroutine; ListenAndServe blocks until Shutdown is called.
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "[FAIL] mock server: %v\n", err)
		}
	}()

	// Tiny grace period for the goroutine to start accepting. 1ms is enough
	// for an in-process listener — we just need Serve() to begin. This
	// avoids the 200ms sleep in spike 001.
	time.Sleep(1 * time.Millisecond)

	port = ln.Addr().(*net.TCPAddr).Port
	shutdown = func() error {
		return srv.Shutdown(context.Background())
	}
	return port, shutdown, nil
}

// runWire is the SC-1 layer (a) wire-format regression test. It asserts
// that Eino ChatModel → mock OpenAI server round-trip successfully serializes
// temperature: 0, response_format: {type: json_object}, and
// thinking: {type: disabled} through openai.WithExtraFields.
//
// Per D-01 + D-02: no real GLM call here. Real API is reserved for
// `spike e2e` in Plan 04.
func runWire(ctx context.Context, args []string) error {
	var captured bytes.Buffer

	port, shutdown, err := startMockServer(&captured)
	if err != nil {
		return fmt.Errorf("start mock: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] mock shutdown: %v\n", err)
		}
	}()

	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		APIKey:  "mock-key", //nolint:gosec // test fixture, not a real secret
		Model:   "test-model",
	})
	if err != nil {
		return fmt.Errorf("init chat model: %w", err)
	}

	// Use the authority SKILL.md fixture (temperature: 0). Per
	// .planning/SKILL_SCHEMA.md v1.1 — compliance Skills default to
	// temperature: 0 + reasoning: false.
	const skillPath = "testdata/skills/authority/SKILL.md"
	skill, err := parseSKILL(skillPath)
	if err != nil {
		return fmt.Errorf("parse SKILL: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(skill.Description + " sample audit"),
	}

	resp, err := cm.Generate(ctx, messages,
		model.WithTemperature(skill.Temperature),
		model.WithMaxTokens(400), // safe with thinking:disabled (Pitfall 5)
		openai.WithExtraFields(map[string]any{
			"response_format": map[string]string{"type": "json_object"},
			"thinking":        map[string]string{"type": "disabled"},
		}),
	)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return fmt.Errorf("empty content from mock")
	}

	// Assert all 3 extras reached the wire.
	body := captured.String()
	wantSubstrings := []string{
		`"temperature"`, // skill.Temperature = 0
		`"json_object"`, // response_format.type
		`"disabled"`,    // thinking.type
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			return fmt.Errorf("missing wire extra %q in captured body", want)
		}
	}

	fmt.Println("[OK] spike wire: temperature=0 passthrough verified")
	return nil
}
