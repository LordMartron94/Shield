package internal

import (
	"essence"
	"fmt"
	"foundation/entropy"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

/*
evaluateGuardSubprocess runs one guard in a shield-transient child and maps process exit to GuardEvaluationResult.
*/
func evaluateGuardSubprocess[TInput, TOutput any](
	guard Guard[TInput, TOutput],
	scenario Scenario[TInput, TOutput],
	executor Executor[TInput, TOutput],
	seed essence.UUID,
	config ScenarioRunConfig,
	provider *entropy.EntropyProvider,
	execCtx ExecutionContext,
) GuardEvaluationResult {
	_ = executor
	_ = provider

	if execCtx.Runtime == nil {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: "subprocess guard isolation requires TestingRuntimeContext on ExecutionContext",
		}
	}
	if execCtx.Runtime.TransientBinaryPath == "" {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: "subprocess guard isolation requires TransientBinaryPath on TestingRuntimeContext",
		}
	}
	if execCtx.Runtime.OperationName == "" {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: "subprocess guard isolation requires OperationName on TestingRuntimeContext",
		}
	}

	registered, ok := registeredOperationFindByName(execCtx.Runtime.OperationName)
	if !ok {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: fmt.Sprintf("unknown operation %q", execCtx.Runtime.OperationName),
		}
	}
	if err := operationStateCodecBoxRequiresCodecForSubprocess(registered.stateCodec, execCtx.Runtime.OperationName); err != nil {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: err.Error(),
		}
	}

	var stateSnapshot []byte
	if execCtx.Runtime.SerializeOperationState != nil {
		serialized, serializeErr := execCtx.Runtime.SerializeOperationState()
		if serializeErr != nil {
			return GuardEvaluationResult{
				guardName:     guard.name,
				passed:        false,
				failureClass:  GuardFailureClassFramework,
				failureReason: fmt.Sprintf("serialize operation state: %v", serializeErr),
			}
		}
		stateSnapshot = serialized
		if execCtx.Runtime.OperationStateSnapshot != nil {
			*execCtx.Runtime.OperationStateSnapshot = serialized
		}
	} else if execCtx.Runtime.OperationStateSnapshot != nil {
		stateSnapshot = *execCtx.Runtime.OperationStateSnapshot
	}

	spec := IsolatedGuardSpec{
		OperationName:       execCtx.Runtime.OperationName,
		ScenarioName:        scenario.name,
		GuardName:           guard.name,
		ConfigurationPath:   execCtx.Runtime.ConfigurationPath,
		Seed:                seed.String(),
		FuzzingPattern:      config.FuzzingPattern,
		MaxIterations:       config.MaxIterations,
		MaxDurationNs:       config.MaxDuration.Nanoseconds(),
		UseDuration:         config.UseDuration,
		Identity:            execCtx.Identity,
		ZonePathParts:       execCtx.ZonePath.parts,
		StateSnapshotBase64: isolatedGuardStateSnapshotBase64(stateSnapshot),
	}

	specPath, specCleanup, err := isolatedGuardSpecWriteTempFile(&spec)
	if err != nil {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: err.Error(),
		}
	}
	defer specCleanup()

	start := time.Now()
	cmd := exec.Command(
		execCtx.Runtime.TransientBinaryPath,
		"-configuration-path", execCtx.Runtime.ConfigurationPath,
		"run-isolated-guard", specPath,
	)
	output, runErr := cmd.CombinedOutput()
	duration := time.Since(start)

	if runErr != nil {
		return isolatedGuardSubprocessFailureFromExit(guard.name, seed, duration, cmd, runErr, output)
	}

	response, parseErr := isolatedGuardSubprocessParseStdout(output)
	if parseErr != nil {
		return GuardEvaluationResult{
			guardName:     guard.name,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: parseErr.Error(),
			duration:      duration,
		}
	}

	result := guardEvaluationResultFromIsolatedResponse(response)
	result.guardName = guard.name
	result.duration = duration

	if execCtx.Runtime.OperationStateSnapshot != nil {
		*execCtx.Runtime.OperationStateSnapshot = isolatedGuardResponseStateSnapshot(response)
	}

	return result
}

func isolatedGuardSpecWriteTempFile(spec *IsolatedGuardSpec) (path string, cleanup func(), err error) {
	data, err := isolatedGuardSpecEncode(spec)
	if err != nil {
		return "", nil, err
	}
	file, err := os.CreateTemp("", "shield-isolated-guard-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("isolated guard spec temp file: %w", err)
	}
	if _, writeErr := file.Write(data); writeErr != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", nil, fmt.Errorf("isolated guard spec write: %w", writeErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(file.Name())
		return "", nil, fmt.Errorf("isolated guard spec close: %w", closeErr)
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func isolatedGuardSubprocessParseStdout(output []byte) (IsolatedGuardResponse, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line[0] != '{' {
			continue
		}
		return isolatedGuardResponseDecode([]byte(line))
	}
	return IsolatedGuardResponse{}, fmt.Errorf("isolated guard subprocess: no JSON response line in child output")
}

func isolatedGuardSubprocessFailureFromExit(
	guardName string,
	seed essence.UUID,
	duration time.Duration,
	cmd *exec.Cmd,
	runErr error,
	output []byte,
) GuardEvaluationResult {
	exitErr, ok := runErr.(*exec.ExitError)
	if !ok {
		return GuardEvaluationResult{
			guardName:     guardName,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: fmt.Sprintf("isolated guard subprocess: %v: %s", runErr, strings.TrimSpace(string(output))),
			duration:      duration,
		}
	}

	if exitErr.ExitCode() == 1 {
		response, parseErr := isolatedGuardSubprocessParseStdout(output)
		if parseErr == nil {
			result := guardEvaluationResultFromIsolatedResponse(response)
			result.guardName = guardName
			result.duration = duration
			return result
		}
	}

	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		signalName := ws.Signal().String()
		return GuardEvaluationResult{
			guardName:     guardName,
			passed:        false,
			failureClass:  GuardFailureClassCritical,
			failedSeed:    seed,
			failureReason: fmt.Sprintf("child process crashed: signal %s", signalName),
			duration:      duration,
		}
	}

	if exitErr.ExitCode() == 2 {
		return GuardEvaluationResult{
			guardName:     guardName,
			passed:        false,
			failureClass:  GuardFailureClassFramework,
			failureReason: fmt.Sprintf("isolated guard child framework error: %s", strings.TrimSpace(string(output))),
			duration:      duration,
		}
	}

	_ = cmd
	return GuardEvaluationResult{
		guardName:     guardName,
		passed:        false,
		failureClass:  GuardFailureClassCritical,
		failedSeed:    seed,
		failureReason: fmt.Sprintf("child process exited abnormally (code %d): %s", exitErr.ExitCode(), strings.TrimSpace(string(output))),
		duration:      duration,
	}
}

/*
IsolatedGuardSubprocessResolveBinaryPath returns the default shield-transient binary path under workspaceRoot.
*/
func IsolatedGuardSubprocessResolveBinaryPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".shield", "transient", "shield-transient")
}
