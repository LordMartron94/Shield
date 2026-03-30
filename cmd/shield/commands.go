package main

import (
	"flag"
	"fmt"
	"os"
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
	commands["run"] = command{
		desc: "Run Shield using the registered harness (flags: --unit, --atom, --mode, --out, --persist)",
		run: func(args []string, state *shieldCliState) {
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			fs.SetOutput(os.Stdout)

			var unitBlacklistRaw string
			var atomBlacklistRaw string
			var modeRaw string
			var outDirRaw string
			var persistEnabled bool

			fs.StringVar(&unitBlacklistRaw, "unit", "", "comma-separated unit blacklist")
			fs.StringVar(&atomBlacklistRaw, "atom", "", "comma-separated atom blacklist")
			fs.StringVar(&modeRaw, "mode", state.config.Run.Mode, "persistence mode: json|txt|both")
			fs.StringVar(&outDirRaw, "out", state.resultsDir, "report output directory")
			fs.BoolVar(&persistEnabled, "persist", state.config.Run.Persist, "persist run report files")

			if err := fs.Parse(args); err != nil {
				return
			}

			cfg, err := shield.CLIHarnessConfigurationBuild()
			if err != nil {
				fmt.Println("harness error:", err.Error())
				return
			}

			unitBlacklist := shieldCliStringListMerge(state.config.Run.UnitBlacklist, shieldCliSplitCSV(unitBlacklistRaw))
			atomBlacklist := shieldCliStringListMerge(state.config.Run.AtomBlacklist, shieldCliSplitCSV(atomBlacklistRaw))

			engine := shield.EngineCreate(*cfg)
			runtimeCfg := shield.RuntimeConfigurationCreate(unitBlacklist, atomBlacklist)

			persist := shield.ReportPersistenceCreate(outDirRaw)
			shield.ReportPersistenceSetEnabled(persist, persistEnabled)
			shield.ReportPersistenceSetMode(persist, shieldCliPersistenceModeParse(modeRaw))

			report := shield.RunWithReportPersistence(engine, runtimeCfg, persist)
			state.lastResult = report.WrittenReportPath
			state.resultsDir = outDirRaw

			if report.WrittenReportPath != "" {
				fmt.Println("report:", report.WrittenReportPath)
			}

			if report.Failed {
				fmt.Println("run result: failed")
				return
			}

			fmt.Println("run result: success")
		},
	}
}

func shieldCliRegisterResultCommands() {
	commands["list"] = command{
		desc: "List persisted Shield run reports",
		run: func(_ []string, state *shieldCliState) {
			results, err := shieldCliResultFilesList(state.resultsDir)
			if err != nil {
				fmt.Println("list error:", err.Error())
				return
			}

			if len(results) == 0 {
				fmt.Println("(no results)")
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
		},
	}

	commands["show"] = command{
		desc: "Show report metadata (usage: show <latest|index|file>)",
		run: func(args []string, state *shieldCliState) {
			if len(args) != 1 {
				fmt.Println("usage: show <latest|index|file>")
				return
			}

			selected, err := shieldCliResultFileSelect(state.resultsDir, args[0])
			if err != nil {
				fmt.Println("show error:", err.Error())
				return
			}

			meta, err := shieldCliResultFileReadMeta(selected.Path)
			if err != nil {
				fmt.Println("show error:", err.Error())
				return
			}

			fmt.Println("path:", selected.Path)
			fmt.Println("written_at:", meta.WrittenAt)
			fmt.Println("run_failed:", meta.RunFailed)
			fmt.Println("elapsed_ns:", meta.ElapsedNs)
			fmt.Println("schema_version:", meta.SchemaVersion)
		},
	}

	commands["delete"] = command{
		desc: "Delete one result (usage: delete <latest|index|file>)",
		run: func(args []string, state *shieldCliState) {
			if len(args) != 1 {
				fmt.Println("usage: delete <latest|index|file>")
				return
			}

			selected, err := shieldCliResultFileSelect(state.resultsDir, args[0])
			if err != nil {
				fmt.Println("delete error:", err.Error())
				return
			}

			if !shieldCliDeleteConfirm(selected.Path) {
				fmt.Println("delete cancelled")
				return
			}

			if err := os.Remove(selected.Path); err != nil {
				fmt.Println("delete error:", err.Error())
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

			fmt.Println("deleted:", selected.Path)
		},
	}

	commands["clean"] = command{
		desc: "Delete all Shield result artifacts from current results directory",
		run: func(_ []string, state *shieldCliState) {
			removed, err := shieldCliResultFilesDeleteAll(state.resultsDir)
			if err != nil {
				fmt.Println("clean error:", err.Error())
				return
			}

			fmt.Println("removed files:", removed)
			state.lastResult = ""
		},
	}

	commands["baseline"] = command{
		desc: "Manage pinned baseline (show|set <latest|index|file>|clear)",
		run: func(args []string, state *shieldCliState) {
			if len(args) == 0 || args[0] == "show" {
				path, err := shieldCliBaselineRead(state.resultsDir)
				if err != nil {
					fmt.Println("baseline error:", err.Error())
					return
				}

				if path == "" {
					fmt.Println("(no baseline pinned)")
					return
				}

				fmt.Println("baseline:", path)
				return
			}

			if args[0] == "clear" {
				if err := shieldCliBaselineClear(state.resultsDir); err != nil {
					fmt.Println("baseline error:", err.Error())
					return
				}
				fmt.Println("baseline cleared")
				return
			}

			if args[0] == "set" {
				if len(args) != 2 {
					fmt.Println("usage: baseline set <latest|index|file>")
					return
				}

				selected, err := shieldCliResultFileSelect(state.resultsDir, args[1])
				if err != nil {
					fmt.Println("baseline error:", err.Error())
					return
				}

				if err := shieldCliBaselineWrite(state.resultsDir, selected.Path); err != nil {
					fmt.Println("baseline error:", err.Error())
					return
				}

				fmt.Println("baseline set:", selected.Path)
				return
			}

			fmt.Println("usage: baseline <show|set <latest|index|file>|clear>")
		},
	}
}

func shieldCliRegisterUtilityCommands() {
	commands["help"] = command{
		desc: "Show command help",
		run: func(_ []string, _ *shieldCliState) {
			keys := make([]string, 0, len(commands))
			for key := range commands {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			fmt.Println("available commands:")
			for _, key := range keys {
				fmt.Printf("  %-10s - %s\n", key, commands[key].desc)
			}
		},
	}

	commands["config"] = command{
		desc: "Show loaded config and effective defaults",
		run: func(_ []string, state *shieldCliState) {
			fmt.Println("config path:", state.configPath)
			fmt.Println("results_dir:", state.resultsDir)
			fmt.Println("run.mode:", state.config.Run.Mode)
			fmt.Println("run.persist:", state.config.Run.Persist)
			fmt.Println("run.unit_blacklist:", strings.Join(state.config.Run.UnitBlacklist, ","))
			fmt.Println("run.atom_blacklist:", strings.Join(state.config.Run.AtomBlacklist, ","))
		},
	}

	commands["quit"] = command{
		desc: "Exit the shell",
		run: func(_ []string, _ *shieldCliState) {
			os.Exit(0)
		},
	}

	commands["exit"] = commands["quit"]
	commands["ls"] = commands["list"]
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
