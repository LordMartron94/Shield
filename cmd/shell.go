package main

import (
	"bufio"
	"fmt"
	"os"
	"shield/internal"
	"strings"
)

func RunLoop(renderer *internal.Renderer) {
	fmt.Println("Welcome to the SHIELD shell!")
	fmt.Println("Type 'help' to see available commands.")

	builder := &strings.Builder{}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		renderer.WriteColor(builder, internal.ColorHighlight)
		builder.WriteString(">>> ")
		renderer.WriteColor(builder, internal.ColorReset)
		fmt.Print(builder.String())
		builder.Reset()

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		requestedCommand := parts[0]

		if cmd, exist := CommandMap[requestedCommand]; !exist {
			renderer.WriteColor(builder, internal.ColorFail)
			builder.WriteString(fmt.Sprintf("Requested Command '%s' does not exist.\n", requestedCommand))
			renderer.WriteColor(builder, internal.ColorReset)
			fmt.Print(builder.String())
			builder.Reset()
		} else {
			shouldExit := cmd.Runner(renderer, builder)

			fmt.Print(builder.String())
			builder.Reset()

			if shouldExit {
				return
			}
		}
	}
}
