package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func launchTransientShell(cfg *ShieldConfiguration, commandArgs []string) error {
	if len(cfg.Discovery.Modules) == 0 {
		return fmt.Errorf("no discovery modules configured. add [discovery].modules to shield_config.toml")
	}

	gitRoot, rootErr := resolveGitRoot()
	if rootErr != nil {
		return fmt.Errorf("failed resolving git root for transient build: %w", rootErr)
	}
	workspaceRoot := resolveWorkspaceRoot(gitRoot)

	_, transientBin, buildErr := buildTransientRunner(workspaceRoot, cfg.Discovery.Modules)
	if buildErr != nil {
		return fmt.Errorf("failed to build transient runner: %w", buildErr)
	}

	transientConfigPath, cfgErr := writeTransientConfigurationCopy(workspaceRoot, cfg)
	if cfgErr != nil {
		return fmt.Errorf("failed to prepare transient config: %w", cfgErr)
	}

	cmdArgs := []string{"-configuration-path", transientConfigPath}
	cmdArgs = append(cmdArgs, commandArgs...)
	runCmd := exec.Command(transientBin, cmdArgs...)
	runCmd.Dir = workspaceRoot
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("transient execution failed: %w", err)
	}
	return nil
}

func buildTransientRunner(workspaceRoot string, discoveryModules []string) (string, string, error) {
	transientDir := filepath.Join(workspaceRoot, ".shield", "transient")
	if err := os.MkdirAll(transientDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed creating transient directory: %w", err)
	}

	transientCmdDir := filepath.Join(transientDir, "cmd")
	if rmErr := os.RemoveAll(transientCmdDir); rmErr != nil {
		return "", "", fmt.Errorf("failed cleaning transient cmd directory: %w", rmErr)
	}
	if mkErr := os.MkdirAll(transientCmdDir, 0755); mkErr != nil {
		return "", "", fmt.Errorf("failed creating transient cmd directory: %w", mkErr)
	}
	if err := ensureTransientGoModule(transientCmdDir); err != nil {
		return "", "", err
	}

	generatedMainPath := filepath.Join(transientCmdDir, "main.go")
	generatedSource := generateTransientMainSource(discoveryModules)
	if writeErr := os.WriteFile(generatedMainPath, []byte(generatedSource), 0644); writeErr != nil {
		return "", "", fmt.Errorf("failed writing transient main.go: %w", writeErr)
	}

	transientBinPath := filepath.Join(transientDir, "shield-transient")
	buildCmd := exec.Command("go", "build", "-o", transientBinPath, transientCmdDir)
	buildCmd.Dir = workspaceRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		rawOutput := strings.TrimSpace(string(output))
		if strings.Contains(rawOutput, "outside modules listed in go.work") || strings.Contains(rawOutput, "not one of the workspace modules listed in go.work") {
			return "", "", fmt.Errorf("%w: %s\nhint: register transient module once with `go work use ./.shield/transient/cmd`", err, rawOutput)
		}
		return "", "", fmt.Errorf("%w: %s", err, rawOutput)
	}
	return generatedMainPath, transientBinPath, nil
}

func writeTransientConfigurationCopy(workspaceRoot string, cfg *ShieldConfiguration) (string, error) {
	transientDir := filepath.Join(workspaceRoot, ".shield", "transient")
	if err := os.MkdirAll(transientDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating transient directory: %w", err)
	}
	copyCfg := *cfg
	copyCfg.Runtime.Transient = true
	transientConfigPath := filepath.Join(transientDir, "shield_config.transient.toml")
	file, err := os.Create(transientConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed creating transient config: %w", err)
	}
	defer file.Close()
	if encodeErr := toml.NewEncoder(file).Encode(copyCfg); encodeErr != nil {
		return "", fmt.Errorf("failed encoding transient config: %w", encodeErr)
	}
	return transientConfigPath, nil
}

func ensureTransientGoModule(transientCmdDir string) error {
	goModPath := filepath.Join(transientCmdDir, "go.mod")
	content := "module shieldtransient\n\ngo 1.25\n"
	if writeErr := os.WriteFile(goModPath, []byte(content), 0644); writeErr != nil {
		return fmt.Errorf("failed writing transient go.mod: %w", writeErr)
	}
	return nil
}

func resolveWorkspaceRoot(gitRoot string) string {
	current := gitRoot
	for {
		if info, err := os.Stat(filepath.Join(current, "go.work")); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return gitRoot
		}
		current = parent
	}
}

func generateTransientMainSource(modules []string) string {
	builder := &strings.Builder{}
	builder.WriteString("package main\n\n")
	builder.WriteString("import (\n")
	builder.WriteString("\t\"flag\"\n")
	builder.WriteString("\t\"shield/runner\"\n")
	for _, mod := range modules {
		builder.WriteString(fmt.Sprintf("\t_ %q\n", mod))
	}
	builder.WriteString(")\n\n")
	builder.WriteString("func main() {\n")
	builder.WriteString("\tcfgPath := flag.String(\"configuration-path\", \"shield_config.toml\", \"the path to the cli configuration path\")\n")
	builder.WriteString("\tflag.Parse()\n")
	builder.WriteString("\tif err := runner.RunShieldEntrypoint(*cfgPath, flag.Args()); err != nil {\n")
	builder.WriteString("\t\tpanic(err)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n")
	return builder.String()
}
