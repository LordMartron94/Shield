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
Case binds a name, input, and expected output for one invocation of an atom’s runner.
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
RuntimeConfiguration selects which units and atoms to skip by exact name match.
Blacklists only; see ADR 0001 in docs/adr.
*/
type RuntimeConfiguration = internal.ShieldRuntimeConfiguration

/*
Engine is the runnable graph built from a frozen Configuration.
*/
type Engine = internal.Shield

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
AtomResultSetSkipFurtherAtomsInUnit, when set true on a validation result, stops running
remaining atoms in the same unit after the current atom finishes (sub-units already run
are unaffected; this applies to the atom loop only).
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
CaseSetDescription sets optional human-readable text included in log labels.
*/
func CaseSetDescription[TInput, TOutput any](c *Case[TInput, TOutput], description string) {
	internal.ShieldCaseSetDescription(c, description)
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
and atoms, and logging timing via echo. It does not return a pass/fail summary; consult
logs for outcomes.
*/
func Run(engine *Engine, runtimeCfg *RuntimeConfiguration) {
	internal.ShieldRun(engine, runtimeCfg)
}
