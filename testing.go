package shield

import "shield/internal"

/*
This is the testing endpoint for SHIELD.

Features:
- Data-Driven Testing Setup
*/

/*
SHIELD_Testing_Guard represents a single check for a given test.

Alternatively one could think of this as an invariant that must hold true,
or a claim to be evaluated.
*/
type SHIELD_Testing_Guard[TInput, TOutput any] = internal.Guard[TInput, TOutput]

/*
SHIELD_Testing_GuardCreateDefault constructs a default guard that must not panic, must not error,
and the output must be equal to the given output.
*/
func SHIELD_Testing_GuardCreateDefault[TInput, TOutput any](
	name string,
	input TInput,
	expectedOutput TOutput,
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreateDefault(name, input, expectedOutput)
}

/*
SHIELD_Testing_GuardCreateMustPanic constructs a guard that must panic for the given input.

As such it expects no output because there is no output associated with the expected result.
*/
func SHIELD_Testing_GuardCreateMustPanic[TInput, TOutput any](
	name string,
	input TInput,
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreateMustPanic[TInput, TOutput](name, input)
}

/*
SHIELD_Testing_GuardCreateMustError constructs a guard that must error for the given input.

As such it expects no output because there is no output associated with the expected result.
*/
func SHIELD_Testing_GuardCreateMustError[TInput, TOutput any](
	name string,
	input TInput,
) SHIELD_Testing_Guard[TInput, TOutput] {
	return internal.GuardCreateMustError[TInput, TOutput](name, input)
}

/*
SHIELD_Testing_Comparator checks whether two TSubjects are equal.
*/
type SHIELD_Testing_Comparator[TSubject any] = internal.Comparator[TSubject]

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
	evaluator SHIELD_Testing_Comparator[TOutput],
	executor SHIELD_Testing_Executor[TInput, TOutput],
) SHIELD_Testing_Scenario[TInput, TOutput] {
	return internal.ScenarioCreate(name, guards, evaluator, executor)
}

/*
SHIELD_Testing_ScenarioRun runs a scenario and returns its result.
*/
func SHIELD_Testing_ScenarioRun[TInput, TOutput any](
	scenario SHIELD_Testing_Scenario[TInput, TOutput],
) SHIELD_Testing_ScenarioRunResult {
	return internal.ScenarioRun(scenario)
}
