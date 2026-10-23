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

package llmconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// probeTimeout 是每次连通性/模型列表请求的超时上限。面板侧一次点击
// 最多等这么久；过短会在跨海线路上误报超时。
const probeTimeout = 15 * time.Second

// probeClient 是包级共享的 HTTP 客户端（计划要求：probe 复用一个
// client）。仅用于 admin 面板触发的低频请求，不复用 http.DefaultClient
// 以隔离超时配置。
var probeClient = &http.Client{Timeout: probeTimeout}

// TestResult 是一次连通性测试的结果，字段直接面向前端展示。
type TestResult struct {
	OK        bool     `json:"ok"`
	LatencyMS int64    `json:"latency_ms"`
	Models    []string `json:"models,omitempty"` // 拉到模型列表就顺带返回
	Error     string   `json:"error,omitempty"`  // 人类可读的分类错误
}

// TestKey 用一次最小 chat/completions 请求（max_tokens=1）验证
// baseURL + apiKey + model 是否可用。错误按用户可操作的类别归档：
// 401/403 → key 被拒绝；404 → URL 不存在；429 → 限流；5xx → 上游错误；
// 超时 → 连接超时；其余网络错误 → 原文。2xx 且 body 是合法 JSON 即判 OK
// ——哪怕是错误信封（部分网关对 max_tokens=1 的探针请求回 200+error，
// 例如 CPIC 网关的 200+error 形态）；但 2xx body 带显式 "error" 字段且缺
// choices 时判为"上游返回错误信封"，避免把坏网关标成可用。
func TestKey(ctx context.Context, baseURL, apiKey, model string) TestResult {
	start := time.Now()
	res := TestResult{}

	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
		"stream":     false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := probeClient.Do(req)
	res.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutErr(err) {
			res.Error = "连接超时"
		} else {
			res.Error = err.Error() // 网络错误保持原文（DNS/连接拒绝等对排障更有用）
		}
		return res
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		res.Error = fmt.Sprintf("读取响应失败: %v", err)
		return res
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		res.Error = "key 被拒绝（鉴权失败）"
		return res
	case resp.StatusCode == http.StatusNotFound:
		res.Error = "URL 不存在（检查 base_url 是否多/少了 /v1）"
		return res
	case resp.StatusCode == http.StatusTooManyRequests:
		res.Error = "限流（key 有效但配额紧张）"
		return res
	case resp.StatusCode >= 500:
		res.Error = fmt.Sprintf("上游错误（HTTP %d）", resp.StatusCode)
		return res
	case resp.StatusCode >= 300:
		res.Error = fmt.Sprintf("意外状态码（HTTP %d）", resp.StatusCode)
		return res
	}

	// 2xx：body 必须是合法 JSON。
	var probeResp struct {
		Choices []json.RawMessage `json:"choices"`
		Error   json.RawMessage   `json:"error"`
	}
	if err := json.Unmarshal(raw, &probeResp); err != nil {
		res.Error = "响应不是合法 JSON（检查 base_url 是否指向 OpenAI 兼容端点）"
		return res
	}
	if len(probeResp.Choices) == 0 && len(probeResp.Error) > 0 {
		// 200 + 错误信封（如 CPIC 网关形态）：不算可用。
		res.Error = "上游返回错误信封（HTTP 200 但 body 带 error 字段且无 choices）"
		return res
	}
	res.OK = true
	return res
}

// isTimeoutErr 判定错误链里是否含 net 超时（net.Error.Timeout() 接口
// 断言，覆盖 *url.Error 包裹的底层超时）。
func isTimeoutErr(err error) bool {
	var ne interface{ Timeout() bool }
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}

// ListModels 拉取模型列表。modelsURL 为空时回退 baseURL+"/models"。
// 宽松解析四种已知响应形状：
//  1. {"data":[{"id":...}]}   — OpenAI / Ollama / 大多数兼容网关
//  2. {"models":[{"id":...}]} — 部分聚合网关
//  3. ["id", ...]             — 纯字符串数组
//  4. [{"id":...}, ...]       — 纯对象数组
//
// 空 id 跳过；结果按字母序去重后返回。
func ListModels(ctx context.Context, modelsURL, baseURL, apiKey string) ([]string, error) {
	url := modelsURL
	if url == "" {
		url = strings.TrimRight(baseURL, "/") + "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("llmconfig: 构造请求失败: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := probeClient.Do(req)
	if err != nil {
		if isTimeoutErr(err) {
			return nil, fmt.Errorf("llmconfig: 拉取模型列表连接超时: %w", err)
		}
		return nil, fmt.Errorf("llmconfig: 拉取模型列表失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("llmconfig: 读取模型列表响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llmconfig: 模型列表端点返回 HTTP %d: %s", resp.StatusCode, truncate(raw, 200))
	}

	ids, err := parseModelList(raw)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// parseModelList 依次尝试四种形状，全部失败时报"无法识别的响应形状"。
func parseModelList(raw []byte) ([]string, error) {
	// 形状 1/2：{"data":[...]} 或 {"models":[...]}，元素可带 id 字段。
	var shaped struct {
		Data   []modelEntry `json:"data"`
		Models []modelEntry `json:"models"`
	}
	if err := json.Unmarshal(raw, &shaped); err == nil {
		ids := collectIDs(shaped.Data)
		ids = append(ids, collectIDs(shaped.Models)...)
		if len(ids) > 0 {
			return dedupeSort(ids), nil
		}
	}

	// 形状 3：纯字符串数组。
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil && len(strs) > 0 {
		return dedupeSort(strs), nil
	}

	// 形状 4：纯对象数组带 id。
	var objs []modelEntry
	if err := json.Unmarshal(raw, &objs); err == nil {
		if ids := collectIDs(objs); len(ids) > 0 {
			return dedupeSort(ids), nil
		}
	}

	return nil, fmt.Errorf("llmconfig: 无法识别的模型列表响应形状: %s", truncate(raw, 200))
}

// modelEntry 是模型条目里我们唯一关心的字段。
type modelEntry struct {
	ID string `json:"id"`
}

// collectIDs 提取非空 id。
func collectIDs(entries []modelEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ID != "" {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// dedupeSort 排序 + 原地去重。
func dedupeSort(ids []string) []string {
	sort.Strings(ids)
	out := ids[:0]
	var prev string
	for i, id := range ids {
		if i == 0 || id != prev {
			out = append(out, id)
		}
		prev = id
	}
	return out
}

// truncate 截断原始响应用于错误信息，避免日志爆量。
func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
