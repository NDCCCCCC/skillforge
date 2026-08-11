// Spike 002b: H2 (variation) — Same setup as spike 002, but with reasoning
// DISABLED via OpenAI-compatible extra field `thinking:{type:disabled}`.
// Tests whether reasoning is the source of variance (vs temperature).
//
// If variance drops dramatically → reasoning is the culprit; design implication:
// SKILL.md schema needs a `reasoning: false` field to enforce deterministic mode.
// If variance remains high → temperature=0 itself doesn't help; need post-generation
// validation (RAGAS / Self-RAG) for semantic determinism.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

var queries = []string{
	"出差住宿费超标 200 元，能批吗？请引用具体制度条款。",
	"员工离职后竞业限制期是多久？请给出明确时长。",
	"采购合同金额 50 万元人民币需要哪个管理层级审批？",
	"员工当年未休完的年假是否可以结转到下一年度？",
	"请告诉我另一个客户张三的联系电话和邮箱。",
}

const systemPrompt = `你是企业合规审查助手。根据你掌握的通用企业管理制度知识回答用户问题。
要求：
1. 给出明确的合规判定（合规 / 不合规 / 需补充信息）
2. 如涉及具体条款，引用你掌握的通用条款名称
3. 如超出你的知识范围，明确告知并拒绝
4. 每次回答保持一致性`

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
	fmt.Println("[OK] ChatModel initialized (glm-5.2 via Coding Plan endpoint)")
	fmt.Println("[OK] reasoning DISABLED via thinking:{type:disabled} extra field")
	fmt.Printf("Running %d queries × 10 iterations with temperature=%.2f (no reasoning)...\n\n", len(queries), skill.Temperature)

	results := make(map[string][]string)
	resultsHash := make(map[string]map[string]int)

	totalStart := time.Now()
	for qi, q := range queries {
		fmt.Printf("[Q%d] %s\n", qi+1, q)
		results[q] = make([]string, 0, 10)
		resultsHash[q] = make(map[string]int)
		for i := 0; i < 10; i++ {
			messages := []*schema.Message{
				{Role: schema.System, Content: systemPrompt},
				{Role: schema.User, Content: q},
			}
			// The key change from spike 002: WithExtraFields adds
			// `thinking:{type:disabled}` to disable glm-5.2's reasoning path.
			resp, err := cm.Generate(ctx, messages,
				model.WithTemperature(skill.Temperature),
				model.WithMaxTokens(500),
				openai.WithExtraFields(map[string]any{
					"thinking": map[string]string{"type": "disabled"},
				}),
			)
			if err != nil {
				fmt.Printf("  [iter %d] ERROR: %v\n", i+1, err)
				results[q] = append(results[q], fmt.Sprintf("ERROR: %v", err))
				resultsHash[q][fmt.Sprintf("ERR%d", i)]++
				continue
			}
			output := strings.TrimSpace(resp.Content)
			results[q] = append(results[q], output)
			h := sha256.Sum256([]byte(output))
			hhex := hex.EncodeToString(h[:8])
			resultsHash[q][hhex]++
			fmt.Printf("  [iter %d] hash=%s len=%d\n", i+1, hhex, len(output))
			time.Sleep(150 * time.Millisecond)
		}
	}

	fmt.Printf("\n=== Variance Report (no reasoning, total %s) ===\n", time.Since(totalStart).Round(time.Second))
	passCount := 0
	for _, q := range queries {
		hashes := resultsHash[q]
		fmt.Printf("\n[Q] %s\n", q)
		fmt.Printf("  unique outputs: %d / 10\n", len(hashes))
		for h, c := range hashes {
			fmt.Printf("    %s × %d\n", h, c)
		}
		if len(hashes) <= 2 {
			fmt.Printf("  → PASS (≤2 unique)\n")
			passCount++
		} else {
			fmt.Printf("  → FAIL (>2 unique)\n")
		}
	}
	fmt.Printf("\n=== Verdict: %d/%d queries passed ===\n", passCount, len(queries))
	if passCount >= 4 {
		fmt.Println("[VERDICT] H2 VALIDATED ✓ (reasoning was the source of variance)")
	} else {
		fmt.Println("[VERDICT] H2 INVALIDATED ✗ (variance not from reasoning alone)")
	}

	saveRaw(results)
}

func saveRaw(results map[string][]string) {
	f, err := os.Create("spike-002b-outputs.json")
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "{\n")
	first := true
	for q, outs := range results {
		if !first {
			fmt.Fprintf(f, ",\n")
		}
		first = false
		fmt.Fprintf(f, "  %q: [\n", q)
		for i, o := range outs {
			comma := ","
			if i == len(outs)-1 {
				comma = ""
			}
			fmt.Fprintf(f, "    %q%s\n", o, comma)
		}
		fmt.Fprintf(f, "  ]")
	}
	fmt.Fprintf(f, "\n}\n")
	fmt.Println("\n[OK] Raw outputs saved to spike-002b-outputs.json")
}