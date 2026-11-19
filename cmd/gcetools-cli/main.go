// Console-only build of the scan and diff commands. On Windows the GUI binary
// uses the GUI subsystem and cannot print to the console, so ship this one
// for scheduled tasks:  go build -o gcetools-cli.exe ./cmd/gcetools-cli
package main

import (
	"os"

	"gcetools/internal/cli"
)

func main() {
	if code, ok := cli.Dispatch(os.Args[1:]); ok {
		os.Exit(code)
	}
	os.Exit(cli.Run(os.Args[1:])) // bare flags: scan, as in earlier versions
}
