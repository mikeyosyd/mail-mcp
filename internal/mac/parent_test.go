package mac

import (
	"strings"
	"testing"
)

func TestFriendlyAppName(t *testing.T) {
	cases := map[string]string{
		"/Applications/Claude.app/Contents/MacOS/Claude":                      "Claude",
		"/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal": "Terminal",
		"/bin/zsh":     "zsh",
		"Claude.app":   "Claude",
		"":             "",
		"  /bin/bash ": "bash",
	}
	for in, want := range cases {
		if got := FriendlyAppName(in); got != want {
			t.Errorf("FriendlyAppName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccessibilityAdviceNamesTheParentInStdioMode(t *testing.T) {
	msg := AccessibilityAdvice("mail-mcp", "Claude")
	for _, want := range []string{"started by 'Claude'", "add 'Claude'", "Quit and reopen 'Claude'", "does not affect this process"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stdio advice missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "launchd restart") && !strings.Contains(msg, "does not affect") {
		t.Errorf("stdio advice must not tell the user to restart the launchd service")
	}
}

func TestAccessibilityAdviceKeepsServiceInstructionsOtherwise(t *testing.T) {
	msg := AccessibilityAdvice("mail-mcp", "")
	for _, want := range []string{"find 'mail-mcp'", "remove the stale entry", "mail-mcp launchd restart", "brew services restart mail-mcp"} {
		if !strings.Contains(msg, want) {
			t.Errorf("service advice missing %q:\n%s", want, msg)
		}
	}
}

func TestParentAppRoundTrip(t *testing.T) {
	SetParentApp("Claude")
	if ParentApp() != "Claude" {
		t.Fatal("SetParentApp/ParentApp round trip failed")
	}
	SetParentApp("")
}
