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
			Runner: func(renderer *internal.Renderer, b *strings.Builder, args []string) bool {
				// 1. Fetch all operations (Filter that always returns true)
				allOps := internal.FilterRegistry(func(op internal.RegisteredOperation) bool {
					return true
				})

				if len(allOps) == 0 {
					renderer.WriteColor(b, internal.ColorMuted)
					b.WriteString("No operations are currently registered.\n")
					renderer.WriteColor(b, internal.ColorReset)
					return false
				}

				// 2. Deterministic Sorting by ZonePath
				sort.SliceStable(allOps, func(i, j int) bool {
					pathI := allOps[i].ZonePath().Render(".")
					pathJ := allOps[j].ZonePath().Render(".")
					if pathI == pathJ {
						return allOps[i].Name() < allOps[j].Name()
					}
					return pathI < pathJ
				})

				// 3. Pagination Math
				const pageSize = 20
				totalOps := len(allOps)
				totalPages := (totalOps + pageSize - 1) / pageSize
				currentPage := 1

				if len(args) > 0 {
					if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
						currentPage = parsed
					}
				}

				if currentPage > totalPages {
					currentPage = totalPages
				}

				startIdx := (currentPage - 1) * pageSize
				endIdx := startIdx + pageSize
				if endIdx > totalOps {
					endIdx = totalOps
				}

				pageOps := allOps[startIdx:endIdx]

				// 4. Render Header
				renderer.WriteColor(b, internal.ColorHeader)
				b.WriteString(fmt.Sprintf("\n=== REGISTERED OPERATIONS (Page %d of %d) ===\n", currentPage, totalPages))
				renderer.WriteColor(b, internal.ColorMuted)
				b.WriteString(fmt.Sprintf("Showing %d-%d of %d total operations\n\n", startIdx+1, endIdx, totalOps))
				renderer.WriteColor(b, internal.ColorReset)

				// 5. Render Tree Illusion for this Page
				var prevPath []string
				for _, op := range pageOps {
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

				// 6. Footer Hint
				if currentPage < totalPages {
					renderer.WriteColor(b, internal.ColorMuted)
					b.WriteString(fmt.Sprintf("\nType 'list %d' for the next page.\n", currentPage+1))
					renderer.WriteColor(b, internal.ColorReset)
				} else {
					b.WriteString("\n")
				}

				return false
			},
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
