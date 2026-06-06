package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type goTestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

type packageStatus struct {
	Status  string
	Elapsed float64
}

type testStatus struct {
	Package string
	Name    string
	Status  string
	Elapsed float64
}

type fileCoverage struct {
	File      string
	Functions float64
	SumPct    float64
}

func parseJSONLog(path string) (
	int,
	int,
	int,
	int,
	int,
	int,
	int,
	int,
	time.Duration,
	[]testStatus,
	error,
) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, 0, nil, err
	}
	defer file.Close()

	packageResults := make(map[string]packageStatus)
	packagesSeen := make(map[string]bool)
	testResults := make(map[string]testStatus)
	testRuns := make(map[string]bool)

	var firstEvent time.Time
	var lastEvent time.Time

	totalRuns := 0
	totalPass := 0
	totalFail := 0
	totalSkip := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var event goTestEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if event.Package != "" {
			packagesSeen[event.Package] = true
		}

		if !event.Time.IsZero() {
			if firstEvent.IsZero() || event.Time.Before(firstEvent) {
				firstEvent = event.Time
			}
			if event.Time.After(lastEvent) {
				lastEvent = event.Time
			}
		}

		if event.Test == "" {
			if event.Action == "pass" || event.Action == "fail" {
				packageResults[event.Package] = packageStatus{Status: event.Action, Elapsed: event.Elapsed}
			}
			continue
		}

		key := event.Package + "::" + event.Test
		if event.Action == "run" {
			if !testRuns[key] {
				totalRuns++
				testRuns[key] = true
			}
			continue
		}

		if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
			testResults[key] = testStatus{
				Package: event.Package,
				Name:    event.Test,
				Status:  event.Action,
				Elapsed: event.Elapsed,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, 0, nil, err
	}

	for _, test := range testResults {
		switch test.Status {
		case "pass":
			totalPass++
		case "fail":
			totalFail++
		case "skip":
			totalSkip++
		}
	}

	packagesTotal := len(packagesSeen)
	packagesPass := 0
	packagesFail := 0
	packagesOther := 0
	for _, pkg := range packageResults {
		if pkg.Status == "pass" {
			packagesPass++
		} else if pkg.Status == "fail" {
			packagesFail++
		} else {
			packagesOther++
		}
	}
	if packagesTotal > packagesPass+packagesFail+packagesOther {
		packagesOther = packagesTotal - packagesPass - packagesFail
	}

	testList := make([]testStatus, 0, len(testResults))
	for _, test := range testResults {
		testList = append(testList, test)
	}
	sort.Slice(testList, func(i, j int) bool {
		return testList[i].Elapsed > testList[j].Elapsed
	})

	duration := time.Duration(0)
	if !firstEvent.IsZero() && !lastEvent.IsZero() {
		duration = lastEvent.Sub(firstEvent)
	}

	return totalRuns, totalPass, totalFail, totalSkip, packagesTotal, packagesPass, packagesFail, packagesOther, duration, testList, nil
}

func parseCoverage(path string) (float64, []fileCoverage, error) {
	if path == "" {
		return 0, nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	defer file.Close()

	coverageByFile := make(map[string]*fileCoverage)
	totalCoverage := 0.0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "total:") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				percentText := strings.TrimSuffix(parts[len(parts)-1], "%")
				if p, err := strconv.ParseFloat(percentText, 64); err == nil {
					totalCoverage = p
				}
			}
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		covText := strings.TrimSuffix(parts[len(parts)-1], "%")
		covPercent, err := strconv.ParseFloat(covText, 64)
		if err != nil {
			continue
		}

		fileName := name
		if idx := strings.Index(name, ":"); idx >= 0 {
			fileName = name[:idx]
		}

		entry, exists := coverageByFile[fileName]
		if !exists {
			entry = &fileCoverage{File: fileName}
			coverageByFile[fileName] = entry
		}
		entry.Functions++
		entry.SumPct += covPercent
	}
	if err := scanner.Err(); err != nil {
		return 0, nil, err
	}

	files := make([]fileCoverage, 0, len(coverageByFile))
	for _, entry := range coverageByFile {
		files = append(files, *entry)
	}
	sort.Slice(files, func(i, j int) bool {
		pi := 0.0
		if files[i].Functions > 0 {
			pi = files[i].SumPct / files[i].Functions
		}
		pj := 0.0
		if files[j].Functions > 0 {
			pj = files[j].SumPct / files[j].Functions
		}
		if pi == pj {
			return files[i].File < files[j].File
		}
		return pi < pj
	})

	return totalCoverage, files, nil
}

func main() {
	jsonPath := flag.String("json", "", "path to go test -json log file")
	coveragePath := flag.String("coverage", "", "path to go tool cover -func output")
	flag.Parse()

	if *jsonPath == "" {
		fmt.Fprintln(os.Stderr, "missing required --json path")
		os.Exit(2)
	}

	totalRuns, totalPass, totalFail, totalSkip, packagesTotal, packagesPass, packagesFail, packagesOther, duration, tests, err := parseJSONLog(*jsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed parsing json log: %v\n", err)
		os.Exit(1)
	}

	totalCoverage, files, err := parseCoverage(*coveragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed parsing coverage summary: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Aggregated Test Stats ===")
	fmt.Printf("Log file: %s\n", filepath.Clean(*jsonPath))
	fmt.Printf("Packages: total=%d pass=%d fail=%d other=%d\n", packagesTotal, packagesPass, packagesFail, packagesOther)
	fmt.Printf("Tests: run=%d pass=%d fail=%d skip=%d\n", totalRuns, totalPass, totalFail, totalSkip)
	if duration > 0 {
		fmt.Printf("Wall clock duration: %s\n", duration.Round(time.Millisecond))
	}

	if len(tests) > 0 {
		limit := 10
		if len(tests) < limit {
			limit = len(tests)
		}
		fmt.Printf("Top %d slowest tests:\n", limit)
		for i := 0; i < limit; i++ {
			t := tests[i]
			fmt.Printf("  %2d. %s/%s [%s] %.6fs\n", i+1, t.Package, t.Name, t.Status, t.Elapsed)
		}
	}

	if totalCoverage > 0 || len(files) > 0 {
		fmt.Println("Coverage summary:")
		if totalCoverage > 0 {
			fmt.Printf("  Total: %.2f%%\n", totalCoverage)
		}
		if len(files) > 0 {
			limit := 10
			if len(files) < limit {
				limit = len(files)
			}
			fmt.Printf("  Lowest %d file coverages (average by function):\n", limit)
			for i := 0; i < limit; i++ {
				f := files[i]
				percent := 0.0
				if f.Functions > 0 {
					percent = f.SumPct / f.Functions
				}
				fmt.Printf("    %2d. %s %.2f%% (%.0f funcs)\n", i+1, f.File, percent, f.Functions)
			}
		}
	}
}
