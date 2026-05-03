package extension

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
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
