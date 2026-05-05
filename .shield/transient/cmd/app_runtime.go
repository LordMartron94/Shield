package main

import (
	"fmt"
	"foundation/system"
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
	Transient bool `toml:"transient"`
}

type ShieldConfiguration struct {
	Environment ShieldEnvironment `toml:"environment"`
	Discovery   ShieldDiscovery   `toml:"discovery"`
	Runtime     ShieldRuntime     `toml:"runtime"`
	ZoneMapping ShieldZoneMap     `toml:"zones"`
}

func (s *ShieldConfiguration) DebugDump() string {
	builder := &strings.Builder{}

	builder.WriteString("### USING SHIELD CONFIGURATION\n")
	builder.WriteString("--- ENVIRONMENT ---\n")
	builder.WriteString(fmt.Sprintf("  Name       : %s\n", s.Environment.Name))
	builder.WriteString(fmt.Sprintf("  DB Path    : %s\n", s.Environment.DBPath))
	builder.WriteString(fmt.Sprintf("  Color Mode : %s\n", s.Environment.ColorMode))
	builder.WriteString("\n")

	builder.WriteString("--- DISCOVERY MODULES ---\n")
	if len(s.Discovery.Modules) == 0 {
		builder.WriteString("  [No modules defined]\n")
	} else {
		for _, mod := range s.Discovery.Modules {
			builder.WriteString(fmt.Sprintf("  » %s\n", mod))
		}
	}
	builder.WriteString("\n")

	builder.WriteString("--- RUNTIME ---\n")
	builder.WriteString(fmt.Sprintf("  Transient  : %t\n", s.Runtime.Transient))
	builder.WriteString("\n")

	builder.WriteString("--- ZONE MAPPINGS ---\n")
	if len(s.ZoneMapping) == 0 {
		builder.WriteString("  [No zones defined]\n")
	} else {
		for path, dir := range s.ZoneMapping {
			builder.WriteString(fmt.Sprintf("  » %-25s -> %s\n", path, dir))
		}
	}
	builder.WriteString("---------------------------\n")

	return builder.String()
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

	cfg.Discovery.Modules = normalizeDiscoveryModules(cfg.Discovery.Modules)
	for _, mod := range cfg.Discovery.Modules {
		if strings.TrimSpace(mod) == "" {
			return nil, fmt.Errorf("discovery.modules cannot contain empty entries")
		}
	}

	if _, valid := colorModeMap[cfg.Environment.ColorMode]; !valid {
		return nil, fmt.Errorf("unknown color mode: '%s'", cfg.Environment.ColorMode)
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

func runShieldEntrypoint(cfgPath string, commandArgs []string) error {
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
