package shield

import "shield/internal"

/*
SHIELD_Testing_Storage_Engine is the public facade handle for SHIELD's internal SQLite-backed testing storage engine.
*/
type SHIELD_Testing_Storage_Engine = internal.TestResultDatabase

/*
SHIELD_Testing_Storage_CohortVersionRecord summarizes one DISTINCT Version string seen among persisted scenario rows that share
a scenario_name plus environment column. LastSeen is the cohort’s freshest stored row timestamp (MAX); RunCount counts matching
runs regardless of pass/failure. Enumerate these before pinning SHIELD_Testing_SystemIdentity.Version for deeper loads
through SHIELD_Testing_Storage_ScenarioResultFindByIdentity.
*/
type SHIELD_Testing_Storage_CohortVersionRecord = internal.CohortVersionRecord

/*
SHIELD_Testing_Storage_EngineCreate creates a storage engine bound to the given SQLite database file path.
*/
func SHIELD_Testing_Storage_EngineCreate(dbFilePath string) *SHIELD_Testing_Storage_Engine {
	return internal.TestResultDatabaseCreate(dbFilePath)
}

/*
SHIELD_Testing_Storage_EngineInitialize ensures the storage engine connection is opened and initialized.
*/
func SHIELD_Testing_Storage_EngineInitialize(engine *SHIELD_Testing_Storage_Engine) error {
	return internal.TestResultDatabaseInitialize(engine)
}

/*
SHIELD_Testing_Storage_EngineClose closes the storage engine connection when it is currently open.
*/
func SHIELD_Testing_Storage_EngineClose(engine *SHIELD_Testing_Storage_Engine) error {
	return internal.TestResultDatabaseClose(engine)
}

/*
SHIELD_Testing_Storage_ScenarioResultAdd persists one scenario run result and all associated guard results as one transaction.
*/
func SHIELD_Testing_Storage_ScenarioResultAdd(
	engine *SHIELD_Testing_Storage_Engine,
	scenarioResult SHIELD_Testing_ScenarioRunResult,
) error {
	return internal.TestResultDatabaseScenarioResultAdd(engine, scenarioResult)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByID fetches a full scenario run result by persisted scenario id, including all guard result details.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByID(
	engine *SHIELD_Testing_Storage_Engine,
	id string,
) (*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByID(engine, id)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByName fetches hydrated scenario aggregates for a scenario name only.

Identity columns are unconstrained—all environments and versions sharing that scenario_name may appear.

Prefer identity-scoped accessors or cohort version discovery when isolating lineage.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByName(
	engine *SHIELD_Testing_Storage_Engine,
	name string,
) ([]*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByName(engine, name)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByIdentity retrieves rows whose scenario name matches name and whose persisted Environment
and Version columns equal identity.Environment and identity.Version.

Repository iteration order is undefined; callers sort by ScenarioRun StartedAt when they need chronological ordering.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByIdentity(
	engine *SHIELD_Testing_Storage_Engine,
	name string,
	identity SHIELD_Testing_SystemIdentity,
) ([]*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByIdentity(engine, name, identity.Environment, identity.Version)
}

/*
SHIELD_Testing_Storage_GetCohortVersions aggregates DISTINCT Version strings ever recorded under scenarioName plus Environment,
ordering groups by freshest activity descending (SQLite MAX persisted timestamp per Version). Limit caps DISTINCT group count;
supply a generously positive ceiling for exploratory UIs because limit 0 follows SQL LIMIT 0 (usually empty).

Combine the returned Versions with ScenarioResultFindByIdentity once operators pick regression comparison targets.
*/
func SHIELD_Testing_Storage_GetCohortVersions(
	engine *SHIELD_Testing_Storage_Engine,
	scenarioName string,
	environment string,
	limit int,
) ([]SHIELD_Testing_Storage_CohortVersionRecord, error) {
	return internal.TestResultDatabaseGetCohortVersions(engine, scenarioName, environment, limit)
}
