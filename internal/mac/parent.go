package mac

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// parentApp is the friendly name of the application that launched this server
// when it runs in stdio mode (for example "Claude" or "Terminal"). Empty when the
// server runs on its own, such as under launchd.
var (
	parentApp   string
	parentAppMu sync.RWMutex
)

// SetParentApp records the application that launched this process. Called from
// main when the stdio transport is selected.
func SetParentApp(name string) {
	parentAppMu.Lock()
	defer parentAppMu.Unlock()
	parentApp = name
}

// ParentApp returns the recorded parent application name, or "".
func ParentApp() string {
	parentAppMu.RLock()
	defer parentAppMu.RUnlock()
	return parentApp
}

// ParentAppName resolves the name of the process that launched this one, using
// ps (always present on macOS). Returns "" if it cannot be determined.
func ParentAppName() string {
	ppid := os.Getppid()
	if ppid <= 1 {
		return ""
	}
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(ppid)).Output()
	if err != nil {
		return ""
	}
	return FriendlyAppName(strings.TrimSpace(string(out)))
}

// FriendlyAppName turns a process path into the name a user sees in System
// Settings: "/Applications/Claude.app/Contents/MacOS/Claude" becomes "Claude",
// "/bin/zsh" becomes "zsh".
func FriendlyAppName(comm string) string {
	comm = strings.TrimSpace(comm)
	if comm == "" {
		return ""
	}
	if i := strings.Index(comm, ".app/"); i >= 0 {
		return strings.TrimSuffix(filepath.Base(comm[:i+4]), ".app")
	}
	if strings.HasSuffix(comm, ".app") {
		return strings.TrimSuffix(filepath.Base(comm), ".app")
	}
	return filepath.Base(comm)
}

// AccessibilityAdvice builds the remediation text for a missing Accessibility
// grant. When the server was launched by another application (stdio mode),
// macOS attributes the permission to that application, so the advice names it
// instead of the service.
func AccessibilityAdvice(executableName, parent string) string {
	if parent != "" {
		return "accessibility permission is required.\n" +
			"This server was started by '" + parent + "' (stdio transport), and macOS attributes Accessibility to the application that launched it, not to '" + executableName + "'.\n" +
			"1. In System Settings > Privacy & Security > Accessibility, add '" + parent + "' and make sure it is enabled.\n" +
			"2. Quit and reopen '" + parent + "' so the new grant is picked up.\n" +
			"Restarting the launchd service does not affect this process. Only the draft-creation tools need this permission; reading, searching, moving, deleting and saving attachments work without it."
	}
	return "accessibility permission is required. Please follow these steps:\n" +
		"1. In System Settings > Privacy & Security > Accessibility, find '" + executableName + "' and ensure it's enabled.\n" +
		"2. If it is already enabled but failing, the binary was likely updated. You must remove the stale entry.\n" +
		"3. Run the tool again to trigger a new macOS permission prompt.\n" +
		"4. IMPORTANT: After granting permission, you MUST restart the service.\n\n" +
		"If you are running the service via Homebrew, execute this command:\n" +
		"brew services restart " + executableName + "\n\n" +
		"Otherwise, execute this command:\n" +
		executableName + " launchd restart"
}
