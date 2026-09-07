# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# Makefile — SkillForge operator entry points.
#
# The default target is verify-phase-11 (the Phase 11 acceptance
# gate). Run `make` or `make verify-phase-11` to execute the gate;
# the banner at the end is the operator's pass/fail signal.

.PHONY: check-cgo verify-phase-11 verify-phase-12 verify-phase-13 verify-phase-14 verify-phase-15 verify-phase-16 verify-phase-17 verify-phase-26 test-unit test-frontend build clean

# check-cgo fails fast with an actionable message when gcc is missing.
# mattn/go-sqlite3 and go-duckdb both require CGO; install MSYS2 UCRT64 gcc
# on Windows, then add C:\msys64\ucrt64\bin to PATH.
check-cgo:
	@if command -v gcc >/dev/null 2>&1; then \
		gcc --version | head -n 1; \
	else \
		echo "[check-cgo] ERROR: gcc not found on PATH." >&2; \
		echo "  Windows: install MSYS2, then run:" >&2; \
		echo "    C:\\msys64\\usr\\bin\\pacman.exe -S --noconfirm mingw-w64-ucrt-x86_64-gcc" >&2; \
		echo "  Then add C:\\msys64\\ucrt64\\bin to your PATH before building." >&2; \
		exit 1; \
	fi

# verify-phase-11 is the Phase 11 acceptance gate (Plan 11-08 Task 6).
# Invokes scripts/verify-phase-11.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
verify-phase-11:
	bash scripts/verify-phase-11.sh

# verify-phase-12 is the Phase 12 acceptance gate (Plan 12-08).
# Invokes scripts/verify-phase-12.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
verify-phase-12:
	bash scripts/verify-phase-12.sh

# verify-phase-13 is the Phase 13 HERMETIC gate (Plan 13-04).
# Invokes scripts/verify-phase-13.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
# Live operator gate (Plan 13-05) is documented in the script header
# and is NOT invoked by this target.
verify-phase-13:
	bash scripts/verify-phase-13.sh

# verify-phase-14 is the Phase 14 HERMETIC gate (Plan 14-07).
# Invokes scripts/verify-phase-14.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
# NOTE: includes go-duckdb tests — run on Linux/WSL2/Docker (Windows-native
# gcc cannot link go-duckdb; see 14-05-SUMMARY emutls note).
# Live operator gates (Plan 14-08: Docker CGO build + GLM live judge) are
# documented in the script header and NOT invoked by this target.
verify-phase-14:
	bash scripts/verify-phase-14.sh

# verify-phase-15 is the Phase 15 HERMETIC gate (Plan 15-07).
# Invokes scripts/verify-phase-15.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
# NOTE: includes go-duckdb tests — run on Linux/WSL2/Docker (Windows-native
# gcc cannot link go-duckdb; see 15-07-SUMMARY emutls note).
verify-phase-15:
	bash scripts/verify-phase-15.sh

# verify-phase-16 is the Phase 16 acceptance gate (Plan 16-09).
# Invokes scripts/verify-phase-16.sh; exits 0 with the PASS banner on
# full pass on a Linux/WSL2 host. Windows-host runs report
# [STAGE-NN-BLOCKED-WITH-NOTES] for the WSL triple-lock + cmd/server link
# (D15-01-1 emutls ABI); all other stages PASS hermetically. See the
# script header for the operator-WSL leg commands.
verify-phase-16:
	bash scripts/verify-phase-16.sh

# verify-phase-17 is the Phase 17 acceptance gate (Plan 17-07).
# Invokes scripts/verify-phase-17.sh; exits 0 with the PASS banner on
# full pass on a Linux/WSL2 host. Windows-host runs report
# [STAGE-NN-BLOCKED-WITH-NOTES] for the WSL CGO leg (D15-01-1 emutls ABI).
verify-phase-17:
	bash scripts/verify-phase-17.sh

# verify-phase-26 is the Phase 26 structural hygiene gate (HYG-01, Plan 26-10).
# Three grep-only structural locks over non-test Go — no Go toolchain needed:
#   01 todo-fixme-zero            (only TODO( / TODO: / FIXME( / FIXME: count;
#                                  the eino type name deep.TODO never matches)
#   02 tierenum-zero              (Phase 26 enum deletion evidence)
#   03 cn-tier-literal-scoped-ban (exact quoted CN tier data values banned;
#                                  two annotated prose whitelists, 复核 2027-03)
# Exits 0 with the PASS banner on full pass, 1 with [STAGE-NN-FAIL] otherwise.
verify-phase-26:
	bash scripts/verify-phase-26.sh

# test-unit runs the full Go test suite for fast iteration.
test-unit:
	go test ./... -count=1

# test-frontend runs the vitest UI suite.
test-frontend:
	cd web && npm test

# build compiles all Go binaries.
build:
	go build ./...

# clean removes generated reports (the acceptance-gate ship evidence).
clean:
	rm -rf reports/
