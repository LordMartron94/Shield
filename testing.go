package shield

import "shield/internal"

/*
This is the testing endpoint for SHIELD.

Features:
- Data-Driven Testing Setup
*/

/*
SHIELD_Testing_GuardPolicy defines a single, composable truth rule for a guard.

Policies are evaluated sequentially by the Engine. If a policy fails,
the evaluation halts and the failure reason is recorded in the telemetry.
*/
type SHIELD_Testing_GuardPolicy[TOutput any] = internal.GuardPolicy[TOutput]

/*
SHIELD_Testing_ScenarioRunConfig sets runtime configuration for a scenario.
*/
type SHIELD_Testing_ScenarioRunConfig = internal.ScenarioRunConfig

/*
SHIELD_Testing_InputGenerator generates an input based on a seed and iteration number.

NOTE: In order for this to work properly, the generated input MUST be DETERMINISTIC relative to seed and iteration.
*/
type SHIELD_Testing_InputGenerator[TInput any] = internal.InputGenerator[TInput]

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
	return internal.GuardCreate(name, func(_, _ uint64) TInput {
		return input
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
SHIELD_Testing_ScenarioCreate constructs a single scenario to be ran.
*/
func SHIELD_Testing_ScenarioCreate[TInput, TOutput any](
	name string,
	guards []SHIELD_Testing_Guard[TInput, TOutput],
	executor SHIELD_Testing_Executor[TInput, TOutput],
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioCreate(name, guards, executor)
}

/*
SHIELD_Testing_ScenarioRun runs a scenario and returns its result.
*/
func SHIELD_Testing_ScenarioRun[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
	runConfig SHIELD_Testing_ScenarioRunConfig,
) SHIELD_Testing_ScenarioRunResult {
	return internal.ScenarioRun(scenario, runConfig)
}
