package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	_, transientBin, buildErr := buildTransientRunner(gitRoot, cfg.Discovery.Modules)
	if buildErr != nil {
		return fmt.Errorf("failed to build transient runner: %w", buildErr)
	}

	transientConfigPath, cfgErr := writeTransientConfigurationCopy(gitRoot, cfg)
	if cfgErr != nil {
		return fmt.Errorf("failed to prepare transient config: %w", cfgErr)
	}

	cmdArgs := []string{"-configuration-path", transientConfigPath}
	cmdArgs = append(cmdArgs, commandArgs...)
	runCmd := exec.Command(transientBin, cmdArgs...)
	runCmd.Dir = gitRoot
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("transient execution failed: %w", err)
	}

	return nil
}

func buildTransientRunner(gitRoot string, discoveryModules []string) (string, string, error) {
	transientDir := filepath.Join(gitRoot, ".shield", "transient")
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

	cmdSourceDir, sourceDirErr := resolveCmdSourceDirectory(gitRoot)
	if sourceDirErr != nil {
		return "", "", sourceDirErr
	}

	if copyErr := copyCmdDirectory(cmdSourceDir, transientCmdDir); copyErr != nil {
		return "", "", copyErr
	}

	moduleErr := ensureTransientGoModule(gitRoot, transientCmdDir)
	if moduleErr != nil {
		return "", "", moduleErr
	}

	generatedMainPath := filepath.Join(transientCmdDir, "main.go")
	generatedSource := generateTransientMainSource(discoveryModules)
	if writeErr := os.WriteFile(generatedMainPath, []byte(generatedSource), 0644); writeErr != nil {
		return "", "", fmt.Errorf("failed writing transient main.go: %w", writeErr)
	}
	if !strings.Contains(generatedSource, "runShieldEntrypoint") {
		return "", "", fmt.Errorf("generated transient main.go is missing runtime entrypoint call")
	}
	for _, mod := range discoveryModules {
		expectedImport := fmt.Sprintf("_ %q", mod)
		if !strings.Contains(generatedSource, expectedImport) {
			return "", "", fmt.Errorf("generated transient main.go is missing discovery import: %s", mod)
		}
	}

	transientBinPath := filepath.Join(transientDir, "shield-transient")
	buildCmd := exec.Command("go", "build", "-o", transientBinPath, transientCmdDir)
	buildCmd.Dir = gitRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		rawOutput := strings.TrimSpace(string(output))
		if strings.Contains(rawOutput, "outside modules listed in go.work") {
			return "", "", fmt.Errorf("%w: %s\nhint: register transient module once with `go work use ./.shield/transient/cmd`", err, rawOutput)
		}
		return "", "", fmt.Errorf("%w: %s", err, rawOutput)
	}

	return generatedMainPath, transientBinPath, nil
}

func writeTransientConfigurationCopy(gitRoot string, cfg *ShieldConfiguration) (string, error) {
	transientDir := filepath.Join(gitRoot, ".shield", "transient")
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

func copyFile(sourcePath string, targetPath string) error {
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, raw, 0644)
}

func copyCmdDirectory(sourceDir string, targetDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed reading source cmd directory %s: %w", sourceDir, err)
	}

	copiedGoSourceCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "main.go" {
			// Transient main.go is generated to inject discovery imports.
			continue
		}
		if name == "shield-transient" || name == "shield_config.transient.toml" {
			continue
		}

		srcPath := filepath.Join(sourceDir, name)
		dstPath := filepath.Join(targetDir, name)
		if copyErr := copyFile(srcPath, dstPath); copyErr != nil {
			return fmt.Errorf("failed copying %s into transient cmd: %w", name, copyErr)
		}
		if strings.HasSuffix(name, ".go") {
			copiedGoSourceCount += 1
		}
	}

	if copiedGoSourceCount == 0 {
		return fmt.Errorf("copied cmd directory %s has no non-test Go files", sourceDir)
	}

	return nil
}

func ensureTransientGoModule(gitRoot string, transientCmdDir string) error {
	moduleName := "shieldtransient"
	goModPath := filepath.Join(transientCmdDir, "go.mod")

	workspaceModules, err := readWorkspaceModulePaths(gitRoot)
	if err != nil {
		return err
	}
	requiredModules := []string{"shield"}
	for _, mod := range normalizeDiscoveryModules(workspaceModules) {
		if mod == moduleName {
			continue
		}
		requiredModules = append(requiredModules, mod)
	}
	sort.Strings(requiredModules)
	requiredModules = normalizeDiscoveryModules(requiredModules)

	modBuilder := &strings.Builder{}
	modBuilder.WriteString("module ")
	modBuilder.WriteString(moduleName)
	modBuilder.WriteString("\n\ngo 1.25\n\n")
	modBuilder.WriteString("require (\n")
	for _, mod := range requiredModules {
		modBuilder.WriteString(fmt.Sprintf("\t%s v0.0.0\n", mod))
	}
	modBuilder.WriteString(")\n")

	if writeErr := os.WriteFile(goModPath, []byte(modBuilder.String()), 0644); writeErr != nil {
		return fmt.Errorf("failed writing transient go.mod: %w", writeErr)
	}
	return nil
}

func readWorkspaceModulePaths(gitRoot string) ([]string, error) {
	goWorkPath := filepath.Join(gitRoot, "go.work")
	raw, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading go.work: %w", err)
	}

	lines := strings.Split(string(raw), "\n")
	inUseBlock := false
	modulePaths := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "use (") {
			inUseBlock = true
			continue
		}
		if inUseBlock {
			if trimmed == ")" {
				inUseBlock = false
				continue
			}
			modulePaths = append(modulePaths, strings.Trim(trimmed, "\""))
			continue
		}
		if strings.HasPrefix(trimmed, "use ") {
			modulePaths = append(modulePaths, strings.TrimSpace(strings.TrimPrefix(trimmed, "use ")))
		}
	}

	moduleNames := make([]string, 0)
	for _, relPath := range modulePaths {
		goModPath := filepath.Join(gitRoot, relPath, "go.mod")
		goModRaw, modErr := os.ReadFile(goModPath)
		if modErr != nil {
			continue
		}
		for _, modLine := range strings.Split(string(goModRaw), "\n") {
			modLine = strings.TrimSpace(modLine)
			if strings.HasPrefix(modLine, "module ") {
				moduleNames = append(moduleNames, strings.TrimSpace(strings.TrimPrefix(modLine, "module ")))
				break
			}
		}
	}

	return moduleNames, nil
}

func resolveCmdSourceDirectory(gitRoot string) (string, error) {
	monorepoStyle := filepath.Join(gitRoot, "tools", "shield", "cmd")
	if info, err := os.Stat(monorepoStyle); err == nil && info.IsDir() {
		return monorepoStyle, nil
	}

	moduleStyle := filepath.Join(gitRoot, "cmd")
	if info, err := os.Stat(moduleStyle); err == nil && info.IsDir() {
		return moduleStyle, nil
	}

	return "", fmt.Errorf("failed locating shield cmd source directory from git root %s", gitRoot)
}

func generateTransientMainSource(modules []string) string {
	builder := &strings.Builder{}
	builder.WriteString("package main\n\n")
	builder.WriteString("import (\n")
	builder.WriteString("\t\"flag\"\n")
	for _, mod := range modules {
		builder.WriteString(fmt.Sprintf("\t_ %q\n", mod))
	}
	builder.WriteString(")\n\n")
	builder.WriteString("func main() {\n")
	builder.WriteString("\tcfgPath := flag.String(\"configuration-path\", \"shield_config.toml\", \"the path to the cli configuration path\")\n")
	builder.WriteString("\tflag.Parse()\n")
	builder.WriteString("\tif err := runShieldEntrypoint(*cfgPath, flag.Args()); err != nil {\n")
	builder.WriteString("\t\tpanic(err)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n")
	return builder.String()
}
