package internal

import (
	"fmt"
	"os"
	"time"
)

type shieldRunSummaryEmitter func(report ShieldRunReport, metrics ShieldRunMetrics)
type shieldRunReportWriter func(report ShieldRunReport, metrics ShieldRunMetrics) string

type shieldRunReporter struct {
	OnRunStart         func(totalUnits int)
	OnRunEnd           func(report ShieldRunReport, metrics ShieldRunMetrics)
	OnUnitStart        func(unitName string)
	OnUnitEnd          func(unitName string, duration time.Duration, report ShieldUnitRunReport, failed bool)
	OnAtomStart        func(unitName, atomName string)
	OnAtomEnd          func(unitName, atomName string, duration time.Duration, failed bool)
	OnCaseStart        func(unitName, atomName, caseName string)
	OnValidationFailed func(unitName, atomName, caseName string, result ShieldAtomResult)
	OnCasePanic        func(unitName, atomName, caseName, stage, failureMessage string)
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
		reporter:       shieldRunReporterCreateGTestConsole(os.Stdout),
		summaryEmitter: func(ShieldRunReport, ShieldRunMetrics) {},
		reportWriter: func(report ShieldRunReport, metrics ShieldRunMetrics) string {
			return shieldRunReportWrite(persist, report, metrics)
		},
	}
}

func shieldRunReporterCreateNoop() shieldRunReporter {
	return shieldRunReporter{
		OnRunStart:         func(int) {},
		OnRunEnd:           func(ShieldRunReport, ShieldRunMetrics) {},
		OnUnitStart:        func(string) {},
		OnUnitEnd:          func(string, time.Duration, ShieldUnitRunReport, bool) {},
		OnAtomStart:        func(string, string) {},
		OnAtomEnd:          func(string, string, time.Duration, bool) {},
		OnCaseStart:        func(string, string, string) {},
		OnValidationFailed: func(string, string, string, ShieldAtomResult) {},
		OnCasePanic:        func(string, string, string, string, string) {},
		OnStageFailure:     func(string, string, string, string) {},
		OnSkip:             func(string, string, string) {},
		OnPolicy:           func(string) {},
	}
}

func shieldRunReporterCreateGTestConsole(out *os.File) shieldRunReporter {
	reporter := shieldRunReporterCreateNoop()

	reporter.OnRunStart = func(totalUnits int) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "==========", fmt.Sprintf("Running %d shield units.", totalUnits))
	}

	reporter.OnRunEnd = func(report ShieldRunReport, metrics ShieldRunMetrics) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "==========", fmt.Sprintf("Shield run complete (%d ms total).", report.Elapsed.Milliseconds()))
		if report.Failed {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", fmt.Sprintf("%d units failed.", metrics.Aggregates.UnitsFailedGate))
			return
		}
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "  PASSED  ", "All units passed.")
	}

	reporter.OnUnitStart = func(unitName string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "----------", fmt.Sprintf("Unit: %s", unitName))
	}

	reporter.OnUnitEnd = func(unitName string, duration time.Duration, _ ShieldUnitRunReport, failed bool) {
		status := "PASSED"
		color := shieldConsoleColorGreen
		if failed {
			status = "FAILED"
			color = shieldConsoleColorRed
		}
		shieldConsoleWriteTagLine(out, color, "----------", fmt.Sprintf("Unit: %s (%d ms total) [%s]", unitName, duration.Milliseconds(), status))
	}

	reporter.OnAtomStart = func(unitName, atomName string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, " RUN      ", fmt.Sprintf("%s.%s", unitName, atomName))
	}

	reporter.OnAtomEnd = func(unitName, atomName string, duration time.Duration, failed bool) {
		message := fmt.Sprintf("%s.%s (%d ms)", unitName, atomName, duration.Milliseconds())
		if failed {
			shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  FAILED  ", message)
			return
		}
		shieldConsoleWriteTagLine(out, shieldConsoleColorGreen, "       OK ", message)
	}

	reporter.OnCaseStart = func(unitName, atomName, caseName string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorCyan, " CASE     ", fmt.Sprintf("%s.%s.%s", unitName, atomName, caseName))
	}

	reporter.OnValidationFailed = func(unitName, atomName, caseName string, result ShieldAtomResult) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  ASSERT  ", fmt.Sprintf("%s.%s.%s failed: %s", unitName, atomName, caseName, result.failureReason))
		if result.note != nil {
			fmt.Fprintf(out, "\tNote: %s\n", *result.note)
		}
	}

	reporter.OnCasePanic = func(unitName, atomName, caseName, stage, failureMessage string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  PANIC   ", fmt.Sprintf("%s.%s.%s (%s)", unitName, atomName, caseName, stage))
		fmt.Fprintln(out, failureMessage)
	}

	reporter.OnStageFailure = func(kind, name, stage, failureMessage string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorRed, "  STAGE   ", fmt.Sprintf("%s %s (%s) failed", kind, name, stage))
		fmt.Fprintln(out, failureMessage)
	}

	reporter.OnSkip = func(kind, name, reason string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, "  SKIP    ", fmt.Sprintf("%s %s (%s)", kind, name, reason))
	}

	reporter.OnPolicy = func(message string) {
		shieldConsoleWriteTagLine(out, shieldConsoleColorYellow, " POLICY   ", message)
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
