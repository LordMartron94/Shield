/*
Package shield is a structured validation and regression harness.

Exercise scenarios through the `testing` surface (`SHIELD_Testing_*`), hydrate prior snapshot configs (`SHIELD_Testing_SnapshotConfig` / replay helpers), and optionally persist aggregates via `SHIELD_Testing_Storage_*`.
Compare runs with `SHIELD_Regression_CheckIdentical`; compare fleets with `SHIELD_Regression_CheckStability`. Load cohorts from SQLite via `SHIELD_Regression_CheckIdenticalFromStorage` and `SHIELD_Regression_CheckStabilityFromStorage` in `regression.go`.

Format ScenarioRunResult slices plus identical or stability regression verdicts via `SHIELD_Rendering_*` in `rendering.go`.
*/
package shield
