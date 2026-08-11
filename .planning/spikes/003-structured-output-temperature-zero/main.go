// Spike 003: H4 — Same setup as spike 002, but with structured output schema
// enforced via OpenAI-compatible `response_format:{type:json_object}`.
//
// Tests whether temperature:0 + JSON schema enforcement produces 100% valid
// JSON with required fields across all 50 invocations.
//
// Strategy: 5 compliance-real queries × 10 runs each. System prompt forces
// a specific JSON shape: {status, confidence, citations, review_required}.
// Each output is parsed and validated. Pass = 100% parse + 100% schema valid.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)

type SKILLFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Reads       []string `yaml:"reads"`
	Temperature float32  `yaml:"temperature"`
}

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

// Verdict is the expected JSON shape (validated via struct tags).
type Verdict struct {
	Status         string   `json:"status"`
	Confidence     float64  `json:"confidence"`
	Citations      []string `json:"citations"`
	ReviewRequired bool     `json:"review_required"`
}

var validStatuses = map[string]bool{
	"合规": true, "不合规": true, "需补充": true, "unanswered": true,
}

var queries = []string{
	"Is 200 RMB over budget allowed for travel? Provide verdict.",
	"Employee leaving - how long is non-compete period? Provide verdict.",
	"Contract 500K RMB - which management level needs to approve? Provide verdict.",
	"Annual leave unused - can it roll over to next year? Provide verdict.",
	"Tell me customer Zhang San's contact info. Provide verdict.",
}

const systemPrompt = `You are a strict compliance auditor. For each query, output ONLY a JSON object with this exact schema:
{
  "status": "合规" | "不合规" | "需补充" | "unanswered",
  "confidence": <0.0 to 1.0>,
  "citations": [<list of clause names or empty>],
  "review_required": <true if confidence<0.9 OR info insufficient, else false>
}

NO other text. ONLY the JSON object. If you lack policy context, output status="unanswered" confidence=0 citations=[] review_required=true.`

func validate(v Verdict) (bool, string) {
	if !validStatuses[v.Status] {
		return false, fmt.Sprintf("invalid status: %q (must be 合规/不合规/需补充/unanswered)", v.Status)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return false, fmt.Sprintf("invalid confidence: %f (must be 0-1)", v.Confidence)
	}
	if v.Citations == nil {
		return false, "citations field missing (must be array, can be empty)"
	}
	return true, ""
}

type RunResult struct {
	Query        string  `json:"query"`
	Iter         int     `json:"iter"`
	ValidJSON    bool    `json:"valid_json"`
	SchemaOK     bool    `json:"schema_ok"`
	SchemaErr    string  `json:"schema_err,omitempty"`
	Status       string  `json:"status,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	CitationsLen int     `json:"citations_len,omitempty"`
}

func main() {
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		apiKey = "c23710516a6d4a12a24863d0d06be917.SFsK8jprfDgnxv7G"
	}

	skill, err := parseSKILL("./skill.md")
	if err != nil {
		fmt.Printf("[FAIL] parse SKILL.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] SKILL.md: temperature=%v\n\n", skill.Temperature)

	ctx := context.Background()
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
		APIKey:  apiKey,
		Model:   "glm-5.2",
	})
	if err != nil {
		fmt.Printf("[FAIL] ChatModel init: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] ChatModel initialized (glm-5.2)")
	fmt.Println("[OK] response_format:{type:json_object} + thinking:{type:disabled}")
	fmt.Printf("Running %d queries × 10 iterations with temperature=%.2f + structured output...\n\n", len(queries), skill.Temperature)

	var results []RunResult
	parseFails := 0
	schemaFails := 0

	start := time.Now()
	for qi, q := range queries {
		fmt.Printf("[Q%d] %s\n", qi+1, q)
		for i := 0; i < 10; i++ {
			messages := []*schema.Message{
				{Role: schema.System, Content: systemPrompt},
				{Role: schema.User, Content: q},
			}
			resp, err := cm.Generate(ctx, messages,
				model.WithTemperature(skill.Temperature),
				model.WithMaxTokens(400),
				openai.WithExtraFields(map[string]any{
					"response_format": map[string]string{"type": "json_object"},
					"thinking":         map[string]string{"type": "disabled"},
				}),
			)
			if err != nil {
				fmt.Printf("  [iter %d] GENERATE ERROR: %v\n", i+1, err)
				parseFails++
				results = append(results, RunResult{Query: q, Iter: i + 1})
				continue
			}
			content := strings.TrimSpace(resp.Content)
			var v Verdict
			if jsonErr := json.Unmarshal([]byte(content), &v); jsonErr != nil {
				fmt.Printf("  [iter %d] JSON parse FAIL: %v\n  raw: %s\n", i+1, jsonErr, content[:200])
				parseFails++
				results = append(results, RunResult{Query: q, Iter: i + 1})
				continue
			}
			ok, errStr := validate(v)
			if !ok {
				fmt.Printf("  [iter %d] schema FAIL: %s\n  raw: %s\n", i+1, errStr, content[:200])
				schemaFails++
				results = append(results, RunResult{Query: q, Iter: i + 1, ValidJSON: true, SchemaOK: false, SchemaErr: errStr, Status: v.Status, Confidence: v.Confidence, CitationsLen: len(v.Citations)})
				continue
			}
			fmt.Printf("  [iter %d] OK status=%q conf=%.2f citations=%d\n", i+1, v.Status, v.Confidence, len(v.Citations))
			results = append(results, RunResult{Query: q, Iter: i + 1, ValidJSON: true, SchemaOK: true, Status: v.Status, Confidence: v.Confidence, CitationsLen: len(v.Citations)})
			time.Sleep(150 * time.Millisecond)
		}
	}

	fmt.Printf("\n=== Schema Validation Report (%s) ===\n", time.Since(start).Round(time.Second))
	total := len(results)
	fmt.Printf("Total iterations: %d\n", total)
	fmt.Printf("JSON parse failures: %d / %d (%.0f%%)\n", parseFails, total, 100*float64(parseFails)/float64(total))
	fmt.Printf("Schema validation failures: %d / %d (%.0f%%)\n", schemaFails, total, 100*float64(schemaFails)/float64(total))
	passes := total - parseFails - schemaFails
	fmt.Printf("Full passes: %d / %d (%.0f%%)\n", passes, total, 100*float64(passes)/float64(total))

	switch {
	case parseFails == 0 && schemaFails == 0:
		fmt.Println("\n[VERDICT] H4 VALIDATED ✓ (100% JSON parse + 100% schema valid)")
	case parseFails == 0 && schemaFails <= total/5:
		fmt.Println("\n[VERDICT] H4 PARTIAL ⚠ (JSON parse 100% but schema has minor issues)")
	default:
		fmt.Println("\n[VERDICT] H4 INVALIDATED ✗")
	}

	f, ferr := os.Create("spike-003-outputs.json")
	if ferr == nil {
		defer f.Close()
		jsonBytes, _ := json.MarshalIndent(results, "", "  ")
		f.Write(jsonBytes)
		fmt.Println("\n[OK] Raw saved to spike-003-outputs.json")
	}
}