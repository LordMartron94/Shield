package internal

import (
	"fmt"
	"sort"
	"time"
)

// --------------------------------------------------------------- GUARD POLICY

type PolicyFlag uint32

const (
	FlagExpectsPanic PolicyFlag = 1 << iota
	FlagExpectsNoPanic
	FlagExpectsError
	FlagExpectsNoError
	FlagEvaluatesOutput
)

func executionPhase(flags PolicyFlag) int {
	if flags&(FlagExpectsPanic|FlagExpectsNoPanic) != 0 {
		return 1
	}
	if flags&(FlagExpectsError|FlagExpectsNoError) != 0 {
		return 2
	}
	return 3
}

type GuardPolicy[TOutput any] struct {
	flags    PolicyFlag
	evaluate func(result executionResult[TOutput]) (passed bool, reason string)
}

func GuardPolicyMustNotPanic[TOutput any]() GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagExpectsNoPanic,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if result.panicked {
				return false, fmt.Sprintf("unexpected panic occurred: %s", result.panicMessage)
			}

			return true, ""
		},
	}
}

func GuardPolicyMustPanic[TOutput any]() GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagExpectsPanic,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if !result.panicked {
				return false, "expected panic, but none occurred"
			}

			return true, ""
		},
	}
}

func GuardPolicyMustNotError[TOutput any]() GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagExpectsNoError,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if result.executionError != nil {
				return false, fmt.Sprintf("unexpected error occurred: %s", result.executionError.Error())
			}

			return true, ""
		},
	}
}

func GuardPolicyMustError[TOutput any]() GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagExpectsError,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if result.executionError == nil {
				return false, "expected error, but none occurred"
			}

			return true, ""
		},
	}
}

func GuardPolicyMustNotEqual[TOutput any](
	comparator func(a, b TOutput) bool,
	formatter func(output TOutput) string,
	notExpected TOutput,
) GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagEvaluatesOutput,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if comparator(result.actual, notExpected) {
				return false, fmt.Sprintf(
					"output must not be %s",
					formatter(notExpected),
				)
			}

			return true, ""
		},
	}
}

func GuardPolicyMustEqual[TOutput any](
	comparator func(a, b TOutput) bool,
	formatter func(output TOutput) string,
	expected TOutput,
) GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagEvaluatesOutput,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			if !comparator(result.actual, expected) {
				return false, fmt.Sprintf(
					"expected %s, got %s",
					formatter(expected),
					formatter(result.actual),
				)
			}

			return true, ""
		},
	}
}

func GuardPolicyPredicate[TOutput any](
	predicate func(actual TOutput) (passed bool, reason string),
) GuardPolicy[TOutput] {
	return GuardPolicy[TOutput]{
		flags: FlagEvaluatesOutput,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			return predicate(result.actual)
		},
	}
}

// --------------------------------------------------------------- GUARDS

type executionResult[TOutput any] struct {
	actual         TOutput
	executionError error

	panicked     bool
	panicMessage string
}

type Guard[TInput, TOutput any] struct {
	name string

	input    TInput
	policies []GuardPolicy[TOutput]
}

func GuardCreate[TInput, TOutput any](
	name string,
	input TInput,
	policies ...GuardPolicy[TOutput],
) Guard[TInput, TOutput] {

	if len(policies) == 0 {
		return injectPoisonPill[TInput, TOutput](name, input, "0 policies provided")
	}

	var totalFlags PolicyFlag
	outputPolicyCount := 0

	for _, p := range policies {
		if p.flags&FlagEvaluatesOutput != 0 {
			outputPolicyCount++
		}
		totalFlags |= p.flags
	}

	if (totalFlags&FlagExpectsPanic != 0) && (totalFlags&FlagExpectsNoPanic != 0) {
		return injectPoisonPill[TInput, TOutput](name, input, "Conflicting policies: MustPanic and MustNotPanic combined")
	}
	if (totalFlags&FlagExpectsError != 0) && (totalFlags&FlagExpectsNoError != 0) {
		return injectPoisonPill[TInput, TOutput](name, input, "Conflicting policies: MustError and MustNotError combined")
	}
	if outputPolicyCount > 1 {
		return injectPoisonPill[TInput, TOutput](name, input, "Conflicting policies: Multiple output evaluations provided")
	}

	if (totalFlags&FlagExpectsPanic != 0) && (totalFlags&FlagEvaluatesOutput != 0) {
		return injectPoisonPill[TInput, TOutput](name, input, "Logical error: Cannot evaluate output of a guard expected to panic")
	}
	if (totalFlags&FlagExpectsError != 0) && (totalFlags&FlagEvaluatesOutput != 0) {
		return injectPoisonPill[TInput, TOutput](name, input, "Logical error: Cannot evaluate output of a guard expected to error")
	}

	if totalFlags&(FlagExpectsPanic|FlagExpectsNoPanic) == 0 {
		policies = append(policies, GuardPolicyMustNotPanic[TOutput]())
	}
	if totalFlags&(FlagExpectsError|FlagExpectsNoError) == 0 && (totalFlags&FlagExpectsPanic == 0) {
		policies = append(policies, GuardPolicyMustNotError[TOutput]())
	}

	sort.SliceStable(policies, func(i, j int) bool {
		weightI := executionPhase(policies[i].flags)
		weightJ := executionPhase(policies[j].flags)
		return weightI < weightJ
	})

	return Guard[TInput, TOutput]{
		name:     name,
		input:    input,
		policies: policies,
	}
}

func injectPoisonPill[TInput, TOutput any](
	name string,
	input TInput,
	reason string,
) Guard[TInput, TOutput] {
	poisonPolicy := GuardPolicy[TOutput]{
		flags: 0,
		evaluate: func(_ executionResult[TOutput]) (bool, string) {
			return false, fmt.Sprintf("FRAMEWORK ERROR: %s", reason)
		},
	}

	return Guard[TInput, TOutput]{
		name:     name,
		input:    input,
		policies: []GuardPolicy[TOutput]{poisonPolicy},
	}
}

// --------------------------------------------------------------- SCENARIO

type Executor[TInput, TOutput any] func(input TInput) (output TOutput, error error)

type Scenario[TInput, TOutput any] struct {
	name string

	guards []Guard[TInput, TOutput]

	executor Executor[TInput, TOutput]
}

func ScenarioCreate[TInput, TOutput any](
	name string,
	guards []Guard[TInput, TOutput],
	executor Executor[TInput, TOutput],
) Scenario[TInput, TOutput] {
	return Scenario[TInput, TOutput]{
		name:     name,
		guards:   guards,
		executor: executor,
	}
}

type GuardEvaluationResult struct {
	guardName string

	passed   bool
	duration time.Duration

	failureReason string
}

/*
Name returns the name of the guard for this result.
*/
func (g *GuardEvaluationResult) Name() string {
	return g.guardName
}

/*
Passed returns whether this guard passed.
*/
func (g *GuardEvaluationResult) Passed() bool {
	return g.passed
}

/*
Duration returns the wall duration for this guard to execute.
*/
func (g *GuardEvaluationResult) Duration() time.Duration {
	return g.duration
}

/*
FailureReason returns the reason for this guard to have failed.
*/
func (g *GuardEvaluationResult) FailureReason() string {
	return g.failureReason
}

type ScenarioRunResult struct {
	// the run result is just how THIS endpoint models its result.
	// this is therefore semantically different from how the engine will store it.
	scenarioName string

	totalDurationWall   time.Duration
	totalDurationSummed time.Duration

	passed bool

	guardResults []GuardEvaluationResult
}

/*
Name returns the scenario name for this result.
*/
func (s *ScenarioRunResult) Name() string {
	return s.scenarioName
}

/*
Passed returns whether this scenario passed completely.
*/
func (s *ScenarioRunResult) Passed() bool {
	return s.passed
}

/*
WallDuration returns the duration it took (wall time) for the scenario to run.
*/
func (s *ScenarioRunResult) WallDuration() time.Duration {
	return s.totalDurationWall
}

/*
SummedDuration returns the duration it took for the scenario to run by summing the durations of its guards.
*/
func (s *ScenarioRunResult) SummedDuration() time.Duration {
	return s.totalDurationSummed
}

/*
GuardResults returns a copy of the guard results.
*/
func (s *ScenarioRunResult) GuardResults() []GuardEvaluationResult {
	cp := make([]GuardEvaluationResult, len(s.guardResults))
	copy(cp, s.guardResults)

	return cp
}

func ScenarioRun[TInput, TOutput any](
	scenario Scenario[TInput, TOutput],
) ScenarioRunResult {
	result := ScenarioRunResult{}
	result.scenarioName = scenario.name

	summedDuration := time.Duration(0)
	guardAmount := len(scenario.guards)
	result.guardResults = make([]GuardEvaluationResult, guardAmount)

	passed := true

	start := time.Now()

	for i := 0; i < guardAmount; i++ {
		guard := scenario.guards[i]
		guardResult := evaluateGuard(guard, scenario.executor)
		result.guardResults[i] = guardResult

		if !guardResult.passed {
			passed = false
		}

		summedDuration += guardResult.duration
	}

	end := time.Now()

	result.passed = passed
	result.totalDurationWall = end.Sub(start)
	result.totalDurationSummed = summedDuration

	return result
}

// --------------------------------------------------------------- PRIVATE HELPERS

func evaluateGuard[TInput, TOutput any](
	guard Guard[TInput, TOutput],
	executor Executor[TInput, TOutput],
) (result GuardEvaluationResult) {
	executionResult := executionResult[TOutput]{}

	defer func() {
		if r := recover(); r != nil {
			executionResult.panicked = true
			executionResult.panicMessage = fmt.Sprintf("%v", r)
		}

		for i := 0; i < len(guard.policies); i++ {
			passed, reason := guard.policies[i].evaluate(executionResult)

			if !passed {
				result.passed = false
				result.failureReason = reason
				break
			}
		}
	}()

	result.guardName = guard.name
	result.passed = true

	start := time.Now()
	output, err := executor(guard.input)
	end := time.Now()
	result.duration = end.Sub(start)

	executionResult.actual = output
	executionResult.executionError = err

	return result
}
