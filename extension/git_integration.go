package extension

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"shield"
)

var ErrDirtyWorktree = errors.New("git worktree is dirty")

/*
SystemIdentityFromGit constructs a system identity based on the git status of the provided targetPath.
targetPath should point to the specific repository or submodule under test.

If the worktree is dirty, it returns ErrDirtyWorktree, signaling that the run should be executed
for immediate feedback but MUST NOT be persisted to the regression database.
*/
func SystemIdentityFromGit(environment string, targetPath string) (shield.SHIELD_Testing_SystemIdentity, error) {
	dirty, err := resolveGitDirty(targetPath)
	if err != nil {
		return shield.SHIELD_Testing_SystemIdentity{}, err
	}

	if dirty {
		return shield.SHIELD_Testing_SystemIdentity{}, ErrDirtyWorktree
	}

	hash, err := resolveGitHash(targetPath)
	if err != nil {
		return shield.SHIELD_Testing_SystemIdentity{}, err
	}

	return shield.SHIELD_Testing_SystemIdentity{
		Version:     fmt.Sprintf("git-%s", hash),
		Environment: environment,
	}, nil
}

/*
SystemIdentityFromMarker retrieves the system identity by inspecting the physical location
of a provided marker function. The markerFunc MUST be a function defined within the target
submodule or package you wish to identify.
*/
func SystemIdentityFromMarker(environment string, markerFunc any) (shield.SHIELD_Testing_SystemIdentity, error) {
	val := reflect.ValueOf(markerFunc)
	if val.Kind() != reflect.Func {
		return shield.SHIELD_Testing_SystemIdentity{}, errors.New("shield extension: markerFunc must be a function")
	}

	pc := val.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return shield.SHIELD_Testing_SystemIdentity{}, errors.New("shield extension: could not determine function location")
	}

	file, _ := fn.FileLine(pc)
	targetDir := filepath.Dir(file)

	return SystemIdentityFromGit(environment, targetDir)
}

/*
SystemIdentityFromCaller retrieves the system identity by inspecting the physical location
of the file that called this function. This is the preferred method for test suites, as it
automatically binds the identity to the submodule containing the test file.
*/
func SystemIdentityFromCaller(environment string) (shield.SHIELD_Testing_SystemIdentity, error) {
	// Skip 1 frame to get the file of the caller, not this extension function
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return shield.SHIELD_Testing_SystemIdentity{}, errors.New("shield extension: could not determine caller location")
	}

	targetDir := filepath.Dir(file)

	return SystemIdentityFromGit(environment, targetDir)
}

// ----------------------------------------------------------------- PRIVATE HELPERS

func resolveGitHash(targetPath string) (string, error) {
	cmd := exec.Command("git", "-C", targetPath, "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to resolve git HEAD in %s: %w", targetPath, err)
	}

	return strings.TrimSpace(out.String()), nil
}

func resolveGitDirty(targetPath string) (bool, error) {
	cmd := exec.Command("git", "-C", targetPath, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("failed to check git status in %s: %w", targetPath, err)
	}

	return strings.TrimSpace(out.String()) != "", nil
}
