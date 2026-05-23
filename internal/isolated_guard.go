package internal

import (
	"encoding/base64"
	"encoding/json"
	"essence"
	"fmt"
	"os"
	"time"
)

const shieldGuardIsolationChildEnv = "SHIELD_GUARD_ISOLATION_CHILD"

/*
IsolatedGuardSpec is the JSON wire format for run-isolated-guard child processes.
*/
type IsolatedGuardSpec struct {
	OperationName       string         `json:"operationName"`
	ScenarioName        string         `json:"scenarioName"`
	GuardName           string         `json:"guardName"`
	ConfigurationPath   string         `json:"configurationPath"`
	StateSnapshotBase64 string         `json:"stateSnapshotBase64,omitempty"`
	Seed                string         `json:"seed"`
	FuzzingPattern      FuzzingPattern `json:"fuzzingPattern"`
	MaxIterations       uint64         `json:"maxIterations"`
	MaxDurationNs       int64          `json:"maxDurationNs"`
	UseDuration         bool           `json:"useDuration"`
	Identity            SystemIdentity `json:"identity"`
	ZonePathParts       []string       `json:"zonePathParts"`
}

/*
IsolatedGuardResponse is printed as one JSON line on stdout by the child process.
*/
type IsolatedGuardResponse struct {
	Passed              bool   `json:"passed"`
	FailureReason       string `json:"failureReason,omitempty"`
	FailureClass        string `json:"failureClass,omitempty"`
	FailedSeed          string `json:"failedSeed,omitempty"`
	FailedIteration     uint64 `json:"failedIteration,omitempty"`
	DurationNs          int64  `json:"durationNs"`
	StateSnapshotBase64 string `json:"stateSnapshotBase64,omitempty"`
}

func isolatedGuardChildProcessActive() bool {
	return os.Getenv(shieldGuardIsolationChildEnv) == "1"
}

func isolatedGuardSpecEncode(spec *IsolatedGuardSpec) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("isolated guard spec: nil")
	}
	return json.Marshal(spec)
}

func isolatedGuardSpecDecode(data []byte) (IsolatedGuardSpec, error) {
	var spec IsolatedGuardSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return IsolatedGuardSpec{}, fmt.Errorf("isolated guard spec decode: %w", err)
	}
	return spec, nil
}

func isolatedGuardResponseEncode(response *IsolatedGuardResponse) ([]byte, error) {
	if response == nil {
		return nil, fmt.Errorf("isolated guard response: nil")
	}
	return json.Marshal(response)
}

func isolatedGuardResponseDecode(line []byte) (IsolatedGuardResponse, error) {
	var response IsolatedGuardResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return IsolatedGuardResponse{}, fmt.Errorf("isolated guard response decode: %w", err)
	}
	return response, nil
}

func isolatedGuardSpecStateSnapshot(spec IsolatedGuardSpec) ([]byte, error) {
	if spec.StateSnapshotBase64 == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(spec.StateSnapshotBase64)
	if err != nil {
		return nil, fmt.Errorf("isolated guard spec state snapshot base64: %w", err)
	}
	return decoded, nil
}

func isolatedGuardResponseStateSnapshot(response IsolatedGuardResponse) []byte {
	if response.StateSnapshotBase64 == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(response.StateSnapshotBase64)
	if err != nil {
		return nil
	}
	return decoded
}

func isolatedGuardStateSnapshotBase64(snapshot []byte) string {
	if len(snapshot) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(snapshot)
}

/*
RunIsolatedGuard executes one guard for a registered operation inside a child SHIELD transient process.
*/
func RunIsolatedGuard(specPath string) (exitCode int, responseLine []byte, runErr error) {
	if err := os.Setenv(shieldGuardIsolationChildEnv, "1"); err != nil {
		return 2, nil, fmt.Errorf("isolated guard: set child env: %w", err)
	}

	specData, err := os.ReadFile(specPath)
	if err != nil {
		return 2, nil, fmt.Errorf("isolated guard: read spec %q: %w", specPath, err)
	}
	spec, err := isolatedGuardSpecDecode(specData)
	if err != nil {
		return 2, nil, err
	}

	registered, ok := registeredOperationFindByName(spec.OperationName)
	if !ok {
		return 2, nil, fmt.Errorf("isolated guard: unknown operation %q", spec.OperationName)
	}
	if err := operationStateCodecBoxRequiresCodecForSubprocess(registered.stateCodec, spec.OperationName); err != nil {
		return 2, nil, err
	}

	stateSnapshot, err := isolatedGuardSpecStateSnapshot(spec)
	if err != nil {
		return 2, nil, err
	}

	var state any
	if len(stateSnapshot) == 0 {
		state, err = registered.startup()
		if err != nil {
			return 2, nil, fmt.Errorf("isolated guard startup: %w", err)
		}
	} else {
		state, err = operationStateCodecBoxDeserialize(registered.stateCodec, stateSnapshot)
		if err != nil {
			return 2, nil, fmt.Errorf("isolated guard deserialize state: %w", err)
		}
	}

	seed, err := essence.UUIDFromString(spec.Seed)
	if err != nil {
		return 2, nil, fmt.Errorf("isolated guard seed: %w", err)
	}
	execCtx := ExecutionContext{
		Identity: spec.Identity,
		ZonePath: ZonePathCreate(spec.ZonePathParts...),
		IsolatedGuardTarget: &IsolatedGuardTarget{
			ScenarioName: spec.ScenarioName,
			GuardName:    spec.GuardName,
		},
		Runtime: &TestingRuntimeContext{
			ConfigurationPath:      spec.ConfigurationPath,
			OperationName:          spec.OperationName,
			OperationStateSnapshot: &stateSnapshot,
			ScenarioSeedOverride:   &seed,
		},
	}

	scenarioResults := registered.runScenarios(state, execCtx)
	operationResult := OperationRunResult{
		operationName:   spec.OperationName,
		zonePath:        execCtx.ZonePath,
		scenarioResults: scenarioResults,
		passed:          true,
	}
	for _, scenarioResult := range scenarioResults {
		if !scenarioResult.passed {
			operationResult.passed = false
		}
	}
	guardResult, found := isolatedGuardExtractGuardResult(operationResult, spec.ScenarioName, spec.GuardName)
	if !found {
		return 2, nil, fmt.Errorf(
			"isolated guard: no result for scenario %q guard %q",
			spec.ScenarioName,
			spec.GuardName,
		)
	}

	newSnapshot, err := operationStateCodecBoxSerialize(registered.stateCodec, state)
	if err != nil {
		return 2, nil, fmt.Errorf("isolated guard serialize state: %w", err)
	}

	response := IsolatedGuardResponse{
		Passed:              guardResult.passed,
		FailureReason:       guardResult.failureReason,
		FailureClass:        string(guardResult.failureClass),
		FailedSeed:          guardResult.failedSeed.String(),
		FailedIteration:     guardResult.failedIteration,
		DurationNs:          guardResult.duration.Nanoseconds(),
		StateSnapshotBase64: isolatedGuardStateSnapshotBase64(newSnapshot),
	}
	responseLine, err = isolatedGuardResponseEncode(&response)
	if err != nil {
		return 2, nil, err
	}

	if !guardResult.passed {
		return 1, responseLine, nil
	}
	return 0, responseLine, nil
}

func isolatedGuardExtractGuardResult(
	operationResult OperationRunResult,
	scenarioName string,
	guardName string,
) (GuardEvaluationResult, bool) {
	for _, scenarioResult := range operationResult.scenarioResults {
		if scenarioResult.scenarioName != scenarioName {
			continue
		}
		for _, guardResult := range scenarioResult.guardResults {
			if guardResult.guardName == guardName {
				return guardResult, true
			}
		}
	}
	return GuardEvaluationResult{}, false
}

func guardEvaluationResultFromIsolatedResponse(response IsolatedGuardResponse) GuardEvaluationResult {
	result := GuardEvaluationResult{
		passed:          response.Passed,
		failureReason:   response.FailureReason,
		failedIteration: response.FailedIteration,
		duration:        time.Duration(response.DurationNs),
		failureClass:    GuardFailureClass(response.FailureClass),
	}
	if response.FailedSeed != "" {
		if seed, err := essence.UUIDFromString(response.FailedSeed); err == nil {
			result.failedSeed = seed
		}
	}
	return result
}
