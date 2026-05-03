package shield

import (
	"fmt"
	"foundation"
	"foundation/entropy"
	"shield/internal"
)

/*
This is the testing endpoint for SHIELD.

Features:
- Data-Driven Testing Setup
- Fuzzing system
- SystemIdentity on every run snapshot (storage + regression filters)
- Operation-scoped lifecycle with deferred teardown; framework panics become synthetic ScenarioRunResults
- Snapshot-driven ScenarioRunConfig reconstruction and SQLite-backed replays (`SHIELD_Testing_ScenarioRunReplay*` / `*_FromSnapshot`)
*/

/*
SHIELD_Testing_GuardPolicy defines a single, composable truth rule for a guard.

Policies are evaluated sequentially by the Engine. If a policy fails,
the evaluation halts and the failure reason is recorded in the telemetry.
*/
type SHIELD_Testing_GuardPolicy[TOutput any] = internal.GuardPolicy[TOutput]

/*
SHIELD_Testing_ScenarioRunConfig sets runtime configuration for a scenario.

Identity must carry non-empty Environment and Version strings; SHIELD_Testing_ScenarioRun panics otherwise so every

persisted row carries an explicit system lineage. Identity is copied verbatim into SnapshotConfig on the result.
*/
type SHIELD_Testing_ScenarioRunConfig = internal.ScenarioRunConfig

/*
SHIELD_Testing_SnapshotConfig is the immutable snapshot carried by ScenarioRunResult.SnapshotConfig().

# It bundles seed, fuzzing pattern, iteration or duration caps, UseDuration, entropy provider diagnostic string, and

Identity (environment + version last seen when the run executed). Storage round-trips those fields; replay helpers

project them back into SHIELD_Testing_ScenarioRunConfig via SHIELD_Testing_ScenarioRunConfigFromSnapshot.
*/
type SHIELD_Testing_SnapshotConfig = internal.SnapshotConfig

/*
SHIELD_Testing_InputGenerator generates an input based on a seed and iteration number.

NOTE: In order for this to work properly, the generated input MUST be DETERMINISTIC relative to seed and iteration.
*/
type SHIELD_Testing_InputGenerator[TInput any] = internal.InputGenerator[TInput]

/*
SHIELD_Testing_ZonePath represents a zone (group) of scenarios.

This can be treated as metadata and is not used by the execution engine.
Only the rendering engine and/or downstream systems (can) make use of this.
*/
type SHIELD_Testing_ZonePath = internal.ZonePath

/*
SHIELD_Testing_SystemIdentity scopes telemetry to a deployment context and build label.

Environment names the logical tier (for example ci, staging). Version names the software artefact or commit label you

want regression filters to key on. Both must be non-empty for SHIELD_Testing_ScenarioRun.

Identical pairwise regression requires matching Environment (Version may differ); cohort stability requires every run in a

slice share the same Environment and Version; storage FindByIdentity narrows rows by scenario name plus both fields.
*/
type SHIELD_Testing_SystemIdentity = internal.SystemIdentity

/*
SHIELD_Testing_ZonePathCreate builds a metadata zone path from ordered segment strings.

Segments are preserved in order from outer to inner grouping (for example suite, then module).
The execution engine ignores this path; storage, reports, or other tooling may use it to
navigate or filter scenarios. Passing no parts yields an empty path, which is valid.
*/
func SHIELD_Testing_ZonePathCreate(parts ...string) SHIELD_Testing_ZonePath {
	return internal.ZonePathCreate(parts...)
}

/*
SHIELD_Testing_ZonePathCreateFromString parses a single string into a zone path using separator.

Every substring between occurrences of separator becomes one segment; order matches the source
left to right. For example ("a/b/c", "/") yields the same segments as ZonePathCreate("a", "b", "c").
If path is empty, the result is one empty segment as produced by strings.Split. A non-empty path
that contains no separator becomes a single-segment path. The separator is not trimmed from
individual segments—normalize input if that matters for your tooling.
*/
func SHIELD_Testing_ZonePathCreateFromString(path string, separator string) SHIELD_Testing_ZonePath {
	return internal.ZonePathCreateFromString(path, separator)
}

/*
SHIELD_Testing_GuardPolicyMustNotPanic enforces a Phase 1 (Severity 0) hardware/state trap.

If the executor panics, the Engine will catch it, fail the guard immediately,
and append the recovered panic message to the telemetry.
*/
func SHIELD_Testing_GuardPolicyMustNotPanic[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotPanic[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustPanic enforces an expected failure state.

The guard will only pass if the executor panics. If the executor returns normally
or returns an error, the guard fails.
*/
func SHIELD_Testing_GuardPolicyMustPanic[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustPanic[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustNotError enforces a Phase 2 (Severity 1) logic trap.

If the executor returns a non-nil error, the guard fails and the error string
is recorded. This policy implicitly requires that no panics occur prior to evaluation.
*/
func SHIELD_Testing_GuardPolicyMustNotError[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotError[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustError ensures the executor yields a valid Go error.

The guard fails if the executor returns a nil error.
*/
func SHIELD_Testing_GuardPolicyMustError[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustError[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustNotEqual validates that the output diverges from a banned value.

Because the Engine operates on generic memory, the client must inject the 'comparator'
to define structural equality, and a 'formatter' to translate the generic memory into
a readable string for the telemetry diff if the policy fails.
*/
func SHIELD_Testing_GuardPolicyMustNotEqual[TOutput any](
	comparator func(actual, notExpected TOutput) bool,
	formatter func(output TOutput) string,
	notExpected TOutput,
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotEqual(comparator, formatter, notExpected)
}

/*
SHIELD_Testing_GuardPolicyMustEqual validates that the output matches a strict expectation.

Because the Engine operates on generic memory, the client must inject the 'comparator'
to define structural equality, and a 'formatter' to translate the generic memory into
a readable string (e.g., "expected X, got Y") for the telemetry diff upon failure.
*/
func SHIELD_Testing_GuardPolicyMustEqual[TOutput any](
	comparator func(actual, expected TOutput) bool,
	formatter func(output TOutput) string,
	expected TOutput,
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustEqual(comparator, formatter, expected)
}

/*
SHIELD_Testing_GuardPolicyPredicate serves as the escape hatch for complex DOD evaluations.

Use this when strict equality is insufficient (e.g., checking numeric ranges,
regex matching, or deep nested structural assertions). The predicate must return
false and a contextual reason string if the actual output is invalid.
*/
func SHIELD_Testing_GuardPolicyPredicate[TOutput any](
	predicate func(actual TOutput) (passed bool, reason string),
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyPredicate(predicate)
}

/*
SHIELD_Testing_Guard represents a single check for a given test.

Alternatively one could think of this as an invariant that must hold true,
or a claim to be evaluated.
*/
type SHIELD_Testing_Guard[TInput, TOutput any] = internal.Guard[TInput, TOutput]

/*
SHIELD_Testing_GuardCreate creates a single guard with a single input to be executed on a scenario.
*/
func SHIELD_Testing_GuardCreate[TInput, TOutput any](
	name string,
	input TInput,
	policies ...SHIELD_Testing_GuardPolicy[TOutput],
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreate(name, func(_ internal.FuzzingContext) internal.InputIterator[TInput] {
		return func() TInput {
			return input
		}
	}, false, policies...)
}

/*
SHIELD_Testing_GuardCreate_Fuzzed creates a single guard with a fuzzer to be executed on a scenario.
*/
func SHIELD_Testing_GuardCreate_Fuzzed[TInput, TOutput any](
	name string,
	inputGenerator SHIELD_Testing_InputGenerator[TInput],
	policies ...SHIELD_Testing_GuardPolicy[TOutput],
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreate(name, inputGenerator, true, policies...)
}

/*
SHIELD_Testing_Executor is a single executor which scenarios use to execute.
*/
type SHIELD_Testing_Executor[TInput, TOutput any] = internal.Executor[TInput, TOutput]

/*
SHIELD_Testing_ScenarioRunResult describes a single scenario's result.
*/
type SHIELD_Testing_ScenarioRunResult = internal.ScenarioRunResult

/*
SHIELD_Testing_Scenario represents a "theory" for what is supposed to happen after an execution.

It holds an array of guards to defend a piece of behaviour.
*/
type SHIELD_Testing_Scenario[TInput, TOutput any] = internal.Scenario[TInput, TOutput]

/*
SHIELD_Testing_ScenarioCreate constructs a single scenario ready to run.

Optional trailing zone arguments form the scenario's metadata ZonePath via
SHIELD_Testing_ZonePathCreate (suite/module-style grouping). Omitted zones use an empty path.
Execution does not consume zones; results and integrations expose them through ZonePath accessors.
*/
func SHIELD_Testing_ScenarioCreate[TInput, TOutput any](
	name string,
	guards []SHIELD_Testing_Guard[TInput, TOutput],
	executor SHIELD_Testing_Executor[TInput, TOutput],
	zones ...string,
) SHIELD_Testing_Scenario[TInput, TOutput] {
	zonePath := SHIELD_Testing_ZonePathCreate(zones...)
	return internal.ScenarioCreate(name, guards, executor, zonePath)
}

/*
SHIELD_Testing_ScenarioRun executes scenario under runConfig including mandatory Identity.

Violating Identity invariants (blank Environment or Version) panics as a framework misuse error from the engine.
*/
func SHIELD_Testing_ScenarioRun[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	runConfig SHIELD_Testing_ScenarioRunConfig,
) SHIELD_Testing_ScenarioRunResult {
	return internal.ScenarioRun(scenario, runConfig)
}

/*
SHIELD_Testing_ScenarioRunConfigFromSnapshot materializes a runnable config from a prior SnapshotConfig.

# SeedOverride receives snapshot.Seed so ScenarioRun matches the stored trajectory when combined with the same entropy

factory semantics. FuzzingPattern, MaxIterations or MaxDuration, UseDuration, and Identity copy across directly.

entropyProviderFactory passes through unchanged; nil keeps SHIELD’s default MixSplit128 provider while still fixing the seed.

# SnapshotConfig.ProviderID remains diagnostic only—bit-identical entropy plumbing versus the original run requires a factory

honoring that identifier when your pipeline relies on it.

Identity must stay non-empty exactly as when the snapshot was produced; otherwise ScenarioRun still panics before work begins.
*/
func SHIELD_Testing_ScenarioRunConfigFromSnapshot(
	snapshot SHIELD_Testing_SnapshotConfig,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) SHIELD_Testing_ScenarioRunConfig {
	return internal.ScenarioRunConfigFromSnapshot(snapshot, entropyProviderFactory)
}

/*
SHIELD_Testing_ScenarioRunReplayFromStoredAggregate replays scenario using the snapshot embedded in a persisted row

(or any hydrated aggregate) without touching SQLite again.

stored must be non-nil. entropyProviderFactory follows SHIELD_Testing_ScenarioRunConfigFromSnapshot.

# The callable scenario definition (guards, executor, zone metadata) is supplied independently—this reapplies knobs plus

Identity from stored.SnapshotConfig(). Mismatch between scenario and stored scenario name or shape is intentional for

bisection; Identity from the row is always replayed verbatim.
*/
func SHIELD_Testing_ScenarioRunReplayFromStoredAggregate[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	stored *SHIELD_Testing_ScenarioRunResult,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) (SHIELD_Testing_ScenarioRunResult, error) {
	if stored == nil {
		var zero SHIELD_Testing_ScenarioRunResult
		return zero, fmt.Errorf("stored scenario aggregate is nil")
	}
	runConfig := internal.ScenarioRunConfigFromSnapshot(stored.SnapshotConfig(), entropyProviderFactory)
	return internal.ScenarioRun(scenario, runConfig), nil
}

/*
SHIELD_Testing_ScenarioRunReplayFromStorageByID hydrates persistedResultID via SHIELD_Testing_Storage_ScenarioResultFindByID,

then invokes SHIELD_Testing_ScenarioRunReplayFromStoredAggregate with the fetched row.

Storage errors propagate; the replay result reflects a fresh ScenarioRun invocation under the revived configuration.
*/
func SHIELD_Testing_ScenarioRunReplayFromStorageByID[TInput, TOutput any](
	engine *SHIELD_Testing_Storage_Engine,
	persistedResultID string,
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) (SHIELD_Testing_ScenarioRunResult, error) {
	row, err := SHIELD_Testing_Storage_ScenarioResultFindByID(engine, persistedResultID)
	if err != nil {
		var zero SHIELD_Testing_ScenarioRunResult
		return zero, err
	}
	return SHIELD_Testing_ScenarioRunReplayFromStoredAggregate(scenario, row, entropyProviderFactory)
}

/*
SHIELD_Testing_Operation groups scenarios that share expensive setup state.

After successful startup the runner calls runScenarios, snapshots WallDuration and sums, then runs deferred teardown.

Startup error or panic skips user scenarios yet still records synthetic Operation_Startup_Failure telemetry.

Panic inside runScenarios or teardown is recovered inside the runner, translated into Operation_Execution_Failure

or appended Operation_Teardown_Failure rows, and does not escape to callers of SHIELD_Testing_OperationRun.

Passed is false on any constituent scenario failure or any synthetic fault row. Teardown is skipped when startup never succeeds.

Zone segments mirror ScenarioCreate trailing zones—metadata-only on attribution.
*/
type SHIELD_Testing_Operation[TState any] = internal.Operation[TState]

/*
SHIELD_Testing_OperationRunResult aggregates WallDuration (pre-teardown snapshot), summed scenario durations,

and scenario rows—including synthetic framing rows emitted on framework faults. StartedAt anchors OperationRun entry.
*/
type SHIELD_Testing_OperationRunResult = internal.OperationRunResult

/*
SHIELD_Testing_OperationCreate configures an Operation with lifecycle hooks.

Trailing zone strings map through SHIELD_Testing_ZonePathCreate like ScenarioCreate—metadata only.

When startup returns `(state, nil)` without recovering a panic the runner invokes runScenarios(state); failures or panics therein never escape.

Deferred teardown always runs once startup succeeds, after WallDuration bookkeeping; teardown panic appends synthetic Operation_Teardown_Failure.

runScenarios usually loops SHIELD_Testing_ScenarioRun while reusing or layering run configuration—the config’s Identity

must remain valid on every inner call.
*/
func SHIELD_Testing_OperationCreate[TState any](
	name string,
	startup func() (TState, error),
	teardown func(state TState),
	runScenarios func(state TState) []SHIELD_Testing_ScenarioRunResult,
	zones ...string,
) SHIELD_Testing_Operation[TState] {
	zonePath := SHIELD_Testing_ZonePathCreate(zones...)
	return internal.OperationCreate(name, zonePath, startup, teardown, runScenarios)
}

/*
SHIELD_Testing_OperationRun executes startup, guarded runScenarios, aggregation snapshot, deferred teardown,

and yields OperationRunResult without propagating panics—framework faults materialize as synthetic telemetry instead.

Caller passes `&operation` populated by OperationCreate.
*/
func SHIELD_Testing_OperationRun[TState any](operation *SHIELD_Testing_Operation[TState]) SHIELD_Testing_OperationRunResult {
	return internal.OperationRun(operation)
}
