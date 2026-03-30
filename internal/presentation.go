package internal

import (
	"echo"
	"fmt"
	"foundation/formatting"
	"time"
)

type shieldRunEventLogger interface {
	LogTemplate(action, label string)
	LogDuration(description string, duration time.Duration)
	LogFailure(failureMessage string)
	LogStageFailure(label, stage, failureMessage string)
	LogLifecycle(stage, label string)
	LogPolicy(message string)
}

type shieldRunSummaryEmitter func(report ShieldRunReport, metrics ShieldRunMetrics)
type shieldRunReportWriter func(report ShieldRunReport, metrics ShieldRunMetrics) string

type shieldRunIO struct {
	logger         shieldRunEventLogger
	summaryEmitter shieldRunSummaryEmitter
	reportWriter   shieldRunReportWriter
}

type shieldEchoRunEventLogger struct{}

func shieldRunIOCreateDefault(persist *ShieldReportPersistence) shieldRunIO {
	return shieldRunIO{
		logger:         shieldEchoRunEventLogger{},
		summaryEmitter: shieldRunSummaryEmit,
		reportWriter: func(report ShieldRunReport, metrics ShieldRunMetrics) string {
			return shieldRunReportWrite(persist, report, metrics)
		},
	}
}

func (shieldEchoRunEventLogger) LogTemplate(action, label string) {
	echo.On(shieldSystemID).Info(fmt.Sprintf("%s %s", action, label))
}

func (shieldEchoRunEventLogger) LogDuration(description string, duration time.Duration) {
	nano := float64(duration.Nanoseconds())
	echo.On(shieldSystemID).Field("duration", formatting.FormatDurationNSF64(nano)).Info(fmt.Sprintf("finished %s", description))
}

func (shieldEchoRunEventLogger) LogFailure(failureMessage string) {
	echo.On(shieldSystemID).Warning(failureMessage)
}

func (shieldEchoRunEventLogger) LogStageFailure(label, stage, failureMessage string) {
	echo.On(shieldSystemID).Error(fmt.Sprintf("error executing %s for %s: %s", stage, label, failureMessage))
}

func (shieldEchoRunEventLogger) LogLifecycle(stage, label string) {
	echo.On(shieldSystemID).Trace(fmt.Sprintf("executing stage %s for %s", stage, label))
}

func (shieldEchoRunEventLogger) LogPolicy(message string) {
	echo.On(shieldSystemID).Notice(message)
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
