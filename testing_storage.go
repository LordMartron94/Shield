package shield

import "shield/internal"

/*
SHIELD_Testing_Storage_Engine is the public facade handle for SHIELD's internal
SQLite-backed testing storage engine.
*/
type SHIELD_Testing_Storage_Engine = internal.TestResultDatabase

/*
SHIELD_Testing_Storage_EngineCreate creates a storage engine bound to the given
SQLite database file path.
*/
func SHIELD_Testing_Storage_EngineCreate(dbFilePath string) *SHIELD_Testing_Storage_Engine {
	return internal.TestResultDatabaseCreate(dbFilePath)
}

/*
SHIELD_Testing_Storage_EngineInitialize ensures the storage engine connection is
opened and initialized.
*/
func SHIELD_Testing_Storage_EngineInitialize(engine *SHIELD_Testing_Storage_Engine) error {
	return internal.TestResultDatabaseInitialize(engine)
}

/*
SHIELD_Testing_Storage_EngineClose closes the storage engine connection when it
is currently open.
*/
func SHIELD_Testing_Storage_EngineClose(engine *SHIELD_Testing_Storage_Engine) error {
	return internal.TestResultDatabaseClose(engine)
}

/*
SHIELD_Testing_Storage_ScenarioResultAdd persists one scenario run result and
all associated guard results as one transaction.
*/
func SHIELD_Testing_Storage_ScenarioResultAdd(
	engine *SHIELD_Testing_Storage_Engine,
	scenarioResult SHIELD_Testing_ScenarioRunResult,
) error {
	return internal.TestResultDatabaseScenarioResultAdd(engine, scenarioResult)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByID fetches a full scenario run result
by persisted scenario id, including all guard result details.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByID(
	engine *SHIELD_Testing_Storage_Engine,
	id string,
) (*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByID(engine, id)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByName fetches hydrated scenario aggregates for a scenario name only.

Identity fields are not filtered—all environments and versions sharing that name may appear. Prefer

SHIELD_Testing_Storage_ScenarioResultFindByIdentity when cohorts must isolate Environment and Version columns.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByName(
	engine *SHIELD_Testing_Storage_Engine,
	name string,
) ([]*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByName(engine, name)
}

/*
SHIELD_Testing_Storage_ScenarioResultFindByIdentity retrieves results whose scenario name matches name and whose

persisted Environment and Version columns equal identity’s fields.

Repository iteration order is undefined; consumers sorting by time should order StartedAt themselves.
*/
func SHIELD_Testing_Storage_ScenarioResultFindByIdentity(
	engine *SHIELD_Testing_Storage_Engine,
	name string,
	identity SHIELD_Testing_SystemIdentity,
) ([]*SHIELD_Testing_ScenarioRunResult, error) {
	return internal.TestResultDatabaseScenarioResultFindByIdentity(engine, name, identity.Environment, identity.Version)
}
