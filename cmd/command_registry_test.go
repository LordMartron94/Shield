package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"shield"
	"shield/internal"
	"strconv"
	"strings"
	"testing"
)

func TestResolveImpactedTargets_CommitDatabaseGate(t *testing.T) {
	t.Run("clean new commit runs", func(t *testing.T) {
		ctx, op := setupImpactedTestContext(t, false)
		persistOperationScenario(t, ctx.Storage, "ops.alpha.case", "git-oldhash", ctx.Config.Environment.Name)

		targets := resolveImpactedTargets(ctx, []internal.RegisteredOperation{op})
		if len(targets) != 1 {
			t.Fatalf("expected operation to run for new commit, got %d targets", len(targets))
		}
	})

	t.Run("clean latest unchanged skips", func(t *testing.T) {
		ctx, op := setupImpactedTestContext(t, false)
		currentVersion := currentGitVersion(t, ctx.GitRoot)
		persistOperationScenario(t, ctx.Storage, "ops.alpha.case", currentVersion, ctx.Config.Environment.Name)

		targets := resolveImpactedTargets(ctx, []internal.RegisteredOperation{op})
		if len(targets) != 0 {
			t.Fatalf("expected operation to skip when latest commit matches, got %d targets", len(targets))
		}
	})

	t.Run("clean commit exists but not latest skips", func(t *testing.T) {
		ctx, op := setupImpactedTestContext(t, false)
		currentVersion := currentGitVersion(t, ctx.GitRoot)
		persistOperationScenario(t, ctx.Storage, "ops.alpha.case", currentVersion, ctx.Config.Environment.Name)
		persistOperationScenario(t, ctx.Storage, "ops.alpha.case", "git-newerhash", ctx.Config.Environment.Name)

		targets := resolveImpactedTargets(ctx, []internal.RegisteredOperation{op})
		if len(targets) != 0 {
			t.Fatalf("expected operation to skip when commit already exists, got %d targets", len(targets))
		}
	})

	t.Run("dirty always runs", func(t *testing.T) {
		ctx, op := setupImpactedTestContext(t, true)
		currentVersion := currentGitVersion(t, ctx.GitRoot)
		persistOperationScenario(t, ctx.Storage, "ops.alpha.case", currentVersion, ctx.Config.Environment.Name)

		targets := resolveImpactedTargets(ctx, []internal.RegisteredOperation{op})
		if len(targets) != 1 {
			t.Fatalf("expected dirty operation to run ephemerally, got %d targets", len(targets))
		}
	})
}

func setupImpactedTestContext(t *testing.T, dirty bool) (*ShellContext, internal.RegisteredOperation) {
	t.Helper()

	repo := createTempGitRepo(t)
	if dirty {
		writeErr := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0644)
		if writeErr != nil {
			t.Fatalf("failed to create dirty file: %v", writeErr)
		}
	}

	dbPath := filepath.Join(t.TempDir(), "impacted.sqlite")
	storage := shield.SHIELD_Testing_Storage_EngineCreate(dbPath)
	if err := shield.SHIELD_Testing_Storage_EngineInitialize(storage); err != nil {
		t.Fatalf("failed to initialize storage: %v", err)
	}
	t.Cleanup(func() {
		_ = shield.SHIELD_Testing_Storage_EngineClose(storage)
	})

	cfg := &ShieldConfiguration{
		Environment: ShieldEnvironment{
			Name: "test-local",
		},
		ZoneMapping: ShieldZoneMap{
			"ops.alpha": ".",
		},
	}

	renderer := internal.RendererCreate(internal.RenderingConfigurationCreate(internal.Render_Color_None))
	ctx := &ShellContext{
		Renderer: renderer,
		Builder:  &strings.Builder{},
		Storage:  storage,
		Config:   cfg,
		GitRoot:  repo,
	}

	opName := "impacted-op-" + strings.ReplaceAll(t.Name(), "/", "-")
	operation := shield.SHIELD_Testing_OperationCreateStateless(
		opName,
		func(_ struct{}, _ shield.SHIELD_Testing_ExecutionContext) []shield.SHIELD_Testing_ScenarioRunResult {
			return nil
		},
		"ops", "alpha",
	)
	shield.SHIELD_Registry_OperationRegister(operation)

	allOps := fetchAndSortOperations()
	for _, op := range allOps {
		if op.Name() == opName {
			return ctx, op
		}
	}

	t.Fatalf("failed to find registered operation %s", opName)
	return nil, internal.RegisteredOperation{}
}

func createTempGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.email", "shield-tests@example.com")
	runCmd(t, repo, "git", "config", "user.name", "shield-tests")

	writeErr := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("seed"), 0644)
	if writeErr != nil {
		t.Fatalf("failed writing tracked file: %v", writeErr)
	}

	runCmd(t, repo, "git", "add", "tracked.txt")
	runCmd(t, repo, "git", "commit", "-m", "init")

	return repo
}

func currentGitVersion(t *testing.T, repo string) string {
	t.Helper()

	out := runCmd(t, repo, "git", "rev-parse", "HEAD")
	return "git-" + strings.TrimSpace(out)
}

func persistOperationScenario(t *testing.T, storage *shield.SHIELD_Testing_Storage_Engine, scenarioZonePath string, version string, environment string) {
	t.Helper()

	guard := shield.SHIELD_Testing_GuardCreate(
		"eq",
		1,
		shield.SHIELD_Testing_GuardPolicyMustEqual(
			func(a, b int) bool { return a == b },
			func(v int) string { return strconv.Itoa(v) },
			1,
		),
	)

	scenario := shield.SHIELD_Testing_ScenarioCreate(
		"persisted-op-scenario",
		[]shield.SHIELD_Testing_Guard[int, int]{guard},
		func(input int) (int, error) { return input, nil },
	)

	execCtx := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     version,
			Environment: environment,
		},
	}
	result := runScenarioThroughOperation(
		t,
		"persist_operation_scenario",
		scenario,
		execCtx,
		shield.SHIELD_Testing_ScenarioRunConfig{MaxIterations: 1},
		strings.Split(scenarioZonePath, ".")...,
	)

	if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(storage, result); err != nil {
		t.Fatalf("failed to persist scenario result: %v", err)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %v: %v (%s)", name, args, err, string(out))
	}
	return string(out)
}

func runScenarioThroughOperation[TInput, TOutput any](
	t *testing.T,
	opName string,
	scenario shield.SHIELD_Testing_Scenario[TInput, TOutput],
	execCtx shield.SHIELD_Testing_ExecutionContext,
	runCfg shield.SHIELD_Testing_ScenarioRunConfig,
	opZones ...string,
) shield.SHIELD_Testing_ScenarioRunResult {
	t.Helper()

	operation := shield.SHIELD_Testing_OperationCreateStateless(
		opName,
		func(_ struct{}, opCtx shield.SHIELD_Testing_ExecutionContext) []shield.SHIELD_Testing_ScenarioRunResult {
			return []shield.SHIELD_Testing_ScenarioRunResult{
				shield.SHIELD_Testing_OperationRunScenario(scenario, opCtx, runCfg),
			}
		},
		opZones...,
	)

	opResult := shield.SHIELD_Testing_OperationRun(&operation, execCtx)
	results := opResult.ScenarioResults()
	if len(results) == 0 {
		t.Fatalf("expected operation to produce scenario result")
	}

	return results[0]
}
