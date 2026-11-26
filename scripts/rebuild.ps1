# SPDX-License-Identifier: Apache-2.0
# scripts/rebuild.ps1 - "I changed code, make it live" for daily iteration.
#
# Thin alias over scripts/build.ps1 (the canonical build). Kept because the
# name is referenced by docs and muscle memory; it must NOT grow its own build
# logic (that is exactly how run.bat drifted out of sync before).
#
# Difference from build.ps1: build.ps1 is the single source of truth and also
# stages web/dist; rebuild.ps1 is the same thing with the frontend step
# skippable, for a backend-only loop.
#
# It does NOT start or stop any process. Start with scripts\run.bat.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\rebuild.ps1
#   powershell ... -File scripts\rebuild.ps1 -SkipWeb     # backend only

param(
    [switch]$SkipWeb
)

$ErrorActionPreference = 'Stop'
$build = Join-Path $PSScriptRoot 'build.ps1'
& powershell -NoProfile -ExecutionPolicy Bypass -File $build -SkipWeb:$SkipWeb
exit $LASTEXITCODE
