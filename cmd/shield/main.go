package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type commandFunc func(args []string, state *shieldCliState)

type command struct {
	desc string
	run  commandFunc
}

type shieldCliState struct {
	resultsDir string
	lastResult string
}

var commands = map[string]command{}

func main() {
	state := shieldCliState{
		resultsDir: shieldCliResultsDirResolve(),
		lastResult: "",
	}

	shieldCliRunShell(&state)
}

func shieldCliRunShell(state *shieldCliState) {
	shieldCliRegisterCommands()

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Shield CLI - type 'help' for commands")

	for {
		fmt.Print("shield> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return
			}

			fmt.Println("read error:", err.Error())
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd, ok := commands[parts[0]]
		if !ok {
			fmt.Println("unknown command; type 'help'")
			continue
		}

		cmd.run(parts[1:], state)
	}
}
