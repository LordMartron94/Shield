package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type shieldCliConfigRun struct {
	UnitBlacklist []string `toml:"unit_blacklist"`
	AtomBlacklist []string `toml:"atom_blacklist"`
	Persist       bool     `toml:"persist"`
	Mode          string   `toml:"mode"`
	OutDir        string   `toml:"out_dir"`
}

type shieldCliConfigRunTarget struct {
	UnitBlacklist  []string `toml:"unit_blacklist"`
	AtomBlacklist  []string `toml:"atom_blacklist"`
	Persist        *bool    `toml:"persist"`
	Mode           *string  `toml:"mode"`
	OutDir         *string  `toml:"out_dir"`
	EntrypointKind *string  `toml:"entrypoint_kind"`
	Entrypoint     *string  `toml:"entrypoint"`
	EntrypointArgs []string `toml:"entrypoint_args"`
	TestRunPattern *string  `toml:"test_run_pattern"`
}

type shieldCliResolvedRunTarget struct {
	Name           string
	Run            shieldCliConfigRun
	EntrypointKind string
	Entrypoint     string
	EntrypointArgs []string
	TestRunPattern string
}

type shieldCliConfig struct {
	ResultsDir string                              `toml:"results_dir"`
	Run        shieldCliConfigRun                  `toml:"run"`
	Targets    map[string]shieldCliConfigRunTarget `toml:"targets"`
}

func shieldCliConfigCreateDefaults() shieldCliConfig {
	return shieldCliConfig{
		ResultsDir: shieldCliDefaultResultsDir,
		Run: shieldCliConfigRun{
			UnitBlacklist: []string{},
			AtomBlacklist: []string{},
			Persist:       true,
			Mode:          "json",
			OutDir:        "",
		},
		Targets: map[string]shieldCliConfigRunTarget{},
	}
}

func shieldCliConfigLoad(path string) (shieldCliConfig, error) {
	config := shieldCliConfigCreateDefaults()

	if strings.TrimSpace(path) == "" {
		return config, nil
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}

		return config, err
	}

	if _, err := toml.DecodeFile(path, &config); err != nil {
		return config, fmt.Errorf("invalid shield CLI config %s: %w", path, err)
	}

	if strings.TrimSpace(config.ResultsDir) == "" {
		config.ResultsDir = shieldCliDefaultResultsDir
	}

	return config, nil
}

func shieldCliRunTargetResolve(config shieldCliConfig, target string) (shieldCliResolvedRunTarget, error) {
	resolved := shieldCliResolvedRunTarget{
		Name:           target,
		Run:            config.Run,
		EntrypointKind: "harness",
		Entrypoint:     "",
		EntrypointArgs: nil,
		TestRunPattern: "",
	}

	if strings.TrimSpace(target) == "" {
		return resolved, nil
	}

	targetCfg, ok := config.Targets[target]
	if !ok {
		return resolved, fmt.Errorf("unknown run target '%s' (available: %s)", target, strings.Join(shieldCliConfigTargetNames(config), ", "))
	}

	if targetCfg.UnitBlacklist != nil {
		resolved.Run.UnitBlacklist = targetCfg.UnitBlacklist
	}

	if targetCfg.AtomBlacklist != nil {
		resolved.Run.AtomBlacklist = targetCfg.AtomBlacklist
	}

	if targetCfg.Persist != nil {
		resolved.Run.Persist = *targetCfg.Persist
	}

	if targetCfg.Mode != nil {
		resolved.Run.Mode = *targetCfg.Mode
	}

	if targetCfg.OutDir != nil {
		resolved.Run.OutDir = *targetCfg.OutDir
	}

	if targetCfg.EntrypointKind != nil {
		resolved.EntrypointKind = strings.TrimSpace(*targetCfg.EntrypointKind)
	}

	if targetCfg.Entrypoint != nil {
		resolved.Entrypoint = strings.TrimSpace(*targetCfg.Entrypoint)
	}

	if targetCfg.EntrypointArgs != nil {
		resolved.EntrypointArgs = targetCfg.EntrypointArgs
	}

	if targetCfg.TestRunPattern != nil {
		resolved.TestRunPattern = strings.TrimSpace(*targetCfg.TestRunPattern)
	}

	return resolved, nil
}

func shieldCliConfigTargetNames(config shieldCliConfig) []string {
	names := make([]string, 0, len(config.Targets))
	for name := range config.Targets {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

func shieldCliStringListMerge(configValues, cliValues []string) []string {
	if len(cliValues) > 0 {
		return cliValues
	}

	if len(configValues) == 0 {
		return nil
	}

	return configValues
}
