---
phase: 07-open-source-release
reviewed: 2026-08-13T00:00:00Z
depth: deep
files_reviewed: 10
files_reviewed_list:
  - internal/skill/examples_validate_test.go
  - scripts/eval_snapshot/main.go
  - scripts/eval_snapshot/main_test.go
  - docker-compose.yml
  - Dockerfile.backend
  - Dockerfile.frontend
  - .env.example
  - examples/skills/calculator/SKILL.md
  - examples/skills/simple-qa/SKILL.md
  - examples/skills/compliance/SKILL.md
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-08-13
**Depth:** deep (per-file + cross-file trace: eval_snapshot → `internal/eval.Report`/`LockFile`; examples_validate_test → `skill.Loader.Scan` → `Validate`/`ResolveTiers`/`TierEnum`)
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Phase 7 是打包/发布阶段，主要交付物是声明式配置文件（compose、Dockerfile、.env.example、3 个 SKILL 模板）和两个 Go 源文件（DIST-08 校验 gate + DIST-09 派生工具）。审查覆盖了所有 CODE-INFRA 文件，并跨文件验证了三个 load-bearing 不变量：

**已核实（非缺陷）：**
- `examples_validate_test.go` 真正调用 `skill.NewLoader(...).Scan()`（间接通过 `parseAndValidate` 调用 `Validate`），路径 `../../examples/skills` 从 `internal/skill/` 解析到 `<root>/examples/skills/` 正确。calculator 的 `reads: [draft/**]` 占位是**诚实**的 —— `draft` 在 `TierEnum`（internal/skill/tier.go:41）中是合法的第四个 tier，没有绕过 validator；占位语义在 SKILL.md body 和 test 注释中均明确披露。
- `deriveRefusalAccuracy` 的除零保护正确（`total==0` 返回 `0,0,0`；渲染器据此显示 `N/A`，不是 `0%`）；与 `eval.Report.Cases []CaseResult` 结构匹配；`LockFile` 字段名与 `internal/eval/frozen.go:72-78` 完全对齐。
- `docker-compose.yml` Milvus pin `v2.4.17`（非 v3，非 `:latest`）；backend `depends_on: milvus: {condition: service_healthy}`；`${VAR:?}` fail-fast 覆盖 `BGE_M3_URL` 和 `GLM_API_KEY`。
- `Dockerfile.backend` Go 1.25-alpine 与 `go.mod` (`go 1.25.0`) 一致；`Dockerfile.frontend` node 22 与 `web/package.json` (`@types/node ^22`) 一致；多阶段，运行时镜像无 build tool。
- `.env.example` 无真实 secret；GLM_API_KEY 为空，由 compose `${VAR:?}` 兜底。
- `examples/skills/compliance/SKILL.md` 与 `skills/compliance/SKILL.md` 二进制 diff 确认完全相同（test 注释声称"verbatim copy"属实）。

**未发现 BLOCKER 级缺陷。** 两个 WARNING 都是会真实影响"陌生人首次起栈"体验的问题，建议修复后再 public。

## Warnings

### WR-01: nginx SSE 反代缺少 `proxy_read_timeout`，长 LLM 思考会断流

**File:** `Dockerfile.frontend:36`
**Issue:**
`location /api` 块只配置了 `proxy_buffering off` + `proxy_http_version 1.1` + `Connection ""`，**没有**显式设置 `proxy_read_timeout` / `proxy_send_timeout`。nginx 默认 `proxy_read_timeout 60s` 是"两次读取之间的最大间隔"，不是总时长。

对 SkillForge 来说这是真实风险：compliance verdict 走 CoT + retrieval + grounding check，LLM 首 token 可能需要 >60s（尤其 GLM-5.2 在复杂条款上的 thinking 阶段）。Hertz SSE 在收到第一个 Eino StreamReader chunk 之前不会写字节，nginx 在 60s 静默后关闭上游连接 → verdict 丢失 → 用户看到截断的流或 502。这正好打到项目的 headline UX（trust verdict pipeline）。

`proxy_buffering off` 只解决"chunk 到达后立刻 flush"的问题，不解决"chunk 到达间隔过长"的问题。两个设置是正交的。

**Fix:**
```dockerfile
RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  location / { try_files $uri /index.html; }\n  location /api { proxy_pass http://backend:8888; proxy_http_version 1.1; proxy_set_header Connection ""; proxy_buffering off; proxy_read_timeout 3600s; proxy_send_timeout 3600s; }\n}\n' > /etc/nginx/conf.d/default.conf
```
（值不必精确 3600s，只要远大于 LLM 首 token 延迟即可；行业惯例 1h-24h。）

### WR-02: `.env.example` 默认 `BGE_M3_URL=http://localhost:11434/...` 对 Docker 用户是陷阱

**File:** `.env.example:31`
**Issue:**
`.env.example` 给出非空默认值 `BGE_M3_URL=http://localhost:11434/v1/embeddings`，注释只说"Local minimum: ollama pull bge-m3 then use the default URL below"。陌生人按 README 流程 `cp .env.example .env && docker compose up` 时，这个 URL 会被注入到 **backend 容器内**，而容器内的 `localhost` ≠ 宿主机的 localhost —— 它指向容器自己。结果：

1. backend 启动看起来正常（BGE-M3 是 lazy 初始化，Pitfall 8：不在启动期 fail）；
2. 第一个 `/api/chat` 请求触发 embedding 调用 → connection refused；
3. 所有 retrieval 返回 0 chunk → 每个问题都被拒答（`no_authorized_source`）；
4. 用户看不到明确错误，会以为 Skill 本身坏了。

`.env.example:25-26` 的 `MONGO_URI` 注释给了多个 example，但 `BGE_M3_URL` 没有给 Docker 路径示例。这是"陌生人首次起栈"D7-04 流程下最容易踩的坑。

**Fix:**（保留 localhost 作为 `go run cmd/server` 直跑的 sensible default，但补一句 Docker 提示）
```bash
# BGE-M3 embedding (user-supplied). Local minimum:
#   ollama pull bge-m3   then use the default URL below.
# Required in both backends. OpenAI-compatible /v1/embeddings endpoint.
#
# Docker users: `localhost` inside the backend container does NOT reach
# the host. Use one of:
#   - Docker Desktop (Mac/Win): http://host.docker.internal:11434/v1/embeddings
#   - Linux host:               http://<host-LAN-IP>:11434/v1/embeddings
BGE_M3_URL=http://localhost:11434/v1/embeddings
```

## Info

### IN-01: `eval_snapshot/main.go run()` 用 `*os.File` 而非 `io.Writer`，牺牲了测试性

**File:** `scripts/eval_snapshot/main.go:90`
**Issue:**
`func run(reportPath, lockPath, runDate string, w *os.File) error` —— `w` 的实际用途只是 `fmt.Fprint(w, ...)`，对 `*os.File` 没有任何额外的方法调用。用 `*os.File` 而不是 `io.Writer` 使得 `run` 无法用 `*bytes.Buffer` 直接测试其 I/O 路径（必须 `os.Pipe()` 或 temp file）。当前测试只覆盖了纯函数 `renderSnapshot`/`deriveRefusalAccuracy`，`run`（包括 `eval.LoadReport`/`LoadLock` 错误传播）零覆盖。

不是 bug，但既然 `run` 是注释里明确说明的"testable seam"，签名应该兑现这个承诺。

**Fix:**
```go
import "io"
// ...
func run(reportPath, lockPath, runDate string, w io.Writer) error {
    // ... body unchanged
}
```
`main()` 调用处 `os.Stdout` 满足 `io.Writer`，无需改 caller。

### IN-02: `docker-compose.yml` 用了 deprecated 的 `MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY`

**File:** `docker-compose.yml:44-45`
**Issue:**
minio 自 RELEASE.2022-01-08 起 deprecated `MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY`，更名为 `MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD`。RELEASE.2024-09-13T20-26-02Z 仍兼容旧名（启动时打 deprecation warning），但未来版本可能移除。

注：这**符合 milvus v2.4.x 官方 standalone compose 上游模式**，所以不是与上游漂移；只是 minio 自身的演进让旧名逐渐失效。当 milvus 上游 compose 更新为新名时，本文件应同步。

**Fix:**（可选，未来 minio 升级时必改）
```yaml
minio:
  environment:
    MINIO_ROOT_USER: minioadmin
    MINIO_ROOT_PASSWORD: minioadmin
```

---

_Reviewed: 2026-08-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
