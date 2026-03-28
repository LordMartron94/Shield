/*
Package shield is a structured validation and regression harness.

It organizes work into units (coarse groupings, often mirroring a subsystem) and atoms
(ordered checks inside a unit). Each atom pairs a runner function with a validator and
one or more cases. Execution order is deterministic: top-level units, direct sub-units,
and atoms each use an explicit integer order field (lower first); cases run in registration
order. Within a unit, sub-units run entirely before that unit’s own atoms.

A unit may stop running further sub-units after a child is classified as failed, and optionally
skip its own atoms in that case—see UnitSetStopRemainingSubUnitsOnChildFailure and
UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits. Failure classification is recursive: each
unit’s UnitRunReport is passed to UnitSetEvaluationGate or UnitEvaluationDefaultFailed.

Runtime filtering uses name-based blacklists for units and atoms (see RuntimeConfigurationCreate
and docs/adr/0001-Runtime-Configuration-Filtering.md). There is no implicit discovery: you
build a Configuration, register units, then Run.

Logging is emitted through the echo subsystem registered by this module (prefix "Shield").
Failures in runner or validator paths are recovered, logged, and do not crash the process.

After each Run, a structured "Shield run summary" line is logged via echo (field `summary` true)
with aggregate counts from the report tree; when any atoms executed, per-atom wall times are copied
into a manual-memory vector and summarized with statarch (mean, population standard deviation) and
blaze (min/max). Resolve statarch/blaze/memforge via your go.work / submodule layout; the shield
module go.mod does not list those requirements.

Typical flow:

 1. ConfigurationCreate, ConfigurationRegisterUnits with units built via UnitCreate and
    UnitRegisterAtom / UnitRegisterSubUnits.
 2. For each atom: AtomCreate(runner, validator), AtomRegisterCase for each case, optional
    AtomSetSetupAndTeardown / AtomSetDescription.
 3. Optional: UnitSetStopRemainingSubUnitsOnChildFailure / UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits /
    UnitSetEvaluationGate on units that model dependencies.
 4. EngineCreate(configuration), report := Run(engine, runtimeConfiguration); use report.Failed
    and report.TopLevel for exit codes or CI without scraping logs.

Validators return AtomResult by value. Use *AtomResultSuccessCreate(), *AtomResultFailureCreate(reason),
and the AtomResultSet* helpers when you need notes or to skip remaining atoms in the current unit.

Complexity: Run is O(total cases) with modest constant overhead per case for logging; sorting
is O(n log n) per container (units, atoms within a unit) at run start.
*/
package shield
