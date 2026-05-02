package shield

import "shield/internal"

/*
SHIELD regression endpoints compare scenario run outcomes:

- Pairwise deterministic comparison (`SHIELD_Regression_CheckIdentical`) for baseline/target runs executed under aligned fuzz snapshots.

- Population stability comparison (`SHIELD_Regression_CheckStability`) for many baseline vs many target runs using asymptotic/statistical summaries.

Neither path performs HTTP; they are ordinary package functions usable from tools and harness code like the testing API in `testing.go`.

All heavy logic lives under `shield/internal`; this file is a typed facade only.
*/

/*
SHIELD_Regression_Severity labels the kind of change detected when regressing guards or populations.

Structured severities stringify to persisted/display values on the underlying `RegressionSeverity`; see constants below for discriminators usable in branching.
*/
type SHIELD_Regression_Severity = internal.RegressionSeverity

/*
SHIELD_Regression_Severity constants mirror internal severity tokens.

Interpretation shorthand:

Improvement → situation got strictly better relative to baseline for that guard or slice.

OutcomeShift → pass/failure mass moved in an adverse direction (stability checks use this after a pooled two-proportion test).

FragilityIncrease → failures surface earlier along the fuzz iteration axis.

FailureDegradation → both baseline and target fail but failure surface worsened (paired guard comparison).

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

Fields carry pass/failure bits, earliest failing iteration bookkeeping, and stringified telemetry reasons reconstructed from Shield's guard evaluation snapshots.

Population stability checks populate no per-guard deltas; this structure is exercised by `SHIELD_Regression_CheckIdentical` today.
*/
type SHIELD_Regression_GuardDelta = internal.GuardRegressionDelta

/*
SHIELD_Regression_Result packages `SHIELD_Regression_CheckIdentical` output.

[Fields]

`Sources`, when produced by identical comparison, retains `[baseline,target]` insertion order and stores each run by value inside the aggregate.

`GuardDeltas` contains only severity-non-none guard transitions (internal filtering).

`IsRegression` is true whenever any tracked delta bears OutcomeShift, FragilityIncrease, or FailureDegradation severities relative to regressing polarity rules.
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

Intersecting guards (matched by exported guard name ordering on target enumeration) accumulate severity-ranked deltas excluding strictly informational none entries.

Improvement severities populate `GuardDeltas` yet do not themselves flip `IsRegression`.

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
