package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"shield"
	"strconv"
	"strings"
)

func shieldCliCompareCommandRun(args []string, state *shieldCliState) {
	if len(args) > 0 && args[0] == "choose" {
		shieldCliCompareChooseCommandRun(args[1:], state)
		return
	}

	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var slowPct float64
	var slowAbsMs int64
	var topN int
	var failOnRegression bool

	fs.Float64Var(&slowPct, "slow-pct", 10.0, "minimum slowdown percentage to count as regression")
	fs.Int64Var(&slowAbsMs, "slow-abs-ms", 1, "minimum absolute slowdown in milliseconds to count as regression")
	fs.IntVar(&topN, "top", 10, "maximum number of slow regressions to show")
	fs.BoolVar(&failOnRegression, "fail-on-regression", false, "exit process with status 1 when regression is detected")

	if err := fs.Parse(args); err != nil {
		return
	}

	positionals := fs.Args()
	baselinePath, candidatePath, err := shieldCliComparePathsResolve(state.resultsDir, positionals)
	if err != nil {
		fmt.Println(shieldCliColorBold("compare error: "+err.Error(), ansiRed))
		return
	}

	options := shield.CompareOptionsDefault()
	options.SlowPctThreshold = slowPct
	options.SlowAbsThresholdNs = slowAbsMs * int64(1e6)
	options.TopSlowRegressionsN = topN

	result, err := shield.CompareReportsFromPaths(baselinePath, candidatePath, options)
	if err != nil {
		fmt.Println(shieldCliColorBold("compare error: "+err.Error(), ansiRed))
		return
	}

	fmt.Println(shieldCliColor("baseline:", ansiCyan), baselinePath)
	fmt.Println(shieldCliColor("candidate:", ansiCyan), candidatePath)
	fmt.Println()
	shieldCliComparePrintColorized(result)

	if failOnRegression && shield.CompareResultHasRegression(result) {
		os.Exit(1)
	}
}

func shieldCliComparePathsResolve(resultsDir string, args []string) (baselinePath string, candidatePath string, err error) {
	switch len(args) {
	case 0:
		pinned, pinnedErr := shieldCliBaselineRead(resultsDir)
		if pinnedErr != nil {
			return "", "", pinnedErr
		}
		if strings.TrimSpace(pinned) == "" {
			return "", "", fmt.Errorf("no pinned baseline; use baseline set <latest|index|file> or pass baseline/candidate explicitly")
		}
		baselinePath = shieldCliPathResolveMaybeRelative(resultsDir, pinned)
		candidate, candidateErr := shieldCliResultFileSelect(resultsDir, "latest")
		if candidateErr != nil {
			return "", "", candidateErr
		}
		candidatePath = candidate.Path

	case 1:
		if args[0] != "latest" {
			return "", "", fmt.Errorf("single-arg compare only supports 'latest' (uses pinned baseline)")
		}
		pinned, pinnedErr := shieldCliBaselineRead(resultsDir)
		if pinnedErr != nil {
			return "", "", pinnedErr
		}
		if strings.TrimSpace(pinned) == "" {
			return "", "", fmt.Errorf("no pinned baseline; use baseline set <latest|index|file>")
		}
		baselinePath = shieldCliPathResolveMaybeRelative(resultsDir, pinned)
		candidate, candidateErr := shieldCliResultFileSelect(resultsDir, "latest")
		if candidateErr != nil {
			return "", "", candidateErr
		}
		candidatePath = candidate.Path

	default:
		if len(args) > 2 {
			return "", "", fmt.Errorf("usage: compare [baseline] [candidate] [flags]")
		}
		baseline, baselineErr := shieldCliResultFileSelect(resultsDir, args[0])
		if baselineErr != nil {
			return "", "", baselineErr
		}
		candidate, candidateErr := shieldCliResultFileSelect(resultsDir, args[1])
		if candidateErr != nil {
			return "", "", candidateErr
		}
		baselinePath = baseline.Path
		candidatePath = candidate.Path
	}

	if filepath.Ext(baselinePath) != ".json" {
		return "", "", fmt.Errorf("baseline must point to JSON report: %s", baselinePath)
	}
	if filepath.Ext(candidatePath) != ".json" {
		return "", "", fmt.Errorf("candidate must point to JSON report: %s", candidatePath)
	}
	if baselinePath == candidatePath {
		return "", "", fmt.Errorf("baseline and candidate are identical: %s", baselinePath)
	}

	return baselinePath, candidatePath, nil
}

func shieldCliPathResolveMaybeRelative(resultsDir string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(resultsDir, path))
}

func shieldCliCompareChooseCommandRun(args []string, state *shieldCliState) {
	fs := flag.NewFlagSet("compare choose", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var slowPct float64
	var slowAbsMs int64
	var topN int
	var failOnRegression bool

	fs.Float64Var(&slowPct, "slow-pct", 10.0, "minimum slowdown percentage to count as regression")
	fs.Int64Var(&slowAbsMs, "slow-abs-ms", 1, "minimum absolute slowdown in milliseconds to count as regression")
	fs.IntVar(&topN, "top", 10, "maximum number of slow regressions to show")
	fs.BoolVar(&failOnRegression, "fail-on-regression", false, "exit process with status 1 when regression is detected")

	if err := fs.Parse(args); err != nil {
		return
	}

	results, err := shieldCliResultFilesList(state.resultsDir)
	if err != nil {
		fmt.Println(shieldCliColorBold("compare choose error: "+err.Error(), ansiRed))
		return
	}

	jsonResults := make([]shieldCliResultFileMeta, 0)
	for _, result := range results {
		if filepath.Ext(result.Path) == ".json" {
			jsonResults = append(jsonResults, result)
		}
	}

	if len(jsonResults) < 2 {
		fmt.Println(shieldCliColorBold("compare choose error: need at least 2 JSON results to choose from", ansiRed))
		return
	}

	fmt.Println(shieldCliColorBold("available JSON results:", ansiCyan))
	for idx, result := range jsonResults {
		status := "unknown"
		if result.RunFailed != nil {
			if *result.RunFailed {
				status = "failed"
			} else {
				status = "success"
			}
		}
		fmt.Printf("  %d) %s  status=%s  written=%s\n", idx+1, result.Name, status, result.WrittenAt)
	}

	reader := bufio.NewReader(os.Stdin)
	baselineIndex, readErr := shieldCliCompareChooseReadIndex(reader, "baseline", len(jsonResults))
	if readErr != nil {
		fmt.Println(shieldCliColorBold("compare choose error: "+readErr.Error(), ansiRed))
		return
	}

	candidateIndex, readErr := shieldCliCompareChooseReadIndex(reader, "candidate", len(jsonResults))
	if readErr != nil {
		fmt.Println(shieldCliColorBold("compare choose error: "+readErr.Error(), ansiRed))
		return
	}

	if baselineIndex == candidateIndex {
		fmt.Println(shieldCliColorBold("compare choose error: baseline and candidate cannot be the same result", ansiRed))
		return
	}

	baselinePath := jsonResults[baselineIndex-1].Path
	candidatePath := jsonResults[candidateIndex-1].Path

	options := shield.CompareOptionsDefault()
	options.SlowPctThreshold = slowPct
	options.SlowAbsThresholdNs = slowAbsMs * int64(1e6)
	options.TopSlowRegressionsN = topN

	result, compareErr := shield.CompareReportsFromPaths(baselinePath, candidatePath, options)
	if compareErr != nil {
		fmt.Println(shieldCliColorBold("compare choose error: "+compareErr.Error(), ansiRed))
		return
	}

	fmt.Println(shieldCliColor("baseline:", ansiCyan), baselinePath)
	fmt.Println(shieldCliColor("candidate:", ansiCyan), candidatePath)
	fmt.Println()
	shieldCliComparePrintColorized(result)

	if failOnRegression && shield.CompareResultHasRegression(result) {
		os.Exit(1)
	}
}

func shieldCliCompareChooseReadIndex(reader *bufio.Reader, label string, max int) (int, error) {
	fmt.Printf("%s index [1-%d]: ", label, max)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}

	raw = strings.TrimSpace(raw)
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 1 || idx > max {
		return 0, fmt.Errorf("invalid %s index", label)
	}

	return idx, nil
}

func shieldCliComparePrintColorized(result shield.CompareResult) {
	text := shield.CompareResultFormatTXT(result)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Println()
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "Shield compare report"):
			fmt.Println(shieldCliColorBold(line, ansiCyan))
		case strings.HasPrefix(trimmed, "Verdict:"):
			fmt.Println(shieldCliColorBold(line, shieldCliCompareVerdictColor(result)))
		case strings.HasPrefix(trimmed, "Warnings"):
			fmt.Println(shieldCliColorBold(line, ansiYellow))
		case strings.HasPrefix(trimmed, "New failing cases"):
			fmt.Println(shieldCliColorBold(line, ansiRed))
		case strings.HasPrefix(trimmed, "Structural changes"):
			fmt.Println(shieldCliColorBold(line, ansiMagenta))
		case strings.HasPrefix(trimmed, "Timing"):
			fmt.Println(shieldCliColorBold(line, ansiBlue))
		case strings.Contains(trimmed, "Slow atom regressions"), strings.Contains(trimmed, "Slow case regressions"):
			fmt.Println(shieldCliColor(line, ansiYellow))
		case strings.HasPrefix(trimmed, "-"), strings.HasPrefix(trimmed, "  -"):
			if strings.Contains(trimmed, "delta=") {
				fmt.Println(shieldCliColor(line, ansiYellow))
			} else if strings.Contains(trimmed, "failed") || strings.Contains(trimmed, "Failing") {
				fmt.Println(shieldCliColor(line, ansiRed))
			} else {
				fmt.Println(shieldCliColor(line, ansiGray))
			}
		default:
			fmt.Println(line)
		}
	}
}

func shieldCliCompareVerdictColor(result shield.CompareResult) string {
	switch result.Verdict {
	case shield.CompareVerdictRegression:
		return ansiRed
	case shield.CompareVerdictMixed:
		return ansiYellow
	default:
		return ansiGreen
	}
}
