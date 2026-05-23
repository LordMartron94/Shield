package internal

import "essence"

/*
TestingRuntimeContext carries subprocess guard execution paths and operation-level state snapshots.

Set on ExecutionContext by the SHIELD runner before OperationRun; scenarios and evaluateGuard read it.
*/
type TestingRuntimeContext struct {
	ConfigurationPath       string
	TransientBinaryPath     string
	OperationName           string
	OperationStateSnapshot  *[]byte
	SerializeOperationState func() ([]byte, error)
	ScenarioSeedOverride    *essence.UUID
}

/*
IsolatedGuardTarget limits ScenarioRun to one guard when running inside an isolated child process.
*/
type IsolatedGuardTarget struct {
	ScenarioName string
	GuardName    string
}
