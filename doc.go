/*
Package shield is a structured validation and regression harness.

Execute scenarios via SHIELD_Testing_* using mandatory SHIELD_Testing_SystemIdentity, replay aggregates from hydrated snapshots,

and persist aggregates through SHIELD_Testing_Storage_* (identity-filtered queries plus cohort version discovery).

Register discoverable bundles with SHIELD_Registry_* when tooling must enumerate workloads by zone metadata.

Compare pairwise or population cohorts with SHIELD_Regression_* (including SQLite loaders for persisted rows),

and render Scenario slices plus regression verdicts through SHIELD_Rendering_* helpers.
*/
package shield
