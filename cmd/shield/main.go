package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type commandFunc func(args []string, state *shieldCliState)

type command struct {
	desc string
	run  commandFunc
}

type commandHelpEntry struct {
	keys []string
	desc string
}

type shieldCliState struct {
	resultsDir string
	lastResult string
	configPath string
	configDir  string
	config     shieldCliConfig
}

var commands = map[string]command{}
var commandHelp = make([]commandHelpEntry, 0)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "shieldconfig.toml", "Path to the Shield CLI configuration TOML file")
	flag.Parse()

	config, err := shieldCliConfigLoad(configPath)
	if err != nil {
		fmt.Println(shieldCliColorBold("failed to load config: "+err.Error(), ansiRed))
		os.Exit(1)
	}

	configAbsPath, err := filepath.Abs(configPath)
	if err != nil {
		fmt.Println(shieldCliColorBold("failed to resolve config path: "+err.Error(), ansiRed))
		os.Exit(1)
	}

	configDir := filepath.Dir(configAbsPath)

	resultsDir, err := shieldCliResultDirResolveFromBase(configDir, config.ResultsDir)
	if err != nil {
		fmt.Println(shieldCliColorBold("failed to resolve results directory: "+err.Error(), ansiRed))
		os.Exit(1)
	}

	state := shieldCliState{
		resultsDir: resultsDir,
		lastResult: "",
		configPath: configPath,
		configDir:  configDir,
		config:     config,
	}

	shieldCliRunShell(&state)
}

func shieldCliRunShell(state *shieldCliState) {
	shieldCliRegisterCommands()

	reader := bufio.NewReader(os.Stdin)
	fmt.Println(shieldCliColorBold("Shield CLI", ansiCyan), "-", shieldCliColor("type 'help' for commands", ansiGray))

	for {
		fmt.Print(shieldCliColorBold("shield> ", ansiBlue))
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return
			}

			fmt.Println(shieldCliColorBold("read error: "+err.Error(), ansiRed))
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd, ok := commands[parts[0]]
		if !ok {
			fmt.Println(shieldCliColor("unknown command; type 'help'", ansiRed))
			continue
		}

		cmd.run(parts[1:], state)
	}
}

func shieldCliRegisterCommand(keys []string, desc string, run commandFunc) {
	if len(keys) == 0 {
		panic("shield cli command registration requires at least one key")
	}

	cmd := command{desc: desc, run: run}
	for _, key := range keys {
		commands[key] = cmd
	}

	commandHelp = append(commandHelp, commandHelpEntry{
		keys: keys,
		desc: desc,
	})
}
