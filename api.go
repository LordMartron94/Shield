package shield

import "shield/internal"

/*
AtomResult records the outcome of validating one case. The zero value is not a valid
success indicator; use AtomResultSuccessCreate or AtomResultFailureCreate.
*/
type AtomResult = internal.ShieldAtomResult

/*
Runner executes the subject under test for a single case input. It must not panic;
panics are recovered, logged, and treated as failure for that case.
*/
type Runner[TInput, TOutput any] = internal.ShieldAtomRunner[TInput, TOutput]

/*
Validator compares actual output to the case’s expected value (and any other invariants).
It returns an AtomResult by value. Helpers return pointers; dereference when returning, e.g.
return *AtomResultFailureCreate("reason").
*/
type Validator[TInput, TOutput any] = internal.ShieldValidator[TInput, TOutput]

/*
Atom is one ordered check within a unit: a runner, a validator, and registered cases.
*/
type Atom[TInput, TOutput any] = internal.ShieldAtom[TInput, TOutput]

/*
Case binds a name and input for one invocation of an atom’s runner, and may optionally
carry an expected output when the validation strategy needs one.
Read fields in validators via CaseNameGet, CaseInputGet, CaseExpectedGet/CaseExpectedTryGet,
and CaseDescriptionGet.
*/
type Case[TInput, TOutput any] = internal.ShieldCase[TInput, TOutput]

/*
Configuration holds the top-level unit list passed to EngineCreate. All units that should
run must be registered here (directly or as sub-units of a registered unit).
*/
type Configuration = internal.ShieldConfiguration

/*
Unit is a named, ordered group of atoms and optional sub-units. Sub-units must not share
the same name as their parent; that situation panics at registration time.
*/
type Unit = internal.ShieldUnit

/*
UnitRunReport summarizes one execution of a unit: blacklist/setup skips, atom failures
(including setup failures before cases), panics, and direct child outcomes in run order.
Custom evaluation gates receive this after the unit finishes the work it was scheduled to perform.
*/
type UnitRunReport = internal.ShieldUnitRunReport

/*
UnitChildOutcome binds a direct sub-unit’s name to whether it was classified as failed and its full report.
*/
type UnitChildOutcome = internal.ShieldUnitChildOutcome

/*
UnitEvaluationGate classifies a finished unit run. Return true when the unit should count as failed
for parents (default logic) and when deciding stop-on-child policies. Nil on a unit restores
UnitEvaluationDefaultFailed.
*/
type UnitEvaluationGate = internal.ShieldUnitEvaluationGate

/*
RuntimeConfiguration selects which units and atoms to skip by exact name match.
Blacklists only; see ADR 0001 in docs/adr.
*/
type RuntimeConfiguration = internal.ShieldRuntimeConfiguration

/*
Engine is the runnable graph built from a frozen Configuration.
*/
type Engine = internal.Shield

/*
RunReport is the outcome of Run: wall-clock Elapsed, whether any top-level unit Failed
(per that unit’s evaluation gate or default), and TopLevel entries in execution order
each carrying the unit name, Failed flag, and full nested UnitRunReport tree.
WrittenReportPath is set when RunWithReportPersistence wrote at least one file successfully
(primary artifact: JSON path if JSON was written, otherwise the text path).
*/
type RunReport = internal.ShieldRunReport

/*
ReportPersistence configures optional on-disk reports under a directory (see RunWithReportPersistence).
Use ReportPersistenceSetMode for built-in JSON / text / both, or ReportPersistenceSetAdapter for a custom writer
(adapter wins over mode when non-nil).
*/
type ReportPersistence = internal.ShieldReportPersistence

/*
ReportPersistenceMode selects built-in persistence output when no custom ReportPersistAdapter is set.
*/
type ReportPersistenceMode = internal.ShieldReportPersistenceMode

const (
	ReportPersistenceModeJSON       = internal.ShieldReportPersistenceModeJSON
	ReportPersistenceModeTXT        = internal.ShieldReportPersistenceModeTXT
	ReportPersistenceModeJSONAndTXT = internal.ShieldReportPersistenceModeJSONAndTXT
)

/*
RunAggregates is the counter bundle inside RunMetrics (tree-walk summary).
*/
type RunAggregates = internal.ShieldRunAggregates

/*
RunMetrics is the post-run snapshot passed to custom persistence adapters and mirrored in persisted JSON.
When atom durations were collected, DurationStatsOK is true and fields include MeanNs, StddevPopNs (n≥2),
MinNs/MaxNs (aligned with the five-number path), DurationSumNs, DurationMedianNs, DurationQ1Ns,
DurationQ3Ns, DurationIQRNs, DurationP95Ns, DurationP99Ns, and DurationCoeffVarPop when n≥2 and |MeanNs|
is above an internal epsilon (otherwise zero and omitted from JSON via omitempty where applicable).
*/
type RunMetrics = internal.ShieldRunMetrics

/*
ReportPersistAdapter writes RunReport and RunMetrics to outputDir; return the primary path for RunReport.WrittenReportPath.
*/
type ReportPersistAdapter = internal.ShieldReportPersistAdapter

/*
ReportDocumentSchemaVersion is the schema_version field written to JSON report files.
*/
const ReportDocumentSchemaVersion = internal.ShieldReportDocumentSchemaVersion

/*
ReportPersistenceCreate returns persistence state for an output directory (cleaned at write time).
Disabled by default; default mode is JSON. Use ReportPersistenceSetEnabled(true) to write after a run.
*/
func ReportPersistenceCreate(outputDir string) *ReportPersistence {
	return internal.ShieldReportPersistenceCreate(outputDir)
}

/*
ReportPersistenceSetEnabled turns disk persistence on or off. When false, RunWithReportPersistence
behaves like Run for I/O.
*/
func ReportPersistenceSetEnabled(p *ReportPersistence, value bool) {
	internal.ShieldReportPersistenceSetEnabled(p, value)
}

/*
ReportPersistenceSetMode selects built-in JSON, text, or both. Ignored when ReportPersistenceSetAdapter is non-nil.
*/
func ReportPersistenceSetMode(p *ReportPersistence, mode ReportPersistenceMode) {
	internal.ShieldReportPersistenceSetMode(p, mode)
}

/*
ReportPersistenceSetAdapter registers a custom writer; nil clears it and restores mode-driven built-ins.
*/
func ReportPersistenceSetAdapter(p *ReportPersistence, adapter ReportPersistAdapter) {
	internal.ShieldReportPersistenceSetAdapter(p, adapter)
}

/*
AtomResultSuccessCreate returns a successful result with no note and without requesting
skip of further atoms in the unit.
*/
func AtomResultSuccessCreate() *AtomResult {
	return internal.ShieldAtomResultSuccessCreate()
}

/*
AtomResultFailureCreate returns a failed result with the given reason string (logged on failure).
*/
func AtomResultFailureCreate(reason string) *AtomResult {
	return internal.ShieldAtomResultFailureCreate(reason)
}

/*
AtomResultSetNote attaches an optional note included in failure log output.
*/
func AtomResultSetNote(result *AtomResult, note string) {
	internal.ShieldAtomResultSetNote(result, note)
}

/*
AtomResultSetSkipFurtherAtomsInUnit when set true on a validation result, stops running
remaining atoms in the same unit after the current atom finishes. Sub-units run before
atoms; this flag does not affect sub-units that already completed.
*/
func AtomResultSetSkipFurtherAtomsInUnit(result *AtomResult, value bool) {
	internal.ShieldAtomResultSetSkipFurtherAtomsInUnit(result, value)
}

/*
ConfigurationCreate returns an empty configuration.
*/
func ConfigurationCreate() *Configuration {
	return internal.ShieldConfigurationCreate()
}

/*
ConfigurationRegisterUnits appends units to the configuration. Units are copied by value;
use pointers from UnitCreate and pass *u where a Unit value is required, or register
sub-units via UnitRegisterSubUnits instead of flattening manually.
*/
func ConfigurationRegisterUnits(cfg *Configuration, units ...Unit) {
	internal.ShieldConfigurationRegisterUnits(cfg, units...)
}

/*
UnitCreate builds a unit with the given sort order and name. Lower order runs first among siblings.
*/
func UnitCreate(order int, name string) *Unit {
	return internal.ShieldUnitCreate(order, name)
}

/*
UnitSetSetupAndTeardown registers optional hooks. If setup panics or fails internally, the
unit is skipped; teardown runs when setup ran, even if later stages fail.
*/
func UnitSetSetupAndTeardown(unit *Unit, setup, teardown func()) {
	internal.ShieldUnitSetSetupAndTeardown(unit, setup, teardown)
}

/*
UnitSetDescription sets optional human-readable text included in log labels.
*/
func UnitSetDescription(unit *Unit, description string) {
	internal.ShieldUnitSetDescription(unit, description)
}

/*
UnitSetStopRemainingSubUnitsOnChildFailure enables skipping remaining direct sub-units (by order)
after the first sub-unit whose gate reports failure. Recursive: each sub-unit uses its own gate
on its own report. Default false.
*/
func UnitSetStopRemainingSubUnitsOnChildFailure(unit *Unit, value bool) {
	internal.ShieldUnitSetStopRemainingSubUnitsOnChildFailure(unit, value)
}

/*
UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits when true together with stop-on-child, skips this
unit’s atoms after the sub-unit loop ends early due to a failed child. Use this when sibling sub-units
model dependencies (e.g. database then behaviour). Default false.
*/
func UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits(unit *Unit, value bool) {
	internal.ShieldUnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits(unit, value)
}

/*
UnitSetEvaluationGate sets how this unit’s run is classified as failed or not. Nil uses
UnitEvaluationDefaultFailed (setup skip, atom setup/validation/panic failures, or any failed direct child;
blacklist-only skip is not a failure).
*/
func UnitSetEvaluationGate(unit *Unit, gate UnitEvaluationGate) {
	internal.ShieldUnitSetEvaluationGate(unit, gate)
}

/*
UnitEvaluationDefaultFailed is the built-in classifier; useful when composing custom gates
(e.g. extra conditions then fall back to default).
*/
func UnitEvaluationDefaultFailed(report UnitRunReport) bool {
	return internal.ShieldUnitEvaluationDefaultFailed(report)
}

/*
UnitRegisterAtom attaches a typed atom to the unit. The atom is type-erased for storage;
runner and validator must agree with the case types registered on that atom.
*/
func UnitRegisterAtom[TInput, TOutput any](unit *Unit, atom *Atom[TInput, TOutput]) {
	internal.ShieldUnitRegisterAtom(unit, *atom)
}

/*
UnitRegisterSubUnits nests units for hierarchical structure. A sub-unit cannot have the
same name as the parent unit (panic).
*/
func UnitRegisterSubUnits(unit *Unit, subUnits ...Unit) {
	internal.ShieldUnitRegisterSubUnits(unit, subUnits...)
}

/*
AtomCreate constructs an atom with explicit order, name, runner, and validator.
Cases are added with AtomRegisterCase.
*/
func AtomCreate[TInput, TOutput any](
	order int, name string,
	runner Runner[TInput, TOutput],
	validator Validator[TInput, TOutput],
) *Atom[TInput, TOutput] {
	return internal.ShieldAtomCreate(order, name, runner, validator)
}

/*
AtomSetSetupAndTeardown registers optional per-atom hooks with the same semantics as unit hooks.
*/
func AtomSetSetupAndTeardown[TInput, TOutput any](atom *Atom[TInput, TOutput], setup, teardown func()) {
	internal.ShieldAtomSetSetupAndTeardown(atom, setup, teardown)
}

/*
AtomSetDescription sets optional human-readable text included in log labels.
*/
func AtomSetDescription[TInput, TOutput any](atom *Atom[TInput, TOutput], description string) {
	internal.ShieldAtomSetDescription(atom, description)
}

/*
AtomRegisterCase appends a case; cases run in registration order. testCase must not be nil.
*/
func AtomRegisterCase[TInput, TOutput any](atom *Atom[TInput, TOutput], testCase *Case[TInput, TOutput]) {
	internal.ShieldAtomRegisterCase(atom, testCase)
}

/*
CaseCreate builds a named case from input and expected output.
*/
func CaseCreate[TInput, TOutput any](name string, input TInput, expected TOutput) *Case[TInput, TOutput] {
	return internal.ShieldCaseCreate(name, input, expected)
}

/*
CaseCreateWithoutExpected builds a named case from input only.
Use this for atoms whose validator checks side effects/invariants without needing an expected output value.
*/
func CaseCreateWithoutExpected[TInput, TOutput any](name string, input TInput) *Case[TInput, TOutput] {
	return internal.ShieldCaseCreateWithoutExpected[TInput, TOutput](name, input)
}

/*
CaseSetDescription sets optional human-readable text included in log labels.
*/
func CaseSetDescription[TInput, TOutput any](c *Case[TInput, TOutput], description string) {
	internal.ShieldCaseSetDescription(c, description)
}

/*
CaseNameGet returns the case name supplied to CaseCreate.
*/
func CaseNameGet[TInput, TOutput any](c Case[TInput, TOutput]) string {
	return internal.ShieldCaseNameGet(c)
}

/*
CaseInputGet returns the input value the runner receives for this case.
*/
func CaseInputGet[TInput, TOutput any](c Case[TInput, TOutput]) TInput {
	return internal.ShieldCaseInputGet(c)
}

/*
CaseExpectedGet returns the expected output clients associate with this case (validator contract).
*/
func CaseExpectedGet[TInput, TOutput any](c Case[TInput, TOutput]) TOutput {
	return internal.ShieldCaseExpectedGet(c)
}

/*
CaseExpectedTryGet returns the expected output and whether it was explicitly provided.
Cases created through CaseCreateWithoutExpected return (zeroValue, false).
*/
func CaseExpectedTryGet[TInput, TOutput any](c Case[TInput, TOutput]) (TOutput, bool) {
	return internal.ShieldCaseExpectedTryGet(c)
}

/*
CaseDescriptionGet returns the optional description pointer, or nil if unset.
*/
func CaseDescriptionGet[TInput, TOutput any](c Case[TInput, TOutput]) *string {
	return internal.ShieldCaseDescriptionGet(c)
}

/*
RuntimeConfigurationCreate builds filtering state. Empty slices mean nothing is blacklisted.
Matching is by unit name or atom name string equality.
*/
func RuntimeConfigurationCreate(blacklistedUnits, blacklistedAtoms []string) *RuntimeConfiguration {
	return internal.ShieldRuntimeConfigurationCreate(blacklistedUnits, blacklistedAtoms)
}

/*
EngineCreate materializes an engine from a configuration snapshot. The engine holds the
unit slice by value as at creation time.
*/
func EngineCreate(cfg Configuration) *Engine {
	return internal.ShieldCreate(cfg)
}

/*
Run walks all registered units in order, respecting runtime blacklists, running sub-units
and atoms, and logging timing via echo. After the run it logs a structured Shield run summary
(counts and, when atoms executed, statarch/blaze-backed duration statistics). The returned
RunReport aggregates top-level failure and full nested reports; Failed follows each unit’s
UnitEvaluationGate or UnitEvaluationDefaultFailed.
*/
func Run(engine *Engine, runtimeCfg *RuntimeConfiguration) RunReport {
	return internal.ShieldRun(engine, runtimeCfg)
}

/*
RunWithReportPersistence runs the engine like Run, then may write reports under
ReportPersistenceCreate’s directory when persistence is enabled. Built-in output is chosen with
ReportPersistenceSetMode (JSON, TXT, or JSONAndTXT). If ReportPersistenceSetAdapter is non-nil,
only the adapter runs. Filenames use a UTC timestamp stem shield-run-YYYYMMDD-hhmmss.nnnnnnnnn;
if a name exists, a numeric suffix is tried. Failures to write are logged via echo and do not fail the run.
*/
func RunWithReportPersistence(
	engine *Engine,
	runtimeCfg *RuntimeConfiguration,
	persist *ReportPersistence,
) RunReport {
	return internal.ShieldRunWithPersistence(engine, runtimeCfg, persist)
}
