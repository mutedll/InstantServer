// privilege/escalate.go
package privilege

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type EscalationResult struct {
	AlreadyElevated bool   `json:"already_elevated"`
	ElevatedSpawned bool   `json:"elevated_spawned"`
	Method          string `json:"method"`
	Message         string `json:"message"`
}

func IsProcessElevated() bool {
	if runtime.GOOS == "windows" {
		f, err := os.Open(`\\.\PHYSICALDRIVE0`)
		if err == nil {
			_ = f.Close()
			return true
		}
		return false
	}
	return os.Geteuid() == 0
}

func RequestNativePrivilegeEscalation(helperCommand []string) EscalationResult {
	if IsProcessElevated() {
		return EscalationResult{
			AlreadyElevated: true,
			ElevatedSpawned: false,
			Method:          "native_token",
			Message:         "Process already holds root/Administrator privileges.",
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	args := helperCommand
	if len(args) == 0 {
		args = []string{exePath}
	}

	switch runtime.GOOS {
	case "linux":
		if pkexecPath, err := exec.LookPath("pkexec"); err == nil {
			cmd := exec.Command(pkexecPath, args...)
			cmd.Env = os.Environ()
			if startErr := cmd.Start(); startErr == nil {
				return EscalationResult{
					AlreadyElevated: false,
					ElevatedSpawned: true,
					Method:          "polkit_pkexec",
					Message:         "Triggered native Linux Polkit (pkexec) authentication dialog.",
				}
			}
		}
		return EscalationResult{
			AlreadyElevated: false,
			ElevatedSpawned: false,
			Method:          "polkit_pkexec",
			Message:         "Polkit authentication required to write raw USB sectors.",
		}

	case "darwin":
		escapedArgs := make([]string, 0, len(args))
		for _, a := range args {
			escapedArgs = append(escapedArgs, fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "'\\''")))
		}
		shellCmd := strings.Join(escapedArgs, " ")
		appleScript := fmt.Sprintf("do shell script %q with administrator privileges", shellCmd)
		cmd := exec.Command("osascript", "-e", appleScript)
		if startErr := cmd.Start(); startErr == nil {
			return EscalationResult{
				AlreadyElevated: false,
				ElevatedSpawned: true,
				Method:          "macos_authorization_services",
				Message:         "Triggered native macOS AuthorizationServices password prompt.",
			}
		}
		return EscalationResult{
			AlreadyElevated: false,
			ElevatedSpawned: false,
			Method:          "macos_authorization_services",
			Message:         "macOS Administrator authorization was cancelled or declined.",
		}

	case "windows":
		psCmd := fmt.Sprintf("Start-Process -FilePath '%s' -Verb RunAs", strings.ReplaceAll(exePath, "'", "''"))
		cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command", psCmd)
		if startErr := cmd.Start(); startErr == nil {
			return EscalationResult{
				AlreadyElevated: false,
				ElevatedSpawned: true,
				Method:          "windows_uac_manifest",
				Message:         "Triggered Windows UAC Administrator prompt.",
			}
		}
		return EscalationResult{
			AlreadyElevated: false,
			ElevatedSpawned: false,
			Method:          "windows_uac_manifest",
			Message:         "Windows UAC elevation required to lock and write raw USB sectors.",
		}
	}

	return EscalationResult{
		AlreadyElevated: false,
		ElevatedSpawned: false,
		Method:          "unknown",
		Message:         "Privilege elevation unsupported on this platform.",
	}
}