package tests

import (
	"path/filepath"
	"shield"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStorageOperationVersionLookups(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shield_storage_scope.sqlite")
	storage := shield.SHIELD_Testing_Storage_EngineCreate(dbPath)
	defer shield.SHIELD_Testing_Storage_EngineClose(storage)

	env := "local"
	operationScope := "ops.alpha"
	scenarioZonePath := "ops.alpha.case"

	persistStorageScenarioVersionForTest(t, storage, env, "git-v1", scenarioZonePath)
	time.Sleep(2 * time.Millisecond)
	persistStorageScenarioVersionForTest(t, storage, env, "git-v2", scenarioZonePath)

	latest, found, latestErr := shield.SHIELD_Testing_Storage_GetLatestOperationVersion(storage, operationScope, env)
	if latestErr != nil {
		t.Fatalf("expected latest operation version lookup to succeed, got: %v", latestErr)
	}
	if !found {
		t.Fatal("expected latest operation version to be found")
	}
	if latest != "git-v2" {
		t.Fatalf("expected latest operation version git-v2, got %s", latest)
	}

	existsV1, errV1 := shield.SHIELD_Testing_Storage_OperationVersionExists(storage, operationScope, env, "git-v1")
	if errV1 != nil {
		t.Fatalf("expected existence lookup for git-v1 to succeed, got: %v", errV1)
	}
	if !existsV1 {
		t.Fatal("expected git-v1 to exist for operation scope")
	}

	existsV3, errV3 := shield.SHIELD_Testing_Storage_OperationVersionExists(storage, operationScope, env, "git-v3")
	if errV3 != nil {
		t.Fatalf("expected existence lookup for git-v3 to succeed, got: %v", errV3)
	}
	if existsV3 {
		t.Fatal("expected git-v3 to not exist for operation scope")
	}
}

func persistStorageScenarioVersionForTest(
	t *testing.T,
	storage *shield.SHIELD_Testing_Storage_Engine,
	environment string,
	version string,
	scenarioZonePath string,
) {
	t.Helper()

	scenario := shield.SHIELD_Testing_ScenarioCreate(
		"storage_scope_scenario",
		[]shield.SHIELD_Testing_Guard[int, int]{
			shield.SHIELD_Testing_GuardCreate(
				"identity",
				1,
				shield.SHIELD_Testing_GuardPolicyMustEqual(
					func(a, b int) bool { return a == b },
					func(v int) string { return strconv.Itoa(v) },
					1,
				),
			),
		},
		func(input int) (int, error) { return input, nil },
	)

	execCtx := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     version,
			Environment: environment,
		},
	}
	result := runSingleScenario(
		t,
		"storage_scope_api_operation",
		scenario,
		execCtx,
		shield.SHIELD_Testing_ScenarioRunConfig{MaxIterations: 1},
		strings.Split(scenarioZonePath, ".")...,
	)

	if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(storage, result); err != nil {
		t.Fatalf("expected scenario result add to succeed, got error: %v", err)
	}
}
