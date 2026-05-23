package runner

import (
	"fmt"
	"os"
	"shield/internal"
)

func runIsolatedGuardCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: run-isolated-guard <spec.json>")
	}
	exitCode, responseLine, err := internal.RunIsolatedGuard(args[0])
	if err != nil {
		return err
	}
	if len(responseLine) > 0 {
		if _, writeErr := os.Stdout.Write(responseLine); writeErr != nil {
			return fmt.Errorf("isolated guard response write: %w", writeErr)
		}
		if _, writeErr := os.Stdout.Write([]byte("\n")); writeErr != nil {
			return fmt.Errorf("isolated guard response newline: %w", writeErr)
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
