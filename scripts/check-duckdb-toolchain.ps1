#Requires -Version 5.1
<#
.SYNOPSIS
  Check that the Windows C toolchain matches what the DuckDB CGO bindings need.

.DESCRIPTION
  go-duckdb / duckdb-go ship prebuilt Windows static libraries built with
  GCC 15 (MinGW-Builds / Rtools42). Those archives reference emulated-TLS
  symbols such as __emutls_v._ZSt11__once_call that MSYS2 UCRT64 GCC 16's
  libstdc++ does not define (GCC 16 uses native TLS).

  With GCC 16 you either fail to link, or (old bindings) link a binary that
  mixes two C++ runtimes and dies at runtime with
  0xC0000374 (STATUS_HEAP_CORRUPTION) / 0xC0000005.

  This script only inspects; it changes nothing.
  See docs/windows-duckdb-toolchain.md.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\check-duckdb-toolchain.ps1
#>
[CmdletBinding()]
param(
    [int]$ExpectedMajor = 15
)

$ErrorActionPreference = 'Continue'
$script:problems = 0

function Write-OkLine  { param($m) Write-Host "[ OK ] $m" -ForegroundColor Green }
function Write-WarnLine{ param($m) Write-Host "[WARN] $m" -ForegroundColor Yellow; $script:problems++ }
function Write-BadLine { param($m) Write-Host "[FAIL] $m" -ForegroundColor Red;    $script:problems++ }

Write-Host "== Go =="
try {
    $goVer = (& go version) 2>&1
    Write-OkLine $goVer
} catch {
    Write-BadLine "go not found on PATH"
}

Write-Host ""
Write-Host "== C compiler =="
$cc = $env:CC
if (-not $cc) { $cc = (Get-Command gcc -ErrorAction SilentlyContinue).Source }
if (-not $cc) {
    Write-BadLine "no gcc on PATH and CC is unset; cgo cannot build"
} else {
    Write-OkLine "CC = $cc"
    $verLine = (& $cc -dumpfullversion -dumpversion) 2>&1 | Select-Object -First 1
    $major = ("$verLine" -split '\.')[0]
    if ($major -eq "$ExpectedMajor") {
        Write-OkLine "gcc major = $major (matches the DuckDB prebuilt libraries)"
    } else {
        Write-BadLine "gcc major = $major but the DuckDB prebuilt libraries need $ExpectedMajor"
        Write-Host "       Building with this toolchain gives one of:" -ForegroundColor DarkGray
        Write-Host "         - link error: undefined reference to __emutls_v._ZSt11__once_call" -ForegroundColor DarkGray
        Write-Host "         - runtime exit 0xC0000374 (STATUS_HEAP_CORRUPTION)" -ForegroundColor DarkGray
        Write-Host "       Fix: docs/windows-duckdb-toolchain.md section 4" -ForegroundColor DarkGray
    }
}

Write-Host ""
Write-Host "== Known GCC 15 locations =="
$candidates = @(
    'D:\tools\gcc15\mingw64\bin\gcc.exe',
    'D:\tools\winlibs-gcc15\bin\gcc.exe',
    'C:\tools\winlibs-gcc15\bin\gcc.exe',
    'C:\msys64\mingw64\bin\gcc.exe'
)
$found = $candidates | Where-Object { Test-Path $_ }
if ($found) {
    foreach ($f in $found) { Write-OkLine "found $f" }
} else {
    Write-WarnLine "no GCC 15 in the known locations; see docs/windows-duckdb-toolchain.md"
}

Write-Host ""
Write-Host "== Verdict =="
if ($script:problems -eq 0) {
    Write-Host "Toolchain matches; go test ./internal/duckdb/ is safe." -ForegroundColor Green
    exit 0
} else {
    Write-Host "$($script:problems) problem(s); DuckDB tests may crash the test process." -ForegroundColor Yellow
    Write-Host "Docs: docs/windows-duckdb-toolchain.md" -ForegroundColor Yellow
    exit 1
}
