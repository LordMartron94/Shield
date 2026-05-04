package main

import (
	"cmp"
	"fmt"
	"foundation/extensions"
	"shield/internal"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	Order int

	Names       []string
	Description string

	Runner func(renderer *internal.Renderer, b *strings.Builder, args []string) (shouldExit bool)
}

var CommandRegistry []Command
var CommandMap map[string]*Command = make(map[string]*Command)

func init() {
	CommandRegistry = []Command{
		{
			Names:       []string{"exit", "e", "quit", "q"},
			Description: "Quits the shell gracefully.",
			Runner: func(renderer *internal.Renderer, b *strings.Builder, _ []string) bool {
				renderer.WriteColor(b, internal.ColorPass)
				b.WriteString("Shutting down SHIELD...\n")
				renderer.WriteColor(b, internal.ColorReset)
				time.Sleep(250 * time.Millisecond)
				return true
			},
			Order: 1,
		},
		{
			Order:       0,
			Names:       []string{"help", "h"},
			Description: "Shows the help information.",
			Runner: func(renderer *internal.Renderer, b *strings.Builder, _ []string) bool {
				renderer.WriteColor(b, internal.ColorHeader)
				b.WriteString("\nAVAILABLE COMMANDS:\n")
				renderer.WriteColor(b, internal.ColorReset)

				for _, cmd := range CommandRegistry {
					nameString := strings.Join(cmd.Names, ", ")

					renderer.WriteColor(b, internal.ColorHighlight)
					b.WriteString(fmt.Sprintf("  %-20s", nameString))
					renderer.WriteColor(b, internal.ColorReset)

					renderer.WriteColor(b, internal.ColorMuted)
					b.WriteString(fmt.Sprintf(" %s\n", cmd.Description))
					renderer.WriteColor(b, internal.ColorReset)
				}
				b.WriteString("\n")
				return false
			},
		},
		{
			Order:       2,
			Names:       []string{"list", "ls"},
			Description: "Lists all registered operations. Usage: list [page]",
			Runner:      runListCommand,
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

func runListCommand(renderer *internal.Renderer, b *strings.Builder, args []string) bool {
	allOps := fetchAndSortOperations()

	if len(allOps) == 0 {
		renderer.WriteColor(b, internal.ColorMuted)
		b.WriteString("No operations are currently registered.\n")
		renderer.WriteColor(b, internal.ColorReset)
		return false
	}

	startIdx, endIdx, currentPage, totalPages := calculatePagination(len(allOps), args)
	pageOps := allOps[startIdx:endIdx]

	renderListHeader(renderer, b, currentPage, totalPages, startIdx, endIdx, len(allOps))
	renderOperationsTree(renderer, b, pageOps)
	renderListFooter(renderer, b, currentPage, totalPages)

	return false
}

// ---------------------------------------------------------------- PRIVATE HELPERS

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
