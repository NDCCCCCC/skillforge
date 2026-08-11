// Spike 001: Verify Eino ChatModelAgent + OpenAI adapter passes SKILL.md
// `temperature` field through to the HTTP wire body.
//
// Strategy: start a local mock OpenAI server at :18080; configure Eino
// ChatModel with BaseURL pointing at the mock; invoke Generate() with the
// `temperature` extracted from SKILL.md frontmatter; the mock handler prints
// the full request body so we can grep for "temperature":0.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)

// SKILLFrontmatter mirrors the 7-field SKILL.md v1.0 schema (subset needed).
type SKILLFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Agent       string   `yaml:"agent"`
	Reads       []string `yaml:"reads"`
	Writes      []string `yaml:"writes"`
	Temperature float32  `yaml:"temperature"`
}

// parseSKILL extracts YAML frontmatter between --- markers.
func parseSKILL(path string) (*SKILLFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("no YAML frontmatter found")
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated frontmatter")
	}
	yamlStr := rest[:end]
	var fm SKILLFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}

// startMockServer boots a local HTTP server that accepts OpenAI-compatible
// /v1/chat/completions requests and logs the full request body to stdout.
func startMockServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Println("\n========== [MOCK OPENAI] REQUEST BODY ==========")
		fmt.Println(string(body))
		fmt.Println("========== [END BODY] ==========")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"t","object":"chat.completion","created":0,"model":"x","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	})
	srv := &http.Server{Addr: ":18080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[FAIL] mock server: %v\n", err)
		}
	}()
	return srv
}

func main() {
	srv := startMockServer()
	defer srv.Close()
	time.Sleep(200 * time.Millisecond) // wait for server ready

	skill, err := parseSKILL("./skill.md")
	if err != nil {
		fmt.Printf("[FAIL] parse SKILL.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Parsed SKILL.md: name=%q temperature=%v\n", skill.Name, skill.Temperature)

	ctx := context.Background()
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: "http://127.0.0.1:18080/v1",
		APIKey:  "mock-key",
		Model:   "test-model",
	})
	if err != nil {
		fmt.Printf("[FAIL] init ChatModel: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] ChatModel initialized, pointing at mock :18080")

	messages := []*schema.Message{
		{Role: schema.User, Content: "test query"},
	}
	_, err = cm.Generate(ctx, messages, model.WithTemperature(skill.Temperature))
	if err != nil {
		fmt.Printf("[FAIL] Generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] Generate() invoked with model.WithTemperature(extracted from SKILL.md)")

	time.Sleep(500 * time.Millisecond) // wait for mock server to log
	fmt.Println("\n[Done] Inspect [MOCK OPENAI] REQUEST BODY above. If body contains `\"temperature\":0`, H1 is VALIDATED.")
}