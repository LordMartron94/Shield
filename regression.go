package shield

import (
	"fmt"
	"shield/internal"
)

/*
SHIELD regression compares scenario outcomes: SHIELD_Regression_CheckIdentical performs pairwise deterministic deltas under

aligned fuzz knobs plus matching SystemIdentity.Environment (Version may deliberately differ). SHIELD_Regression_CheckStability

consumes large homogeneous cohorts per Environment+Version. SQLite helpers load persisted aggregates; stability paths pair

SHIELD_Testing_Storage_ScenarioResultFindByIdentity selections. Heavy logic stays in shield/internal—this file is a facade only.
*/

func slicesFromStoredScenarioPointers(
	ps []*internal.ScenarioRunResult,
) ([]internal.ScenarioRunResult, error) {
	out := make([]internal.ScenarioRunResult, len(ps))
	for i, ptr := range ps {
		if ptr == nil {
			return nil, fmt.Errorf("scenario run slice contains nil aggregate at index %d", i)
		}
		out[i] = *ptr
	}
	return out, nil
}

// SHIELD_Regression_Severity enumerates guard or population change classes; string forms mirror persisted RegressionSeverity constants.
type SHIELD_Regression_Severity = internal.RegressionSeverity

/*
SHIELD regression severity shorthand: Improvement means strictly better guard outcomes. OutcomeShift captures adverse pass/fail moves

(identical runs include missing baseline guards; stability pairs it with significant pooled failure-rate shifts).

SHIELD_Regression_Severity_FragilityShift persists as “FragilityIncrease” text—paired runs where both failed with identical guard

FailureReason strings yet the target failed sooner on the fuzz iteration axis. SHIELD_Regression_Severity_Degradation persists as

“FailureDegradation”—both failed but FailureReason strings diverged before iteration comparison. None suppresses regression polarity.
*/
const (
	SHIELD_Regression_Severity_None           = internal.RegressionSeverity_None
	SHIELD_Regression_Severity_Improvement    = internal.RegressionSeverity_Improvement
	SHIELD_Regression_Severity_OutcomeShift   = internal.RegressionSeverity_OutcomeShift
	SHIELD_Regression_Severity_Degradation    = internal.RegressionSeverity_Degradation
	SHIELD_Regression_Severity_FragilityShift = internal.RegressionSeverity_FragilityShift
)

/*
SHIELD_Regression_GuardDelta reports one paired deterministic guard difference. Baseline order drives pairing; missing target guards

synthesize OutcomeShift rows. Stability analysis never fills this struct—CheckIdentical consumes it exclusively.
*/
type SHIELD_Regression_GuardDelta = internal.GuardRegressionDelta

/*
SHIELD_Regression_Result wraps CheckIdentical output: Sources retains [baseline,target] insertion order.

GuardDeltas includes only actionable severities plus informative Improvements filtered internally.

IsRegression flips true when OutcomeShift, Degradation, or FragilityShift appear—Improvement alone does not.
*/
type SHIELD_Regression_Result = internal.RegressionResult

/*
SHIELD_Regression_StabilityStats collapses many ScenarioRunResult rows into TotalRuns, FailedRuns, FailureRate, and mean

AverageFailedIter across failing runs only. Identity mirrors the enforced homogeneous SystemIdentity; EarliestRun/LatestRun bound

StartedAt samples (zero only if upstream forgets stamping). Consumers treat counts as cardinality and FailureRate fractions in [0,1].
*/
type SHIELD_Regression_StabilityStats = internal.StabilityStats

/*
SHIELD_Regression_Stability_Result is CheckStability’s verdict pairing Baseline/Target stats, Severity token, explanatory Reason,

and IsRegression flagged when regression rules deem the population shift actionable.
*/
type SHIELD_Regression_Stability_Result = internal.StabilityRegressionResult

/*
SHIELD_Regression_CheckIdentical enforces pairwise snapshot parity (Seed, FuzzingPattern, MaxIterations)

plus SystemIdentity.Environment equality before diffing guards; Version mismatch is tolerated so artifacts can advance while deployments

stay comparable. Divergence yields wrapped errors annotated with the offending knob. Returned Sources preserves argument ordering.

Improvement deltas never assert IsRegression. Pure read-only over arguments.
*/
func SHIELD_Regression_CheckIdentical(
	baseline SHIELD_Testing_ScenarioRunResult,
	target SHIELD_Testing_ScenarioRunResult,
) (SHIELD_Regression_Result, error) {
	return internal.CheckIdenticalRegression(baseline, target)
}

/*
SHIELD_Regression_CheckStability compares baseline vs target ScenarioRun slices (each len ≥50) under identical ConfidenceLevel semantics.

Slices must internally agree on SnapshotConfig.Identity Environment+Version or validation fails with contamination diagnostics.

Phase one applies a pooled two-proportion Z-test targeting strictly worse target failure fractions with p<alpha.

Phase two optionally runs Mann-Whitney U on earliest failing iterations when populations include enough failures, flagging FragilityIncrease.

Otherwise severity None emerges. Does not persist; numerical edge cases reuse finite floats inside stats without auxiliary errors today.
*/
func SHIELD_Regression_CheckStability(
	baseline []SHIELD_Testing_ScenarioRunResult,
	target []SHIELD_Testing_ScenarioRunResult,
	confidenceLevel float64,
) (SHIELD_Regression_Stability_Result, error) {
	return internal.CheckStabilityRegression(baseline, target, confidenceLevel)
}

/*
SHIELD_Regression_CheckIdenticalFromStorage loads baseline/target aggregates by persisted id through SHIELD_Testing_Storage_ScenarioResultFindByID,

then executes CheckIdentical. Missing IDs bubble repository errors verbatim; caller manages transactional engine lifecycle.
*/
func SHIELD_Regression_CheckIdenticalFromStorage(
	engine *SHIELD_Testing_Storage_Engine,
	baselineResultID string,
	targetResultID string,
) (SHIELD_Regression_Result, error) {
	baselineAgg, err := SHIELD_Testing_Storage_ScenarioResultFindByID(engine, baselineResultID)
	if err != nil {
		var zero SHIELD_Regression_Result
		return zero, fmt.Errorf("load baseline scenario row %s: %w", baselineResultID, err)
	}
	targetAgg, err := SHIELD_Testing_Storage_ScenarioResultFindByID(engine, targetResultID)
	if err != nil {
		var zero SHIELD_Regression_Result
		return zero, fmt.Errorf("load target scenario row %s: %w", targetResultID, err)
	}

	return SHIELD_Regression_CheckIdentical(*baselineAgg, *targetAgg)
}

/*
SHIELD_Regression_CheckStabilityFromStorage loads baseline and target populations via SHIELD_Testing_Storage_ScenarioResultFindByIdentity,

mirroring Scenario names plus explicit identities, optionally across two engines. Rows arrive in repository order—sort externally if

timeline analytics matter. ConfidenceLevel then mirrors CheckStability once slices hydrate and nil aggregates are rejected.
*/
func SHIELD_Regression_CheckStabilityFromStorage(
	baselineEngine *SHIELD_Testing_Storage_Engine,
	baselineScenarioName string,
	baselineIdentity SHIELD_Testing_SystemIdentity,
	targetEngine *SHIELD_Testing_Storage_Engine,
	targetScenarioName string,
	targetIdentity SHIELD_Testing_SystemIdentity,
	confidenceLevel float64,
) (SHIELD_Regression_Stability_Result, error) {
	var zero SHIELD_Regression_Stability_Result

	basePtrs, err := SHIELD_Testing_Storage_ScenarioResultFindByIdentity(baselineEngine, baselineScenarioName, baselineIdentity)
	if err != nil {
		return zero, fmt.Errorf("load baseline cohort %q (env: %s): %w", baselineScenarioName, baselineIdentity.Environment, err)
	}

	tgtPtrs, err := SHIELD_Testing_Storage_ScenarioResultFindByIdentity(targetEngine, targetScenarioName, targetIdentity)
	if err != nil {
		return zero, fmt.Errorf("load target cohort %q (env: %s): %w", targetScenarioName, targetIdentity.Environment, err)
	}

	baselineRuns, err := slicesFromStoredScenarioPointers(basePtrs)
	if err != nil {
		return zero, fmt.Errorf("baseline aggregate slice invalid: %w", err)
	}

	targetRuns, err := slicesFromStoredScenarioPointers(tgtPtrs)
	if err != nil {
		return zero, fmt.Errorf("target aggregate slice invalid: %w", err)
	}

	return SHIELD_Regression_CheckStability(baselineRuns, targetRuns, confidenceLevel)
}
