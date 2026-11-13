# SPDX-License-Identifier: Apache-2.0
# scripts/build-release.ps1 - local release builds for SkillForge.
#
# Why a script: cmd/server links duckdb via CGO, so a plain
#   $env:GOOS='linux'; go build
# silently fails (or errors) on Windows - the linux/amd64 target needs a
# Linux or musl toolchain. Coverage:
#
#   windows/amd64  built locally with the MinGW gcc in PATH (gcc >= 15
#                  required: duckdb's prebuilt static lib references
#                  __emutls_v.* symbols that GCC 16's native-TLS libstdc++
#                  no longer provides).
#   linux/amd64    delegated to WSL (default distro) when available -
#                  the repo is mounted at the same path, so the build runs
#                  in-place and the artifact is copied back to dist/.
#                  With no WSL, prints the CI fallback (git tag -> the
#                  goreleaser release workflow per .goreleaser.yaml).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-release.ps1            # both targets
#   powershell ... -File scripts\build-release.ps1 -Targets windows,linux                    # pick targets
#   powershell ... -File scripts\build-release.ps1 -WslDistro Ubuntu-22.04                   # pick distro
#
# Output: dist\skillforge-server_<os>_amd64<.exe>

param(
    [string[]]$Targets = @('windows', 'linux'),
    [string]$WslDistro = ''
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$dist = Join-Path $root 'dist'
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$ldflags = '-s -w'
$failed = @()

foreach ($target in $Targets) {
    switch ($target.ToLower()) {
        'windows' {
            Write-Host "=== build windows/amd64 (local gcc: $((gcc --version)[0]))"
            $env:CGO_ENABLED = '1'
            $env:GOOS = 'windows'
            $env:GOARCH = 'amd64'
            $out = Join-Path $dist 'skillforge-server_windows_amd64.exe'
            go build -trimpath -ldflags $ldflags -o $out ./cmd/server
            if ($LASTEXITCODE -ne 0) { $failed += 'windows'; continue }
            Write-Host "  OK -> $out"
        }
        'linux' {
            $wsl = if ($WslDistro) { "wsl.exe -d $WslDistro" } else { 'wsl.exe' }
            $hasWsl = ($null -ne (Get-Command wsl.exe -ErrorAction SilentlyContinue))
            if ($hasWsl) {
                Write-Host "=== build linux/amd64 (via WSL)"
                # -buildvcs=false: the repo may carry a dirty worktree and a
                # read-only .git mount from Windows; vcs stamping is not worth
                # the failure mode.
                $cmd = "cd /mnt/c/$($root.Substring(3) -replace '\\','/') && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GOFLAGS=-buildvcs=false go build -trimpath -ldflags '$ldflags' -o /tmp/skillforge-server_linux ./cmd/server"
                & wsl.exe bash -lc $cmd
                if ($LASTEXITCODE -eq 0) {
                    $out = Join-Path $dist 'skillforge-server_linux_amd64'
                    & wsl.exe bash -lc "cp /tmp/skillforge-server_linux '$(($out -replace '\\','/'))'"
                    if ($LASTEXITCODE -eq 0) { Write-Host "  OK -> $out" } else { $failed += 'linux' }
                } else {
                    $failed += 'linux'
                }
            } else {
                Write-Host '=== build linux/amd64: SKIPPED - no WSL on this host.'
                Write-Host '  Fallback: push a v* tag and let the goreleaser workflow (.goreleaser.yaml +'
                Write-Host '  .github/workflows/release.yml) produce both targets, or build inside Docker:'
                Write-Host '    docker build -f Dockerfile.backend -t skillforge-server:linux .'
                Write-Host '  (cross-compiling linux/amd64 from Windows needs a musl/mingw-linux'
                Write-Host '   toolchain that MSYS2 does not ship).'
            }
        }
        default { Write-Host "unknown target '$target' - use windows / linux" }
    }
}

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
if ($failed.Count -gt 0) {
    Write-Host ("[build-release] FAILED targets: " + ($failed -join ', '))
    exit 1
}
Write-Host '[build-release] done.'
