package main

import (
	"bufio"
	"fmt"
	"os"
	"shield"
	"shield/internal"
	"strings"
)

type ShellContext struct {
	Renderer *internal.Renderer
	Builder  *strings.Builder
	Scanner  *bufio.Scanner
	Storage  *shield.SHIELD_Testing_Storage_Engine
	Config   *ShieldConfiguration
}

func RunLoop(cfg *ShieldConfiguration, storage *shield.SHIELD_Testing_Storage_Engine, renderer *internal.Renderer) {
	fmt.Println("Welcome to the SHIELD shell!")
	fmt.Println("Type 'help' to see available commands.")

	ctx := &ShellContext{
		Renderer: renderer,
		Builder:  &strings.Builder{},
		Scanner:  bufio.NewScanner(os.Stdin),
		Storage:  storage,
		Config:   cfg,
	}

	for {
		renderer.WriteColor(ctx.Builder, internal.ColorHighlight)
		ctx.Builder.WriteString(">>> ")
		renderer.WriteColor(ctx.Builder, internal.ColorReset)
		fmt.Print(ctx.Builder.String())
		ctx.Builder.Reset()

		if !ctx.Scanner.Scan() {
			break
		}

		input := strings.TrimSpace(ctx.Scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		requestedCommand := parts[0]
		args := parts[1:]

		if cmd, exist := CommandMap[requestedCommand]; !exist {
			renderer.WriteColor(ctx.Builder, internal.ColorFail)
			ctx.Builder.WriteString(fmt.Sprintf("Requested Command '%s' does not exist.\n", requestedCommand))
			renderer.WriteColor(ctx.Builder, internal.ColorReset)
			fmt.Print(ctx.Builder.String())
			ctx.Builder.Reset()
		} else {
			shouldExit := cmd.Runner(ctx, args)

			fmt.Print(ctx.Builder.String())
			ctx.Builder.Reset()

			if shouldExit {
				return
			}
		}
	}
}
