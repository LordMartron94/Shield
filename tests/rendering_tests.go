package tests

import (
	"fmt"
	"shield"
	"testing"
	"time"
)

func TestShieldRendering(t *testing.T) {
	runConfig := shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}
	execCtx := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     "v1",
			Environment: "local",
		},
	}

	scenarios := []shield.SHIELD_Testing_ScenarioRunResult{
		runSingleScenario(t, "render_sum_positive", buildPassScenario("Sum_Positive"), execCtx, runConfig, "Math", "Addition"),
		runSingleScenario(t, "render_sum_negative", buildFailScenario("Sum_Negative"), execCtx, runConfig, "Math", "Addition"),
		runSingleScenario(t, "render_divide", buildPassScenario("Divide_Floats"), execCtx, runConfig, "Math", "Division"),
		runSingleScenario(t, "render_pg", buildFailScenario("Connect_Postgres"), execCtx, runConfig, "Infrastructure", "Database"),
		runSingleScenario(t, "render_redis", buildPassScenario("Ping_Redis"), execCtx, runConfig, "Infrastructure", "Database"),
		runSingleScenario(t, "render_cfg", buildPassScenario("Parse_Config"), execCtx, runConfig, "Core", "Config"),
	}

	modes := []struct {
		name string
		mode shield.SHIELD_Rendering_ColorMode
	}{
		{"TrueColor (24-bit)", shield.SHIELD_Rendering_Color_True},
		{"ANSI16 (4-bit)", shield.SHIELD_Rendering_Color_ANSI16},
		{"None (Plaintext)", shield.SHIELD_Rendering_Color_None},
	}

	for _, tc := range modes {
		fmt.Printf("\n\n========================================\n")
		fmt.Printf(" MODE: %s\n", tc.name)
		fmt.Printf("========================================\n\n")

		cfg := shield.SHIELD_Rendering_ConfigurationCreate(tc.mode)
		renderer := shield.SHIELD_Rendering_RendererCreate(cfg)

		sliceCopy := make([]shield.SHIELD_Testing_ScenarioRunResult, len(scenarios))
		copy(sliceCopy, scenarios)

		output := shield.SHIELD_Rendering_FormatScenarioRunResults(renderer, sliceCopy)
		fmt.Println(output)
	}
}

func TestShieldRegressionRendering(t *testing.T) {
	runConfigBase := shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}

	runConfigTarget := shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}
	execCtxBase := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     "v1",
			Environment: "local",
		},
	}
	execCtxTarget := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     "v2",
			Environment: "local",
		},
	}

	// Generate physical runs strictly to satisfy the "Sources" temporal rendering
	dummyBase := runSingleScenario(t, "render_dummy_base", buildPassScenario("Dummy"), execCtxBase, runConfigBase, "Render", "Dummy")
	dummyTarget := runSingleScenario(t, "render_dummy_target", buildPassScenario("Dummy"), execCtxTarget, runConfigTarget, "Render", "Dummy")

	// -------------------------------------------------------------------------
	// 1. Synthesize Identical Regression Data (Public API)
	// -------------------------------------------------------------------------
	identicalData := shield.SHIELD_Regression_Result{
		IsRegression: true,
		Sources:      []shield.SHIELD_Testing_ScenarioRunResult{dummyBase, dummyTarget},
		GuardDeltas: []shield.SHIELD_Regression_GuardDelta{
			{
				GuardName:      "Parse_Config_Malformed",
				Severity:       shield.SHIELD_Regression_Severity_OutcomeShift,
				BaselinePassed: true,
				TargetPassed:   false,
				TargetIter:     0,
				TargetReason:   "expected error, got none",
			},
			{
				GuardName:      "Calculate_Trajectory",
				Severity:       shield.SHIELD_Regression_Severity_Degradation,
				BaselinePassed: false,
				TargetPassed:   false,
				BaselineIter:   0,
				TargetIter:     0,
				BaselineReason: "expected 5.0, got 4.9",
				TargetReason:   "unexpected panic: nil pointer dereference",
			},
			{
				GuardName:      "Fuzz_Hash_Collision",
				Severity:       shield.SHIELD_Regression_Severity_FragilityShift,
				BaselinePassed: false,
				TargetPassed:   false,
				BaselineIter:   95000,
				TargetIter:     12,
				BaselineReason: "collision detected",
				TargetReason:   "collision detected",
			},
			{
				GuardName:      "Network_Timeout",
				Severity:       shield.SHIELD_Regression_Severity_Improvement,
				BaselinePassed: false,
				TargetPassed:   true,
				BaselineIter:   4,
				BaselineReason: "deadline exceeded",
			},
		},
	}

	// -------------------------------------------------------------------------
	// 2. Synthesize Stability Regression Data (Public API)
	// -------------------------------------------------------------------------
	now := time.Now()
	stabilityData := shield.SHIELD_Regression_Stability_Result{
		IsRegression: true,
		Severity:     shield.SHIELD_Regression_Severity_OutcomeShift,
		Reason:       "Statistically significant failure rate increase (p=0.0124). Rate shifted from 2.00% to 14.00%",
		Baseline: shield.SHIELD_Regression_StabilityStats{
			TotalRuns:         500,
			FailedRuns:        10,
			FailureRate:       0.02,
			AverageFailedIter: 85000,
			EarliestRun:       now.Add(-48 * time.Hour),
			LatestRun:         now.Add(-24 * time.Hour),
			Identity:          execCtxBase.Identity,
		},
		Target: shield.SHIELD_Regression_StabilityStats{
			TotalRuns:         500,
			FailedRuns:        70,
			FailureRate:       0.14,
			AverageFailedIter: 12000,
			EarliestRun:       now.Add(-24 * time.Hour),
			LatestRun:         now,
			Identity:          execCtxTarget.Identity,
		},
	}

	// -------------------------------------------------------------------------
	// 3. Render Across Modes
	// -------------------------------------------------------------------------
	modes := []struct {
		name string
		mode shield.SHIELD_Rendering_ColorMode
	}{
		{"TrueColor (24-bit)", shield.SHIELD_Rendering_Color_True},
		{"ANSI16 (4-bit)", shield.SHIELD_Rendering_Color_ANSI16},
		{"None (Plaintext)", shield.SHIELD_Rendering_Color_None},
	}

	for _, tc := range modes {
		fmt.Printf("\n\n========================================\n")
		fmt.Printf(" MODE: %s\n", tc.name)
		fmt.Printf("========================================\n\n")

		cfg := shield.SHIELD_Rendering_ConfigurationCreate(tc.mode)
		renderer := shield.SHIELD_Rendering_RendererCreate(cfg)

		// 1. Identical Output
		fmt.Println(shield.SHIELD_Rendering_FormatIdenticalRegressionReport(renderer, identicalData))

		// 2. Stability Output
		fmt.Println(shield.SHIELD_Rendering_FormatStabilityRegressionReport(renderer, stabilityData))
	}
}

// --------------------------------------------------------------- DUMMY BUILDERS

func buildPassScenario(name string) shield.SHIELD_Testing_Scenario[int, int] {
	guard := shield.SHIELD_Testing_GuardCreate(
		"returns_input",
		1,
		shield.SHIELD_Testing_GuardPolicyMustEqual(
			func(a, b int) bool { return a == b },
			func(item int) string { return fmt.Sprintf("%d", item) },
			1,
		),
	)

	return shield.SHIELD_Testing_ScenarioCreate(
		name,
		[]shield.SHIELD_Testing_Guard[int, int]{guard},
		func(input int) (int, error) {
			return input, nil
		},
	)
}

func buildFailScenario(name string) shield.SHIELD_Testing_Scenario[int, int] {
	guard := shield.SHIELD_Testing_GuardCreate(
		"expected_to_fail",
		1,
		shield.SHIELD_Testing_GuardPolicyMustEqual(
			func(a, b int) bool { return a == b },
			func(item int) string { return fmt.Sprintf("%d", item) },
			99,
		),
	)

	return shield.SHIELD_Testing_ScenarioCreate(
		name,
		[]shield.SHIELD_Testing_Guard[int, int]{guard},
		func(input int) (int, error) {
			return input, nil
		},
	)
}
