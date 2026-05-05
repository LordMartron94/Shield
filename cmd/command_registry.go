package main

import (
	"cmp"
	"fmt"
	"foundation/extensions"
	"path/filepath"
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
			Description: "Executes operations. Usage: run [zone_prefix | 'impacted'] (or no args for interactive)",
			Runner:      runExecuteCommand,
		},
		{
			Order:       4,
			Names:       []string{"results", "res"},
			Description: "Lists persisted scenario results. Usage: results [page]",
			Runner:      runResultsCommand,
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

		var absPhysicalDir string
		if physicalDir == "<unmapped>" || physicalDir == "<always>" {
			absPhysicalDir = physicalDir
		} else {
			absPhysicalDir = filepath.ToSlash(filepath.Join(ctx.GitRoot, physicalDir))
		}

		identity, isDirty := resolveOperationIdentity(ctx, absPhysicalDir)
		execCtx := internal.ExecutionContext{
			Identity: identity,
		}

		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
		ctx.Builder.WriteString(fmt.Sprintf("Running %s [Identity: %s | Path: %s]...\n", op.Name(), identity.Version, physicalDir))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		fmt.Print(ctx.Builder.String())
		ctx.Builder.Reset()

		// 4. Execute
		opResults := op.Run(execCtx)
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

func runResultsCommand(ctx *ShellContext, args []string) bool {
	totalRows, countErr := shield.SHIELD_Testing_Storage_CountScenarioResults(ctx.Storage)
	if countErr != nil {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorFail)
		ctx.Builder.WriteString(fmt.Sprintf("Failed to count stored results: %v\n", countErr))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return false
	}

	if totalRows == 0 {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
		ctx.Builder.WriteString("No persisted scenario results found.\n")
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return false
	}

	startIdx, endIdx, currentPage, totalPages := calculatePagination(totalRows, args)
	pageSize := endIdx - startIdx

	rows, listErr := shield.SHIELD_Testing_Storage_ListScenarioResultsPage(ctx.Storage, pageSize, startIdx)
	if listErr != nil {
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorFail)
		ctx.Builder.WriteString(fmt.Sprintf("Failed to read stored results: %v\n", listErr))
		ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
		return false
	}

	renderResultsHeader(ctx.Renderer, ctx.Builder, currentPage, totalPages, startIdx, endIdx, totalRows)
	renderResultsTable(ctx.Renderer, ctx.Builder, rows)
	renderResultsFooter(ctx.Renderer, ctx.Builder, currentPage, totalPages)
	return false
}

// ---------------------------------------------------------------- PRIVATE HELPERS

func resolvePhysicalDirectory(ctx *ShellContext, zonePath internal.ZonePath) string {
	rendered := zonePath.Render(".")
	bestMatchLength := -1
	bestPath := "<unmapped>"

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
	if physicalDir == "<unmapped>" || physicalDir == "<always>" {
		return internal.SystemIdentity{
			Version:     physicalDir,
			Environment: ctx.Config.Environment.Name,
		}, true
	}

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
		if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(ctx.Storage, opName, result); err != nil {
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

	if len(args) > 0 && args[0] == "impacted" {
		return resolveImpactedTargets(ctx, allOps)
	}

	if len(args) > 0 {
		prefix := args[0]
		return internal.FilterRegistry(func(op internal.RegisteredOperation) bool {
			return strings.HasPrefix(op.ZonePath().Render("."), prefix)
		})
	}

	return promptInteractiveTargetSelection(ctx, allOps)
}

func resolveImpactedTargets(ctx *ShellContext, allOps []internal.RegisteredOperation) []internal.RegisteredOperation {
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
	ctx.Builder.WriteString("\n=== IMPACT DIAGNOSTICS ===\n")
	ctx.Builder.WriteString("Evaluating operation commit lineage from persisted scenario rows.\n")
	ctx.Builder.WriteString("==========================\n")
	fmt.Print(ctx.Builder.String())
	ctx.Builder.Reset()

	var targets []internal.RegisteredOperation

	for _, op := range allOps {
		physicalDir := resolvePhysicalDirectory(ctx, op.ZonePath())

		if physicalDir == "<always>" {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", "<always>", "always-on operation")
			continue
		}

		if physicalDir == "<unmapped>" {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", "<unmapped>", "unmapped zone, cannot infer commit from git location")
			continue
		}

		absPhysicalDir := filepath.ToSlash(filepath.Join(ctx.GitRoot, physicalDir))
		identity, isDirty := resolveOperationIdentity(ctx, absPhysicalDir)
		opScope := op.Name()

		if isDirty {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", identity.Version, "dirty worktree (ephemeral run)")
			continue
		}

		latestVersion, latestExists, latestErr := shield.SHIELD_Testing_Storage_GetLatestOperationVersion(
			ctx.Storage,
			opScope,
			ctx.Config.Environment.Name,
		)
		if latestErr != nil {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", identity.Version, fmt.Sprintf("storage lookup failed: %v", latestErr))
			continue
		}

		if !latestExists {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", identity.Version, "no persisted history for operation scope")
			continue
		}

		if latestVersion == identity.Version {
			writeImpactedDecision(ctx, op, "SKIP", identity.Version, "latest persisted commit matches current commit")
			continue
		}

		exists, existsErr := shield.SHIELD_Testing_Storage_OperationVersionExists(
			ctx.Storage,
			opScope,
			ctx.Config.Environment.Name,
			identity.Version,
		)
		if existsErr != nil {
			targets = append(targets, op)
			writeImpactedDecision(ctx, op, "RUN", identity.Version, fmt.Sprintf("version existence check failed: %v", existsErr))
			continue
		}

		if exists {
			writeImpactedDecision(ctx, op, "SKIP", identity.Version, "commit already persisted for operation scope")
			continue
		}

		targets = append(targets, op)
		writeImpactedDecision(ctx, op, "RUN", identity.Version, "new commit not persisted for operation scope")
	}

	ctx.Builder.WriteString("\n")
	return targets
}

func writeImpactedDecision(ctx *ShellContext, op internal.RegisteredOperation, action string, version string, reason string) {
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorMuted)
	ctx.Builder.WriteString(fmt.Sprintf("  [%s] %s | zone=%s | version=%s | %s\n", action, op.Name(), op.ZonePath().Render("."), version, reason))
	ctx.Renderer.WriteColor(ctx.Builder, internal.ColorReset)
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
		if err := shield.SHIELD_Testing_Storage_ScenarioResultAdd(ctx.Storage, "<unknown-operation>", result); err != nil {
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

func renderResultsHeader(renderer *internal.Renderer, b *strings.Builder, current, total, start, end, totalRows int) {
	renderer.WriteColor(b, internal.ColorHeader)
	b.WriteString(fmt.Sprintf("\n=== STORED RESULTS (Page %d of %d) ===\n", current, total))
	renderer.WriteColor(b, internal.ColorMuted)
	b.WriteString(fmt.Sprintf("Showing %d-%d of %d total rows\n\n", start+1, end, totalRows))
	renderer.WriteColor(b, internal.ColorReset)
}

func renderResultsTable(renderer *internal.Renderer, b *strings.Builder, rows []shield.SHIELD_Testing_Storage_StoredScenarioSummary) {
	renderer.WriteColor(b, internal.ColorHighlight)
	b.WriteString(fmt.Sprintf("%-6s %-28s %-28s %-8s %-12s %-19s\n", "State", "Scenario", "Zone", "Env", "Version", "Timestamp"))
	renderer.WriteColor(b, internal.ColorReset)

	for _, row := range rows {
		state := "PASS"
		stateColor := internal.ColorPass
		if !row.Passed {
			state = "FAIL"
			stateColor = internal.ColorFail
		}

		renderer.WriteColor(b, stateColor)
		b.WriteString(fmt.Sprintf("%-6s ", state))
		renderer.WriteColor(b, internal.ColorReset)

		b.WriteString(fmt.Sprintf(
			"%-28s %-28s %-8s %-12s %-19s\n",
			truncateColumn(row.Name, 28),
			truncateColumn(row.ZonePath, 28),
			truncateColumn(row.Environment, 8),
			truncateColumn(row.Version, 12),
			row.Timestamp.Local().Format("2006-01-02 15:04:05"),
		))
	}

	b.WriteString("\n")
}

func renderResultsFooter(renderer *internal.Renderer, b *strings.Builder, current, total int) {
	if current < total {
		renderer.WriteColor(b, internal.ColorMuted)
		b.WriteString(fmt.Sprintf("Type 'results %d' for the next page.\n", current+1))
		renderer.WriteColor(b, internal.ColorReset)
	}
}

func truncateColumn(value string, width int) string {
	if width <= 0 {
		return ""
	}

	if len(value) <= width {
		return value
	}

	if width <= 3 {
		return value[:width]
	}

	return value[:width-3] + "..."
}
