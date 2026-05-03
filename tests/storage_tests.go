package tests

import (
	"os"
	"path/filepath"
	"shield"
	"testing"
)

func TestShieldStoragePublicAPI(t *testing.T) {
	// 1. Setup and Execution
	scenario := buildSumScenario()
	targetVersion := "v1"
	targetEnv := "local"

	runResult := shield.SHIELD_Testing_ScenarioRun(scenario, shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     targetVersion,
			Environment: targetEnv,
		},
	})

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
