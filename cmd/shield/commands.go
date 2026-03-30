package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"shield"
	"sort"
	"strconv"
	"strings"
)

func shieldCliRegisterCommands() {
	shieldCliRegisterRunCommands()
	shieldCliRegisterResultCommands()
	shieldCliRegisterUtilityCommands()
}

func shieldCliRegisterRunCommands() {
	shieldCliRegisterCommand([]string{"run"}, "Run Shield using optional target from TOML (usage: run [target] [flags])", func(args []string, state *shieldCliState) {
		target := ""
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			target = args[0]
			args = args[1:]
		}

		targetCfg, err := shieldCliRunTargetResolve(state.config, target)
		if err != nil {
			fmt.Println(shieldCliColorBold("run error: "+err.Error(), ansiRed))
			return
		}

		runCfg := targetCfg.Run

		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(os.Stdout)

		var unitBlacklistRaw string
		var atomBlacklistRaw string
		var modeRaw string
		var outDirRaw string
		var persistEnabled bool

		fs.StringVar(&unitBlacklistRaw, "unit", "", "comma-separated unit blacklist")
		fs.StringVar(&atomBlacklistRaw, "atom", "", "comma-separated atom blacklist")
		defaultOutDir := state.resultsDir
		if strings.TrimSpace(runCfg.OutDir) != "" {
			defaultOutDir = runCfg.OutDir
		}

		fs.StringVar(&modeRaw, "mode", runCfg.Mode, "persistence mode: json|txt|both")
		fs.StringVar(&outDirRaw, "out", defaultOutDir, "report output directory")
		fs.BoolVar(&persistEnabled, "persist", runCfg.Persist, "persist run report files")

		if err := fs.Parse(args); err != nil {
			return
		}

		switch targetCfg.EntrypointKind {
		case "", "harness":
			// default in-process harness execution
		case "go_test":
			if strings.TrimSpace(targetCfg.Entrypoint) == "" {
				fmt.Println(shieldCliColorBold("run error: go_test target requires 'entrypoint' in TOML", ansiRed))
				return
			}

			if target != "" {
				fmt.Println(shieldCliColor("target:", ansiCyan), target)
			}

			if err := shieldCliRunTargetGoTestExecute(targetCfg, state.configDir); err != nil {
				fmt.Println(shieldCliColorBold("run result: failed ("+err.Error()+")", ansiRed))
				return
			}

			fmt.Println(shieldCliColorBold("run result: success", ansiGreen))
			return
		default:
			fmt.Println(shieldCliColorBold("run error: unsupported entrypoint_kind '"+targetCfg.EntrypointKind+"'", ansiRed))
			return
		}

		cfg, err := shield.CLIHarnessConfigurationBuild()
		if err != nil {
			fmt.Println(shieldCliColorBold("harness error: "+err.Error(), ansiRed))
			return
		}

		unitBlacklist := shieldCliStringListMerge(runCfg.UnitBlacklist, shieldCliSplitCSV(unitBlacklistRaw))
		atomBlacklist := shieldCliStringListMerge(runCfg.AtomBlacklist, shieldCliSplitCSV(atomBlacklistRaw))

		engine := shield.EngineCreate(*cfg)
		runtimeCfg := shield.RuntimeConfigurationCreate(unitBlacklist, atomBlacklist)

		persist := shield.ReportPersistenceCreate(outDirRaw)
		shield.ReportPersistenceSetEnabled(persist, persistEnabled)
		shield.ReportPersistenceSetMode(persist, shieldCliPersistenceModeParse(modeRaw))

		report := shield.RunWithReportPersistence(engine, runtimeCfg, persist)
		state.lastResult = report.WrittenReportPath
		state.resultsDir = outDirRaw

		if target != "" {
			fmt.Println(shieldCliColor("target:", ansiCyan), target)
		}

		if report.WrittenReportPath != "" {
			fmt.Println(shieldCliColor("report:", ansiCyan), report.WrittenReportPath)
		}

		if report.Failed {
			fmt.Println(shieldCliColorBold("run result: failed", ansiRed))
			return
		}

		fmt.Println(shieldCliColorBold("run result: success", ansiGreen))
	})
}

func shieldCliRegisterResultCommands() {
	shieldCliRegisterCommand([]string{"list", "ls"}, "List persisted Shield run reports", func(_ []string, state *shieldCliState) {
		results, err := shieldCliResultFilesList(state.resultsDir)
		if err != nil {
			fmt.Println(shieldCliColorBold("list error: "+err.Error(), ansiRed))
			return
		}

		if len(results) == 0 {
			fmt.Println(shieldCliColor("(no results)", ansiGray))
			return
		}

		for index, result := range results {
			status := "unknown"
			if result.RunFailed != nil {
				if *result.RunFailed {
					status = "failed"
				} else {
					status = "success"
				}
			}

			fmt.Printf("%d) %s  status=%s  written=%s\n",
				index+1, result.Name, status, result.WrittenAt)
		}
	})

	shieldCliRegisterCommand([]string{"show"}, "Show report metadata (usage: show <latest|index|file>)", func(args []string, state *shieldCliState) {
		if len(args) != 1 {
			fmt.Println(shieldCliColor("usage: show <latest|index|file>", ansiYellow))
			return
		}

		selected, err := shieldCliResultFileSelect(state.resultsDir, args[0])
		if err != nil {
			fmt.Println(shieldCliColorBold("show error: "+err.Error(), ansiRed))
			return
		}

		meta, err := shieldCliResultFileReadMeta(selected.Path)
		if err != nil {
			fmt.Println(shieldCliColorBold("show error: "+err.Error(), ansiRed))
			return
		}

		fmt.Println("path:", selected.Path)
		fmt.Println("written_at:", meta.WrittenAt)
		fmt.Println("run_failed:", meta.RunFailed)
		fmt.Println("elapsed_ns:", meta.ElapsedNs)
		fmt.Println("schema_version:", meta.SchemaVersion)
	})

	shieldCliRegisterCommand([]string{"delete"}, "Delete one result (usage: delete <latest|index|file>)", func(args []string, state *shieldCliState) {
		if len(args) != 1 {
			fmt.Println(shieldCliColor("usage: delete <latest|index|file>", ansiYellow))
			return
		}

		selected, err := shieldCliResultFileSelect(state.resultsDir, args[0])
		if err != nil {
			fmt.Println(shieldCliColorBold("delete error: "+err.Error(), ansiRed))
			return
		}

		if !shieldCliDeleteConfirm(selected.Path) {
			fmt.Println(shieldCliColor("delete cancelled", ansiGray))
			return
		}

		if err := os.Remove(selected.Path); err != nil {
			fmt.Println(shieldCliColorBold("delete error: "+err.Error(), ansiRed))
			return
		}

		stem := strings.TrimSuffix(selected.Name, filepath.Ext(selected.Name))
		companion := filepath.Join(filepath.Dir(selected.Path), stem+".txt")
		if filepath.Ext(selected.Path) == ".txt" {
			companion = filepath.Join(filepath.Dir(selected.Path), stem+".json")
		}
		_ = os.Remove(companion)

		if state.lastResult == selected.Path {
			state.lastResult = ""
		}

		fmt.Println(shieldCliColorBold("deleted:", ansiGreen), selected.Path)
	})

	shieldCliRegisterCommand([]string{"clean"}, "Delete all Shield result artifacts from current results directory", func(_ []string, state *shieldCliState) {
		removed, err := shieldCliResultFilesDeleteAll(state.resultsDir)
		if err != nil {
			fmt.Println(shieldCliColorBold("clean error: "+err.Error(), ansiRed))
			return
		}

		fmt.Println(shieldCliColorBold("removed files:", ansiGreen), removed)
		state.lastResult = ""
	})

	shieldCliRegisterCommand([]string{"baseline"}, "Manage pinned baseline (show|set <latest|index|file>|clear)", func(args []string, state *shieldCliState) {
		if len(args) == 0 || args[0] == "show" {
			path, err := shieldCliBaselineRead(state.resultsDir)
			if err != nil {
				fmt.Println(shieldCliColorBold("baseline error: "+err.Error(), ansiRed))
				return
			}

			if path == "" {
				fmt.Println(shieldCliColor("(no baseline pinned)", ansiGray))
				return
			}

			fmt.Println(shieldCliColor("baseline:", ansiCyan), path)
			return
		}

		if args[0] == "clear" {
			if err := shieldCliBaselineClear(state.resultsDir); err != nil {
				fmt.Println(shieldCliColorBold("baseline error: "+err.Error(), ansiRed))
				return
			}
			fmt.Println(shieldCliColorBold("baseline cleared", ansiGreen))
			return
		}

		if args[0] == "set" {
			if len(args) != 2 {
				fmt.Println(shieldCliColor("usage: baseline set <latest|index|file>", ansiYellow))
				return
			}

			selected, err := shieldCliResultFileSelect(state.resultsDir, args[1])
			if err != nil {
				fmt.Println(shieldCliColorBold("baseline error: "+err.Error(), ansiRed))
				return
			}

			if err := shieldCliBaselineWrite(state.resultsDir, selected.Path); err != nil {
				fmt.Println(shieldCliColorBold("baseline error: "+err.Error(), ansiRed))
				return
			}

			fmt.Println(shieldCliColorBold("baseline set:", ansiGreen), selected.Path)
			return
		}

		fmt.Println(shieldCliColor("usage: baseline <show|set <latest|index|file>|clear>", ansiYellow))
	})
}

func shieldCliRunTargetGoTestExecute(targetCfg shieldCliResolvedRunTarget, workingDir string) error {
	goArgs := []string{"test", targetCfg.Entrypoint}

	if strings.TrimSpace(targetCfg.TestRunPattern) != "" {
		goArgs = append(goArgs, "-run", targetCfg.TestRunPattern)
	}

	goArgs = append(goArgs, targetCfg.EntrypointArgs...)

	cmd := exec.Command("go", goArgs...)
	cmd.Dir = workingDir
	env := os.Environ()

	defaultOutDir := shieldCliDefaultResultsDir
	if strings.TrimSpace(targetCfg.Run.OutDir) != "" {
		defaultOutDir = targetCfg.Run.OutDir
	}

	resolvedOutDir := defaultOutDir
	if !filepath.IsAbs(resolvedOutDir) {
		resolvedOutDir = filepath.Join(workingDir, resolvedOutDir)
	}

	resolvedLogDir := filepath.Join(workingDir, "logs")

	env = append(env, "SHIELD_RESULTS_DIR="+resolvedOutDir)
	env = append(env, "RULEFORGE_LOG_DIR="+resolvedLogDir)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func shieldCliRegisterUtilityCommands() {
	shieldCliRegisterCommand([]string{"help"}, "Show command help", func(_ []string, _ *shieldCliState) {
		entries := make([]commandHelpEntry, len(commandHelp))
		copy(entries, commandHelp)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].keys[0] < entries[j].keys[0]
		})

		fmt.Println(shieldCliColorBold("available commands:", ansiCyan))
		for _, entry := range entries {
			fmt.Printf("  %-16s - %s\n", shieldCliColor(shieldCliCommandKeysFormat(entry.keys), ansiGreen), entry.desc)
		}
	})

	shieldCliRegisterCommand([]string{"config"}, "Show loaded config and effective defaults", func(_ []string, state *shieldCliState) {
		fmt.Println("config path:", state.configPath)
		fmt.Println("results_dir:", state.resultsDir)
		fmt.Println("run.mode:", state.config.Run.Mode)
		fmt.Println("run.persist:", state.config.Run.Persist)
		fmt.Println("run.out_dir:", state.config.Run.OutDir)
		fmt.Println("run.unit_blacklist:", strings.Join(state.config.Run.UnitBlacklist, ","))
		fmt.Println("run.atom_blacklist:", strings.Join(state.config.Run.AtomBlacklist, ","))
		targets := shieldCliConfigTargetNames(state.config)
		if len(targets) == 0 {
			fmt.Println("targets:", "(none)")
		} else {
			fmt.Println("targets:", strings.Join(targets, ","))
		}
	})

	shieldCliRegisterCommand([]string{"quit", "exit"}, "Exit the shell", func(_ []string, _ *shieldCliState) {
		fmt.Println(shieldCliColor("Goodbye!", ansiMagenta))
		os.Exit(0)
	})
}

func shieldCliSplitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}

	return out
}

func shieldCliPersistenceModeParse(modeRaw string) shield.ReportPersistenceMode {
	switch strings.ToLower(strings.TrimSpace(modeRaw)) {
	case "json":
		return shield.ReportPersistenceModeJSON
	case "txt":
		return shield.ReportPersistenceModeTXT
	case "both":
		return shield.ReportPersistenceModeJSONAndTXT
	default:
		return shield.ReportPersistenceModeJSON
	}
}

func shieldCliIndexParse(selector string) (int, bool) {
	index, err := strconv.Atoi(selector)
	if err != nil || index < 1 {
		return 0, false
	}

	return index, true
}

func shieldCliCommandKeysFormat(keys []string) string {
	if len(keys) == 1 {
		return keys[0]
	}

	return "[" + strings.Join(keys, "|") + "]"
}
