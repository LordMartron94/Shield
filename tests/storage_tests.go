package tests

import (
	"os"
	"path/filepath"
	"shield"
	"testing"
)

func TestShieldStoragePublicAPI(t *testing.T) {
	scenario := buildSumScenario()
	runResult := shield.SHIELD_Testing_ScenarioRun(scenario, shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	})

	dbPath := filepath.Join(t.TempDir(), "shield_storage_test.sqlite")
	storage := shield.SHIELD_Testing_Storage_EngineCreate(dbPath)

	if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(storage, runResult); err != nil {
		t.Fatalf("expected scenario result to persist, got error: %v", err)
	}

	storedScenarios, err := shield.SHIELD_Testing_Storage_ScenarioResultFindByName(storage, runResult.Name())
	if err != nil {
		t.Fatalf("expected query by name to succeed, got error: %v", err)
	}
	if len(storedScenarios) != 1 {
		t.Fatalf("expected one stored scenario, got %d", len(storedScenarios))
	}

	storedScenario := storedScenarios[0]
	if storedScenario.ScenarioName != runResult.Name() {
		t.Fatalf("expected scenario name %s, got %s", runResult.Name(), storedScenario.ScenarioName)
	}

	guards, err := shield.SHIELD_Testing_Storage_GuardsFindByScenarioID(storage, storedScenario.ResultID)
	if err != nil {
		t.Fatalf("expected guard query to succeed, got error: %v", err)
	}
	if len(guards) != len(runResult.GuardResults()) {
		t.Fatalf("expected %d guards, got %d", len(runResult.GuardResults()), len(guards))
	}

	roundTrippedResult, err := shield.SHIELD_Testing_Storage_ScenarioResultFindByID(storage, storedScenario.ResultID)
	if err != nil {
		t.Fatalf("expected query by id to succeed, got error: %v", err)
	}
	if roundTrippedResult.Name() != runResult.Name() {
		t.Fatalf("expected round-tripped scenario name %s, got %s", runResult.Name(), roundTrippedResult.Name())
	}
	if !roundTrippedResult.Passed() {
		t.Fatal("expected round-tripped scenario to pass")
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite database file to exist, got error: %v", err)
	}
}
