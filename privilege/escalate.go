// privilege/escalate.go
package privilege

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"syscall"
)

type EscalationResult struct {
	AlreadyElevated bool   `json:"already_elevated"`
	ElevatedSpawned bool   `json:"elevated_spawned"`
	Method          string `json:"method"`
	Message         string `json:"message"`
}

// hideConsoleWindowOnWindows sets HideWindow=true and CreationFlags=CREATE_NO_WINDOW (0x08000000)
// on Windows exec.Cmd instances without requiring platform-specific build tags, preventing
// conhost.exe / cmd.exe / powershell.exe from flashing a black console window.
func hideConsoleWindowOnWindows(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" || cmd == nil {
		return
	}
	var attr syscall.SysProcAttr
	v := reflect.ValueOf(&attr).Elem()
	if f := v.FieldByName("HideWindow"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Bool {
		f.SetBool(true)
	}
	if f := v.FieldByName("CreationFlags"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Uint32 {
		f.SetUint(0x08000000) // CREATE_NO_WINDOW
	}
	cmd.SysProcAttr = &attr
}

func IsProcessElevated() bool {
	if runtime.GOOS == "windows" {
		for _, dev := range []string{`\\.\PHYSICALDRIVE0`, `\\.\PHYSICALDRIVE1`, `\\.\PHYSICALDRIVE2`} {
			if f, err := os.Open(dev); err == nil {
				_ = f.Close()
				return true
			}
		}
		cmd := exec.Command("fltmc.exe")
		hideConsoleWindowOnWindows(cmd)
		if err := cmd.Run(); err == nil {
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
			pkArgs := []string{"env"}
			for _, envKey := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
				if val := os.Getenv(envKey); val != "" {
					pkArgs = append(pkArgs, fmt.Sprintf("%s=%s", envKey, val))
				}
			}
			pkArgs = append(pkArgs, args...)
			cmd := exec.Command(pkexecPath, pkArgs...)
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
		psCmd := fmt.Sprintf("$ErrorActionPreference='Stop'; Start-Process -FilePath '%s' -Verb RunAs", strings.ReplaceAll(exePath, "'", "''"))
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psCmd)
		hideConsoleWindowOnWindows(cmd)
		// Run() waits only for the UAC consent prompt decision; once the elevated instance launches,
		// Start-Process returns 0 immediately so the caller can close the old unprivileged window cleanly.
		if runErr := cmd.Run(); runErr == nil {
			return EscalationResult{
				AlreadyElevated: false,
				ElevatedSpawned: true,
				Method:          "windows_uac_manifest",
				Message:         "Launched elevated Administrator instance and closing unprivileged window.",
			}
		}
		return EscalationResult{
			AlreadyElevated: false,
			ElevatedSpawned: false,
			Method:          "windows_uac_manifest",
			Message:         "Windows UAC elevation was declined or cancelled.",
		}
	}

	return EscalationResult{
		AlreadyElevated: false,
		ElevatedSpawned: false,
		Method:          "unknown",
		Message:         "Privilege elevation unsupported on this platform.",
	}
}
