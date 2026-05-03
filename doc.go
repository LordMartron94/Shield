/*
Package shield is a structured validation and regression harness.

Exercise scenarios through the `testing` surface (`SHIELD_Testing_*`) supplying `SHIELD_Testing_SystemIdentity`, hydrate prior snapshot configs for replay helpers, optionally persist aggregates via `SHIELD_Testing_Storage_*` (identity-scoped cohort queries),

and compare runs or fleets through `SHIELD_Regression_*` with environment-aware pairwise checks and homogeneous stability cohort validation.
Compare runs with `SHIELD_Regression_CheckIdentical`; compare fleets with `SHIELD_Regression_CheckStability`. Load cohorts from SQLite via `SHIELD_Regression_CheckIdenticalFromStorage` and `SHIELD_Regression_CheckStabilityFromStorage` in `regression.go`.

Format ScenarioRunResult slices plus identical or stability regression verdicts via `SHIELD_Rendering_*` in `rendering.go`.
*/
package shield
