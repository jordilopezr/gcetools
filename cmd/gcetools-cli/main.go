// Console-only build of the scan command. On Windows the GUI binary uses the
// GUI subsystem and cannot print to the console, so ship this one for
// scheduled tasks:  go build -o gcetools-cli.exe ./cmd/gcetools-cli
package main

import (
	"os"

	"gcetools/internal/cli"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version") {
		os.Exit(cli.Version())
	}
	if len(args) > 0 && args[0] == "scan" {
		args = args[1:]
	}
	os.Exit(cli.Run(args))
}
