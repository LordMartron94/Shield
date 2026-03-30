package internal

import (
	"cmp"
	"fmt"
	"foundation/extensions"
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

type ShieldUnitAtomOutcome struct {
	Name                  string
	Failed                bool
	SkippedDueToBlacklist bool
	SkippedDueToSetup     bool
	ValidationFailures    int
	Panics                int
	Elapsed               time.Duration
	Cases                 []ShieldAtomCaseReport
}

type ShieldAtomCaseReport struct {
	Name                   string
	Failed                 bool
	Skipped                bool
	ValidationFailure      bool
	Panic                  bool
	SkipFurtherAtomsInUnit bool
	Elapsed                time.Duration
}

type ShieldUnitRunReport struct {
	Name string

	SkippedDueToBlacklist bool
	SkippedDueToSetup     bool

	AtomSetupFailureCount  int
	AtomValidationFailures int
	AtomPanics             int

	TerminatedSubUnitLoopEarly bool

	Atoms []ShieldUnitAtomOutcome

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
	if len(atom.cases) == 0 {
		panic("registered atom must have at least 1 case")
	}

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

	input         TInput
	expected      TOutput
	hasExpected   bool
	evaluation    ShieldValidator[TInput, TOutput]
	hasEvaluation bool
}

func ShieldCaseCreate[TInput, TOutput any](name string, input TInput, expected TOutput) *ShieldCase[TInput, TOutput] {
	return &ShieldCase[TInput, TOutput]{
		name:        name,
		description: nil,
		input:       input,
		expected:    expected,
		hasExpected: true,
	}
}

func ShieldCaseCreateWithoutExpected[TInput, TOutput any](name string, input TInput) *ShieldCase[TInput, TOutput] {
	return &ShieldCase[TInput, TOutput]{
		name:        name,
		description: nil,
		input:       input,
		hasExpected: false,
	}
}

func ShieldCaseCreateWithEvaluation[TInput, TOutput any](
	name string,
	input TInput,
	evaluation ShieldValidator[TInput, TOutput],
) *ShieldCase[TInput, TOutput] {
	return &ShieldCase[TInput, TOutput]{
		name:          name,
		description:   nil,
		input:         input,
		hasExpected:   false,
		evaluation:    evaluation,
		hasEvaluation: true,
	}
}

func ShieldCaseSetDescription[TInput, TOutput any](shieldCase *ShieldCase[TInput, TOutput], description string) {
	shieldCase.description = &description
}

func ShieldCaseNameGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) string {
	return c.name
}

func ShieldCaseInputGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) TInput {
	return c.input
}

func ShieldCaseExpectedGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) TOutput {
	return c.expected
}

func ShieldCaseExpectedTryGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) (TOutput, bool) {
	return c.expected, c.hasExpected
}

func ShieldCaseEvaluationTryGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) (ShieldValidator[TInput, TOutput], bool) {
	return c.evaluation, c.hasEvaluation
}

func ShieldCaseDescriptionGet[TInput, TOutput any](c ShieldCase[TInput, TOutput]) *string {
	return c.description
}

func (s *ShieldCase[TInput, TOutput]) toAny() ShieldCase[any, any] {
	var evaluation ShieldValidator[any, any]
	if s.hasEvaluation && s.evaluation != nil {
		evaluation = func(output any, testCase ShieldCase[any, any]) ShieldAtomResult {
			return s.evaluation(output.(TOutput), shieldCaseToTyped[TInput, TOutput](testCase))
		}
	}

	return ShieldCase[any, any]{
		name:          s.name,
		description:   s.description,
		input:         s.input,
		expected:      s.expected,
		hasExpected:   s.hasExpected,
		evaluation:    evaluation,
		hasEvaluation: s.hasEvaluation,
	}
}

func shieldCaseToTyped[TInput, TOutput any](shieldCase ShieldCase[any, any]) ShieldCase[TInput, TOutput] {
	var evaluation ShieldValidator[TInput, TOutput]
	if shieldCase.hasEvaluation && shieldCase.evaluation != nil {
		evaluation = func(output TOutput, testCase ShieldCase[TInput, TOutput]) ShieldAtomResult {
			testCaseAny := testCase.toAny()
			return shieldCase.evaluation(output, testCaseAny)
		}
	}

	return ShieldCase[TInput, TOutput]{
		name:          shieldCase.name,
		description:   shieldCase.description,
		input:         shieldCase.input.(TInput),
		expected:      shieldCase.expected.(TOutput),
		hasExpected:   shieldCase.hasExpected,
		evaluation:    evaluation,
		hasEvaluation: shieldCase.hasEvaluation,
	}
}

// ------------------------------------------------------------- SHIELD RUNTIME CFG

type ShieldRuntimeConfiguration struct {
	blacklistedUnits []string
	blacklistedAtoms []string
	verbosity        ShieldRunVerbosity

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
		verbosity:        ShieldRunVerbosityNormal,
	}
}

func ShieldRuntimeConfigurationSetVerbosity(cfg *ShieldRuntimeConfiguration, verbosity ShieldRunVerbosity) {
	if cfg == nil {
		return
	}

	switch verbosity {
	case ShieldRunVerbosityQuiet, ShieldRunVerbosityNormal, ShieldRunVerbosityVerbose:
		cfg.verbosity = verbosity
	default:
		cfg.verbosity = ShieldRunVerbosityNormal
	}
}

// ------------------------------------------------------------- SHIELD FRAMEWORK

type Shield struct {
	units []ShieldUnit
}

type ShieldRunVerbosity uint8

const (
	ShieldRunVerbosityQuiet ShieldRunVerbosity = iota + 1
	ShieldRunVerbosityNormal
	ShieldRunVerbosityVerbose
)

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

type ShieldRunOutcome struct {
	Report            ShieldRunReport
	WrittenReportPath string
}

func ShieldRun(shield *Shield, runtimeCfg *ShieldRuntimeConfiguration) ShieldRunReport {
	outcome := shieldRunExecute(shield, runtimeCfg, nil)
	return outcome.Report
}

func ShieldRunWithPersistence(
	shield *Shield,
	runtimeCfg *ShieldRuntimeConfiguration,
	persist *ShieldReportPersistence,
) ShieldRunOutcome {
	return shieldRunExecute(shield, runtimeCfg, persist)
}

func shieldRunExecute(
	shield *Shield,
	runtimeCfg *ShieldRuntimeConfiguration,
	persist *ShieldReportPersistence,
) ShieldRunOutcome {
	if runtimeCfg == nil {
		runtimeCfg = ShieldRuntimeConfigurationCreate(nil, nil)
	}

	sorted := extensions.SortedCopyShallow(shield.units, func(a, b ShieldUnit) int {
		return cmp.Compare(a.order, b.order)
	})

	startShield := time.Now()

	report := ShieldRunReport{
		TopLevel: make([]ShieldUnitChildOutcome, 0, len(sorted)),
	}

	tel := &shieldRunTelemetry{atomDurationsNs: make([]float64, 0)}
	runIO := shieldRunIOCreateDefault(persist)
	runIO.reporter.OnRunStart(len(sorted), runtimeCfg.verbosity)

	for _, unit := range sorted {
		unitReport := shieldUnitRun(unit, runtimeCfg, tel, runIO.reporter, 0)
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

	metrics := shieldRunMetricsBuild(report, tel)
	runIO.reporter.OnRunEnd(report, metrics)
	runIO.summaryEmitter(report, metrics)
	writtenReportPath := runIO.reportWriter(report, metrics)

	return ShieldRunOutcome{
		Report:            report,
		WrittenReportPath: writtenReportPath,
	}
}

// ------------------------------------------------------------- PRIVATE HELPERS

type shieldAtomRunStats struct {
	setupFailedBeforeCases bool
	skippedDueToBlacklist  bool
	validationFailures     int
	panics                 int
	skipRestOfUnit         bool
	elapsed                time.Duration
	failed                 bool
	caseOutcomes           []ShieldAtomCaseReport
}

type shieldAtomCaseOutcome struct {
	skipRestOfUnit       bool
	hadValidationFailure bool
	hadPanic             bool
	panicStage           string
	panicMessage         string
	validationResult     *ShieldAtomResult
	elapsed              time.Duration
}

func shieldUnitRun(
	unit ShieldUnit,
	runtimeCfg *ShieldRuntimeConfiguration,
	tel *shieldRunTelemetry,
	reporter shieldRunReporter,
	depth int,
) ShieldUnitRunReport {
	report := ShieldUnitRunReport{
		Name:  unit.name,
		Atoms: make([]ShieldUnitAtomOutcome, 0),
	}

	if success := runSetup("unit", unit.name, unit.setup, reporter); !success {
		reporter.OnSkip("unit", unit.name, "setup_failed")
		report.SkippedDueToSetup = true

		return report
	}

	defer runTeardown("unit", unit.name, unit.teardown, reporter)

	if slices.Contains(runtimeCfg.blacklistedUnits, unit.name) {
		reporter.OnSkip("unit", unit.name, "blacklist")
		report.SkippedDueToBlacklist = true

		return report
	}

	atoms := shieldUnitGetSortedAtoms(unit)
	sortedSubs := shieldUnitGetSortedSubUnits(unit)

	reporter.OnUnitStart(unit, depth)
	startUnit := time.Now()

	childFailureStopped := false

	for _, subUnit := range sortedSubs {
		subReport := shieldUnitRun(subUnit, runtimeCfg, tel, reporter, depth+1)
		subFailed := shieldUnitEvaluationFailed(subUnit, subReport)

		report.DirectChildren = append(report.DirectChildren, ShieldUnitChildOutcome{
			Name:   subUnit.name,
			Failed: subFailed,
			Report: subReport,
		})

		if subFailed && unit.stopRemainingSubUnitsOnChildFailure {
			report.TerminatedSubUnitLoopEarly = true
			childFailureStopped = true

			reporter.OnPolicy(fmt.Sprintf(
				"stopping remaining sub-units of '%s' after failed sub-unit '%s' (policy)",
				unit.name, subUnit.name))

			break
		}
	}

	skipAtoms := childFailureStopped && unit.skipOwnAtomsWhenChildFailureStopsSubUnits

	if !skipAtoms {
		for _, atom := range atoms {
			atomStats := shieldAtomRun(unit.name, atom, runtimeCfg, tel, reporter)
			report.Atoms = append(report.Atoms, ShieldUnitAtomOutcome{
				Name:                  atom.name,
				Failed:                atomStats.failed,
				SkippedDueToBlacklist: atomStats.skippedDueToBlacklist,
				SkippedDueToSetup:     atomStats.setupFailedBeforeCases,
				ValidationFailures:    atomStats.validationFailures,
				Panics:                atomStats.panics,
				Elapsed:               atomStats.elapsed,
				Cases:                 atomStats.caseOutcomes,
			})

			if atomStats.setupFailedBeforeCases {
				report.AtomSetupFailureCount++
			}

			report.AtomValidationFailures += atomStats.validationFailures
			report.AtomPanics += atomStats.panics

			if atomStats.skipRestOfUnit {
				reporter.OnPolicy(fmt.Sprintf("skipping rest of unit '%s' in accordance to evaluation result", unit.name))
				break
			}
		}
	} else {
		reporter.OnPolicy(fmt.Sprintf(
			"skipping atoms of unit '%s' after sub-unit failure (policy)",
			unit.name))
	}

	finishedUnit := time.Now()
	elapsedUnit := finishedUnit.Sub(startUnit)
	failed := shieldUnitEvaluationFailed(unit, report)
	reporter.OnUnitEnd(unit, depth, elapsedUnit, report, failed)

	return report
}

func shieldAtomRun(
	unitName string,
	atom ShieldAtom[any, any],
	runtimeCfg *ShieldRuntimeConfiguration,
	tel *shieldRunTelemetry,
	reporter shieldRunReporter,
) shieldAtomRunStats {
	stats := shieldAtomRunStats{
		caseOutcomes: make([]ShieldAtomCaseReport, 0, len(atom.cases)),
	}

	if success := runSetup("atom", atom.name, atom.setup, reporter); !success {
		reporter.OnSkip("atom", atom.name, "setup_failed")
		stats.setupFailedBeforeCases = true
		stats.failed = true

		return stats
	}

	defer runTeardown("atom", atom.name, atom.teardown, reporter)

	if slices.Contains(runtimeCfg.blacklistedAtoms, atom.name) {
		reporter.OnSkip("atom", atom.name, "blacklist")
		stats.skippedDueToBlacklist = true

		return stats
	}

	reporter.OnAtomStart(unitName, atom, len(atom.cases))

	startAtom := time.Now()
	atomFailed := false

	for _, shieldCase := range atom.cases {
		reporter.OnCaseStart(unitName, atom.name, shieldCase)
		caseStarted := time.Now()

		outcome := atomEvaluateCase(unitName, atom, shieldCase, reporter)
		caseElapsed := time.Since(caseStarted)
		outcome.elapsed = caseElapsed

		if outcome.hadValidationFailure {
			stats.validationFailures++
			atomFailed = true
			if outcome.validationResult != nil {
				reporter.OnValidationFailed(unitName, atom.name, shieldCase.name, *outcome.validationResult, runtimeCfg.verbosity)
			}
		}

		if outcome.hadPanic {
			stats.panics++
			atomFailed = true
			reporter.OnCasePanic(unitName, atom.name, shieldCase.name, outcome.panicStage, outcome.panicMessage, runtimeCfg.verbosity)
		}

		caseFailed := outcome.hadValidationFailure || outcome.hadPanic
		reporter.OnCaseEnd(unitName, atom.name, shieldCase, caseFailed, false, caseElapsed)
		stats.caseOutcomes = append(stats.caseOutcomes, ShieldAtomCaseReport{
			Name:                   shieldCase.name,
			Failed:                 caseFailed,
			Skipped:                false,
			ValidationFailure:      outcome.hadValidationFailure,
			Panic:                  outcome.hadPanic,
			SkipFurtherAtomsInUnit: outcome.skipRestOfUnit,
			Elapsed:                caseElapsed,
		})

		if outcome.skipRestOfUnit {
			stats.skipRestOfUnit = true
			break
		}
	}

	finishedAtom := time.Now()

	elapsedAtom := finishedAtom.Sub(startAtom)
	stats.elapsed = elapsedAtom
	stats.failed = atomFailed
	reporter.OnAtomEnd(unitName, atom.name, elapsedAtom, atomFailed)

	if tel != nil {
		tel.atomDurationsNs = append(tel.atomDurationsNs, float64(elapsedAtom.Nanoseconds()))
	}

	return stats
}

func atomEvaluateCase(
	unitName string,
	atom ShieldAtom[any, any],
	shieldCase ShieldCase[any, any],
	_ shieldRunReporter,
) (outcome shieldAtomCaseOutcome) {
	defer func() {
		if r := recover(); r != nil {
			outcome.hadPanic = true

			stack := debug.Stack()
			outcome.panicStage = "execution/validation"
			outcome.panicMessage = fmt.Sprintf(
				"panic recovered in %s.%s.%s: %v\nStack trace:\n%s",
				unitName, atom.name, shieldCase.name, r, stack)
		}
	}()

	output := atom.runner(shieldCase.input)
	var validated ShieldAtomResult
	if shieldCase.hasEvaluation && shieldCase.evaluation != nil {
		validated = shieldCase.evaluation(output, shieldCase)
	} else {
		validated = atom.validator(output, shieldCase)
	}
	outcome.validationResult = &validated

	if !validated.success {
		outcome.hadValidationFailure = true
	}

	outcome.skipRestOfUnit = validated.skipFurtherAtomsInUnit

	return outcome
}

func runSetup(kind, name string, setup func(), reporter shieldRunReporter) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			stack := debug.Stack()
			failureMsg := fmt.Sprintf("panic recovered: %v\nStack trace:\n%s", r, stack)
			reporter.OnStageFailure(kind, name, "setup", failureMsg)
		}
	}()

	if setup == nil {
		return true
	}

	setup()

	return true
}

func runTeardown(kind, name string, teardown func(), reporter shieldRunReporter) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			failureMsg := fmt.Sprintf("panic recovered: %v\nStack trace:\n%s", r, stack)
			reporter.OnStageFailure(kind, name, "teardown", failureMsg)
		}
	}()

	if teardown == nil {
		return
	}

	teardown()
}
