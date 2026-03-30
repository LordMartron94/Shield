package internal

import (
	"fmt"
	"foundation/formatting"
	"os"
	"strings"
	"time"
)

type shieldRunSummaryEmitter func(report ShieldRunReport, metrics ShieldRunMetrics)
type shieldRunReportWriter func(report ShieldRunReport, metrics ShieldRunMetrics) string

type shieldRunReporter struct {
	OnRunStart         func(totalUnits int, verbosity ShieldRunVerbosity)
	OnRunEnd           func(report ShieldRunReport, metrics ShieldRunMetrics)
	OnUnitStart        func(unit ShieldUnit, depth int)
	OnUnitEnd          func(unit ShieldUnit, depth int, duration time.Duration, report ShieldUnitRunReport, failed bool)
	OnAtomStart        func(unitName string, atom ShieldAtom[any, any], caseCount int)
	OnAtomEnd          func(unitName, atomName string, duration time.Duration, failed bool)
	OnCaseStart        func(unitName, atomName string, shieldCase ShieldCase[any, any])
	OnCaseEnd          func(unitName, atomName string, shieldCase ShieldCase[any, any], failed bool, skipped bool, elapsed time.Duration)
	OnValidationFailed func(unitName, atomName, caseName string, result ShieldAtomResult, verbosity ShieldRunVerbosity)
	OnCasePanic        func(unitName, atomName, caseName, stage, failureMessage string, verbosity ShieldRunVerbosity)
	OnStageFailure     func(kind, name, stage, failureMessage string)
	OnSkip             func(kind, name, reason string)
	OnPolicy           func(message string)
}

type shieldRunIO struct {
	reporter       shieldRunReporter
	summaryEmitter shieldRunSummaryEmitter
	reportWriter   shieldRunReportWriter
}

func shieldRunIOCreateDefault(persist *ShieldReportPersistence) shieldRunIO {
	return shieldRunIO{
		reporter:       shieldRunReporterCreateGTestConsole(os.Stdout, os.Stderr),
		summaryEmitter: func(ShieldRunReport, ShieldRunMetrics) {},
		reportWriter: func(report ShieldRunReport, metrics ShieldRunMetrics) string {
			return shieldRunReportWrite(persist, report, metrics)
		},
	}
}

func shieldRunReporterCreateNoop() shieldRunReporter {
	return shieldRunReporter{
		OnRunStart:         func(int, ShieldRunVerbosity) {},
		OnRunEnd:           func(ShieldRunReport, ShieldRunMetrics) {},
		OnUnitStart:        func(ShieldUnit, int) {},
		OnUnitEnd:          func(ShieldUnit, int, time.Duration, ShieldUnitRunReport, bool) {},
		OnAtomStart:        func(string, ShieldAtom[any, any], int) {},
		OnAtomEnd:          func(string, string, time.Duration, bool) {},
		OnCaseStart:        func(string, string, ShieldCase[any, any]) {},
		OnCaseEnd:          func(string, string, ShieldCase[any, any], bool, bool, time.Duration) {},
		OnValidationFailed: func(string, string, string, ShieldAtomResult, ShieldRunVerbosity) {},
		OnCasePanic:        func(string, string, string, string, string, ShieldRunVerbosity) {},
		OnStageFailure:     func(string, string, string, string) {},
		OnSkip:             func(string, string, string) {},
		OnPolicy:           func(string) {},
	}
}

func shieldRunReporterCreateGTestConsole(out *os.File, errOut *os.File) shieldRunReporter {
	reporter := shieldRunReporterCreateNoop()
	verbosity := ShieldRunVerbosityNormal
	failedCases := make([]string, 0)
	atomDurations := make([]time.Duration, 0)

	reporter.OnRunStart = func(totalUnits int, runVerbosity ShieldRunVerbosity) {
		verbosity = runVerbosity
		failedCases = failedCases[:0]
		atomDurations = atomDurations[:0]
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "==========", fmt.Sprintf("Running %d shield units.", totalUnits))
	}

	reporter.OnRunEnd = func(report ShieldRunReport, metrics ShieldRunMetrics) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "==========", fmt.Sprintf("Shield run complete (%s total).", formatting.FormatDurationNSF64(float64(report.Elapsed.Nanoseconds()))))

		if metrics.Aggregates.UnitsFailed > 0 || metrics.Aggregates.AtomsFailed > 0 || metrics.Aggregates.CasesFailed > 0 {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", fmt.Sprintf("%d units, %d atoms, %d cases failed.", metrics.Aggregates.UnitsFailed, metrics.Aggregates.AtomsFailed, metrics.Aggregates.CasesFailed))
		}
		if metrics.Aggregates.UnitsPassed > 0 || metrics.Aggregates.AtomsPassed > 0 || metrics.Aggregates.CasesPassed > 0 {
			shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "  PASSED  ", fmt.Sprintf("%d units, %d atoms, %d cases.", metrics.Aggregates.UnitsPassed, metrics.Aggregates.AtomsPassed, metrics.Aggregates.CasesPassed))
		}
		if metrics.Aggregates.AtomsSkipped > 0 || metrics.Aggregates.CasesSkipped > 0 {
			shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, "  SKIPPED ", fmt.Sprintf("%d atoms, %d cases.", metrics.Aggregates.AtomsSkipped, metrics.Aggregates.CasesSkipped))
		}

		if len(failedCases) > 0 {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", fmt.Sprintf("%d cases, listed below:", len(failedCases)))
			for _, casePath := range failedCases {
				shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", casePath)
			}
		}

		if report.Failed {
			return
		}
	}

	reporter.OnUnitStart = func(unit ShieldUnit, depth int) {
		indent := strings.Repeat("  ", depth)
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "----------", fmt.Sprintf("%sUnit: %s", indent, shieldUnitFormatLabel(unit)))
	}

	reporter.OnUnitEnd = func(unit ShieldUnit, depth int, duration time.Duration, _ ShieldUnitRunReport, failed bool) {
		status := "PASSED"
		color := shieldConsoleColorGreen
		if failed {
			status = "FAILED"
			color = shieldConsoleColorRed
		}
		indent := strings.Repeat("  ", depth)
		shieldConsoleWriteTagLine(out, color, "----------", fmt.Sprintf("%sUnit: %s (%s total) [%s]", indent, unit.name, formatting.FormatDurationNSF64(float64(duration.Nanoseconds())), status))
	}

	reporter.OnAtomStart = func(unitName string, atom ShieldAtom[any, any], caseCount int) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, " RUN      ", fmt.Sprintf("%s.%s (%d cases) %s", unitName, atom.name, caseCount, shieldAtomFormatLabel(atom)))
	}

	reporter.OnAtomEnd = func(unitName, atomName string, duration time.Duration, failed bool) {
		message := fmt.Sprintf("%s.%s (%s)", unitName, atomName, formatting.FormatDurationNSF64(float64(duration.Nanoseconds())))
		if shieldConsoleDurationIsSlow(duration, atomDurations) {
			message += " [SLOW]"
		}
		atomDurations = append(atomDurations, duration)
		if failed {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", message)
			return
		}
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "       OK ", message)
	}

	reporter.OnCaseStart = func(unitName, atomName string, shieldCase ShieldCase[any, any]) {
		if verbosity == ShieldRunVerbosityQuiet {
			return
		}
		shieldConsoleWriteTagLine(out, shieldConsoleColorCyan, " CASE     ", fmt.Sprintf("%s.%s.%s %s", unitName, atomName, shieldCase.name, shieldCaseFormatLabel(shieldCase)))
	}

	reporter.OnCaseEnd = func(unitName, atomName string, shieldCase ShieldCase[any, any], failed bool, skipped bool, elapsed time.Duration) {
		if skipped {
			shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, "  SKIPPED ", fmt.Sprintf("%s.%s.%s", unitName, atomName, shieldCase.name))
			return
		}
		if verbosity == ShieldRunVerbosityQuiet && !failed {
			return
		}
		if failed {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", fmt.Sprintf("%s.%s.%s (%s)", unitName, atomName, shieldCase.name, formatting.FormatDurationNSF64(float64(elapsed.Nanoseconds()))))
			failedCases = append(failedCases, fmt.Sprintf("%s.%s.%s", unitName, atomName, shieldCase.name))
			return
		}
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "       OK ", fmt.Sprintf("%s.%s.%s (%s)", unitName, atomName, shieldCase.name, formatting.FormatDurationNSF64(float64(elapsed.Nanoseconds()))))
	}

	reporter.OnValidationFailed = func(unitName, atomName, caseName string, result ShieldAtomResult, _ ShieldRunVerbosity) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  ASSERT  ", fmt.Sprintf("%s.%s.%s failed: %s", unitName, atomName, caseName, result.failureReason))
		if result.note != nil {
			shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, "  NOTE    ", *result.note)
		}
	}

	reporter.OnCasePanic = func(unitName, atomName, caseName, stage, failureMessage string, _ ShieldRunVerbosity) {
		shieldConsoleWriteTagLine(errOut, shieldConsoleColorRed, "  PANIC   ", fmt.Sprintf("%s.%s.%s (%s)", unitName, atomName, caseName, stage))
		fmt.Fprintln(errOut, failureMessage)
	}

	reporter.OnStageFailure = func(kind, name, stage, failureMessage string) {
		shieldConsoleWriteTagLine(errOut, shieldConsoleColorRed, "  STAGE   ", fmt.Sprintf("%s %s (%s) failed", kind, name, stage))
		fmt.Fprintln(errOut, failureMessage)
	}

	reporter.OnSkip = func(kind, name, reason string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, "  SKIP    ", fmt.Sprintf("%s %s (%s)", kind, name, reason))
	}

	reporter.OnPolicy = func(message string) {
		shieldConsoleWriteTagLine(errOut, shieldConsoleColorYellow, "  POLICY  ", message)
	}

	return reporter
}

const (
	shieldConsoleColorReset  = "\033[0m"
	shieldConsoleColorRed    = "\033[31m"
	shieldConsoleColorGreen  = "\033[32m"
	shieldConsoleColorYellow = "\033[33m"
	shieldConsoleColorCyan   = "\033[36m"
)

func shieldConsoleWriteTagLine(out *os.File, color, tag, message string) {
	fmt.Fprintf(out, "%s[%s]%s %s\n", color, tag, shieldConsoleColorReset, message)
}

func shieldConsoleDurationIsSlow(duration time.Duration, history []time.Duration) bool {
	if len(history) < 5 {
		return false
	}

	max := history[0]
	for _, d := range history[1:] {
		if d > max {
			max = d
		}
	}

	return duration > max
}

func shieldAtomResultFormat[TInput, TOutput any](atom ShieldAtom[TInput, TOutput], result ShieldAtomResult) string {
	if result.note != nil {
		return fmt.Sprintf("atom '%s' failed with reason: %s\n\n\tNote: %s", atom.name, result.failureReason, *result.note)
	}

	return fmt.Sprintf("atom '%s' failed with reason: %s", atom.name, result.failureReason)
}

func shieldUnitFormatLabel(unit ShieldUnit) string {
	if unit.description != nil {
		return fmt.Sprintf("'%s': %s", unit.name, *unit.description)
	}

	return fmt.Sprintf("'%s'", unit.name)
}

func shieldAtomFormatLabel[TInput, TOutput any](atom ShieldAtom[TInput, TOutput]) string {
	if atom.description != nil {
		return fmt.Sprintf("'%s': %s", atom.name, *atom.description)
	}

	return fmt.Sprintf("'%s'", atom.name)
}

func shieldCaseFormatLabel[TInput, TOutput any](shieldCase ShieldCase[TInput, TOutput]) string {
	if shieldCase.description != nil {
		return fmt.Sprintf("'%s': %s", shieldCase.name, *shieldCase.description)
	}

	return fmt.Sprintf("'%s'", shieldCase.name)
}
