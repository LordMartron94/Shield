package main

import (
	"cmp"
	"fmt"
	"foundation/extensions"
	"shield"
	"shield/extension"
	"shield/internal"
	"sort"
	"strconv"
	"strings"
)

type Command struct {
	Order int

	Names       []string
	Description string

	Runner func(ctx *ShellContext, args []string) (shouldExit bool)
}

var CommandRegistry []Command
var CommandMap map[string]*Command = make(map[string]*Command)

func init() {
	CommandRegistry = []Command{
		{
			Names:       []string{"exit", "e", "quit", "q"},
			Description: "Quits the shell gracefully.",
			Runner: func(ctx *ShellContext, _ []string) bool {
				ctx.Renderer.WriteColor(ctx.Builder, internal.ColorPass)
				ctx.Builder.WriteString("Shutting down SHIELD...\n")
				ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
				return true
			},
			Order: 1,
		},
		{
			Order:       0,
			Names:       []string{"help", "h"},
			Description: "Shows the help information.",
			Runner: func(ctx *ShellContext, _ []string) bool {
				ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHeader)
				ctx.Builder.WriteString("\nAVAILABLE COMMANDS:\n")
				ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)

				for _, cmd := range CommandRegistry {
					nameString := strings.Join(cmd.Names, ", ")

					ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHighlight)
					ctx.Builder.WriteString(fmt.Sprintf("  %-20s", nameString))
					ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)

					ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
					ctx.Builder.WriteString(fmt.Sprintf(" %s\n", cmd.Description))
					ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
				}
				ctx.Builder.WriteString("\n")
				return false
			},
		},
		{
			Order:       2,
			Names:       []string{"list", "ls"},
			Description: "Lists all registered operations. Usage: list [page]",
			Runner:      runListCommand,
		},
		{
			Order:       3,
			Names:       []string{"run", "r"},
			Description: "Executes operations. Usage: run [zone_prefix] (or run with no args for interactive)",
			Runner:      runExecuteCommand,
		},
	}

	extensions.SortedCopyShallow(CommandRegistry, func(a, b Command) int {
		return cmp.Compare(a.Order, b.Order)
	})

	for i := range CommandRegistry {
		cmd := &CommandRegistry[i]
		for _, name := range cmd.Names {
			if _, exist := CommandMap[name]; exist {
				panic(fmt.Errorf("engine error: command-name '%s' already exists", name))
			}
			CommandMap[name] = cmd
		}
	}
}

// ---------------------------------------------------------------- COMMAND RUNNERS

func runListCommand(ctx *ShellContext, args []string) bool {
	allOps := fetchAndSortOperations()

	if len(allOps) == 0 {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
		ctx.Builder.WriteString("No operations are currently registered.\n")
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return false
	}

	startIdx, endIdx, currentPage, totalPages := calculatePagination(len(allOps), args)
	pageOps := allOps[startIdx:endIdx]

	renderListHeader(ctx.Renderer, ctx.Builder, currentPage, totalPages, startIdx, endIdx, len(allOps))
	renderOperationsTree(ctx.Renderer, ctx.Builder, pageOps)
	renderListFooter(ctx.Renderer, ctx.Builder, currentPage, totalPages)

	return false
}

func runExecuteCommand(ctx *ShellContext, args []string) bool {
	targets := resolveRunTargets(ctx, args)
	if len(targets) == 0 {
		return false
	}

	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHeader)
	ctx.Builder.WriteString(fmt.Sprintf("\n=== EXECUTING %d OPERATIONS ===\n\n", len(targets)))
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)

	fmt.Print(ctx.Builder.String())
	ctx.Builder.Reset()

	var allResults []internal.ScenarioRunResult

	for _, op := range targets {
		physicalDir := resolvePhysicalDirectory(ctx, op.ZonePath())

		identity, isDirty := resolveOperationIdentity(ctx, physicalDir)

		runCfg := internal.ScenarioRunConfig{
			Identity: identity,
			// FuzzingPattern: internal.FuzzingPattern_Standard,
			// MaxIterations:  1000,
			// UseDuration:    false,
		}

		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
		ctx.Builder.WriteString(fmt.Sprintf("Running %s [Identity: %s | Path: %s]...\n", op.Name(), identity.Version, physicalDir))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		fmt.Print(ctx.Builder.String())
		ctx.Builder.Reset()

		// 4. Execute
		opResults := op.Run(runCfg)
		resultsSlice := opResults.ScenarioResults()
		allResults = append(allResults, resultsSlice...)

		// 5. Persist (Isolated Bouncer)
		persistOperationResults(ctx, op.Name(), resultsSlice, isDirty)
	}

	// 6. Render Global Output
	report := internal.RenderScenarios(ctx.Renderer, allResults)
	ctx.Builder.WriteString(report)

	return false
}

// ---------------------------------------------------------------- PRIVATE HELPERS

func resolvePhysicalDirectory(ctx *ShellContext, zonePath internal.ZonePath) string {
	rendered := zonePath.Render(".")
	bestMatchLength := -1
	bestPath := "."

	for mappedZone, physicalDir := range ctx.Config.ZoneMapping {
		mappedZoneStr := string(mappedZone)

		if strings.HasPrefix(rendered, mappedZoneStr) {
			if len(mappedZoneStr) > bestMatchLength {
				bestMatchLength = len(mappedZoneStr)
				bestPath = string(physicalDir)
			}
		}
	}
	return bestPath
}

func resolveOperationIdentity(ctx *ShellContext, physicalDir string) (internal.SystemIdentity, bool) {
	identity, err := extension.SystemIdentityFromGit(ctx.Config.Environment.Name, physicalDir)

	if err == extension.ErrDirtyWorktree {
		return internal.SystemIdentity{
			Version:     "<uncommitted-dirty>",
			Environment: ctx.Config.Environment.Name,
		}, true
	}

	if err != nil {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorFail)
		ctx.Builder.WriteString(fmt.Sprintf("Failed to resolve git identity for '%s': %v\n", physicalDir, err))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return internal.SystemIdentity{
			Version:     "<resolution-error>",
			Environment: ctx.Config.Environment.Name,
		}, true
	}

	return internal.SystemIdentity(identity), false
}

func persistOperationResults(ctx *ShellContext, opName string, results []internal.ScenarioRunResult, isDirty bool) {
	if len(results) == 0 {
		return
	}

	if isDirty {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHighlight)
		ctx.Builder.WriteString(fmt.Sprintf("  ⚠  [%s] Worktree is dirty. Executed ephemerally. Results NOT persisted.\n\n", opName))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)

		fmt.Print(ctx.Builder.String())
		ctx.Builder.Reset()
		return
	}

	for _, result := range results {
		if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(ctx.Storage, result); err != nil {
			ctx.Renderer.WriteColor(ctx.Builder, internal.ColorFail)
			ctx.Builder.WriteString(fmt.Sprintf("  Failed to persist scenario %s: %v\n", result.Name(), err))
			ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		}
	}
}

func resolveRunTargets(ctx *ShellContext, args []string) []internal.RegisteredOperation {
	allOps := fetchAndSortOperations()

	if len(allOps) == 0 {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
		ctx.Builder.WriteString("No operations are currently registered.\n")
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return nil
	}

	// Zone-Based Execution
	if len(args) > 0 {
		prefix := args[0]
		return internal.FilterRegistry(func(op internal.RegisteredOperation) bool {
			return strings.HasPrefix(op.ZonePath().Render("."), prefix)
		})
	}

	// Interactive Execution
	return promptInteractiveTargetSelection(ctx, allOps)
}

func promptInteractiveTargetSelection(ctx *ShellContext, allOps []internal.RegisteredOperation) []internal.RegisteredOperation {
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHeader)
	ctx.Builder.WriteString("\n=== SELECT OPERATIONS ===\n")
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)

	for i, op := range allOps {
		ctx.Builder.WriteString(fmt.Sprintf("  [%d] %s (Zone: %s)\n", i+1, op.Name(), op.ZonePath().Render(".")))
	}

	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHighlight)
	ctx.Builder.WriteString("\nEnter indices to run (comma-separated), 'all', or 'cancel': ")
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
	fmt.Print(ctx.Builder.String())
	ctx.Builder.Reset()

	if !ctx.Scanner.Scan() {
		return nil
	}

	input := strings.TrimSpace(strings.ToLower(ctx.Scanner.Text()))
	if input == "cancel" || input == "" {
		return nil
	}
	if input == "all" {
		return allOps
	}

	var selected []internal.RegisteredOperation
	parts := strings.Split(input, ",")
	for _, part := range parts {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && idx > 0 && idx <= len(allOps) {
			selected = append(selected, allOps[idx-1])
		}
	}

	return selected
}

func persistRunResults(ctx *ShellContext, results []internal.ScenarioRunResult, isDirty bool) {
	if isDirty {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorHighlight)
		ctx.Builder.WriteString("⚠️  Worktree is dirty. Executing ephemerally. Results WILL NOT be persisted to the regression database.\n\n")
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return
	}

	for _, result := range results {
		if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(ctx.Storage, result); err != nil {
			ctx.Renderer.WriteColor(ctx.Builder, internal.ColorFail)
			ctx.Builder.WriteString(fmt.Sprintf("Failed to persist scenario %s: %v\n", result.Name(), err))
			ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		}
	}
}

func fetchAndSortOperations() []internal.RegisteredOperation {
	ops := internal.FilterRegistry(func(op internal.RegisteredOperation) bool {
		return true
	})

	sort.SliceStable(ops, func(i, j int) bool {
		pathI := ops[i].ZonePath().Render(".")
		pathJ := ops[j].ZonePath().Render(".")
		if pathI == pathJ {
			return ops[i].Name() < ops[j].Name()
		}
		return pathI < pathJ
	})

	return ops
}

func calculatePagination(totalOps int, args []string) (startIdx, endIdx, currentPage, totalPages int) {
	const pageSize = 20
	totalPages = (totalOps + pageSize - 1) / pageSize
	currentPage = 1

	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			currentPage = parsed
		}
	}

	if currentPage > totalPages {
		currentPage = totalPages
	}

	startIdx = (currentPage - 1) * pageSize
	endIdx = startIdx + pageSize
	if endIdx > totalOps {
		endIdx = totalOps
	}

	return startIdx, endIdx, currentPage, totalPages
}

func renderListHeader(renderer *internal.Renderer, b *strings.Builder, current, total, start, end, totalOps int) {
	renderer.WriteColor(b, internal.ColorHeader)
	b.WriteString(fmt.Sprintf("\n=== REGISTERED OPERATIONS (Page %d of %d) ===\n", current, total))
	renderer.WriteColor(b, internal.ColorMuted)
	b.WriteString(fmt.Sprintf("Showing %d-%d of %d total operations\n\n", start+1, end, totalOps))
	renderer.WriteColor(b, internal.ColorReset)
}

func renderOperationsTree(renderer *internal.Renderer, b *strings.Builder, ops []internal.RegisteredOperation) {
	var prevPath []string

	for _, op := range ops {
		currPath := op.ZonePath().Parts()

		divergenceIndex := 0
		for divergenceIndex < len(prevPath) &&
			divergenceIndex < len(currPath) &&
			prevPath[divergenceIndex] == currPath[divergenceIndex] {
			divergenceIndex++
		}

		for i := divergenceIndex; i < len(currPath); i++ {
			indent := strings.Repeat("  ", i)
			b.WriteString(indent)
			renderer.WriteColor(b, internal.ColorHighlight)
			b.WriteString(currPath[i])
			renderer.WriteColor(b, internal.ColorReset)
			b.WriteString("\n")
		}

		opIndent := strings.Repeat("  ", len(currPath))
		b.WriteString(opIndent)
		renderer.WriteColor(b, internal.ColorPass)
		b.WriteString("» ")
		renderer.WriteColor(b, internal.ColorReset)
		b.WriteString(op.Name())
		b.WriteString("\n")

		prevPath = currPath
	}
}

func renderListFooter(renderer *internal.Renderer, b *strings.Builder, current, total int) {
	if current < total {
		renderer.WriteColor(b, internal.ColorMuted)
		b.WriteString(fmt.Sprintf("\nType 'list %d' for the next page.\n", current+1))
		renderer.WriteColor(b, internal.ColorReset)
	} else {
		b.WriteString("\n")
	}
}
