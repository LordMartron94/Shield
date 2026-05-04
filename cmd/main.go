package main

import (
	"flag"
	"fmt"
	"foundation/system"
	"shield"
	"shield/internal"
	"strings"

	"github.com/BurntSushi/toml"
)

// TODO - create specific library for CLI

type ShieldEnvironment struct {
	Name      string `toml:"name"`
	DBPath    string `toml:"db_path"`
	ColorMode string `toml:"color_mode"`
}

type ShieldZonePath string
type ShieldPhysicalDirectory string

type ShieldZoneMap map[ShieldZonePath]ShieldPhysicalDirectory

type ShieldConfiguration struct {
	Environment ShieldEnvironment `toml:"environment"`
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

func main() {
	cfgPath := flag.String("configuration-path", "shield_config.toml", "the path to the cli configuration path")
	flag.Parse()

	var cfg ShieldConfiguration

	tomlData, err := system.FileReadAllBytes(*cfgPath)
	if err != nil {
		panic(fmt.Errorf("could not read configuration file at '%s': %w", *cfgPath, err))
	}

	if decodeErr := toml.Unmarshal(tomlData, &cfg); decodeErr != nil {
		panic(fmt.Errorf("there was an error parsing the configuration: %w", decodeErr))
	}

	modeEnum, valid := colorModeMap[cfg.Environment.ColorMode]
	if !valid {
		panic(fmt.Errorf("unknown color mode: '%s'", cfg.Environment.ColorMode))
	}

	storage := shield.SHIELD_Testing_Storage_EngineCreate(cfg.Environment.DBPath)

	if initializationErr := shield.SHIELD_Testing_Storage_EngineInitialize(storage); initializationErr != nil {
		panic(fmt.Errorf("could not initialize database: %w", initializationErr))
	}

	defer shield.SHIELD_Testing_Storage_EngineClose(storage)

	renderer := internal.RendererCreate(internal.RenderingConfigurationCreate(modeEnum))
	RunLoop(renderer)
}
