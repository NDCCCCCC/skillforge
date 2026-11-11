# Windows 上 DuckDB 测试堆损坏：根因与修复跑道

> 结论先行：这不是 SkillForge 的业务代码缺陷，也不是某次改动引入的回归。
> 它是 **MSYS2 UCRT64 GCC 16 与 DuckDB 预编译库（GCC 15 构建）之间的 C++ 运行时 ABI 不兼容**。
> 触发时机是 2026-08-22 首次引入 `go-duckdb` 的同一天装的 GCC 16，所以它从第一天就是红的。

## 1. 现象

`go test ./internal/duckdb/ ./internal/data/ ./internal/server/` 会出现：

- 包级 `FAIL <pkg>`，但**没有任何 `--- FAIL: TestXxx`**；
- 日志里是 `Exception 0xc0000005`（access violation）或 `exit status 0xc0000374`（`STATUS_HEAP_CORRUPTION`）+ 寄存器 dump；
- 崩点随机：`duckdb_close`（`duckdb_destroy_database`）、`Appender.AppendRow`、`Flush` 都出现过；
- Go 的 `recover` 拦不住——原生访问违例直接带走整个测试进程。

## 2. 复现（最小、不依赖业务代码）

`internal/duckdb` 里放一个只依赖 `go-duckdb` 的探针，绕过本仓库全部封装：

```go
c, _ := duckdb.NewConnector(":memory:", func(driver.ExecerContext) error { return nil })
db := sql.OpenDB(c)
db.ExecContext(ctx, "CREATE TABLE wide (...)")
dc, _ := c.Connect(ctx)
app, _ := duckdb.NewAppender(dc, "", "main", "wide")
for i := 0; i < 100_000; i++ { app.AppendRow(...) }
app.Flush()
// defer: app.Close() → dc.Close() → db.Close()
```

实测（GCC 16.1.0）：**8 次运行 4 次崩溃**，退出码 `0xC0000374`。
同一个探针加 `SET threads=1`：8 次崩 4 次 —— 线程数无关。

结论：**与 `internal/duckdb` 的封装无关**，裸驱动同样崩。

## 3. 根因证据链

| 证据 | 内容 |
|---|---|
| `go.mod` 历史 | `git log -S'go-duckdb' -- go.mod` 只有一条：`5bb2765`（2026-08-22，PHASE 14-05）引入 `go-duckdb/v2 v2.4.3` → `duckdb-go-bindings v0.1.21`（DuckDB 1.4.1）。此后**从未升级或降级** |
| `pacman.log` | `2026-08-22T08:03:14` 安装 `mingw-w64-ucrt-x86_64-gcc 16.1.0-5`（同一天） |
| 库的编译工具链 | `libduckdb_static.a` 版本串 `GCC: (GNU)`；`zstd` / `icu_extension` 却是 `GCC: (Rev5, Built by MSYS2 project)` = **同一归档里混编** |
| 新版库 | `bindings v0.1.18 / v0.1.24 / v0.10505.0` 全部由 `MinGW-Builds project`（GCC 15 + emulated TLS）构建，引用 `__emutls_v._ZSt11__once_call` |
| GCC 16 侧 | `libstdc++.a` 与 `libstdc++-6.dll` **都不含** `__emutls_v._ZSt11__once_call` / `__emutls_v._ZSt15__once_callable`（GCC 16 用 native TLS） |
| 上游结论 | [duckdb-go-bindings#105](https://github.com/duckdb/duckdb-go-bindings/issues/105)：同环境（MSYS2 UCRT64 GCC 16.1.0）确认预编译库由 GCC 15（Rtools42）构建、与 GCC 16 静态 `libstdc++` 不兼容；官方 **closed as not planned / wontfix**，建议「使用完整的 MSYS2 UCRT64 GCC 15 工具链」 |

因此：

- 版本 ≥ v0.1.22 的库：**链接期就失败**（`undefined reference to __emutls_v._ZSt11__once_call`）。
- v0.1.21（本项目锁定版）：靠归档内 MSYS2 GCC 16 编译的扩展侥幸补齐了符号，**能链接，但一个二进制里塞了两套 C++ 运行时** → 运行期随机堆损坏。

这解释了「基线同样复现」「与代码改动无关」「非确定性」。

## 4. 修复跑道（按优先级）

### A. 统一到 GCC 15（官方推荐，治本）

1. 安装 MSYS2 UCRT64 GCC 15 完整工具链（winlibs `15.2.0posix-14.0.0-ucrt-r7` 解压即用），目录例如 `D:\tools\winlibs-gcc15`。
2. 编译前把它的 `bin` 放到 `PATH` 最前，并设置 `CC` / `CXX`：

```powershell
$env:PATH = 'D:\tools\winlibs-gcc15\bin;' + $env:PATH
$env:CC  = 'D:\tools\winlibs-gcc15\bin\gcc.exe'
$env:CXX = 'D:\tools\winlibs-gcc15\bin\g++.exe'
gcc --version            # 必须显示 15.x
go clean -cache          # cgo 产物按 CC 计入 build id，换工具链后重建
go test ./internal/duckdb/ -count=1
```

3. 验收标准：`go test ./internal/duckdb/ ./internal/data/ -count=1` 连续 10 次全绿（当前基线是 ~50% 概率进程崩）。

### B. 升级到官方驱动 + GCC 15（顺带解决弃用）

`github.com/marcboeker/go-duckdb` 从 v2.5.0 起迁到 `github.com/duckdb/duckdb-go`。迁移只需：

```powershell
go get github.com/duckdb/duckdb-go/v2@v2.10505.0     # DuckDB 1.5.6（1.5 稳定线）
gofmt -w -r '"github.com/marcboeker/go-duckdb/v2" -> "github.com/duckdb/duckdb-go/v2"' (受影响的 6 个文件)
go mod tidy
```

已验证：`internal/duckdb` 包**编译通过，零源码改动**（`NewConnector` / `NewAppender` / `NewAppenderFromConn` API 未变）。
但 **v2.10505.0 / v2.20000.0 同样要求 GCC 15**，所以 B 必须建立在 A 之上。
（v2.20000.0 = DuckDB 2.0，预编译库额外带 tpch 扩展，对 MinGW 的 TLS 要求更严。）

### C. 动态库（`-tags duckdb_use_lib`）——不推荐

`go-duckdb` 支持 `-tags duckdb_use_lib` 改为链接 `libduckdb.dll`（C ABI 边界，天然免疫 C++ ABI 问题），
代价是每个 Windows 部署都要带 `libduckdb.dll`，且需要 `dlltool` 从 `.lib` 生成 MinGW 导入库。工程复杂度不值当。

## 5. 已排除的「修复」

| 尝试 | 结果 |
|---|---|
| `db.SetMaxIdleConns(0)` 改为默认连接池 | 无效（裸驱动 50% 崩） |
| 手工 `connector.Connect` → `sql.Conn.Raw` | 无效（裸驱动 50% 崩） |
| `SET threads=1` | 无效（8 次崩 4 次） |
| 升级 `duckdb-go-bindings` 到 v0.1.24 | 链接失败（GCC 15 + emutls） |
| 降级 `go-duckdb` 到 v2.3.8 / bindings v0.1.18 | 链接失败（同上） |
| 换官方 `duckdb-go/v2` v2.10505.0 | `internal/duckdb` 编译通过，`cmd/server` 链接失败（同上） |

## 6. 本机网络限制（本会话未能完成 A 的原因）

- GitHub release 下载被中间代理拦截：`curl` 得到 `HTTP=404 SIZE=19`（假的 404，API 元数据正常）；
- SourceForge 镜像可用但约 1 MB/分钟（107 MB 包需十余小时）；
- `winget` 报 `0x80072f0d`；
- 只有 Go module proxy 畅通——而 GCC 15 无法经 module proxy 获取。

因此：**本机目前没有 GCC 15，修复的最后一步需要在可下载的环境执行**（如上 A）。

## 7. 现在起怎么用 Windows 跑后端测试

在没有 GCC 15 之前，先绕开会崩的重负载用例（不是掩盖，是明确边界）：

```powershell
go test ./internal/duckdb/ -count=1 -run 'TestDuckDBBootQueries|TestValidateDatasetID|TestAttachDataset'
```

CI 在 Linux 上跑完整套件，不受影响（Linux 预编译库与本机 GCC 无关）。
