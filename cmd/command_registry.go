package main

import (
	"cmp"
	"fmt"
	"foundation/extensions"
	"shield/internal"
	"strings"
	"time"
)

type Command struct {
	Order int

	Names       []string
	Description string

	Runner func(renderer *internal.Renderer, b *strings.Builder) (shouldExit bool)
}

var CommandRegistry []Command
var CommandMap map[string]*Command = make(map[string]*Command)

func init() {
	CommandRegistry = []Command{
		{
			Names:       []string{"exit", "e", "quit", "q"},
			Description: "Quits the shell gracefully.",
			Runner: func(renderer *internal.Renderer, b *strings.Builder) bool {
				renderer.WriteColor(b, internal.ColorPass)
				b.WriteString("Shutting down SHIELD...\n")
				renderer.WriteColor(b, internal.ColorReset)
				time.Sleep(250 * time.Millisecond)
				return true // Signal the RunLoop to return
			},
			Order: 1,
		},
		{
			Order:       0,
			Names:       []string{"help", "h"},
			Description: "Shows the help information.",
			Runner: func(renderer *internal.Renderer, b *strings.Builder) bool {
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
