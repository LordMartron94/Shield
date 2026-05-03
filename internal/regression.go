package internal

import (
	"fmt"
	"memarch"
	"memcore"
	"memforge"
	"memstruct"
	"statarch/core"
	"statarch/hypothesis"
	"time"
)

// --------------------------------------------------------------- REGRESSION TYPES

type RegressionSeverity string

const (
	RegressionSeverity_None           RegressionSeverity = "None"
	RegressionSeverity_Improvement    RegressionSeverity = "Improvement"        // Failed -> Passed
	RegressionSeverity_OutcomeShift   RegressionSeverity = "OutcomeShift"       // Passed -> Failed
	RegressionSeverity_Degradation    RegressionSeverity = "FailureDegradation" // Failed -> Failed (but worse panic/error)
	RegressionSeverity_FragilityShift RegressionSeverity = "FragilityIncrease"  // Failed -> Failed (at earlier iteration)
)

type GuardRegressionDelta struct {
	GuardName string
	Severity  RegressionSeverity

	BaselinePassed bool
	TargetPassed   bool

	BaselineIter uint64
	TargetIter   uint64

	BaselineReason string
	TargetReason   string
}

type RegressionResult struct {
	Sources      []ScenarioRunResult
	IsRegression bool
	GuardDeltas  []GuardRegressionDelta
}

type StabilityStats struct {
	Identity SystemIdentity

	TotalRuns         int
	FailedRuns        int
	FailureRate       float64
	AverageFailedIter uint64

	EarliestRun time.Time
	LatestRun   time.Time
}

type StabilityRegressionResult struct {
	Baseline     StabilityStats
	Target       StabilityStats
	IsRegression bool
	Severity     RegressionSeverity
	Reason       string
}

// --------------------------------------------------------------- IDENTICAL REGRESSION

func CheckIdenticalRegression(baseline ScenarioRunResult, target ScenarioRunResult) (RegressionResult, error) {
	out := RegressionResult{
		Sources:     []ScenarioRunResult{baseline, target},
		GuardDeltas: make([]GuardRegressionDelta, 0),
	}

	if err := validateIdenticalSignatures(baseline.SnapshotConfig(), target.SnapshotConfig()); err != nil {
		return out, err
	}

	deltas := compareGuards(baseline.GuardResults(), target.GuardResults())

	out.GuardDeltas = deltas
	out.IsRegression = evaluateOverallRegression(deltas)

	return out, nil
}

// --------------------------------------------------------------- STABILITY REGRESSION

func CheckStabilityRegression(
	baseline []ScenarioRunResult,
	target []ScenarioRunResult,
	confidenceLevel float64,
) (StabilityRegressionResult, error) {
	if len(baseline) < 50 || len(target) < 50 {
		return StabilityRegressionResult{}, fmt.Errorf("insufficient total runs for stability testing (min 50)")
	}

	if err := validateCohortPurity(baseline, "baseline"); err != nil {
		return StabilityRegressionResult{}, err
	}

	if err := validateCohortPurity(target, "target"); err != nil {
		return StabilityRegressionResult{}, err
	}

	baseStats, baseFailedIters := calculateStabilityStats(baseline)
	tgtStats, tgtFailedIters := calculateStabilityStats(target)

	return evaluateStabilityDelta(baseStats, tgtStats, baseFailedIters, tgtFailedIters, confidenceLevel), nil
}

// --------------------------------------------------------------- PRIVATE HELPERS

func calculateStabilityStats(runs []ScenarioRunResult) (StabilityStats, []float64) {
	var failedCount int
	var totalFailedIters uint64
	var iters []float64

	earliest, latest := runs[0].StartedAt(), runs[0].StartedAt()

	for _, run := range runs {
		currentStart := run.StartedAt()

		if currentStart.Before(earliest) {
			earliest = currentStart
		}
		if currentStart.After(latest) {
			latest = currentStart
		}

		if !run.Passed() {
			failedCount++
			iter := getEarliestFailureIteration(run.GuardResults())
			totalFailedIters += iter
			iters = append(iters, float64(iter))
		}
	}

	avgIter := uint64(0)
	if failedCount > 0 {
		avgIter = totalFailedIters / uint64(failedCount)
	}

	stats := StabilityStats{
		Identity:          runs[0].SnapshotConfig().Identity,
		EarliestRun:       earliest,
		LatestRun:         latest,
		TotalRuns:         len(runs),
		FailedRuns:        failedCount,
		FailureRate:       float64(failedCount) / float64(len(runs)),
		AverageFailedIter: avgIter,
	}

	return stats, iters
}

func getEarliestFailureIteration(guards []GuardEvaluationResult) uint64 {
	minIter := ^uint64(0)
	hasFailure := false

	for _, g := range guards {
		if !g.Passed() && g.failedIteration < minIter {
			minIter = g.failedIteration
			hasFailure = true
		}
	}

	if !hasFailure {
		return 0
	}
	return minIter
}

func evaluateStabilityDelta(
	base StabilityStats,
	tgt StabilityStats,
	baseIters []float64,
	tgtIters []float64,
	confidenceLevel float64,
) StabilityRegressionResult {
	res := StabilityRegressionResult{
		Baseline: base,
		Target:   tgt,
	}

	alpha := 1.0 - confidenceLevel

	// ---------------------------------------------------------
	// 1. Yield Drop (Two-Proportion Z-Test)
	// ---------------------------------------------------------

	zProportionTestResult := hypothesis.StatArchHypothesisVectorTwoProportionZF64(
		uint64(base.TotalRuns),
		uint64(tgt.TotalRuns),
		base.FailureRate,
		tgt.FailureRate,
	)

	// If the p-value is significant AND the target failure rate is strictly worse
	if zProportionTestResult.PValue < alpha && tgt.FailureRate > base.FailureRate {
		res.IsRegression = true
		res.Severity = RegressionSeverity_OutcomeShift
		res.Reason = fmt.Sprintf(
			"Statistically significant failure rate increase (p=%.4f). Rate shifted from %.2f%% to %.2f%%",
			zProportionTestResult.PValue, base.FailureRate*100, tgt.FailureRate*100,
		)
		return res
	}

	// ---------------------------------------------------------
	// 2. Fragility Shift (Mann-Whitney U Test)
	// ---------------------------------------------------------

	if len(baseIters) >= 5 && len(tgtIters) >= 5 {
		isSignificantlyMoreFragile, pValFragility := checkFragilityDegradation(baseIters, tgtIters, confidenceLevel)

		if isSignificantlyMoreFragile && tgt.AverageFailedIter < base.AverageFailedIter {
			res.IsRegression = true
			res.Severity = RegressionSeverity_FragilityShift
			res.Reason = fmt.Sprintf(
				"Statistically significant fragility increase (p=%.4f). Mean iterations dropped from %d to %d",
				pValFragility, base.AverageFailedIter, tgt.AverageFailedIter,
			)
			return res
		}
	}

	res.IsRegression = false
	res.Severity = RegressionSeverity_None
	res.Reason = "Aggregate stability within acceptable statistical variance"
	return res
}

func checkFragilityDegradation(baseIters, tgtIters []float64, confidenceLevel float64) (bool, float64) {
	allocator := memforge.DynamicLinearAllocatorCreateFunction(uint64(memcore.MegaByte*10), memforge.DynamicLinearAllocatorGrowthTemplateDoubleOrNeededWithMaxPanic(uint64(1*memcore.GigaByte)))
	defer memforge.DynamicLinearAllocatorDestroy(allocator)

	allocFn := func(sizeBytes, alignment uint64) memcore.MarkRaw {
		return memforge.FixedLinearAllocatorMalloc(allocator, sizeBytes, alignment)
	}

	// Allocate and populate vectors
	baseMark, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(len(baseIters)))
	memstruct.VectorSetFromSliceUnsafe(baseMark, baseIters)

	tgtMark, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(len(tgtIters)))
	memstruct.VectorSetFromSliceUnsafe(tgtMark, tgtIters)

	// Create analysis contexts
	analysisBase := core.StatArchAnalysisCreate[float64](baseMark, allocFn)
	analysisTgt := core.StatArchAnalysisCreate[float64](tgtMark, allocFn)

	// Execute Mann-Whitney U test
	mwuResult := hypothesis.StatArchHypothesisVectorMannWhitneyUF64(analysisBase, analysisTgt, allocFn)

	alpha := 1.0 - confidenceLevel
	isSignificant := mwuResult.PValue < alpha

	return isSignificant, mwuResult.PValue
}

/*
validateIdenticalSignatures enforces pairwise determinism prerequisites: fuzz snapshot fields plus Identity.Environment parity (Version deliberate mismatch allowed).
*/
func validateIdenticalSignatures(base SnapshotConfig, tgt SnapshotConfig) error {
	if base.Seed != tgt.Seed {
		return fmt.Errorf("invalid comparison: seed mismatch (%s vs %s)", base.Seed.String(), tgt.Seed.String())
	}
	if base.FuzzingPattern != tgt.FuzzingPattern {
		return fmt.Errorf("invalid comparison: fuzzing pattern mismatch")
	}
	if base.MaxIterations != tgt.MaxIterations {
		return fmt.Errorf("invalid comparison: max iterations mismatch")
	}
	if base.Identity.Environment != tgt.Identity.Environment {
		return fmt.Errorf("invalid comparison: environment mismatch (%s vs %s)", base.Identity.Environment, tgt.Identity.Environment)
	}
	return nil
}

func validateCohortPurity(runs []ScenarioRunResult, cohortName string) error {
	if len(runs) == 0 {
		return nil
	}
	expectedEnv := runs[0].SnapshotConfig().Identity.Environment
	expectedVer := runs[0].SnapshotConfig().Identity.Version

	for i, run := range runs {
		actualEnv := run.SnapshotConfig().Identity.Environment
		if actualEnv != expectedEnv {
			return fmt.Errorf("invalid %s cohort: environment contamination detected. Expected %q, found %q at index %d",
				cohortName, expectedEnv, actualEnv, i)
		}

		actualVer := run.SnapshotConfig().Identity.Version
		if actualVer != expectedVer {
			return fmt.Errorf("invalid %s cohort: version contamination detected. Expected %q, found %q at index %d",
				cohortName, expectedVer, actualVer, i)
		}
	}
	return nil
}

/*
compareGuards pairs by guard name with baseline as the driver: missing target rows become OutcomeShift;

left-over target-only guards are ignored. Per-pair logic is evaluateGuardDelta.
*/
func compareGuards(baseGuards []GuardEvaluationResult, tgtGuards []GuardEvaluationResult) []GuardRegressionDelta {
	tgtMap := make(map[string]GuardEvaluationResult, len(tgtGuards))
	for _, g := range tgtGuards {
		tgtMap[g.Name()] = g
	}

	deltas := make([]GuardRegressionDelta, 0)

	for _, baseGuard := range baseGuards {
		tgtGuard, exists := tgtMap[baseGuard.Name()]

		if !exists {
			deltas = append(deltas, GuardRegressionDelta{
				GuardName:      baseGuard.Name(),
				Severity:       RegressionSeverity_OutcomeShift,
				BaselinePassed: baseGuard.Passed(),
				TargetPassed:   false,
				BaselineIter:   baseGuard.failedIteration,
				TargetIter:     0,
				BaselineReason: baseGuard.FailureReason(),
				TargetReason:   "Missing data: Guard disappeared from target run (Execution aborted?)",
			})
			continue
		}

		delta := evaluateGuardDelta(baseGuard, tgtGuard)
		if delta.Severity != RegressionSeverity_None {
			deltas = append(deltas, delta)
		}
	}

	return deltas
}

/*
evaluateGuardDelta classifies a name-aligned pair. After pass/fail handling, both-fail cases compare

FailureReason() strings first (divergence → FailureDegradation), then earlier target failure iteration (FragilityIncrease).
*/
func evaluateGuardDelta(base GuardEvaluationResult, tgt GuardEvaluationResult) GuardRegressionDelta {
	delta := GuardRegressionDelta{
		GuardName:      tgt.Name(),
		BaselinePassed: base.Passed(),
		TargetPassed:   tgt.Passed(),
		BaselineIter:   base.failedIteration,
		TargetIter:     tgt.failedIteration,
		BaselineReason: base.FailureReason(),
		TargetReason:   tgt.FailureReason(),
	}

	// 1. Binary Outcome Shifts
	if base.Passed() && !tgt.Passed() {
		delta.Severity = RegressionSeverity_OutcomeShift
		return delta
	}
	if !base.Passed() && tgt.Passed() {
		delta.Severity = RegressionSeverity_Improvement
		return delta
	}
	if base.Passed() && tgt.Passed() {
		delta.Severity = RegressionSeverity_None
		return delta
	}

	// 2. Both Failed - Check Failure Mode Degradation
	if base.FailureReason() != tgt.FailureReason() {
		delta.Severity = RegressionSeverity_Degradation
		return delta
	}

	// 3. Both Failed exactly the same way - Check Fragility Index
	if tgt.failedIteration < base.failedIteration {
		delta.Severity = RegressionSeverity_FragilityShift
		return delta
	}

	delta.Severity = RegressionSeverity_None
	return delta
}

func evaluateOverallRegression(deltas []GuardRegressionDelta) bool {
	for _, delta := range deltas {
		if delta.Severity == RegressionSeverity_OutcomeShift ||
			delta.Severity == RegressionSeverity_Degradation ||
			delta.Severity == RegressionSeverity_FragilityShift {
			return true
		}
	}
	return false
}
