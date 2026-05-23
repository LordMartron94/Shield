package internal

import (
	"essence"
	"fmt"
	"foundation"
	"foundation/entropy"
	"foundation/formatting"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"
)

/*
shieldTestingRecoveredPanicFormat turns a recovered panic value into a diagnostic string with stack trace.

Called from defer recover() handlers so debug.Stack reflects the panicking goroutine.
*/
func shieldTestingRecoveredPanicFormat(recovered any) string {
	return fmt.Sprintf("%v\n\n%s", recovered, string(debug.Stack()))
}

// --------------------------------------------------------------- GUARD POLICY

type PolicyFlag uint32

const (
	FlagExpectsPanic PolicyFlag = 1 << iota
	FlagExpectsNoPanic
	FlagExpectsError
	FlagExpectsNoError
	FlagEvaluatesStaticOutput
	FlagEvaluatesProperty
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
				return false, fmt.Sprintf("unexpected panic occurred:\n%s", result.panicMessage)
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
		flags: FlagEvaluatesStaticOutput,
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
		flags: FlagEvaluatesStaticOutput,
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
		flags: FlagEvaluatesProperty,
		evaluate: func(result executionResult[TOutput]) (passed bool, reason string) {
			return predicate(result.actual)
		},
	}
}

// --------------------------------------------------------------- FUZZING

type FuzzingPattern uint8

const (
	FuzzingPattern_None FuzzingPattern = iota + 1
	FuzzingPattern_EdgeCases
	FuzzingPattern_Standard
	FuzzingPattern_Adversarial
)

// --------------------------------------------------------------- GUARDS

type executionResult[TOutput any] struct {
	actual         TOutput
	executionError error

	panicked     bool
	panicMessage string
}

type FuzzingContext struct {
	EntropyProvider *entropy.EntropyProvider
	Effort          FuzzingPattern
}

type InputIterator[TInput any] func() TInput

type InputGenerator[TInput any] func(ctx FuzzingContext) InputIterator[TInput]

type Guard[TInput, TOutput any] struct {
	name string

	inputGenerator InputGenerator[TInput]
	policies       []GuardPolicy[TOutput]

	isPoisoned   bool
	poisonReason string
}

func GuardCreate[TInput, TOutput any](
	name string,
	generator InputGenerator[TInput],
	isFuzzer bool,
	policies ...GuardPolicy[TOutput],
) Guard[TInput, TOutput] {
	if poisonReason := validateGuardPolicies(isFuzzer, policies); poisonReason != "" {
		return injectPoisonPill[TInput, TOutput](name, generator, poisonReason)
	}

	policies = applyDefaultPolicies(policies)

	sort.SliceStable(policies, func(i, j int) bool {
		return executionPhase(policies[i].flags) < executionPhase(policies[j].flags)
	})

	return Guard[TInput, TOutput]{
		name:           name,
		inputGenerator: generator,
		policies:       policies,
		isPoisoned:     false,
	}
}

func validateGuardPolicies[TOutput any](isFuzzer bool, policies []GuardPolicy[TOutput]) string {
	if len(policies) == 0 {
		return "0 policies provided"
	}

	var totalFlags PolicyFlag
	var staticOutputCount, propertyCount int

	for _, p := range policies {
		totalFlags |= p.flags
		if p.flags&FlagEvaluatesStaticOutput != 0 {
			staticOutputCount++
		}
		if p.flags&FlagEvaluatesProperty != 0 {
			propertyCount++
		}
	}

	if (totalFlags&FlagExpectsPanic != 0) && (totalFlags&FlagExpectsNoPanic != 0) {
		return "Conflicting policies: MustPanic and MustNotPanic combined"
	}
	if (totalFlags&FlagExpectsError != 0) && (totalFlags&FlagExpectsNoError != 0) {
		return "Conflicting policies: MustError and MustNotError combined"
	}
	if (staticOutputCount + propertyCount) > 1 {
		return "Conflicting policies: Multiple output evaluations provided"
	}
	if (totalFlags&FlagExpectsPanic != 0) && (staticOutputCount+propertyCount > 0) {
		return "Logical error: Cannot evaluate output of a guard expected to panic"
	}
	if (totalFlags&FlagExpectsError != 0) && (staticOutputCount+propertyCount > 0) {
		return "Logical error: Cannot evaluate output of a guard expected to error"
	}

	if isFuzzer && staticOutputCount > 0 {
		return "Logical error: Generative fuzzing guards cannot use static equality policies (MustEqual/MustNotEqual). Use Predicate."
	}

	return ""
}

func applyDefaultPolicies[TOutput any](policies []GuardPolicy[TOutput]) []GuardPolicy[TOutput] {
	var totalFlags PolicyFlag
	for _, p := range policies {
		totalFlags |= p.flags
	}

	if totalFlags&(FlagExpectsPanic|FlagExpectsNoPanic) == 0 {
		policies = append(policies, GuardPolicyMustNotPanic[TOutput]())
	}
	if totalFlags&(FlagExpectsError|FlagExpectsNoError) == 0 && (totalFlags&FlagExpectsPanic == 0) {
		policies = append(policies, GuardPolicyMustNotError[TOutput]())
	}

	return policies
}

func injectPoisonPill[TInput, TOutput any](
	name string,
	inputGenerator InputGenerator[TInput],
	reason string,
) Guard[TInput, TOutput] {
	return Guard[TInput, TOutput]{
		name:           name,
		inputGenerator: inputGenerator,
		policies:       nil,
		isPoisoned:     true,
		poisonReason:   reason,
	}
}

// --------------------------------------------------------------- GROUPING METADATA

type ZonePath struct {
	parts []string
}

func ZonePathCreate(parts ...string) ZonePath {
	return ZonePath{
		parts: parts,
	}
}

func ZonePathCreateFromString(path string, separator string) ZonePath {
	parts := strings.Split(path, separator)
	return ZonePathCreate(parts...)
}

/*
Add appends one or multiple parts to the ZonePath, returning a new path.
*/
func (z ZonePath) Add(parts ...string) ZonePath {
	return ZonePathCreate(slices.Concat(z.parts, parts)...)
}

/*
Parts returns a copy of the zonepath's parts.
*/
func (z ZonePath) Parts() []string {
	return slices.Clone(z.parts)
}

/*
Render renders the path using the provided separator.
*/
func (z ZonePath) Render(separator string) string {
	return formatting.FormatStringSlice(z.parts, formatting.FormatSliceOptions[string]{
		Separator: separator,
		Prefix:    "",
		Suffix:    "",
	})
}

// --------------------------------------------------------------- SCENARIO

/*
SystemIdentity tags who produced a ScenarioRun snapshot: logical deployment tier (Environment) plus build/software Version.

Both strings must be non-empty before ScenarioRun accepts a ScenarioRunConfig; they persist through storage and regressions compare environments for identical runs while stability cohorts require homogeneous env+version.
*/
type SystemIdentity struct {
	Version     string
	Environment string
}

type ExecutionContext struct {
	Identity SystemIdentity
	ZonePath ZonePath

	Runtime             *TestingRuntimeContext
	IsolatedGuardTarget *IsolatedGuardTarget
}

type Executor[TInput, TOutput any] func(input TInput) (output TOutput, error error)

type Scenario[TInput, TOutput any] struct {
	name        string
	description string

	guards []Guard[TInput, TOutput]

	executor Executor[TInput, TOutput]

	guardIsolation ScenarioGuardIsolation
}

func ScenarioCreate[TInput, TOutput any](
	name string,
	description string,
	guards []Guard[TInput, TOutput],
	executor Executor[TInput, TOutput],
) Scenario[TInput, TOutput] {
	return Scenario[TInput, TOutput]{
		name:        name,
		description: description,
		guards:      guards,
		executor:    executor,
	}
}

/*
ScenarioCreateWithGuardIsolation builds a scenario and applies guard subprocess isolation at creation time.
*/
func ScenarioCreateWithGuardIsolation[TInput, TOutput any](
	name string,
	description string,
	guards []Guard[TInput, TOutput],
	executor Executor[TInput, TOutput],
	guardIsolation ScenarioGuardIsolation,
) Scenario[TInput, TOutput] {
	scenario := ScenarioCreate(name, description, guards, executor)
	scenario.guardIsolation = guardIsolation
	return scenario
}

type GuardFailureClass string

const (
	GuardFailureClassPolicy    GuardFailureClass = "policy"
	GuardFailureClassPanic     GuardFailureClass = "panic"
	GuardFailureClassCritical  GuardFailureClass = "critical"
	GuardFailureClassFramework GuardFailureClass = "framework"
)

type GuardEvaluationResult struct {
	guardName string
	passed    bool
	duration  time.Duration

	failedSeed      essence.UUID
	failedIteration uint64
	failureReason   string
	failureClass    GuardFailureClass
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
	if g.passed {
		return ""
	}
	prefix := ""
	if g.failureClass == GuardFailureClassCritical {
		prefix = "CRITICAL: "
	}
	return fmt.Sprintf("Failed at Seed %s, Iteration %d: %s%s", g.failedSeed.String(), g.failedIteration, prefix, g.failureReason)
}

/*
FailureClass returns how the guard failed (policy, panic, critical subprocess crash, framework).
*/
func (g *GuardEvaluationResult) FailureClass() GuardFailureClass {
	return g.failureClass
}

type ScenarioRunResult struct {
	// the run result is just how THIS endpoint models its result.
	// this is therefore semantically different from how the engine will store it.
	scenarioName string
	description  string
	zonePath     ZonePath

	startedAt time.Time

	totalDurationWall   time.Duration
	totalDurationSummed time.Duration

	passed bool

	guardResults []GuardEvaluationResult

	runConfig SnapshotConfig
}

type SnapshotConfig struct {
	Seed           essence.UUID
	FuzzingPattern FuzzingPattern
	MaxIterations  uint64
	MaxDuration    time.Duration
	UseDuration    bool
	ProviderID     string
	Identity       SystemIdentity // persisted environment + version echo for regression + rendering context
}

/*
Name returns the scenario name for this result.
*/
func (s *ScenarioRunResult) Name() string {
	return s.scenarioName
}

/*
Description returns in-memory scenario description metadata.
*/
func (s *ScenarioRunResult) Description() string {
	return s.description
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

/*
SnapshotConfig returns the immutable snapshot for this scenario run—fuzz knobs, entropy provider tag, SystemIdentity lineage.
*/
func (s *ScenarioRunResult) SnapshotConfig() SnapshotConfig {
	return s.runConfig
}

/*
ZonePath returns the zonepath of the scenario at the time this was ran.
*/
func (s *ScenarioRunResult) ZonePath() ZonePath {
	return s.zonePath
}

/*
StartedAt returns when this scenario run started.
*/
func (s *ScenarioRunResult) StartedAt() time.Time {
	return s.startedAt
}

type ScenarioRunConfig struct {
	SeedOverride *essence.UUID

	EntropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string)

	FuzzingPattern FuzzingPattern

	MaxIterations uint64
	MaxDuration   time.Duration
	UseDuration   bool
}

func ScenarioRunConfigFromSnapshot(
	snapshot SnapshotConfig,
	entropyProviderFactory func(seed foundation.Uint128) (provider *entropy.EntropyProvider, id string),
) ScenarioRunConfig {
	return ScenarioRunConfig{
		SeedOverride:           &snapshot.Seed,
		FuzzingPattern:         snapshot.FuzzingPattern,
		MaxIterations:          snapshot.MaxIterations,
		MaxDuration:            snapshot.MaxDuration,
		UseDuration:            snapshot.UseDuration,
		EntropyProviderFactory: entropyProviderFactory,
	}
}

func ScenarioRun[TInput, TOutput any](
	scenario Scenario[TInput, TOutput],
	execCtx ExecutionContext,
	config ScenarioRunConfig,
) ScenarioRunResult {
	if execCtx.Identity.Version == "" || execCtx.Identity.Environment == "" {
		panic("engine error: execution context identity must be set")
	}
	if len(execCtx.ZonePath.parts) == 0 {
		panic("engine error: standalone scenario execution is unsupported (missing execution context zone path)")
	}

	seed, _ := essence.UUIDv7GenerateRandom()
	if config.SeedOverride != nil {
		seed = *config.SeedOverride
	}
	if execCtx.Runtime != nil && execCtx.Runtime.ScenarioSeedOverride != nil {
		seed = *execCtx.Runtime.ScenarioSeedOverride
	}

	var entropyProvider *entropy.EntropyProvider
	var entropyProviderID string

	if config.EntropyProviderFactory == nil {
		entropyProvider = entropy.EntropyProviderCreateMixSplit128(seed.ToUint128())
		entropyProviderID = "shield:default-mixsplit-128-compressed"
	} else {
		provider, name := config.EntropyProviderFactory(seed.ToUint128())
		entropyProvider = provider
		entropyProviderID = name
	}

	if config.FuzzingPattern == 0 {
		config.FuzzingPattern = FuzzingPattern_None
	}

	result := ScenarioRunResult{
		scenarioName: scenario.name,
		description:  scenario.description,
		passed:       true,
		guardResults: make([]GuardEvaluationResult, len(scenario.guards)),
		runConfig: SnapshotConfig{
			Seed:           seed,
			FuzzingPattern: config.FuzzingPattern,
			MaxIterations:  config.MaxIterations,
			MaxDuration:    config.MaxDuration,
			UseDuration:    config.UseDuration,
			ProviderID:     entropyProviderID,
			Identity:       execCtx.Identity,
		},
		zonePath: execCtx.ZonePath,
	}

	start := time.Now()
	var summedDuration time.Duration

	if target := execCtx.IsolatedGuardTarget; target != nil {
		if target.ScenarioName != scenario.name {
			result.passed = true
			result.guardResults = []GuardEvaluationResult{}
			result.totalDurationWall = time.Since(start)
			result.startedAt = start
			return result
		}
		for _, guard := range scenario.guards {
			if guard.name != target.GuardName {
				continue
			}
			guardResult := evaluateGuard(guard, scenario, scenario.executor, seed, config, entropyProvider, execCtx)
			result.guardResults = []GuardEvaluationResult{guardResult}
			if !guardResult.passed {
				result.passed = false
			}
			result.totalDurationWall = time.Since(start)
			result.totalDurationSummed = guardResult.duration
			result.startedAt = start
			return result
		}
		result.passed = false
		result.guardResults = []GuardEvaluationResult{{
			guardName:     target.GuardName,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: fmt.Sprintf("guard %q not found in scenario %q", target.GuardName, scenario.name),
		}}
		result.totalDurationWall = time.Since(start)
		result.startedAt = start
		return result
	}

	for i, guard := range scenario.guards {
		guardResult := evaluateGuard(guard, scenario, scenario.executor, seed, config, entropyProvider, execCtx)
		result.guardResults[i] = guardResult

		if !guardResult.passed {
			result.passed = false
		}
		summedDuration += guardResult.duration
	}

	result.totalDurationWall = time.Since(start)
	result.totalDurationSummed = summedDuration
	result.startedAt = start

	return result
}

// --------------------------------------------------------------- OPERATION

type OperationRunResult struct {
	operationName string
	zonePath      ZonePath

	startedAt time.Time

	totalDurationWall   time.Duration
	totalDurationSummed time.Duration

	passed bool

	scenarioResults []ScenarioRunResult
}

/*
Name returns the operation name for this result.
*/
func (o *OperationRunResult) Name() string {
	return o.operationName
}

/*
Passed reports whether startup, aggregated scenarios, and teardown (when reached) succeeded.

Startup panic or non-nil error still yields false alongside a synthetic Operation_Startup_Failure row.
Recovered panics inside runScenarios or teardown similarly append synthetic failures and force false here.
*/
func (o *OperationRunResult) Passed() bool {
	return o.passed
}

/*
WallDuration is wall time from OperationRun entry until WallDuration bookkeeping finishes (excluding deferred teardown work),

capturing startup / runScenarios / aggregation only. Shortcut failure returns snapshot immediately after injecting synthetics,

before any teardown registered on startup success executes.
*/
func (o *OperationRunResult) WallDuration() time.Duration {
	return o.totalDurationWall
}

/*
SummedDuration returns the sum of each aggregated ScenarioRunResult.SummedDuration after runScenarios.

Startup-only or runScenarios-panic shortcuts skip that loop entirely, yielding zero alongside synthetic telemetry rows.
*/
func (o *OperationRunResult) SummedDuration() time.Duration {
	return o.totalDurationSummed
}

/*
ScenarioResults returns a copy of aggregated rows for this operation, including synthetic

Operation_*_Failure scenarios recorded when startup, runScenarios, or teardown fails inside guarded execution.
*/
func (o *OperationRunResult) ScenarioResults() []ScenarioRunResult {
	cp := make([]ScenarioRunResult, len(o.scenarioResults))
	copy(cp, o.scenarioResults)

	return cp
}

/*
ZonePath returns the operation's zone metadata at run time (execution ignores it aside from attribution).
*/
func (o *OperationRunResult) ZonePath() ZonePath {
	return o.zonePath
}

/*
StartedAt timestamps OperationRun entry (before startup)—the baseline stamped onto synthetic ScenarioRunResults

and unrelated to timestamps inside ScenarioRun snapshots produced deeper in runScenarios.
*/
func (o *OperationRunResult) StartedAt() time.Time {
	return o.startedAt
}

type Operation[TState any] struct {
	name        string
	description string
	zonePath    ZonePath

	startup  func() (TState, error)
	teardown func(state TState)

	runScenarios func(state TState, execCtx ExecutionContext) []ScenarioRunResult

	stateCodec *OperationStateCodec[TState]
}

func OperationCreate[TState any](
	name string,
	description string,
	zonePath ZonePath,
	startup func() (TState, error),
	teardown func(state TState),
	runScenarios func(state TState, execCtx ExecutionContext) []ScenarioRunResult,
	stateCodec *OperationStateCodec[TState],
) Operation[TState] {
	return Operation[TState]{
		name:         name,
		description:  description,
		zonePath:     zonePath,
		startup:      startup,
		teardown:     teardown,
		runScenarios: runScenarios,
		stateCodec:   stateCodec,
	}
}

func operationTeardownState[TState any](operation *Operation[TState], state TState) {
	if operation == nil || operation.teardown == nil {
		return
	}
	operation.teardown(state)
}

func OperationRun[TState any](operation *Operation[TState], execCtx ExecutionContext) (result OperationRunResult) {
	start := time.Now()

	result = OperationRunResult{
		operationName: operation.name,
		zonePath:      operation.zonePath,
		startedAt:     start,
		passed:        false,
	}

	createSyntheticScenario := func(scenarioName, guardName, reason string) ScenarioRunResult {
		return ScenarioRunResult{
			scenarioName: scenarioName,
			zonePath:     operation.zonePath,
			startedAt:    start,
			passed:       false,
			guardResults: []GuardEvaluationResult{
				{
					guardName:     guardName,
					passed:        false,
					duration:      time.Since(start),
					failureReason: reason,
				},
			},
		}
	}

	// 1. Safely Execute Startup
	var state TState
	var startupErr error
	var startupPanicked bool
	var startupPanicMsg string

	func() {
		defer func() {
			if r := recover(); r != nil {
				startupPanicked = true
				startupPanicMsg = shieldTestingRecoveredPanicFormat(r)
			}
		}()
		state, startupErr = operation.startup()
	}()

	if startupPanicked {
		result.scenarioResults = []ScenarioRunResult{
			createSyntheticScenario("Operation_Startup_Failure", "startup_execution", fmt.Sprintf("startup panicked:\n%s", startupPanicMsg)),
		}
		result.totalDurationWall = time.Since(start)
		return result
	}

	if startupErr != nil {
		result.scenarioResults = []ScenarioRunResult{
			createSyntheticScenario("Operation_Startup_Failure", "startup_execution", fmt.Sprintf("startup failed: %v", startupErr)),
		}
		result.totalDurationWall = time.Since(start)
		return result
	}

	// Deferred teardown after startup success: runs on all returns from this scope; panics isolate to synthesised telemetry.
	defer func() {
		defer func() {
			if r := recover(); r != nil {
				result.passed = false
				teardownFail := createSyntheticScenario(
					"Operation_Teardown_Failure",
					"teardown_execution",
					fmt.Sprintf("teardown panicked:\n%s", shieldTestingRecoveredPanicFormat(r)),
				)
				result.scenarioResults = append(result.scenarioResults, teardownFail)
			}
		}()
		operation.teardown(state)
	}()

	// 3. Safely Execute Scenarios
	var scenarioResults []ScenarioRunResult
	var scenariosPanicked bool
	var scenariosPanicMsg string

	var operationSnapshot []byte
	enrichedCtx := ExecutionContext{
		Identity: execCtx.Identity,
		ZonePath: operation.zonePath,
		Runtime:  execCtx.Runtime,
	}
	if execCtx.Runtime != nil {
		runtimeCopy := *execCtx.Runtime
		runtimeCopy.OperationName = operation.name
		runtimeCopy.OperationStateSnapshot = &operationSnapshot
		if operation.stateCodec != nil {
			runtimeCopy.SerializeOperationState = func() ([]byte, error) {
				return operation.stateCodec.Serialize(state)
			}
		}
		enrichedCtx.Runtime = &runtimeCopy
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				scenariosPanicked = true
				scenariosPanicMsg = shieldTestingRecoveredPanicFormat(r)
			}
		}()
		scenarioResults = operation.runScenarios(state, enrichedCtx)
	}()

	if len(operationSnapshot) > 0 && operation.stateCodec != nil {
		if operation.stateCodec.Apply != nil {
			if applyErr := operation.stateCodec.Apply(state, operationSnapshot); applyErr != nil {
				result.passed = false
				scenarioResults = append(scenarioResults, createSyntheticScenario(
					"Operation_State_Apply_Failure",
					"operation_state_apply",
					fmt.Sprintf("apply operation state snapshot: %v", applyErr),
				))
			}
		} else {
			deserialized, deserializeErr := operation.stateCodec.Deserialize(operationSnapshot)
			if deserializeErr != nil {
				result.passed = false
				scenarioResults = append(scenarioResults, createSyntheticScenario(
					"Operation_State_Deserialize_Failure",
					"operation_state_deserialize",
					fmt.Sprintf("deserialize operation state snapshot: %v", deserializeErr),
				))
			} else {
				operationTeardownState(operation, state)
				state = deserialized
			}
		}
	}

	if scenariosPanicked {
		result.passed = false
		result.scenarioResults = []ScenarioRunResult{
			createSyntheticScenario("Operation_Execution_Failure", "runScenarios_execution", fmt.Sprintf("runScenarios panicked:\n%s", scenariosPanicMsg)),
		}
		result.totalDurationWall = time.Since(start)
		return result
	}

	// 4. Standard Aggregation
	result.passed = true
	var summedDuration time.Duration

	for _, scenarioResult := range scenarioResults {
		summedDuration += scenarioResult.totalDurationSummed
		if !scenarioResult.passed {
			result.passed = false
		}
	}

	result.totalDurationWall = time.Since(start)
	result.totalDurationSummed = summedDuration
	result.scenarioResults = scenarioResults

	return result
}

// --------------------------------------------------------------- PRIVATE HELPERS

func evaluateGuard[TInput, TOutput any](
	guard Guard[TInput, TOutput],
	scenario Scenario[TInput, TOutput],
	executor Executor[TInput, TOutput],
	seed essence.UUID,
	config ScenarioRunConfig,
	provider *entropy.EntropyProvider,
	execCtx ExecutionContext,
) GuardEvaluationResult {
	if guard.isPoisoned {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: fmt.Sprintf("FRAMEWORK ERROR: %s", guard.poisonReason),
			duration:      0,
		}
	}

	if ScenarioGuardIsolationEnabled(scenario.guardIsolation, guard.name) &&
		!isolatedGuardChildProcessActive() {
		return evaluateGuardSubprocess(guard, scenario, executor, seed, config, provider, execCtx)
	}

	result := GuardEvaluationResult{
		guardName: guard.name,
		passed:    true,
	}

	start := time.Now()

	iterator := guard.inputGenerator(FuzzingContext{
		EntropyProvider: provider,
		Effort:          config.FuzzingPattern,
	})

	for i := uint64(0); !isEffortExhausted(start, i, config); i++ {
		iterationInput := iterator()

		execRes := executeSingleIteration(executor, iterationInput)
		passed, reason := evaluatePolicies(guard.policies, execRes)

		if !passed {
			result.passed = false
			result.failedSeed = seed
			result.failedIteration = i
			result.failureReason = reason
			if execRes.panicked {
				result.failureClass = GuardFailureClassPanic
			} else {
				result.failureClass = GuardFailureClassPolicy
			}
			break
		}
	}

	result.duration = time.Since(start)
	return result
}

func isEffortExhausted(start time.Time, iterations uint64, config ScenarioRunConfig) bool {
	if config.UseDuration {
		return time.Since(start) >= config.MaxDuration
	}
	return iterations >= config.MaxIterations
}

func executeSingleIteration[TInput, TOutput any](
	executor Executor[TInput, TOutput],
	input TInput,
) (res executionResult[TOutput]) {
	defer func() {
		if r := recover(); r != nil {
			res.panicked = true
			res.panicMessage = shieldTestingRecoveredPanicFormat(r)
		}
	}()

	res.actual, res.executionError = executor(input)
	return res
}

func evaluatePolicies[TOutput any](
	policies []GuardPolicy[TOutput],
	res executionResult[TOutput],
) (bool, string) {
	for _, policy := range policies {
		passed, reason := policy.evaluate(res)
		if !passed {
			return false, reason
		}
	}
	return true, ""
}
