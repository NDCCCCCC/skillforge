#Requires -Version 5.1
<#
.SYNOPSIS
  检查 Windows 上编译 SkillForge DuckDB(CGO) 所需的工具链是否匹配。

.DESCRIPTION
  go-duckdb 的 Windows 预编译静态库由 GCC 15（MinGW-Builds / Rtools42）构建，
  含 __emutls_v.* 这类 emulated-TLS 符号；MSYS2 UCRT64 GCC 16 的 libstdc++
  用 native TLS，既没有这些符号定义，运行时也会与库里的 C++ 运行时混用。

  结果：用 GCC 16 编译能"侥幸链接"（v0.1.21），但测试进程会随机以
  0xC0000374 / 0xC0000005 原生崩溃，Go 的 recover 拦不住。

  本脚本只做体检，不改任何东西。详见 docs/windows-duckdb-toolchain.md。

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\check-duckdb-toolchain.ps1
#>
[CmdletBinding()]
param(
    # 期望的 GCC 主版本号
    [int]$ExpectedMajor = 15
)

$ErrorActionPreference = 'Continue'
$script:problems = 0

function Write-OK    { param($m) Write-Host "[ OK ] $m"   -ForegroundColor Green }
function Write-Warn2 { param($m) Write-Host "[WARN] $m"   -ForegroundColor Yellow; $script:problems++ }
function Write-Bad   { param($m) Write-Host "[FAIL] $m"   -ForegroundColor Red;    $script:problems++ }

Write-Host "== Go =="
try {
    $goVer = (& go version) 2>&1
    Write-OK $goVer
} catch {
    Write-Bad "找不到 go，可执行文件不在 PATH 上"
}

Write-Host "`n== C 编译器 =="
$cc = $env:CC
if (-not $cc) { $cc = (Get-Command gcc -ErrorAction SilentlyContinue).Source }
if (-not $cc) {
    Write-Bad "PATH 上没有 gcc，也没有设置 CC；cgo 无法编译"
} else {
    Write-OK "CC = $cc"
    $verLine = (& $cc -dumpfullversion -dumpversion) 2>&1 | Select-Object -First 1
    $major = ($verLine -split '\.')[0]
    if ($major -eq "$ExpectedMajor") {
        Write-OK "gcc 主版本 = $major（与 DuckDB 预编译库一致）"
    } else {
        Write-Bad "gcc 主版本 = $major，而 DuckDB 预编译库需要 $ExpectedMajor"
        Write-Host "       用当前工具链跑 go test ./internal/duckdb/ 会随机堆损坏：" -ForegroundColor DarkGray
        Write-Host "       exit status 0xC0000374 (STATUS_HEAP_CORRUPTION) / 0xC0000005" -ForegroundColor DarkGray
        Write-Host "       修复见 docs/windows-duckdb-toolchain.md 第 4 节" -ForegroundColor DarkGray
    }
}

Write-Host "`n== 候选的 GCC 15 工具链 =="
$candidates = @(
    'D:\tools\winlibs-gcc15\bin\gcc.exe',
    'C:\tools\winlibs-gcc15\bin\gcc.exe',
    'C:\msys64\mingw64\bin\gcc.exe'
)
$found = $candidates | Where-Object { Test-Path $_ }
if ($found) {
    foreach ($f in $found) { Write-OK "发现 $f" }
} else {
    Write-Warn2 "未发现已知路径下的 GCC 15；如需修复请按文档安装 winlibs 15.2.0 UCRT 工具链"
}

Write-Host "`n== 结论 =="
if ($script:problems -eq 0) {
    Write-Host "工具链匹配，可以放心跑 go test ./internal/duckdb/" -ForegroundColor Green
    exit 0
} else {
    Write-Host "存在 $($script:problems) 个问题；DuckDB 相关测试在 Windows 上可能直接崩掉测试进程。" -ForegroundColor Yellow
    Write-Host "文档：docs/windows-duckdb-toolchain.md" -ForegroundColor Yellow
    exit 1
}
