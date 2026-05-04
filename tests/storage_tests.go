package tests

import (
	"os"
	"path/filepath"
	"shield"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestShieldStoragePublicAPI(t *testing.T) {
	// 1. Setup and Execution
	scenario := buildSumScenario()
	targetVersion := "v1"
	targetEnv := "local"
	execCtx := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     targetVersion,
			Environment: targetEnv,
		},
	}

	runResult := runSingleScenario(t, "storage_public_api_operation", scenario, execCtx, shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}, "SHIELD", "Simple", "Sum")

	dbPath := filepath.Join(t.TempDir(), "shield_storage_test.sqlite")
	storage := shield.SHIELD_Testing_Storage_EngineCreate(dbPath)
	defer shield.SHIELD_Testing_Storage_EngineClose(storage)

	// 2. Persistence
	if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(storage, runResult); err != nil {
		t.Fatalf("expected scenario result to persist, got error: %v", err)
	}

	// 3. Verify Discovery API (The Catalog)
	versions, err := shield.SHIELD_Testing_Storage_GetCohortVersions(storage, runResult.Name(), targetEnv, 10)
	if err != nil {
		t.Fatalf("expected cohort version discovery to succeed, got error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 discovered version, got %d", len(versions))
	}
	if versions[0].Version != targetVersion {
		t.Fatalf("expected discovered version to be %s, got %s", targetVersion, versions[0].Version)
	}
	if versions[0].RunCount != 1 {
		t.Fatalf("expected version run count to be 1, got %d", versions[0].RunCount)
	}

	// 4. Verify Identity-Bound Hydration (The Cohort Purity)
	storedScenarios, err := shield.SHIELD_Testing_Storage_ScenarioResultFindByIdentity(
		storage,
		runResult.Name(),
		shield.SHIELD_Testing_SystemIdentity{
			Version:     targetVersion,
			Environment: targetEnv,
		},
	)
	if err != nil {
		t.Fatalf("expected query by identity to succeed, got error: %v", err)
	}
	if len(storedScenarios) != 1 {
		t.Fatalf("expected one stored scenario, got %d", len(storedScenarios))
	}

	storedScenario := storedScenarios[0]
	if storedScenario.Name() != runResult.Name() {
		t.Fatalf("expected scenario name %s, got %s", runResult.Name(), storedScenario.Name())
	}
	if len(storedScenario.GuardResults()) != len(runResult.GuardResults()) {
		t.Fatalf("expected %d guards, got %d", len(runResult.GuardResults()), len(storedScenario.GuardResults()))
	}
	if !storedScenario.Passed() {
		t.Fatal("expected stored scenario to pass")
	}

	// 5. File System Verification
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite database file to exist, got error: %v", err)
	}
}

func TestShieldStorageOperationVersionLookups(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shield_storage_scope.sqlite")
	storage := shield.SHIELD_Testing_Storage_EngineCreate(dbPath)
	defer shield.SHIELD_Testing_Storage_EngineClose(storage)

	env := "local"
	operationScope := "ops.alpha"
	scenarioZonePath := "ops.alpha.case"

	persistStorageScenarioVersion(t, storage, env, "git-v1", scenarioZonePath)
	time.Sleep(2 * time.Millisecond)
	persistStorageScenarioVersion(t, storage, env, "git-v2", scenarioZonePath)

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

func persistStorageScenarioVersion(
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
		"storage_scope_operation",
		scenario,
		execCtx,
		shield.SHIELD_Testing_ScenarioRunConfig{
			MaxIterations: 1,
		},
		strings.Split(scenarioZonePath, ".")...,
	)

	if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(storage, result); err != nil {
		t.Fatalf("expected scenario result add to succeed, got error: %v", err)
	}
}
