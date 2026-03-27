package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func pickProjectLocationNative() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("native project location picker is unsupported on %s", runtime.GOOS)
	}

	script := `try
set selectedFolder to POSIX path of (choose folder with prompt "Select project location")
return selectedFolder
on error errMsg number errNum
if errNum is -128 then
return "__CANCELLED__"
end if
error errMsg number errNum
end try`

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return "", fmt.Errorf("open folder picker: %s", stderr)
			}
		}
		return "", fmt.Errorf("open folder picker: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if path == "" || path == "__CANCELLED__" {
		return "", fmt.Errorf("project location selection cancelled")
	}

	return path, nil
}
