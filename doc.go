/*
Package shield is a structured validation and regression harness.

Run scenarios via SHIELD_Testing_* with mandatory SHIELD_Testing_SystemIdentity, replay prior snapshots,

persist aggregates through SHIELD_Testing_Storage_* (including identity-filtered lookups and cohort version discovery),

and compare pairwise or populations with SHIELD_Regression_* plus optional SQLite loaders.

Render ScenarioRun slices and regression verdicts via SHIELD_Rendering_* in rendering.go.
*/
package shield
