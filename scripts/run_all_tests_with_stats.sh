#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

START_EPOCH="$(date +%s)"
START_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

RUN_ID="$(date +%Y%m%d-%H%M%S)"
LOG_DIR="$ROOT_DIR/.test-logs/$RUN_ID"
mkdir -p "$LOG_DIR"

JSON_LOG="$LOG_DIR/go-test.jsonl"
COVERAGE_PROFILE="$LOG_DIR/coverage.out"
COVERAGE_FUNC="$LOG_DIR/coverage.func.txt"
SUMMARY_LOG="$LOG_DIR/summary.txt"

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
	C_RESET=$'\033[0m'
	C_BOLD=$'\033[1m'
	C_CYAN=$'\033[36m'
	C_GREEN=$'\033[32m'
	C_RED=$'\033[31m'
	C_YELLOW=$'\033[33m'
	C_BLUE=$'\033[34m'
else
	C_RESET=""
	C_BOLD=""
	C_CYAN=""
	C_GREEN=""
	C_RED=""
	C_YELLOW=""
	C_BLUE=""
fi

printf "%s[1/4]%s %sRunning full Go test suite with JSON output...%s\n" "$C_BOLD$C_CYAN" "$C_RESET" "$C_BLUE" "$C_RESET"
set +e
go test -count=1 -json ./... | tee "$JSON_LOG" >/dev/null
TEST_EXIT=${PIPESTATUS[0]}
set -e

printf "%s[2/4]%s %sRunning coverage pass...%s\n" "$C_BOLD$C_CYAN" "$C_RESET" "$C_BLUE" "$C_RESET"
set +e
go test -count=1 -covermode=atomic -coverprofile="$COVERAGE_PROFILE" ./...
COVER_EXIT=$?
set -e

printf "%s[3/4]%s %sBuilding coverage function summary...%s\n" "$C_BOLD$C_CYAN" "$C_RESET" "$C_BLUE" "$C_RESET"
if [[ -f "$COVERAGE_PROFILE" ]]; then
	go tool cover -func="$COVERAGE_PROFILE" > "$COVERAGE_FUNC"
else
	: > "$COVERAGE_FUNC"
fi

printf "%s[4/4]%s %sAggregating stats...%s\n" "$C_BOLD$C_CYAN" "$C_RESET" "$C_BLUE" "$C_RESET"
go run ./scripts/teststats --json "$JSON_LOG" --coverage "$COVERAGE_FUNC" | tee "$SUMMARY_LOG"
STATS_EXIT=${PIPESTATUS[0]}

printf "\n%sArtifacts saved in:%s %s\n" "$C_BOLD$C_CYAN" "$C_RESET" "$LOG_DIR"
printf "  - %sJSON events:%s %s\n" "$C_CYAN" "$C_RESET" "$JSON_LOG"
printf "  - %sCoverage profile:%s %s\n" "$C_CYAN" "$C_RESET" "$COVERAGE_PROFILE"
printf "  - %sCoverage breakdown:%s %s\n" "$C_CYAN" "$C_RESET" "$COVERAGE_FUNC"
printf "  - %sAggregated summary:%s %s\n" "$C_CYAN" "$C_RESET" "$SUMMARY_LOG"

TOTALS_LINE="$(awk '
/^Tests:/ {
	run = ""
	pass = ""
	fail = ""
	skip = ""
	for (i = 1; i <= NF; i++) {
		if ($i ~ /^run=/) {
			split($i, a, "=")
			run = a[2]
		} else if ($i ~ /^pass=/) {
			split($i, a, "=")
			pass = a[2]
		} else if ($i ~ /^fail=/) {
			split($i, a, "=")
			fail = a[2]
		} else if ($i ~ /^skip=/) {
			split($i, a, "=")
			skip = a[2]
		}
	}
	if (run != "" && pass != "" && fail != "" && skip != "") {
		printf "%s %s %s %s\n", run, pass, fail, skip
	}
}
' "$SUMMARY_LOG")"

if [[ -n "$TOTALS_LINE" ]]; then
	read -r TOTAL_TESTS PASSED_TESTS FAILED_TESTS SKIPPED_TESTS <<<"$TOTALS_LINE"
	printf "\n%s=== Final Test Totals ===%s\n" "$C_BOLD" "$C_RESET"
	printf "  Total tests: %s%s%s\n" "$C_CYAN" "$TOTAL_TESTS" "$C_RESET"
	printf "  Passed:      %s%s%s\n" "$C_GREEN" "$PASSED_TESTS" "$C_RESET"
	printf "  Failed:      %s%s%s\n" "$C_RED" "$FAILED_TESTS" "$C_RESET"
	printf "  Skipped:     %s%s%s\n" "$C_YELLOW" "$SKIPPED_TESTS" "$C_RESET"
else
	TOTAL_TESTS=0
	PASSED_TESTS=0
	FAILED_TESTS=0
	SKIPPED_TESTS=0
fi

PACKAGES_LINE="$(awk '
/^Packages:/ {
	total = ""
	pass = ""
	fail = ""
	other = ""
	for (i = 1; i <= NF; i++) {
		if ($i ~ /^total=/) {
			split($i, a, "=")
			total = a[2]
		} else if ($i ~ /^pass=/) {
			split($i, a, "=")
			pass = a[2]
		} else if ($i ~ /^fail=/) {
			split($i, a, "=")
			fail = a[2]
		} else if ($i ~ /^other=/) {
			split($i, a, "=")
			other = a[2]
		}
	}
	if (total != "" && pass != "" && fail != "" && other != "") {
		printf "%s %s %s %s\n", total, pass, fail, other
	}
}
' "$SUMMARY_LOG")"

if [[ -n "$PACKAGES_LINE" ]]; then
	read -r TOTAL_PACKAGES PASSED_PACKAGES FAILED_PACKAGES OTHER_PACKAGES <<<"$PACKAGES_LINE"
else
	TOTAL_PACKAGES=0
	PASSED_PACKAGES=0
	FAILED_PACKAGES=0
	OTHER_PACKAGES=0
fi

COVERAGE_TOTAL="$(awk '
/^[[:space:]]*Total:/ {
	gsub("%", "", $2)
	print $2
	exit
}
' "$SUMMARY_LOG")"

if [[ -z "$COVERAGE_TOTAL" ]]; then
	COVERAGE_TOTAL="N/A"
fi

PASS_RATE="$(awk -v pass="$PASSED_TESTS" -v total="$TOTAL_TESTS" 'BEGIN { if (total > 0) printf "%.2f", (pass * 100.0) / total; else print "0.00" }')"
FAIL_RATE="$(awk -v fail="$FAILED_TESTS" -v total="$TOTAL_TESTS" 'BEGIN { if (total > 0) printf "%.2f", (fail * 100.0) / total; else print "0.00" }')"

END_EPOCH="$(date +%s)"
END_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
RUNTIME_SEC=$((END_EPOCH - START_EPOCH))
GO_VERSION="$(go version 2>/dev/null || echo unknown)"
GO_OS="$(go env GOOS 2>/dev/null || echo unknown)"
GO_ARCH="$(go env GOARCH 2>/dev/null || echo unknown)"
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
GIT_DIRTY="clean"
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
	GIT_DIRTY="dirty"
fi

{
	echo ""
	echo "=== Final Test Totals ==="
	echo "  Total tests: $TOTAL_TESTS"
	echo "  Passed:      $PASSED_TESTS"
	echo "  Failed:      $FAILED_TESTS"
	echo "  Skipped:     $SKIPPED_TESTS"
	echo "  Pass rate:   ${PASS_RATE}%"
	echo "  Fail rate:   ${FAIL_RATE}%"
	echo ""
	echo "=== Run Metadata ==="
	echo "  Run ID: $RUN_ID"
	echo "  Started (UTC): $START_TS"
	echo "  Finished (UTC): $END_TS"
	echo "  Runtime (sec): $RUNTIME_SEC"
	echo "  Working dir: $ROOT_DIR"
	echo "  Git branch: $GIT_BRANCH"
	echo "  Git commit: $GIT_COMMIT"
	echo "  Git status: $GIT_DIRTY"
	echo "  Go version: $GO_VERSION"
	echo "  GOOS/GOARCH: $GO_OS/$GO_ARCH"
	echo "  Package totals: total=$TOTAL_PACKAGES pass=$PASSED_PACKAGES fail=$FAILED_PACKAGES other=$OTHER_PACKAGES"
	echo "  Coverage total: ${COVERAGE_TOTAL}%"
	echo "  Exit codes: json_test=$TEST_EXIT coverage=$COVER_EXIT stats=$STATS_EXIT"
} >> "$SUMMARY_LOG"

printf "\n%s=== Extra Run Metadata ===%s\n" "$C_BOLD$C_CYAN" "$C_RESET"
printf "  %sPass rate:%s %s%%\n" "$C_CYAN" "$C_RESET" "$PASS_RATE"
printf "  %sFail rate:%s %s%%\n" "$C_CYAN" "$C_RESET" "$FAIL_RATE"
printf "  %sCoverage total:%s %s%%\n" "$C_CYAN" "$C_RESET" "$COVERAGE_TOTAL"
printf "  %sRun time:%s %ss\n" "$C_CYAN" "$C_RESET" "$RUNTIME_SEC"
printf "  %sGit:%s %s@%s (%s)\n" "$C_CYAN" "$C_RESET" "$GIT_BRANCH" "$GIT_COMMIT" "$GIT_DIRTY"
printf "  %sGo:%s %s\n" "$C_CYAN" "$C_RESET" "$GO_VERSION"

if [[ $TEST_EXIT -ne 0 || $COVER_EXIT -ne 0 || $STATS_EXIT -ne 0 ]]; then
	printf "\n%sOne or more test phases failed.%s\n" "$C_RED" "$C_RESET" >&2
	exit 1
fi

printf "\n%sAll test phases completed successfully.%s\n" "$C_GREEN" "$C_RESET"
