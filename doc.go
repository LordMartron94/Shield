/*
Package shield is a structured validation and regression harness.

Exercise scenarios through the `testing` surface (`SHIELD_Testing_*`) and optionally persist aggregates via `SHIELD_Testing_Storage_*`.
Compare runs with `SHIELD_Regression_CheckIdentical`; compare fleets with `SHIELD_Regression_CheckStability`. Load cohorts from SQLite via `SHIELD_Regression_CheckIdenticalFromStorage` and `SHIELD_Regression_CheckStabilityFromStorage` in `regression.go`.

Format ScenarioRunResult slices for terminals or logs with `SHIELD_Rendering_*` in `rendering.go`.
*/
package shield
