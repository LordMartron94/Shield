package main

import "fmt"

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
)

func shieldCliColor(text string, color string) string {
	return fmt.Sprintf("%s%s%s", color, text, ansiReset)
}

func shieldCliColorBold(text string, color string) string {
	return fmt.Sprintf("%s%s%s%s", color, ansiBold, text, ansiReset)
}
