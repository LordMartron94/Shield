package tests

import (
	"fmt"
	"shield"
	"shield/internal"
	"testing"
)

func TestShieldRendering(t *testing.T) {
	runConfig := shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}

	// 1. Generate a diverse dataset spanning multiple zones with passing and failing states.
	scenarios := []shield.SHIELD_Testing_ScenarioRunResult{
		shield.SHIELD_Testing_ScenarioRun(buildPassScenario("Sum_Positive", "Math", "Addition"), runConfig),
		shield.SHIELD_Testing_ScenarioRun(buildFailScenario("Sum_Negative", "Math", "Addition"), runConfig),
		shield.SHIELD_Testing_ScenarioRun(buildPassScenario("Divide_Floats", "Math", "Division"), runConfig),
		shield.SHIELD_Testing_ScenarioRun(buildFailScenario("Connect_Postgres", "Infrastructure", "Database"), runConfig),
		shield.SHIELD_Testing_ScenarioRun(buildPassScenario("Ping_Redis", "Infrastructure", "Database"), runConfig),
		shield.SHIELD_Testing_ScenarioRun(buildPassScenario("Parse_Config", "Core", "Config"), runConfig),
	}

	modes := []struct {
		name string
		mode shield.SHIELD_Rendering_ColorMode
	}{
		{"TrueColor (24-bit)", shield.SHIELD_Rendering_Color_True},
		{"ANSI16 (4-bit)", shield.SHIELD_Rendering_Color_ANSI16},
		{"None (Plaintext)", shield.SHIELD_Rendering_Color_None},
	}

	// 2. Render the exact same dataset across all three color modes.
	for _, tc := range modes {
		fmt.Printf("\n\n========================================\n")
		fmt.Printf(" MODE: %s\n", tc.name)
		fmt.Printf("========================================\n\n")

		cfg := shield.SHIELD_Rendering_ConfigurationCreate(tc.mode)
		renderer := shield.SHIELD_Rendering_RendererCreate(cfg)

		// Note: We duplicate the slice because the renderer sorts it in-place
		// and we want to ensure the function works regardless of initial order.
		sliceCopy := make([]shield.SHIELD_Testing_ScenarioRunResult, len(scenarios))
		copy(sliceCopy, scenarios)

		output := shield.SHIELD_Rendering_FormatScenarioRunResults(renderer, sliceCopy)

		// Print directly to stdout to bypass test-logger ANSI stripping
		fmt.Println(output)
	}
}

func TestShieldRegressionRendering(t *testing.T) {
	// -------------------------------------------------------------------------
	// 1. Synthesize Identical Regression Data
	// -------------------------------------------------------------------------
	identicalData := internal.RegressionResult{
		IsRegression: true,
		GuardDeltas: []internal.GuardRegressionDelta{
			{
				GuardName:      "Parse_Config_Malformed",
				Severity:       internal.RegressionSeverity_OutcomeShift,
				BaselinePassed: true,
				TargetPassed:   false,
				TargetIter:     0,
				TargetReason:   "expected error, got none",
			},
			{
				GuardName:      "Calculate_Trajectory",
				Severity:       internal.RegressionSeverity_Degradation,
				BaselinePassed: false,
				TargetPassed:   false,
				BaselineIter:   0,
				TargetIter:     0,
				BaselineReason: "expected 5.0, got 4.9",
				TargetReason:   "unexpected panic: nil pointer dereference",
			},
			{
				GuardName:      "Fuzz_Hash_Collision",
				Severity:       internal.RegressionSeverity_FragilityShift,
				BaselinePassed: false,
				TargetPassed:   false,
				BaselineIter:   95000,
				TargetIter:     12,
				BaselineReason: "collision detected",
				TargetReason:   "collision detected",
			},
			{
				GuardName:      "Network_Timeout",
				Severity:       internal.RegressionSeverity_Improvement,
				BaselinePassed: false,
				TargetPassed:   true,
				BaselineIter:   4,
				BaselineReason: "deadline exceeded",
			},
		},
	}

	// -------------------------------------------------------------------------
	// 2. Synthesize Stability Regression Data
	// -------------------------------------------------------------------------
	stabilityData := internal.StabilityRegressionResult{
		IsRegression: true,
		Severity:     internal.RegressionSeverity_OutcomeShift,
		Reason:       "Statistically significant failure rate increase (p=0.0124). Rate shifted from 2.00% to 14.00%",
		Baseline: internal.StabilityStats{
			TotalRuns:         500,
			FailedRuns:        10,
			FailureRate:       0.02,
			AverageFailedIter: 85000,
		},
		Target: internal.StabilityStats{
			TotalRuns:         500,
			FailedRuns:        70,
			FailureRate:       0.14,
			AverageFailedIter: 12000,
		},
	}

	// -------------------------------------------------------------------------
	// 3. Render Across Modes
	// -------------------------------------------------------------------------
	modes := []struct {
		name string
		mode internal.RenderingColorMode
	}{
		{"TrueColor (24-bit)", internal.Render_Color_True},
		{"ANSI16 (4-bit)", internal.Render_Color_ANSI16},
		{"None (Plaintext)", internal.Render_Color_None},
	}

	for _, tc := range modes {
		fmt.Printf("\n\n========================================\n")
		fmt.Printf(" MODE: %s\n", tc.name)
		fmt.Printf("========================================\n\n")

		cfg := internal.RenderingConfigurationCreate(tc.mode)
		renderer := internal.RendererCreate(cfg)

		// 1. Identical Output
		fmt.Println(internal.RenderIdenticalRegression(renderer, identicalData))

		// 2. Stability Output
		fmt.Println(internal.RenderStabilityRegression(renderer, stabilityData))
	}
}

// --------------------------------------------------------------- DUMMY BUILDERS

func buildPassScenario(name string, zones ...string) shield.SHIELD_Testing_Scenario[int, int] {
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
		zones...,
	)
}

func buildFailScenario(name string, zones ...string) shield.SHIELD_Testing_Scenario[int, int] {
	guard := shield.SHIELD_Testing_GuardCreate(
		"expected_to_fail",
		1,
		shield.SHIELD_Testing_GuardPolicyMustEqual(
			func(a, b int) bool { return a == b },
			func(item int) string { return fmt.Sprintf("%d", item) },
			99, // Expect 99, but executor returns 1. This guarantees a failure reason.
		),
	)

	return shield.SHIELD_Testing_ScenarioCreate(
		name,
		[]shield.SHIELD_Testing_Guard[int, int]{guard},
		func(input int) (int, error) {
			return input, nil
		},
		zones...,
	)
}
