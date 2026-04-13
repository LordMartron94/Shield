package internal

import (
	"fmt"
	"time"
)

// --------------------------------------------------------------- GUARDS

type GuardOptions struct {
	mustPanic bool
	mustError bool
}

type Guard[TInput, TOutput any] struct {
	name string

	input          TInput
	expectedOutput TOutput

	options GuardOptions
}

func GuardCreateDefault[TInput, TOutput any](
	name string,
	input TInput,
	expectedOutput TOutput,
) Guard[TInput, TOutput] {
	return Guard[TInput, TOutput]{
		name:           name,
		input:          input,
		expectedOutput: expectedOutput,
		options:        GuardOptions{},
	}
}

func GuardCreateMustPanic[TInput, TOutput any](
	name string,
	input TInput,
) Guard[TInput, TOutput] {
	var zero TOutput

	return Guard[TInput, TOutput]{
		name:           name,
		input:          input,
		expectedOutput: zero,
		options: GuardOptions{
			mustPanic: true,
		},
	}
}

func GuardCreateMustError[TInput, TOutput any](
	name string,
	input TInput,
) Guard[TInput, TOutput] {
	var zero TOutput

	return Guard[TInput, TOutput]{
		name:           name,
		input:          input,
		expectedOutput: zero,
		options: GuardOptions{
			mustError: true,
		},
	}
}

// --------------------------------------------------------------- SCENARIO

type Comparator[TSubject any] func(a, b TSubject) bool

type Executor[TInput, TOutput any] func(input TInput) (output TOutput, error error)

type Scenario[TInput, TOutput any] struct {
	name string

	guards     []Guard[TInput, TOutput]
	comparator Comparator[TOutput]

	executor Executor[TInput, TOutput]
}

func ScenarioCreate[TInput, TOutput any](
	name string,
	guards []Guard[TInput, TOutput],
	comparator Comparator[TOutput],
	executor Executor[TInput, TOutput],
) Scenario[TInput, TOutput] {
	return Scenario[TInput, TOutput]{
		name:       name,
		guards:     guards,
		comparator: comparator,
		executor:   executor,
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
		guardResult := evaluateGuard(guard, scenario.comparator, scenario.executor)
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
	comparator Comparator[TOutput],
	executor Executor[TInput, TOutput],
) (result GuardEvaluationResult) {
	defer func() {
		if r := recover(); r != nil {
			if !guard.options.mustPanic {
				result.passed = false
				result.failureReason = fmt.Sprintf("unexpected panic: %v", r)
			}
		}
	}()

	result.guardName = guard.name
	result.passed = true

	start := time.Now()
	output, err := executor(guard.input)
	end := time.Now()
	result.duration = end.Sub(start)

	if guard.options.mustError {
		if err == nil {
			result.passed = false
			result.failureReason = "expected error, got none"
		}
	}

	if !guard.options.mustPanic { // we must check for this, because if mustPanic is true, then TOutput is zero value
		equal := comparator(output, guard.expectedOutput)
		if !equal {
			result.passed = false
			result.failureReason = "output shape does not match expected shape"
		}
	}

	return result
}
