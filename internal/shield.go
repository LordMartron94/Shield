package internal

import (
	"cmp"
	"echo"
	"fmt"
	"foundation/extensions"
	"foundation/formatting"
	"runtime/debug"
	"slices"
	"time"
)

// ------------------------------------------------------------- RESULT

type ShieldAtomResult struct {
	success                bool
	failureReason          string
	skipFurtherAtomsInUnit bool
	note                   *string
}

func ShieldAtomResultSuccessCreate() *ShieldAtomResult {
	return &ShieldAtomResult{
		success:       true,
		failureReason: "",
		note:          nil,
	}
}

func ShieldAtomResultFailureCreate(reason string) *ShieldAtomResult {
	return &ShieldAtomResult{
		success:       false,
		failureReason: reason,
		note:          nil,
	}
}

func ShieldAtomResultSetNote(result *ShieldAtomResult, note string) {
	result.note = &note
}

func ShieldAtomResultSetSkipFurtherAtomsInUnit(result *ShieldAtomResult, value bool) {
	result.skipFurtherAtomsInUnit = value
}

func shieldAtomResultFormat[TInput, TOutput any](atom ShieldAtom[TInput, TOutput], result ShieldAtomResult) string {
	if result.note != nil {
		return fmt.Sprintf("atom '%s' failed with reason: %s\n\n\tNote: %s", atom.name, result.failureReason, *result.note)
	}

	return fmt.Sprintf("atom '%s' failed with reason: %s", atom.name, result.failureReason)
}

// ------------------------------------------------------------- EXECUTION & VALIDATION

type ShieldAtomRunner[TInput, TOutput any] func(in TInput) TOutput

type ShieldValidator[TInput, TOutput any] func(output TOutput, testCase ShieldCase[TInput, TOutput]) ShieldAtomResult

// ------------------------------------------------------------- CONFIGURATION

type ShieldConfiguration struct {
	units []ShieldUnit
}

func ShieldConfigurationCreate() *ShieldConfiguration {
	return &ShieldConfiguration{
		units: make([]ShieldUnit, 0),
	}
}

func ShieldConfigurationRegisterUnits(cfg *ShieldConfiguration, units ...ShieldUnit) {
	cfg.units = append(cfg.units, units...)
}

// ------------------------------------------------------------- UNIT RUN REPORT

type ShieldUnitChildOutcome struct {
	Name   string
	Failed bool
	Report ShieldUnitRunReport
}

type ShieldUnitRunReport struct {
	Name string

	SkippedDueToBlacklist bool
	SkippedDueToSetup     bool

	AtomSetupFailureCount  int
	AtomValidationFailures int
	AtomPanics             int

	TerminatedSubUnitLoopEarly bool

	DirectChildren []ShieldUnitChildOutcome
}

type ShieldUnitEvaluationGate func(report ShieldUnitRunReport) bool

// ------------------------------------------------------------- UNIT

type ShieldUnit struct {
	order       int
	name        string
	description *string

	atoms []ShieldAtom[any, any]

	subUnits []ShieldUnit

	setup    func()
	teardown func()

	stopRemainingSubUnitsOnChildFailure       bool
	skipOwnAtomsWhenChildFailureStopsSubUnits bool
	evaluationGate                            ShieldUnitEvaluationGate
}

func ShieldUnitCreate(order int, name string) *ShieldUnit {
	return &ShieldUnit{
		order:       order,
		name:        name,
		description: nil,
		atoms:       make([]ShieldAtom[any, any], 0),
		subUnits:    make([]ShieldUnit, 0),
		setup:       nil,
		teardown:    nil,
	}
}

func ShieldUnitSetSetupAndTeardown(unit *ShieldUnit, setup, teardown func()) {
	unit.setup = setup
	unit.teardown = teardown
}

func ShieldUnitSetDescription(unit *ShieldUnit, description string) {
	unit.description = &description
}

func ShieldUnitSetStopRemainingSubUnitsOnChildFailure(unit *ShieldUnit, value bool) {
	unit.stopRemainingSubUnitsOnChildFailure = value
}

func ShieldUnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits(unit *ShieldUnit, value bool) {
	unit.skipOwnAtomsWhenChildFailureStopsSubUnits = value
}

func ShieldUnitSetEvaluationGate(unit *ShieldUnit, gate ShieldUnitEvaluationGate) {
	unit.evaluationGate = gate
}

func ShieldUnitRegisterAtom[TInput, TOutput any](unit *ShieldUnit, atom ShieldAtom[TInput, TOutput]) {
	unit.atoms = append(unit.atoms, atom.toAny())
}

func ShieldUnitRegisterSubUnits(unit *ShieldUnit, subUnits ...ShieldUnit) {
	for _, subUnit := range subUnits {
		if subUnit.name == unit.name {
			panic("cycle detected: subunit cannot be descendant of current unit")
		}

		unit.subUnits = append(unit.subUnits, subUnit)
	}
}

func shieldUnitGetSortedAtoms(unit ShieldUnit) []ShieldAtom[any, any] {
	sorted := extensions.SortedCopyShallow(unit.atoms, func(a, b ShieldAtom[any, any]) int {
		return cmp.Compare(a.order, b.order)
	})

	return sorted
}

func shieldUnitGetSortedSubUnits(unit ShieldUnit) []ShieldUnit {
	return extensions.SortedCopyShallow(unit.subUnits, func(a, b ShieldUnit) int {
		return cmp.Compare(a.order, b.order)
	})
}

func ShieldUnitEvaluationDefaultFailed(report ShieldUnitRunReport) bool {
	if report.SkippedDueToBlacklist {
		return false
	}

	if report.SkippedDueToSetup {
		return true
	}

	if report.AtomSetupFailureCount > 0 || report.AtomValidationFailures > 0 || report.AtomPanics > 0 {
		return true
	}

	for _, ch := range report.DirectChildren {
		if ch.Failed {
			return true
		}
	}

	return false
}

func shieldUnitEvaluationFailed(unit ShieldUnit, report ShieldUnitRunReport) bool {
	if unit.evaluationGate != nil {
		return unit.evaluationGate(report)
	}

	return ShieldUnitEvaluationDefaultFailed(report)
}

func shieldUnitFormatLabel(unit ShieldUnit) string {
	if unit.description != nil {
		return fmt.Sprintf("'%s': %s", unit.name, *unit.description)
	}

	return fmt.Sprintf("'%s'", unit.name)
}

// ------------------------------------------------------------- ATOM

type ShieldAtom[TInput, TOutput any] struct {
	order       int
	name        string
	description *string

	runner    ShieldAtomRunner[TInput, TOutput]
	validator ShieldValidator[TInput, TOutput]

	cases []ShieldCase[TInput, TOutput]

	setup    func()
	teardown func()
}

func ShieldAtomCreate[TInput, TOutput any](
	order int, name string,
	runner ShieldAtomRunner[TInput, TOutput],
	validator ShieldValidator[TInput, TOutput],
) *ShieldAtom[TInput, TOutput] {
	return &ShieldAtom[TInput, TOutput]{
		order:       order,
		name:        name,
		description: nil,
		runner:      runner,
		validator:   validator,
		setup:       nil,
		teardown:    nil,
	}
}

func ShieldAtomSetSetupAndTeardown[TInput, TOutput any](atom *ShieldAtom[TInput, TOutput], setup, teardown func()) {
	atom.setup = setup
	atom.teardown = teardown
}

func ShieldAtomSetDescription[TInput, TOutput any](atom *ShieldAtom[TInput, TOutput], description string) {
	atom.description = &description
}

func ShieldAtomRegisterCase[TInput, TOutput any](atom *ShieldAtom[TInput, TOutput], testCase *ShieldCase[TInput, TOutput]) {
	atom.cases = append(atom.cases, *testCase)
}

func shieldAtomFormatLabel[TInput, TOutput any](atom ShieldAtom[TInput, TOutput]) string {
	if atom.description != nil {
		return fmt.Sprintf("'%s': %s", atom.name, *atom.description)
	}

	return fmt.Sprintf("'%s'", atom.name)
}

func (a *ShieldAtom[TInput, TOutput]) toAny() ShieldAtom[any, any] {
	newCases := make([]ShieldCase[any, any], len(a.cases))

	for i, oldCase := range a.cases {
		newCases[i] = oldCase.toAny()
	}

	return ShieldAtom[any, any]{
		order:       a.order,
		name:        a.name,
		description: a.description,
		runner: func(in any) any {
			return a.runner(in.(TInput))
		},
		validator: func(output any, testCase ShieldCase[any, any]) ShieldAtomResult {
			return a.validator(output.(TOutput), shieldCaseToTyped[TInput, TOutput](testCase))
		},
		setup:    a.setup,
		teardown: a.teardown,
		cases:    newCases,
	}
}

// ------------------------------------------------------------- SHIELD CASE

type ShieldCase[TInput, TOutput any] struct {
	name        string
	description *string

	input    TInput
	expected TOutput
}

func ShieldCaseCreate[TInput, TOutput any](name string, input TInput, expected TOutput) *ShieldCase[TInput, TOutput] {
	return &ShieldCase[TInput, TOutput]{
		name:        name,
		description: nil,
		input:       input,
		expected:    expected,
	}
}

func ShieldCaseSetDescription[TInput, TOutput any](shieldCase *ShieldCase[TInput, TOutput], description string) {
	shieldCase.description = &description
}

func (s *ShieldCase[TInput, TOutput]) toAny() ShieldCase[any, any] {
	return ShieldCase[any, any]{
		name:        s.name,
		description: s.description,
		input:       s.input,
		expected:    s.expected,
	}
}

func shieldCaseToTyped[TInput, TOutput any](shieldCase ShieldCase[any, any]) ShieldCase[TInput, TOutput] {
	return ShieldCase[TInput, TOutput]{
		name:        shieldCase.name,
		description: shieldCase.description,
		input:       shieldCase.input.(TInput),
		expected:    shieldCase.expected.(TOutput),
	}
}

func shieldCaseFormatLabel[TInput, TOutput any](shieldCase ShieldCase[TInput, TOutput]) string {
	if shieldCase.description != nil {
		return fmt.Sprintf("'%s': %s", shieldCase.name, *shieldCase.description)
	}

	return fmt.Sprintf("'%s'", shieldCase.name)
}

// ------------------------------------------------------------- SHIELD RUNTIME CFG

type ShieldRuntimeConfiguration struct {
	blacklistedUnits []string
	blacklistedAtoms []string

	// Architecture decision: explicit blacklisting instead of whitelisting.
	// See docs/adr/0001-Runtime-Configuration-Filtering.md for detail.
}

func ShieldRuntimeConfigurationCreate(
	blacklistedUnits []string,
	blacklistedAtoms []string,
) *ShieldRuntimeConfiguration {
	return &ShieldRuntimeConfiguration{
		blacklistedUnits: blacklistedUnits,
		blacklistedAtoms: blacklistedAtoms,
	}
}

// ------------------------------------------------------------- SHIELD FRAMEWORK

type Shield struct {
	units []ShieldUnit
}

func ShieldCreate(cfg ShieldConfiguration) *Shield {
	return &Shield{
		units: cfg.units,
	}
}

type ShieldRunReport struct {
	Elapsed  time.Duration
	TopLevel []ShieldUnitChildOutcome
	Failed   bool
}

func ShieldRun(shield *Shield, runtimeCfg *ShieldRuntimeConfiguration) ShieldRunReport {
	sorted := extensions.SortedCopyShallow(shield.units, func(a, b ShieldUnit) int {
		return cmp.Compare(a.order, b.order)
	})

	startShield := time.Now()

	report := ShieldRunReport{
		TopLevel: make([]ShieldUnitChildOutcome, 0, len(sorted)),
	}

	for _, unit := range sorted {
		unitReport := shieldUnitRun(unit, runtimeCfg)
		failed := shieldUnitEvaluationFailed(unit, unitReport)

		report.TopLevel = append(report.TopLevel, ShieldUnitChildOutcome{
			Name:   unit.name,
			Failed: failed,
			Report: unitReport,
		})

		if failed {
			report.Failed = true
		}
	}

	endShield := time.Now()
	report.Elapsed = endShield.Sub(startShield)

	logDuration("SHIELD run", report.Elapsed)

	return report
}

// ------------------------------------------------------------- PRIVATE HELPERS

type shieldAtomRunStats struct {
	setupFailedBeforeCases bool
	validationFailures     int
	panics                 int
	skipRestOfUnit         bool
}

type shieldAtomCaseOutcome struct {
	skipRestOfUnit       bool
	hadValidationFailure bool
	hadPanic             bool
}

func shieldUnitRun(unit ShieldUnit, runtimeCfg *ShieldRuntimeConfiguration) ShieldUnitRunReport {
	report := ShieldUnitRunReport{Name: unit.name}

	label := fmt.Sprintf("unit %s", shieldUnitFormatLabel(unit))

	if success := runSetup(label, unit.setup); !success {
		logTemplate("skipping", label)
		report.SkippedDueToSetup = true

		return report
	}

	defer runTeardown(label, unit.teardown)

	if slices.Contains(runtimeCfg.blacklistedUnits, unit.name) {
		logTemplate("skipping", label)
		report.SkippedDueToBlacklist = true

		return report
	}

	atoms := shieldUnitGetSortedAtoms(unit)
	sortedSubs := shieldUnitGetSortedSubUnits(unit)

	logTemplate("starting", label)
	startUnit := time.Now()

	childFailureStopped := false

	for _, subUnit := range sortedSubs {
		subReport := shieldUnitRun(subUnit, runtimeCfg)
		subFailed := shieldUnitEvaluationFailed(subUnit, subReport)

		report.DirectChildren = append(report.DirectChildren, ShieldUnitChildOutcome{
			Name:   subUnit.name,
			Failed: subFailed,
			Report: subReport,
		})

		if subFailed && unit.stopRemainingSubUnitsOnChildFailure {
			report.TerminatedSubUnitLoopEarly = true
			childFailureStopped = true

			echo.On(shieldSystemID).Notice(fmt.Sprintf(
				"stopping remaining sub-units of '%s' after failed sub-unit '%s' (policy)",
				unit.name, subUnit.name))

			break
		}
	}

	skipAtoms := childFailureStopped && unit.skipOwnAtomsWhenChildFailureStopsSubUnits

	if !skipAtoms {
		for _, atom := range atoms {
			atomStats := shieldAtomRun(atom, runtimeCfg)

			if atomStats.setupFailedBeforeCases {
				report.AtomSetupFailureCount++
			}

			report.AtomValidationFailures += atomStats.validationFailures
			report.AtomPanics += atomStats.panics

			if atomStats.skipRestOfUnit {
				echo.On(shieldSystemID).Notice(fmt.Sprintf("skipping rest of unit '%s' in accordance to evaluation result", unit.name))
				break
			}
		}
	} else {
		echo.On(shieldSystemID).Notice(fmt.Sprintf(
			"skipping atoms of unit '%s' after sub-unit failure (policy)",
			unit.name))
	}

	finishedUnit := time.Now()
	elapsedUnit := finishedUnit.Sub(startUnit)

	logDuration(fmt.Sprintf("unit '%s'", unit.name), elapsedUnit)

	return report
}

func shieldAtomRun(atom ShieldAtom[any, any], runtimeCfg *ShieldRuntimeConfiguration) shieldAtomRunStats {
	stats := shieldAtomRunStats{}

	label := fmt.Sprintf("atom %s", shieldAtomFormatLabel(atom))

	if success := runSetup(label, atom.setup); !success {
		logTemplate("skipping", label)
		stats.setupFailedBeforeCases = true

		return stats
	}

	defer runTeardown(label, atom.teardown)

	if slices.Contains(runtimeCfg.blacklistedAtoms, atom.name) {
		logTemplate("skipping", label)

		return stats
	}

	logTemplate("starting", label)

	startAtom := time.Now()

	for _, shieldCase := range atom.cases {
		logTemplate("running", fmt.Sprintf("case %s", shieldCaseFormatLabel(shieldCase)))

		outcome := atomEvaluateCase(atom, shieldCase)

		if outcome.hadValidationFailure {
			stats.validationFailures++
		}

		if outcome.hadPanic {
			stats.panics++
		}

		if outcome.skipRestOfUnit {
			stats.skipRestOfUnit = true
			break
		}
	}

	finishedAtom := time.Now()

	elapsedAtom := finishedAtom.Sub(startAtom)
	logDuration(fmt.Sprintf("atom '%s'", atom.name), elapsedAtom)

	return stats
}

func atomEvaluateCase(atom ShieldAtom[any, any], shieldCase ShieldCase[any, any]) (outcome shieldAtomCaseOutcome) {
	defer func() {
		if r := recover(); r != nil {
			outcome.hadPanic = true

			label := fmt.Sprintf("atom '%s' - case '%s'", atom.name, shieldCase.name)
			stack := debug.Stack()
			failureMsg := fmt.Sprintf("panic recovered: %v\nStack trace:\n%s", r, stack)

			logStageFailure(label, "execution/validation", failureMsg)
		}
	}()

	output := atom.runner(shieldCase.input)
	validated := atom.validator(output, shieldCase)

	if !validated.success {
		outcome.hadValidationFailure = true

		failureMessage := shieldAtomResultFormat(atom, validated)
		logFailure(failureMessage)
	}

	outcome.skipRestOfUnit = validated.skipFurtherAtomsInUnit

	return outcome
}

func logTemplate(action, label string) {
	echo.On(shieldSystemID).Info(fmt.Sprintf("%s %s", action, label))
}

func logDuration(description string, duration time.Duration) {
	nano := float64(duration.Nanoseconds())
	echo.On(shieldSystemID).Field("duration", formatting.FormatDurationNSF64(nano)).Info(fmt.Sprintf("finished %s", description))
}

func logFailure(failureMessage string) {
	echo.On(shieldSystemID).Warning(failureMessage)
}

func runSetup(label string, setup func()) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			stack := debug.Stack()
			failureMsg := fmt.Sprintf("panic recovered: %v\nStack trace:\n%s", r, stack)

			logStageFailure(label, "setup", failureMsg)
		}
	}()

	if setup == nil {
		return true
	}

	logLifecycle("setup", label)
	setup()

	return true
}

func runTeardown(label string, teardown func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			failureMsg := fmt.Sprintf("panic recovered: %v\nStack trace:\n%s", r, stack)

			logStageFailure(label, "teardown", failureMsg)
		}
	}()

	if teardown == nil {
		return
	}

	logLifecycle("teardown", label)
	teardown()
}

func logStageFailure(stage, label, setupFailureMessage string) {
	echo.On(shieldSystemID).Error(fmt.Sprintf("error executing %s for %s: %s", stage, label, setupFailureMessage))
}

func logLifecycle(stage, label string) {
	echo.On(shieldSystemID).Trace(fmt.Sprintf("executing stage %s for %s", stage, label))
}
