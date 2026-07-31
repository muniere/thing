package daemon

import (
	"os/exec"
	"runtime"
)

// OpenBrowser best-effort launches the platform's URL opener. Failures are
// silent: the URL is printed for the user to click regardless.
func OpenBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, url)...).Start()
}
