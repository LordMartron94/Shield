package tests

import (
	"fmt"
	"foundation"
	"foundation/formatting"
	"shield"
	"testing"
)

func TestShield(t *testing.T) {
	scenario := buildSumScenario()
	execCtx := shield.SHIELD_Testing_ExecutionContext{
		Identity: shield.SHIELD_Testing_SystemIdentity{
			Version:     "v1",
			Environment: "local",
		},
	}
	result := runSingleScenario(t, "test_shield_operation", scenario, execCtx, shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}, "SHIELD", "Simple", "Sum")
	overhead := extractResultOverhead(result)

	if !result.Passed() {
		guardResults := result.GuardResults()
		for _, guardResult := range guardResults {
			duration := formatting.FormatDurationNSF64(float64(guardResult.Duration().Nanoseconds()))

			if guardResult.Passed() {
				t.Logf("guard '%s' succeeded in %s", guardResult.Name(), duration)
			} else {
				t.Logf("guard '%s' failed in %s with reason: %s", guardResult.Name(), duration, guardResult.FailureReason())
			}
		}

		t.Fatal(formatOverheadReport(result.Name(), "failed", overhead))
	}

	t.Log(formatOverheadReport(result.Name(), "succeeded", overhead))
}

func init() {
	scenario := buildSumScenario()

	runCfg := shield.SHIELD_Testing_ScenarioRunConfig{
		MaxIterations: 1,
	}

	operation := shield.SHIELD_Testing_OperationCreateStateless(
		"sum_test_operation",
		func(_ struct{}, execCtx shield.SHIELD_Testing_ExecutionContext) []shield.SHIELD_Testing_ScenarioRunResult {
			return []shield.SHIELD_Testing_ScenarioRunResult{
				runScenarioWithContext(scenario, execCtx, runCfg),
			}
		},
		[]string{"SHIELD", "Internal", "Sum"}...,
	)
	shield.SHIELD_Registry_OperationRegister(operation)
}

func buildSumScenario() shield.SHIELD_Testing_Scenario[[]int, int] {
	guards := make([]shield.SHIELD_Testing_Guard[[]int, int], 1)
	guards[0] = shield.SHIELD_Testing_GuardCreate(
		"sum_works",
		[]int{1, 4, 6},
		shield.SHIELD_Testing_GuardPolicyMustEqual(
			func(a, b int) bool {
				return a == b
			},
			func(item int) string {
				return fmt.Sprintf("%d", item)
			},
			11,
		),
	)

	return shield.SHIELD_Testing_ScenarioCreate(
		"test_scenario",
		guards,
		func(input []int) (int, error) {
			counter := 0
			for _, v := range input {
				counter += v
			}

			return counter, nil
		},
	)
}

func runScenarioWithContext[TInput, TOutput any](
	scenario shield.SHIELD_Testing_Scenario[TInput, TOutput],
	execCtx shield.SHIELD_Testing_ExecutionContext,
	runCfg shield.SHIELD_Testing_ScenarioRunConfig,
) shield.SHIELD_Testing_ScenarioRunResult {
	return shield.SHIELD_Testing_OperationRunScenario(scenario, execCtx, runCfg)
}

func runSingleScenario[TInput, TOutput any](
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
				runScenarioWithContext(scenario, opCtx, runCfg),
			}
		},
		opZones...,
	)

	opResult := shield.SHIELD_Testing_OperationRun(&operation, execCtx)
	scenarioResults := opResult.ScenarioResults()
	if len(scenarioResults) == 0 {
		t.Fatalf("expected operation to return one scenario result, got 0")
	}

	return scenarioResults[0]
}

func extractResultOverhead(result shield.SHIELD_Testing_ScenarioRunResult) foundation.Overhead {
	wallNS := float64(result.WallDuration().Nanoseconds())
	summedNS := float64(result.SummedDuration().Nanoseconds())
	return foundation.ComputeOverheadNS(wallNS, summedNS)
}

func formatOverheadReport(name, status string, overhead foundation.Overhead) string {
	return fmt.Sprintf(
		"scenario '%s' %s in %s wall and %s summed | Lost: %s (%.2f%%)",
		name,
		status,
		formatting.FormatDurationNSF64(overhead.WallNS),
		formatting.FormatDurationNSF64(overhead.SummedNS),
		formatting.FormatDurationNSF64(overhead.AbsoluteLostNS),
		overhead.LostPercentage,
	)
}
