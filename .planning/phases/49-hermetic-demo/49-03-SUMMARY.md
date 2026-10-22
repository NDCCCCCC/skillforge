---
phase: 49-hermetic-demo
plan: 3
subsystem: 后端同源静态托管前端（web/dist + SPA fallback）
tags: [webui, static-hosting, spa-fallback, hermetic-demo, wave-2]
requires:
  - internal/webui（新建）
  - cmd/server/main.go（挂载点）
  - github.com/cloudwego/hertz v0.10.6（pkg/common/adaptor.HertzHandler，零新依赖）
provides:
  - 后端 :8888 同源伺服 web/dist（D-13），hermetic quickstart 只开一个口
  - SPA fallback 语义（无扩展名缺失 ⇒ index.html；有扩展名缺失 ⇒ 404）
  - web/dist 缺失 ⇒ 503 + 可操作构建提示（不静默 404、不列目录、不托管仓库源码）
  - 目录穿越与符号链接逃逸拒绝
affects:
  - cmd/server/main.go（NoRoute 挂载，位于全部路由之后、h.Spin() 之前）
  - 49-04（quickstart 门禁的 P6 分区依赖本静态层）
actuals:
  tokens: 42000
  tasks: 2
  commits: 0
tech-stack:
  patterns:
    - 用 Hertz 的 NoRoute 挂载而非 catch-all GET 路由：NoRoute 只在没有任何路由命中时触发，/api/* 与 /reports* 结构性地不可能落进静态 handler（Pitfall 13 的结构性解法）
    - http.Handler → Hertz 的桥接用 pkg/common/adaptor.HertzHandler（v0.10.6 已在 go.mod），零新依赖
    - SPA fallback 用「有无扩展名」分流：有扩展名的缺失路径返 404（把缺失的 js 变成 HTML 会让浏览器报 Unexpected token '<'）
    - 源码 lock 测试用「前置换行 + Tab 锚定」定位代码行，避免 strings.Index 命中解释性注释里的字面量
key-files:
  created:
    - internal/webui/static.go
    - internal/webui/static_test.go
  modified:
    - cmd/server/main.go
key-decisions:
  - "挂载点用 NoRoute 而不是 catch-all 路由：既满足 Pitfall 13，又避免 /api 未实现路径被伪装成前端路由（404 而非 SPA HTML）"
  - "用 hertz 的 adaptor.HertzHandler 而不是自己写 ResponseWriter 适配层：零新依赖，且与 Docker 镜像里的 nginx 语义各自独立可测"
---

## Accomplishments

- `internal/webui/static.go`：包文档逐字引用 `Dockerfile.frontend:39` 的 nginx 基线 `location / { try_files $uri /index.html; }`，写明三条硬语义（无扩展名缺失 ⇒ SPA fallback；有扩展名缺失 ⇒ 404；`/api` 与 `/api/` 前缀永不进静态处理）；`web/dist` 或其 `index.html` 缺失 ⇒ **503 + 可操作构建提示**，不静默 404、不列目录、不托管仓库源码；`filepath.Clean` + `filepath.Join` + `http.FileServer` 防目录穿越，符号链接逃逸一并拒绝；`index.html` 走 no-cache，静态资产可缓存；`/api` 不加写缓冲（SSE 必需）。
- `cmd/server/main.go`：新增 `webui.Mount(h, webui.DistDirDefault)`，位置在**全部路由注册之后、`h.Spin()` 之前**，注释说明 NoRoute 的结构性保证与 Pitfall 13 的意图；路由清单里补一行 `GET / — static web UI (web/dist, SPA fallback; 503 until built)`。
- `internal/webui/static_test.go`：14 个用例 —— index at root / asset / SPA fallback（无扩展名）/ 缺失 asset 返 404 / 缺失 dist 返 503 / `/api` 前缀永不回退 HTML / 目录穿越与符号链接拒绝 / 绝不列目录 / index no-cache / **路由顺序源码 lock（server.go 的 4 个 API 锚点 + main.go 的 Mount 与 Spin 相对位置）** / 同一 Hertz 实例上 API 与 SPA 共存（真实连接） / 200 并发请求确定性（只见 200/404/503，绝不 500 或 panic）。

## Task Commits

由编排者统一提交（本 plan 不做 git 操作）。

## Files Created / Modified

见 frontmatter key-files（2 created + 1 modified，共 3 项，无越界）。

## Deviations

- **D-01（执行者中断后由编排者接手）**：执行者 `20d7458c` 在写完 `static.go` / `static_test.go` / `main.go` 挂载后中断，未写 SUMMARY。编排者接手并修掉它留下的两个真实缺陷：
  1. `h.Listener` 不是 Hertz v0.10.6 的公开 API（`*server.Hertz` 无该字段）⇒ 测试编译失败。修法：自己用 `net.Listen("tcp", "127.0.0.1:0")` 预留端口、关闭后交给 Hertz 绑定该地址（`:0` 由 Hertz 内部分配后无从读回）。
  2. `go h.Spin()` 后立刻 GET 会 `connection refused`（goroutine 返回时内核尚未完成 listen）。修法：加一个最多 5 秒的 dial 轮询，等端口真正接受连接再放行后续断言。
- **D-02**：路由顺序源码 lock 的 `strings.Index` 首次匹配命中了 `main.go:1281` 解释性注释里的 “h.Spin()” 字样（真实顺序本来正确：Mount 在 1283 行、Spin 在 1286 行），导致假 FAIL。修法：改用「前置换行 + Tab 锚定」（`"\n\twebui.Mount("` / `"\n\th.Spin()"` / `"\n\th := server.New("`）只匹配代码行。**断言强度未削弱**：真的把 Mount 挪到 `h.Spin()` 之后仍然会被抓住。
- **D-03**：`gofmt -w` 修正测试文件 import 排序（`net` / `time` 的加入位置）。

## Verification

- `go build ./...`（WSL）→ **exit 0**
- `go vet ./internal/webui/ ./cmd/server/`（WSL）→ **exit 0**
- `go test ./internal/webui/ -count=1`（WSL）→ **ok**（14 个用例全绿，5.1s；含真实 Hertz 实例上的 API+SPA 共存与 200 并发确定性）
- `go test ./cmd/server/ ./internal/server/stubchat/ ./internal/verdict/ ./internal/retriever/embedded/ -count=1`（WSL）→ **四个包全 ok**（49-01/49-02 无回归）
- 依赖清单 diff（go.mod / go.sum / web/package.json / web/package-lock.json）→ **UNCHANGED**（零新依赖）

## Notes for the next agent

- `webui.DistDirDefault` 是固定相对路径 `web/dist`，**不读环境变量**（D-15 要求配置体系一字不动）。若 49-04 的 quickstart 要换目录，只能靠改仓库根的工作目录，不能加 env。
- `web/dist` 缺失时的 503 响应体含可操作的构建提示（`npm run build` 之类）；49-04 的门禁 P6 分区据此断言「未构建 ⇒ 503 而非 404」。
- `webui.Mount` 必须留在 `h.Spin()` 之前 —— 路由顺序 lock 会抓住回归。
- 静态层的 `/api` 前缀永不回退 HTML：`/api/nonexistent` 必须 404，不能是 SPA 的 index.html。
