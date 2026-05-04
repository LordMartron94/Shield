package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	GitRoot  string
}

func RunLoop(
	cfg *ShieldConfiguration,
	storage *shield.SHIELD_Testing_Storage_Engine,
	renderer *internal.Renderer,
) {
	fmt.Println("Welcome to the SHIELD shell!")
	fmt.Println("Type 'help' to see available commands.")

	gitRoot, err := resolveGitRoot()
	if err != nil {
		panic(fmt.Errorf("fatal infrastructure error: shield must be run inside a git repository: %w", err))
	}

	ctx := &ShellContext{
		Renderer: renderer,
		Builder:  &strings.Builder{},
		Scanner:  bufio.NewScanner(os.Stdin),
		Storage:  storage,
		Config:   cfg,
		GitRoot:  gitRoot,
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

func resolveGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to resolve git root: %w", err)
	}
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(out.String()))), nil
}
