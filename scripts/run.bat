@echo off
REM SPDX-License-Identifier: Apache-2.0
REM scripts\run.bat - thin cmd wrapper for scripts\run.ps1.
REM
REM All logic lives in run.ps1 (env parsing, DSN pinning, build-if-missing,
REM launch). This file exists only so a cmd prompt / double-click / IDE "run"
REM button works without remembering the powershell invocation.
REM
REM IMPORTANT: the binary is launched from OUTSIDE the repo. Corporate DLP
REM blocks writes from unsigned exes started under the repo directory, which
REM looks exactly like "sqlite.migrate: attempt to write a readonly database".
REM See scripts\run.ps1 for the measured evidence.
REM
REM Usage:
REM   scripts\run.bat
REM   set SF_RUN_DIR=D:\CODE\sf-run  &  scripts\run.bat
REM   powershell -File scripts\run.ps1 -NoBuild     (skip the build step)

setlocal
cd /d "%~dp0.."
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0run.ps1" %*
endlocal
