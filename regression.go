package shield

import (
	"fmt"
	"shield/internal"
)

/*
SHIELD regression endpoints compare scenario run outcomes:

- Pairwise deterministic comparison (`SHIELD_Regression_CheckIdentical`) for baseline/target runs executed under aligned fuzz snapshots.

- Population stability comparison (`SHIELD_Regression_CheckStability`) for many baseline vs many target runs using asymptotic/statistical summaries.

- Convenience loaders (`SHIELD_Regression_CheckIdenticalFromStorage`, `SHIELD_Regression_CheckStabilityFromStorage`) hydrate those comparisons from persisted aggregates via `SHIELD_Testing_Storage_*`.

All heavy logic lives under `shield/internal`; this file is a typed facade only.
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

/*
SHIELD_Regression_Severity labels the kind of change detected when regressing guards or populations.

Structured severities stringify to persisted/display values on the underlying `RegressionSeverity`; see constants below for discriminators usable in branching.
*/
type SHIELD_Regression_Severity = internal.RegressionSeverity

/*
SHIELD_Regression_Severity constants mirror internal severity tokens.

Interpretation shorthand:

Improvement → situation got strictly better relative to baseline for that guard or slice.

OutcomeShift → pass/failure mass moved in an adverse direction (identical runs: pass→fail or a baseline guard missing from the target run); stability uses this after a pooled two-proportion test.

SHIELD_Regression_Severity_FragilityShift (serialised "FragilityIncrease") → paired identical runs only: both failed with the same SHIELD guard FailureReason string, but the target failed strictly earlier on the fuzz-iteration axis.

SHIELD_Regression_Severity_Degradation (serialised "FailureDegradation") → paired identical runs only: both failed, and the stringified FailureReason telemetry differs between baseline and target before iteration is considered.

None → no regressing severity for that delta or verdict.
*/
const (
	SHIELD_Regression_Severity_None           = internal.RegressionSeverity_None
	SHIELD_Regression_Severity_Improvement    = internal.RegressionSeverity_Improvement
	SHIELD_Regression_Severity_OutcomeShift   = internal.RegressionSeverity_OutcomeShift
	SHIELD_Regression_Severity_Degradation    = internal.RegressionSeverity_Degradation
	SHIELD_Regression_Severity_FragilityShift = internal.RegressionSeverity_FragilityShift
)

/*
SHIELD_Regression_GuardDelta reports one paired-guard divergence between deterministic baseline and target runs.

Pairing walks baseline guard order and joins each row to the target run by guard name; extra target-only guards are ignored.

When a baseline guard has no same-named target entry, TargetPassed is false and TargetReason documents the absence (OutcomeShift).

Fields carry pass bits, recorded failed-iteration indices from each evaluation snapshot, and FailureReason strings as exposed by the testing API.

Population stability checks emit no per-guard deltas; this structure is used by `SHIELD_Regression_CheckIdentical`.
*/
type SHIELD_Regression_GuardDelta = internal.GuardRegressionDelta

/*
SHIELD_Regression_Result packages `SHIELD_Regression_CheckIdentical` output.

[Fields]

`Sources`, when produced by identical comparison, retains `[baseline,target]` insertion order and stores each run by value inside the aggregate.

`GuardDeltas` contains only severity-non-none guard transitions (internal filtering).

`IsRegression` is true whenever any tracked delta bears OutcomeShift, Degradation, or FragilityShift severities under the internal polarity rules.
*/
type SHIELD_Regression_Result = internal.RegressionResult

/*
SHIELD_Regression_StabilityStats collapses repeated scenario executions into coarse rates.

TotalRuns mirrors slice length feeding stability evaluation.

FailedRuns counts runs whose aggregate guard verdict failed.

FailureRate equals `FailedRuns / TotalRuns`.

AverageFailedIter is the arithmetic mean (unsigned) over observed earliest failing iterations per failing run only; zero when no failures occurred.

Consumers should treat ints as cardinality data and rates as fractions in `[0,1]` for successful slices.
*/
type SHIELD_Regression_StabilityStats = internal.StabilityStats

/*
SHIELD_Regression_Stability_Result is the verdict object from stability comparison.

Baseline/Target embed `SHIELD_Regression_StabilityStats` for both populations examined.

Severity encodes coarse classification identical to pairwise severities (`OutcomeShift` for statistically significant degradation, `FragilityIncrease` after Mann-Whitney analysis, otherwise `None`).

Reason is a human-readable sentence suitable for telemetry; empty only if internal invariants violated (should not happen).

`IsRegression` is true when Severities denote actionable regress according to Shield's phased rules.
*/
type SHIELD_Regression_Stability_Result = internal.StabilityRegressionResult

/*
SHIELD_Regression_CheckIdentical performs deterministic pairwise regression between two finalized scenario runs.

[Algorithm]

Validated snapshot knobs (Seed, FuzzingPattern, MaxIterations) must match exactly; divergence aborts before guard comparison.

Guard regression walks baseline `GuardResults` in order, resolving each name against the target run. For each pair: pass/fail flips map to Improvement or OutcomeShift; when both fail, Shield first compares FailureReason strings—any change yields FailureDegradation; if reasons match, strictly earlier target failure iteration yields FragilityIncrease (FragilityShift constant). Missing target guards synthesize an OutcomeShift row. Target-only guards are not compared. Deltas omit severities of None. Improvements are listed but do not set `IsRegression`.

[Returns]

Filled `Sources` preserving argument order `[baseline,target]`.

[Errors]

Malformed comparisons return wrapped errors distinguishing seed, fuzzing-pattern, or max-iteration mismatch.

Pure read-only function over supplied runs.
*/
func SHIELD_Regression_CheckIdentical(
	baseline SHIELD_Testing_ScenarioRunResult,
	target SHIELD_Testing_ScenarioRunResult,
) (SHIELD_Regression_Result, error) {
	return internal.CheckIdenticalRegression(baseline, target)
}

/*
SHIELD_Regression_CheckStability compares two populations (`baseline` slice vs `target` slice).

[ Preconditions ]

Each slice must contain at least 50 `ScenarioRunResult` entries (`len(slice) ≥ 50`); violating this yields a non-nil error with an explanatory wrapped message.

ConfidenceLevel denotes the desired simultaneous confidence mass for asymptotic thresholds (converted internally to `alpha = 1 − confidenceLevel`).

[ Algorithm Phases ]

1. Failure-rate shift: pooled two-proportion Z-test comparing empirical failure fractions with cohort sizes drawn from lengths of each slice. Regression triggers only when asymptotic two-tailed p-value falls below `alpha` AND `target` failure fraction strictly worsens baseline.

2. Fragility tightening: Provided each population recorded at least five failing runs with iterable indices, Shield applies Mann-Whitney U hypothesis testing (`statarch/hypothesis`) comparing earliest failing iteration envelopes. Statistical significance pairing with lowered target-average failing iteration declares `FragilityIncrease`.

If neither criterion fires the verdict declares non-regression with neutral severity `None`.

[Returns ]

Filled `Baseline`/`Target` stats aggregates for observability, boolean regression flag, chosen severity semantic, explanatory string.

[Pure Data Path ]

Function does not persist results; callers integrate with storage voluntarily.

[Errors ]

Only insufficient sample sizes currently surface errors; numerical degeneracies propagate as finite floats inside stats without auxiliary errors today.
*/
func SHIELD_Regression_CheckStability(
	baseline []SHIELD_Testing_ScenarioRunResult,
	target []SHIELD_Testing_ScenarioRunResult,
	confidenceLevel float64,
) (SHIELD_Regression_Stability_Result, error) {
	return internal.CheckStabilityRegression(baseline, target, confidenceLevel)
}

/*
SHIELD_Regression_CheckIdenticalFromStorage loads two aggregates by persisted IDs and evaluates identical-regression parity.

baselineResultID selects the authoritative row; targetResultID selects the candidate row inside the configured storage engine.

[Side Effects]

Performs SQLite reads (`SHIELD_Testing_Storage_ScenarioResultFindByID`) ensuring the transactional connection lifecycle managed by callers.

Returns the same RegressionResult semantics as manual `SHIELD_Regression_CheckIdentical` invocation.

Both IDs must correspond to materially comparable scenarios; IDs missing from storage propagate repository errors verbatim.
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
SHIELD_Regression_CheckStabilityFromStorage gathers stored cohort aggregates for paired scenario names across optional distinct engines before delegating stability analysis.

baselineEngine/baselineScenarioName designates the authoritative population; targetEngine/targetScenarioName designates the challenger population.

Use the same engine pointer twice when both cohorts share one SQLite ledger but differing scenario_name labels (for example partitioned datasets).

Ordering note:

Stored rows follow repository iteration order for the filter (`scenario_name`). Do not infer temporal ordering unless the caller establishes it elsewhere.

ConfidenceLevel behaves identically to `SHIELD_Regression_CheckStability` after hydrating slices via `FindByName`.
*/
func SHIELD_Regression_CheckStabilityFromStorage(
	baselineEngine *SHIELD_Testing_Storage_Engine,
	baselineScenarioName string,
	targetEngine *SHIELD_Testing_Storage_Engine,
	targetScenarioName string,
	confidenceLevel float64,
) (SHIELD_Regression_Stability_Result, error) {
	var zero SHIELD_Regression_Stability_Result

	basePtrs, err := SHIELD_Testing_Storage_ScenarioResultFindByName(baselineEngine, baselineScenarioName)
	if err != nil {
		return zero, fmt.Errorf(
			"load baseline cohort %q: %w", baselineScenarioName, err)
	}

	tgtPtrs, err := SHIELD_Testing_Storage_ScenarioResultFindByName(targetEngine, targetScenarioName)
	if err != nil {
		return zero, fmt.Errorf(
			"load target cohort %q: %w", targetScenarioName, err)
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
