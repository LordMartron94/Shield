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

The module also ships an optional interactive CLI at ./cmd/shield. The CLI intentionally
does not discover domain tests implicitly; instead clients register a builder callback through
CLIHarnessBuilderRegister so the CLI can request a fully-formed Configuration.

After each Run, a structured "Shield run summary" line is logged via echo (field `summary` true)
with aggregate counts from the report tree; when any atoms executed, per-atom wall times are copied
into a manual-memory vector and summarized with statarch (mean, population standard deviation, sum,
five-number min/Q1/median/Q3/max, IQR, p95/p99, and population coefficient of variation only when
there are at least two samples and the mean magnitude exceeds a tiny epsilon—otherwise CV is omitted
to avoid division by zero in statarch). Resolve statarch/memforge via your go.work / submodule layout;
the shield module go.mod does not list those requirements.

Typical flow:

 1. ConfigurationCreate, ConfigurationRegisterUnits with units built via UnitCreate and
    UnitRegisterAtom / UnitRegisterSubUnits.
 2. For each atom: AtomCreate(runner, validator), AtomRegisterCase for each case (via
    CaseCreate or CaseCreateWithoutExpected), optional AtomSetSetupAndTeardown /
    AtomSetDescription.
 3. Optional: UnitSetStopRemainingSubUnitsOnChildFailure / UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits /
    UnitSetEvaluationGate on units that model dependencies.
 4. EngineCreate(configuration), report := Run(engine, runtimeConfiguration); use report.Failed
    and report.TopLevel for exit codes or CI without scraping logs.
 5. Optional: ReportPersistenceCreate(outputDir), ReportPersistenceSetEnabled(true),
    ReportPersistenceSetMode(ReportPersistenceModeJSON|TXT|JSONAndTXT), or ReportPersistenceSetAdapter
    for a custom persister, then RunWithReportPersistence. Persisted JSON uses schema_version
    ReportDocumentSchemaVersion (2 adds extended duration aggregates); see README for filename pattern
    and limitations (no per-atom rows; quartiles/tail percentiles are noisy for very small n).
 6. Optional CLI flow: register CLIHarnessBuilderRegister(func() (*Configuration, error) { ... }),
    then run `go run ./cmd/shield -config=shieldconfig.toml` and execute `run`, `list`, `show`, `delete`, `clean`,
    `baseline`, `help`.

Validators return AtomResult by value. Use *AtomResultSuccessCreate(), *AtomResultFailureCreate(reason),
and the AtomResultSet* helpers when you need notes or to skip remaining atoms in the current unit.

Complexity: Run is O(total cases) with modest constant overhead per case for logging; sorting
is O(n log n) per container (units, atoms within a unit) at run start.
*/
package shield
