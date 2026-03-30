package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type shieldCliConfigRun struct {
	UnitBlacklist []string `toml:"unit_blacklist"`
	AtomBlacklist []string `toml:"atom_blacklist"`
	Persist       bool     `toml:"persist"`
	Mode          string   `toml:"mode"`
}

type shieldCliConfig struct {
	ResultsDir string             `toml:"results_dir"`
	Run        shieldCliConfigRun `toml:"run"`
}

func shieldCliConfigCreateDefaults() shieldCliConfig {
	return shieldCliConfig{
		ResultsDir: shieldCliDefaultResultsDir,
		Run: shieldCliConfigRun{
			UnitBlacklist: []string{},
			AtomBlacklist: []string{},
			Persist:       true,
			Mode:          "json",
		},
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

func shieldCliStringListMerge(configValues, cliValues []string) []string {
	if len(cliValues) > 0 {
		return cliValues
	}

	if len(configValues) == 0 {
		return nil
	}

	return configValues
}
