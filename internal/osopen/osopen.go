// Package osopen opens a URL or a local file with the user's default
// application (browser, HTML viewer…).
package osopen

import (
	"os/exec"
	"runtime"
)

// Open starts the default handler for target without waiting for it.
// Errors are returned but callers usually treat this as best effort.
func Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap the child
	return nil
}
