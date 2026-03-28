/*
Package shield is a structured validation and regression harness.

It organizes work into units (coarse groupings, often mirroring a subsystem) and atoms
(ordered checks inside a unit). Each atom pairs a runner function with a validator and
one or more cases. Execution order is deterministic: units and atoms are sorted by an
explicit integer order field; cases run in registration order.

Runtime filtering uses name-based blacklists for units and atoms (see RuntimeConfigurationCreate
and docs/adr/0001-Runtime-Configuration-Filtering.md). There is no implicit discovery: you
build a Configuration, register units, then Run.

Logging is emitted through the echo subsystem registered by this module (prefix "Shield").
Failures in runner or validator paths are recovered, logged, and do not crash the process.

Typical flow:

 1. ConfigurationCreate, ConfigurationRegisterUnits with units built via UnitCreate and
    UnitRegisterAtom / UnitRegisterSubUnits.
 2. For each atom: AtomCreate(runner, validator), AtomRegisterCase for each case, optional
    AtomSetSetupAndTeardown / AtomSetDescription.
 3. EngineCreate(configuration), Run(engine, runtimeConfiguration).

Validators return AtomResult by value. Use *AtomResultSuccessCreate(), *AtomResultFailureCreate(reason),
and the AtomResultSet* helpers when you need notes or to skip remaining atoms in the current unit.

Complexity: Run is O(total cases) with modest constant overhead per case for logging; sorting
is O(n log n) per container (units, atoms within a unit) at run start.
*/
package shield
