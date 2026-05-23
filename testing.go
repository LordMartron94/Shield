package shield

import (
	"fmt"
	"foundation"
	"foundation/entropy"
	"shield/internal"
)

/*
This file hosts SHIELD testing facades spanning data-driven scenarios, fuzzing, runner-supplied execution identity snapshots,

Operation lifecycles, and SQLite-backed replay bundles.
*/

/*
SHIELD_Testing_GuardPolicy defines composable evaluation rules enforced sequentially until failure halts telemetry capture.
*/
type SHIELD_Testing_GuardPolicy[TOutput any] = internal.GuardPolicy[TOutput]

/*
SHIELD_Testing_ScenarioRunConfig carries ScenarioRun knobs controlled by test authors (fuzzing, entropy, effort limits).
*/
type SHIELD_Testing_ScenarioRunConfig = internal.ScenarioRunConfig

/*
SHIELD_Testing_SnapshotConfig is the immutable ScenarioRun telemetry bundle exposing seed, fuzz pattern, scheduling limits,

entropy provider tagging, plus Identity fingerprints for storage, regression validators, rendering, and replay reconstruction.
*/
type SHIELD_Testing_SnapshotConfig = internal.SnapshotConfig

/*
SHIELD_Testing_InputGenerator derives deterministic Scenario inputs keyed by entropy seed plus iteration ordinal.

Generators must stay deterministic versus those inputs—non-deterministic branches break replay guarantees outright.
*/
type SHIELD_Testing_InputGenerator[TInput any] = internal.InputGenerator[TInput]

/*
SHIELD_Testing_ZonePath attaches metadata breadcrumbs (suite/module style) untouched by Scenario execution yet consumed by renders,

SQLite zone_path columns, Operation attribution, etc.
*/
type SHIELD_Testing_ZonePath = internal.ZonePath

/*
SHIELD_Testing_SystemIdentity couples Environment tier labels (such as staging) with Version strings labeling software artifacts.

Both strings must populate ScenarioRun configs. Pairwise regressions insist on Environment alignment while tolerating differing Versions;

storage identity queries and cohort statistics require homogeneous env+Version slices alongside SHIELD_Testing_Storage_GetCohortVersions discovery.
*/
type SHIELD_Testing_SystemIdentity = internal.SystemIdentity
type SHIELD_Testing_ExecutionContext = internal.ExecutionContext

/*
SHIELD_Testing_ZonePathCreate stitches ordered zone segments into a metadata path (empty parts remain valid sentinel paths).
*/
func SHIELD_Testing_ZonePathCreate(parts ...string) SHIELD_Testing_ZonePath {
	return internal.ZonePathCreate(parts...)
}

/*
SHIELD_Testing_ZonePathCreateFromString splits path on separator much like strings.Split—empty input yields a single empty segment,

non-empty strings without separators become one segment, and trimming never happens automatically.
*/
func SHIELD_Testing_ZonePathCreateFromString(path string, separator string) SHIELD_Testing_ZonePath {
	return internal.ZonePathCreateFromString(path, separator)
}

/*
SHIELD_Testing_GuardPolicyMustNotPanic traps unexpected panics as guard failures with the panic value and stack trace.
*/
func SHIELD_Testing_GuardPolicyMustNotPanic[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotPanic[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustPanic expects panics; normal returns or errors fail the guard.
*/
func SHIELD_Testing_GuardPolicyMustPanic[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustPanic[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustNotError fails when executors return non-nil errors (post-panic phase).
*/
func SHIELD_Testing_GuardPolicyMustNotError[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotError[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustError forces a non-nil Go error from the executor.
*/
func SHIELD_Testing_GuardPolicyMustError[TOutput any]() SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustError[TOutput]()
}

/*
SHIELD_Testing_GuardPolicyMustNotEqual rejects outputs matching notExpected using caller-provided structural equality plus string

formatters for telemetry diffs.
*/
func SHIELD_Testing_GuardPolicyMustNotEqual[TOutput any](
	comparator func(actual, notExpected TOutput) bool,
	formatter func(output TOutput) string,
	notExpected TOutput,
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustNotEqual(comparator, formatter, notExpected)
}

/*
SHIELD_Testing_GuardPolicyMustEqual mirrors MustNotEqual but asserts equality with expected outputs using the same comparator/formatter contract.
*/
func SHIELD_Testing_GuardPolicyMustEqual[TOutput any](
	comparator func(actual, expected TOutput) bool,
	formatter func(output TOutput) string,
	expected TOutput,
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyMustEqual(comparator, formatter, expected)
}

/*
SHIELD_Testing_GuardPolicyPredicate provides arbitrary property checks returning pass/fail plus textual rationale.
*/
func SHIELD_Testing_GuardPolicyPredicate[TOutput any](
	predicate func(actual TOutput) (passed bool, reason string),
) SHIELD_Testing_GuardPolicy[TOutput] {
	return internal.GuardPolicyPredicate(predicate)
}

/*
SHIELD_Testing_Guard binds policies to generated inputs for a named defensive claim.
*/
type SHIELD_Testing_Guard[TInput, TOutput any] = internal.Guard[TInput, TOutput]

/*
SHIELD_Testing_GuardCreate configures a deterministic single-input guard; SHIELD_Testing_GuardCreate_Fuzzed swaps in generative iterators.
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

func SHIELD_Testing_GuardCreate_Fuzzed[TInput, TOutput any](
	name string,
	inputGenerator SHIELD_Testing_InputGenerator[TInput],
	policies ...SHIELD_Testing_GuardPolicy[TOutput],
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreate(name, inputGenerator, true, policies...)
}

/*
SHIELD_Testing_Executor is Scenario-scoped runnable logic feeding guard evaluation.
*/
type SHIELD_Testing_Executor[TInput, TOutput any] = internal.Executor[TInput, TOutput]

/*
SHIELD_Testing_ScenarioRunResult captures telemetry for one Scenario invocation.
*/
type SHIELD_Testing_ScenarioRunResult = internal.ScenarioRunResult

/*
SHIELD_Testing_Scenario groups guards protecting a single executor under Scenario metadata (name only).
*/
type SHIELD_Testing_Scenario[TInput, TOutput any] = internal.Scenario[TInput, TOutput]

/*
SHIELD_Testing_ScenarioCreate builds Scenario definitions without geographical metadata.

Description is required at creation time; pass an explicit empty string if intentionally omitted.
*/
func SHIELD_Testing_ScenarioCreate[TInput, TOutput any](
	name string,
	description string,
	guards []SHIELD_Testing_Guard[TInput, TOutput],
	executor SHIELD_Testing_Executor[TInput, TOutput],
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioCreate(name, description, guards, executor)
}

/*
SHIELD_Testing_ScenarioCreateWithGuardIsolation builds a scenario with per-guard subprocess isolation configured at creation time.
*/
func SHIELD_Testing_ScenarioCreateWithGuardIsolation[TInput, TOutput any](
	name string,
	description string,
	guards []SHIELD_Testing_Guard[TInput, TOutput],
	executor SHIELD_Testing_Executor[TInput, TOutput],
	guardIsolation SHIELD_Testing_ScenarioGuardIsolation,
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioCreateWithGuardIsolation(name, description, guards, executor, guardIsolation)
}

/*
SHIELD_Testing_ScenarioGuardIsolation configures which guards run in a subprocess (see internal.ScenarioGuardIsolation).
*/
type SHIELD_Testing_ScenarioGuardIsolation = internal.ScenarioGuardIsolation

/*
SHIELD_Testing_ScenarioGuardIsolationAllGuards marks every guard in the scenario for subprocess isolation.
*/
func SHIELD_Testing_ScenarioGuardIsolationAllGuards() SHIELD_Testing_ScenarioGuardIsolation {
	return internal.ScenarioGuardIsolationAllGuards()
}

/*
SHIELD_Testing_ScenarioGuardIsolationPerGuard marks only the named guards for subprocess isolation.
*/
func SHIELD_Testing_ScenarioGuardIsolationPerGuard(perGuard map[string]bool) SHIELD_Testing_ScenarioGuardIsolation {
	return internal.ScenarioGuardIsolationPerGuard(perGuard)
}

/*
SHIELD_Testing_ScenarioGuardIsolationAllGuardsSet configures subprocess isolation for all guards on scenario.
*/
func SHIELD_Testing_ScenarioGuardIsolationAllGuardsSet[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	enabled bool,
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioGuardIsolationAllGuardsSet(scenario, enabled)
}

/*
SHIELD_Testing_ScenarioGuardIsolationPerGuardSet configures subprocess isolation for specific guards on scenario.
*/
func SHIELD_Testing_ScenarioGuardIsolationPerGuardSet[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	perGuard map[string]bool,
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioGuardIsolationPerGuardSet(scenario, perGuard)
}

/*
SHIELD_Testing_OperationStateCodec serializes operation state across subprocess guard boundaries.

Deserialize builds cold state in isolated child processes. Apply merges portable snapshot fields into
live operation state after scenarios finish without discarding in-process resources.
*/
type SHIELD_Testing_OperationStateCodec[TState any] = internal.OperationStateCodec[TState]

/*
SHIELD_Testing_GuardFailureClass classifies guard failures (policy, panic, critical subprocess crash, framework).
*/
type SHIELD_Testing_GuardFailureClass = internal.GuardFailureClass

const (
	SHIELD_Testing_GuardFailureClassPolicy    = internal.GuardFailureClassPolicy
	SHIELD_Testing_GuardFailureClassPanic     = internal.GuardFailureClassPanic
	SHIELD_Testing_GuardFailureClassCritical  = internal.GuardFailureClassCritical
	SHIELD_Testing_GuardFailureClassFramework = internal.GuardFailureClassFramework
)

/*
SHIELD_Testing_OperationRunScenario executes a scenario inside an operation callback using the callback's execution context.
*/
func SHIELD_Testing_OperationRunScenario[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	execCtx SHIELD_Testing_ExecutionContext,
	runConfig SHIELD_Testing_ScenarioRunConfig,
) SHIELD_Testing_ScenarioRunResult {
	return internal.ScenarioRun(scenario, execCtx, runConfig)
}

/*
SHIELD_Testing_ScenarioRunConfigFromSnapshot rebuilds ScenarioRunConfig from Snapshot mirrors: SeedOverride copies snapshot.Seed,

fuzz limits flow unchanged, entropy factories pass through (nil keeps MixSplit defaults).

ProviderID stays diagnostic—recreate original entropy plumbing via custom factories when fidelity demands it.
*/
func SHIELD_Testing_ScenarioRunConfigFromSnapshot(
	snapshot SHIELD_Testing_SnapshotConfig,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) SHIELD_Testing_ScenarioRunConfig {
	return internal.ScenarioRunConfigFromSnapshot(snapshot, entropyProviderFactory)
}

/*
SHIELD_Testing_Operation models shared setup across multiple Scenario executions—startup allocates state, runScenarios fans out,

defer’d teardown executes even when inner batches panic internally, emitting synthetic ScenarioRun scaffolding on faults.
*/
type SHIELD_Testing_Operation[TState any] = internal.Operation[TState]

/*
SHIELD_Testing_OperationRunResult aggregates nested Scenario summaries plus framing synthetic rows on framework anomalies.
*/
type SHIELD_Testing_OperationRunResult = internal.OperationRunResult

/*
SHIELD_Testing_OperationCreate registers lifecycle closures plus trailing zone segments like ScenarioCreate.

Description is required at creation time; pass an explicit empty string if intentionally omitted.
*/
func SHIELD_Testing_OperationCreate[TState any](
	name string,
	description string,
	startup func() (TState, error),
	teardown func(state TState),
	runScenarios func(state TState, execCtx SHIELD_Testing_ExecutionContext) []SHIELD_Testing_ScenarioRunResult,
	stateCodec *SHIELD_Testing_OperationStateCodec[TState],
	zones ...string,
) SHIELD_Testing_Operation[TState] {
	zonePath := SHIELD_Testing_ZonePathCreate(zones...)
	return internal.OperationCreate(name, description, zonePath, startup, teardown, runScenarios, stateCodec)
}

/*
SHIELD_Testing_OperationCreateStateless is like SHIELD_Testing_OperationCreate except it doesn't use state.

Description is required at creation time; pass an explicit empty string if intentionally omitted.
*/
func SHIELD_Testing_OperationCreateStateless(
	name string,
	description string,
	runScenarios func(_ struct{}, execCtx SHIELD_Testing_ExecutionContext) []SHIELD_Testing_ScenarioRunResult,
	zones ...string,
) SHIELD_Testing_Operation[struct{}] {
	zonePath := SHIELD_Testing_ZonePathCreate(zones...)
	return internal.OperationCreate(
		name, description, zonePath,
		func() (struct{}, error) {
			return struct{}{}, nil
		},
		func(_ struct{}) {},
		runScenarios,
		nil,
	)
}

/*
SHIELD_Testing_OperationRun executes guarded startup/scenario/teardown choreography returning OperationRun aggregates without bubbling panics to callers.
*/
func SHIELD_Testing_OperationRun[TState any](
	operation *SHIELD_Testing_Operation[TState],
	execCtx SHIELD_Testing_ExecutionContext,
) SHIELD_Testing_OperationRunResult {
	return internal.OperationRun(operation, execCtx)
}

/*
SHIELD_Testing_OperationRunReplayFromStoredAggregate replays a scenario through an operation-scoped execution path.
*/
func SHIELD_Testing_OperationRunReplayFromStoredAggregate[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	execCtx SHIELD_Testing_ExecutionContext,
	stored *SHIELD_Testing_ScenarioRunResult,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) (SHIELD_Testing_OperationRunResult, error) {
	if stored == nil {
		var zero SHIELD_Testing_OperationRunResult
		return zero, fmt.Errorf("stored scenario aggregate is nil")
	}

	runConfig := internal.ScenarioRunConfigFromSnapshot(stored.SnapshotConfig(), entropyProviderFactory)

	op := SHIELD_Testing_OperationCreateStateless(
		"replay_operation",
		"",
		func(_ struct{}, opCtx SHIELD_Testing_ExecutionContext) []SHIELD_Testing_ScenarioRunResult {
			return []SHIELD_Testing_ScenarioRunResult{
				internal.ScenarioRun(scenario, opCtx, runConfig),
			}
		},
	)

	return internal.OperationRun(&op, execCtx), nil
}

/*
SHIELD_Testing_OperationRunReplayFromStorageByID composes ScenarioResultFindByID with OperationRunReplayFromStoredAggregate.
*/
func SHIELD_Testing_OperationRunReplayFromStorageByID[TInput, TOutput any](
	engine *SHIELD_Testing_Storage_Engine,
	persistedResultID string,
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	execCtx SHIELD_Testing_ExecutionContext,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) (SHIELD_Testing_OperationRunResult, error) {
	row, err := SHIELD_Testing_Storage_ScenarioResultFindByID(engine, persistedResultID)
	if err != nil {
		var zero SHIELD_Testing_OperationRunResult
		return zero, err
	}
	return SHIELD_Testing_OperationRunReplayFromStoredAggregate(scenario, execCtx, row, entropyProviderFactory)
}
