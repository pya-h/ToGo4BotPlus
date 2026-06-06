package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestParseJSONLogAggregatesTestsAndPackages(t *testing.T) {
	jsonLines := strings.Join([]string{
		`{"Time":"2026-06-06T10:00:00Z","Action":"run","Package":"ToGo4BotPlus","Test":"TestOne"}`,
		`{"Time":"2026-06-06T10:00:01Z","Action":"pass","Package":"ToGo4BotPlus","Test":"TestOne","Elapsed":0.01}`,
		`{"Time":"2026-06-06T10:00:02Z","Action":"run","Package":"ToGo4BotPlus","Test":"TestTwo"}`,
		`{"Time":"2026-06-06T10:00:03Z","Action":"fail","Package":"ToGo4BotPlus","Test":"TestTwo","Elapsed":0.02}`,
		`{"Time":"2026-06-06T10:00:04Z","Action":"pass","Package":"ToGo4BotPlus","Elapsed":0.05}`,
		`{"Time":"2026-06-06T10:00:05Z","Action":"output","Package":"ToGo4BotPlus/ex","Output":"?\tToGo4BotPlus/ex\t[no test files]"}`,
	}, "\n")

	path := writeTempFile(t, jsonLines)

	run, pass, fail, skip, pkgTotal, pkgPass, pkgFail, pkgOther, duration, tests, err := parseJSONLog(path)
	if err != nil {
		t.Fatalf("parseJSONLog returned error: %v", err)
	}

	if run != 2 || pass != 1 || fail != 1 || skip != 0 {
		t.Fatalf("unexpected test counters: run=%d pass=%d fail=%d skip=%d", run, pass, fail, skip)
	}
	if pkgTotal != 2 || pkgPass != 1 || pkgFail != 0 || pkgOther != 1 {
		t.Fatalf("unexpected package counters: total=%d pass=%d fail=%d other=%d", pkgTotal, pkgPass, pkgFail, pkgOther)
	}
	if duration <= 0 {
		t.Fatalf("expected positive duration, got %s", duration)
	}
	if len(tests) != 2 {
		t.Fatalf("expected 2 test entries, got %d", len(tests))
	}
	if tests[0].Elapsed < tests[1].Elapsed {
		t.Fatalf("expected tests sorted by elapsed descending, got %.6f then %.6f", tests[0].Elapsed, tests[1].Elapsed)
	}
}

func TestParseCoverageAggregatesByFile(t *testing.T) {
	coverage := strings.Join([]string{
		"ToGo4BotPlus/main.go:SendTextMessage\t0.0%",
		"ToGo4BotPlus/main.go:SplitArguments\t20.0%",
		"ToGo4BotPlus/Togo/Togo.go:Load\t50.0%",
		"total:\t\t\t\t\t(statements)\t27.3%",
	}, "\n")
	path := writeTempFile(t, coverage)

	total, files, err := parseCoverage(path)
	if err != nil {
		t.Fatalf("parseCoverage returned error: %v", err)
	}
	if total != 27.3 {
		t.Fatalf("expected total 27.3, got %.2f", total)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	if files[0].File != "ToGo4BotPlus/main.go" {
		t.Fatalf("expected lowest-coverage file first to be main.go, got %s", files[0].File)
	}
	avg := files[0].SumPct / files[0].Functions
	if avg < 9.9 || avg > 10.1 {
		t.Fatalf("expected main.go average around 10.0, got %.2f", avg)
	}
}

func TestParseCoverageMissingFileReturnsEmpty(t *testing.T) {
	total, files, err := parseCoverage("/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("expected no error for missing coverage file, got: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total coverage 0 for missing file, got %.2f", total)
	}
	if len(files) != 0 {
		t.Fatalf("expected no file coverage entries for missing file, got %d", len(files))
	}
}
