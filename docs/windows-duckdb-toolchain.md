# Windows 上 DuckDB 堆损坏：根因、修复与工具链要求

> **状态：已修复（2026-11-11）。**
> 根因是 MSYS2 UCRT64 GCC 16 与 DuckDB 预编译库（GCC 15 构建）的 C++ 运行时 ABI 不兼容。
> 修复方式：换 **GCC 15.2.0** 工具链 + 迁移到官方 `github.com/duckdb/duckdb-go/v2`（DuckDB 1.5.6）。
> 验证：`go test ./internal/duckdb/ -count=1` 连跑 10 次全绿（修复前约 50% 概率整进程崩溃）。

## 1. 现象（修复前）

- 包级 `FAIL <pkg>`，但**没有任何 `--- FAIL: TestXxx`**；
- 日志里是 `Exception 0xc0000005`（access violation）或 `exit status 0xc0000374`
  （`STATUS_HEAP_CORRUPTION`）+ 寄存器 dump；
- 崩点随机：`duckdb_destroy_database`、`Appender.AppendRow`、`Flush` 都出现过；
- Go 的 `recover` 拦不住——原生访问违例直接带走整个测试进程。

## 2. 引入时点：不是回归，是「引入即带病」

| 层面 | 时间 | 证据 |
|---|---|---|
| 代码/依赖 | 2026-08-22，commit `5bb2765`（PHASE 14-05） | `git log -S'go-duckdb' -- go.mod` 只有这一条：`go-duckdb/v2 v2.4.3` → `duckdb-go-bindings v0.1.21`（DuckDB 1.4.1），此后从未升降 |
| 编译环境 | 同日 08:03 | `C:\msys64\var\log\pacman.log`：`installed mingw-w64-ucrt-x86_64-gcc (16.1.0-5)` |

所以「`git stash` 掉改动后基线一样崩」不是玄学，是从第一天就红的。

## 3. 根因证据链

**最小复现**（不依赖本仓库任何封装）：`NewConnector(:memory:)` + `Appender` 写 100k 行，
绕过 `internal/duckdb` 全部代码 → 8 次运行崩 4 次，`0xC0000374`。`SET threads=1` 无效；
连 `TestDuckDBBootQueries` 也崩（0.4 s）。**与业务代码无关。**

| 证据 | 内容 |
|---|---|
| 旧库（v0.1.21） | `libduckdb_static.a` 是老 GCC 构建，但 `zstd`/`icu_extension` 标着 `GCC: (Rev5, Built by MSYS2 project)`（= GCC 16）——同一归档内**混编**；还带一个 `emutls_shim.o` 手动补符号 |
| 新库（≥v0.1.22） | 全部由 `MinGW-Builds project`（GCC 15 + emulated TLS）构建，引用 `__emutls_v._ZSt11__once_call` |
| GCC 16 侧 | `libstdc++.a` 与 `libstdc++-6.dll` 都**不含** `__emutls_v._ZSt11__once_call` / `__emutls_v._ZSt15__once_callable`（GCC 16 用 native TLS） |
| 上游 | [duckdb-go-bindings#105](https://github.com/duckdb/duckdb-go-bindings/issues/105)：同环境（MSYS2 UCRT64 GCC 16.1.0）确认预编译库由 GCC 15（Rtools42）构建、与 GCC 16 静态 libstdc++ 不兼容；官方 **closed as not planned**，建议「使用完整的 MSYS2 UCRT64 GCC 15 工具链」 |

推论：v0.1.21 能链接（混编 + shim 侥幸），但二进制里塞了两套 C++ 运行时 → 运行期随机堆损坏；
≥v0.1.22 干净构建，反而在 GCC 16 下**链接期**就失败。

## 4. 修复（已执行）

### 4.1 工具链：GCC 15.2.0

与预编译库同一构建品牌（mingw-builds UCRT posix-seh）：

```powershell
# 包：winlibs / mingw-builds 15.2.0，解压到 D:\tools\gcc15（含 mingw64\bin）
$env:PATH = 'D:\tools\gcc15\mingw64\bin;' + $env:PATH
$env:CC   = 'D:\tools\gcc15\mingw64\bin\gcc.exe'
$env:CXX  = 'D:\tools\gcc15\mingw64\bin\g++.exe'
gcc --version      # 必须 15.x
```

### 4.2 驱动：迁移到官方 duckdb-go

`marcboeker/go-duckdb` 从 v2.5.0 起迁到 `github.com/duckdb/duckdb-go`（DuckDB 官方维护）。

```powershell
go get github.com/duckdb/duckdb-go/v2@v2.10505.0   # = DuckDB 1.5.6
# 6 个文件的 import 路径改名，业务代码零改动
go mod tidy
```

`NewConnector` / `NewAppender` / `NewAppenderFromConn` 签名不变，`internal/duckdb` 与
`cmd/server` 均直接编译通过。

### 4.3 验证

| 检查 | 结果 |
|---|---|
| `go test ./internal/duckdb/ -count=1` × 10 | **10/10 全绿**（修复前 8 次崩 4 次） |
| `go test ./internal/data/ ./internal/kpi/...` | 全绿 |
| `go build ./cmd/server/` | 通过 |

### 4.4 升级驱动带来的一个测试修正

DuckDB 1.5 对每个数据库文件加**独占锁**（1.4 容忍重叠）。`internal/kpi/wsl` 的夹具
`writeLoansStaging` 原来用 `t.Cleanup` 关 staging 连接，而 promote（改名进 datasets/）
发生在它之前 → LRU 的 `ATTACH` 报「另一个程序正在使用此文件」。改为 `defer` 关闭即可
（CHECKPOINT 已落盘）。生产路径 `WriteStagingBatch` 本来就是 defer 关闭，不受影响。

## 5. 工具链体检

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\check-duckdb-toolchain.ps1
```

脚本检查 `go version`、`CC`、gcc 主版本是否为 15，并列出已知的 GCC 15 候选路径；
不匹配时给出可读的失败原因而不是让测试进程崩掉。

## 6. 与本次无关的既有红（别误判）

`go test ./internal/server/` 目前有约 20 条失败，**与 DuckDB 无关**：
其中 19 条是 `actual: 403`，源于 2026-11-09 的授权门禁 `b39c566`
（「最小身份层 + 会话/审计/知识读取的授权门禁」）——`internal/server/knowledge_test.go`
最后一次改动停在 **2026-09-07**，测试没跟上代码修订。
这批属于「测试漂移」，需要按新授权语义补上调用方身份后再修，单独跟进。

## 7. 旧文档保留的排错方法（仍适用）

- 看到包级 FAIL 但没有 `--- FAIL`：先搜 `Exception` / 寄存器 dump，那是 CGO 原生崩溃。
- 不要用「只跑轻量用例」绕行：GCC 版本不对时，任何触及 DuckDB 的用例都可能崩。
- 判断是不是自己的改动：用「裸驱动」最小探针复现，能复现就是环境/依赖问题。
