package main

import (
	"flag"
	"shield/runner"

	_ "splash/tests"
)

func main() {
	cfgPath := flag.String("configuration-path", "shield_config.toml", "the path to the cli configuration path")
	flag.Parse()
	if err := runner.RunShieldEntrypoint(*cfgPath, flag.Args()); err != nil {
		panic(err)
	}
}
