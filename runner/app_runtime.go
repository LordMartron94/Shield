package runner

import (
	"bufio"
	"fmt"
	"foundation/system"
	"os"
	"path/filepath"
	"shield"
	"shield/internal"
	"strings"

	"github.com/BurntSushi/toml"
)

type ShieldEnvironment struct {
	Name      string `toml:"name"`
	DBPath    string `toml:"db_path"`
	ColorMode string `toml:"color_mode"`
}

type ShieldDiscovery struct {
	Modules []string `toml:"modules"`
}

type ShieldZonePath string
type ShieldPhysicalDirectory string

type ShieldZoneMap map[ShieldZonePath]ShieldPhysicalDirectory

type ShieldRuntime struct {
	Transient                 bool     `toml:"transient"`
	MemoryDiagnostics         string   `toml:"memory_diagnostics"`
	MemoryTimeline            string   `toml:"memory_timeline"`
	MemoryTimelinePath        string   `toml:"memory_timeline_path"`
	MemoryTimelineViews       []string `toml:"memory_timeline_views"`
	MemoryStackIgnorePrefixes []string `toml:"memory_stack_ignore_prefixes"`
	MemoryStackIgnoreContains []string `toml:"memory_stack_ignore_contains"`
	MemoryStackMaxDepth       int      `toml:"memory_stack_max_depth"`
	MemoryStackBoundaryMode   bool     `toml:"memory_stack_boundary_mode"`
}

type ShieldConfiguration struct {
	Environment ShieldEnvironment `toml:"environment"`
	Discovery   ShieldDiscovery   `toml:"discovery"`
	Runtime     ShieldRuntime     `toml:"runtime"`
	ZoneMapping ShieldZoneMap     `toml:"zones"`
}

var colorModeMap = map[string]internal.RenderingColorMode{
	"none":       internal.Render_Color_None,
	"ansi16":     internal.Render_Color_ANSI16,
	"color-true": internal.Render_Color_True,
}

func loadShieldConfiguration(cfgPath string) (*ShieldConfiguration, error) {
	var cfg ShieldConfiguration
	tomlData, err := system.FileReadAllBytes(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("could not read configuration file at '%s': %w", cfgPath, err)
	}
	if decodeErr := toml.Unmarshal(tomlData, &cfg); decodeErr != nil {
		return nil, fmt.Errorf("there was an error parsing the configuration: %w", decodeErr)
	}

	resolvedModules, resolveErr := resolveDiscoveryModulesFromConfig(cfgPath, cfg.Discovery.Modules)
	if resolveErr != nil {
		return nil, resolveErr
	}
	cfg.Discovery.Modules = resolvedModules

	if _, valid := colorModeMap[cfg.Environment.ColorMode]; !valid {
		return nil, fmt.Errorf("unknown color mode: '%s'", cfg.Environment.ColorMode)
	}
	if _, err := parseMemoryDiagnosticsMode(cfg.Runtime.MemoryDiagnostics); err != nil {
		return nil, err
	}
	if _, err := parseMemoryTimelineMode(cfg.Runtime.MemoryTimeline); err != nil {
		return nil, err
	}
	if err := validateMemoryTimelineConfig(cfg.Runtime.MemoryTimeline, cfg.Runtime.MemoryTimelinePath); err != nil {
		return nil, err
	}
	if err := validateMemoryTimelineViews(cfg.Runtime.MemoryTimelineViews); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func normalizeDiscoveryModules(modules []string) []string {
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(modules))
	for _, mod := range modules {
		trimmed := strings.TrimSpace(mod)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func resolveDiscoveryModulesFromConfig(cfgPath string, discoveryModules []string) ([]string, error) {
	cfgAbsPath, absErr := filepath.Abs(cfgPath)
	if absErr != nil {
		return nil, fmt.Errorf("failed to resolve absolute config path: %w", absErr)
	}
	cfgDir := filepath.Dir(cfgAbsPath)

	resolved := make([]string, 0, len(discoveryModules))
	for _, moduleEntry := range normalizeDiscoveryModules(discoveryModules) {
		trimmed := strings.TrimSpace(moduleEntry)
		if trimmed == "" {
			return nil, fmt.Errorf("discovery.modules cannot contain empty entries")
		}

		targetDir := trimmed
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(cfgDir, targetDir)
		}
		targetDirAbs, targetAbsErr := filepath.Abs(targetDir)
		if targetAbsErr != nil {
			return nil, fmt.Errorf("failed to resolve discovery directory '%s': %w", trimmed, targetAbsErr)
		}

		info, statErr := os.Stat(targetDirAbs)
		if statErr != nil {
			// Transient mode persists already-resolved import paths into the copied config.
			// If the entry is not a reachable directory from cfgPath, accept it as an import path.
			if strings.Contains(trimmed, "/") || trimmed == "shield" {
				resolved = append(resolved, filepath.ToSlash(trimmed))
				continue
			}
			return nil, fmt.Errorf("discovery module directory '%s' not found from config '%s': %w", trimmed, cfgPath, statErr)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("discovery module '%s' must point to a directory", trimmed)
		}

		moduleRoot, modulePath, moduleErr := resolveGoModuleForDirectory(targetDirAbs)
		if moduleErr != nil {
			return nil, fmt.Errorf("failed resolving go module for discovery directory '%s': %w", trimmed, moduleErr)
		}

		relPath, relErr := filepath.Rel(moduleRoot, targetDirAbs)
		if relErr != nil {
			return nil, fmt.Errorf("failed computing discovery import path for '%s': %w", trimmed, relErr)
		}
		importPath := modulePath
		if relPath != "." {
			importPath = modulePath + "/" + filepath.ToSlash(relPath)
		}
		resolved = append(resolved, importPath)
	}

	return normalizeDiscoveryModules(resolved), nil
}

func resolveGoModuleForDirectory(startDir string) (string, string, error) {
	current := startDir
	for {
		goModPath := filepath.Join(current, "go.mod")
		info, err := os.Stat(goModPath)
		if err == nil && !info.IsDir() {
			modulePath, parseErr := parseModulePathFromGoMod(goModPath)
			if parseErr != nil {
				return "", "", parseErr
			}
			return current, modulePath, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", fmt.Errorf("no go.mod found for directory '%s'", startDir)
		}
		current = parent
	}
}

func parseModulePathFromGoMod(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed reading go.mod at '%s': %w", goModPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if modulePath == "" {
				return "", fmt.Errorf("go.mod at '%s' has empty module path", goModPath)
			}
			return modulePath, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed scanning go.mod at '%s': %w", goModPath, err)
	}
	return "", fmt.Errorf("go.mod at '%s' does not define a module path", goModPath)
}

func RunShieldEntrypoint(cfgPath string, commandArgs []string) error {
	cfg, err := loadShieldConfiguration(cfgPath)
	if err != nil {
		return err
	}

	if !cfg.Runtime.Transient {
		return launchTransientShell(cfg, commandArgs)
	}

	modeEnum := colorModeMap[cfg.Environment.ColorMode]
	storage := shield.SHIELD_Testing_Storage_EngineCreate(cfg.Environment.DBPath)
	if initializationErr := shield.SHIELD_Testing_Storage_EngineInitialize(storage); initializationErr != nil {
		return fmt.Errorf("could not initialize database: %w", initializationErr)
	}
	defer shield.SHIELD_Testing_Storage_EngineClose(storage)

	renderer := internal.RendererCreate(internal.RenderingConfigurationCreate(modeEnum))
	if len(commandArgs) > 0 {
		return RunSingleCommand(cfg, storage, renderer, strings.Join(commandArgs, " "), cfgPath)
	}

	RunLoop(cfg, storage, renderer, cfgPath)
	return nil
}
